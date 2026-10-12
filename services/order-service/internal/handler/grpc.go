package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"time"

	orderpb "github.com/Fulim13/nearbuy/gen/golang/v1/order"
	"github.com/Fulim13/nearbuy/pkg/kafkax"
	"github.com/Fulim13/nearbuy/services/order-service/internal/repository"
	"github.com/Fulim13/nearbuy/services/order-service/internal/saga"
	"github.com/Fulim13/nearbuy/services/order-service/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCHandler struct {
	orderpb.UnimplementedOrderServer
	service  *service.OrderService
	producer *kafkax.Producer
}

func NewGRPCHandler(s *grpc.Server, svc *service.OrderService, producer *kafkax.Producer) {
	orderpb.RegisterOrderServer(s, &GRPCHandler{service: svc, producer: producer})
}

func (h *GRPCHandler) GetOrder(ctx context.Context, in *orderpb.GetOrderRequest) (*orderpb.GetOrderResponse, error) {
	if in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be positive")
	}
	o, err := h.service.GetOrder(ctx, in.GetId())
	if errors.Is(err, service.ErrOrderNotFound) {
		// NotFound is the CORRECT answer while a Saga is still running: the
		// gateway handed out this id in a 202 before the row existed. The
		// gateway turns it into a 404 and the client polls again -- which is
		// exactly what "pending" means here.
		return nil, status.Error(codes.NotFound, "order not found")
	}
	if err != nil {
		slog.Error("get order", "id", in.GetId(), "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &orderpb.GetOrderResponse{Order: orderToPB(o)}, nil
}

func (h *GRPCHandler) ListOrdersByUser(ctx context.Context, in *orderpb.ListOrdersByUserRequest) (*orderpb.ListOrdersByUserResponse, error) {
	if in.GetUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
	}
	limit := in.GetLimit()
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := h.service.ListOrdersByUser(ctx, in.GetUserId(), limit, in.GetOffset())
	if err != nil {
		slog.Error("list orders", "user_id", in.GetUserId(), "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	out := make([]*orderpb.OrderInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, orderToPB(r))
	}
	return &orderpb.ListOrdersByUserResponse{Orders: out}, nil
}

func orderToPB(o repository.VoucherOrder) *orderpb.OrderInfo {
	return &orderpb.OrderInfo{
		Id: o.ID, UserId: o.UserID, VoucherId: o.VoucherID,
		PayValue: o.PayValue, PayType: string(o.PayType), Status: string(o.Status),
		CreateTime: o.CreateTime.UTC().Format(time.RFC3339),
		PayTime:    rfc3339(o.PayTime),
		UseTime:    rfc3339(o.UseTime),
		RefundTime: rfc3339(o.RefundTime),
	}
}

func rfc3339(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

// PayOrder starts S2 and blocks on it, which is why its error mapping is longer
// than the read paths': every outcome the user could cause has to come back as a
// distinct status code.
func (h *GRPCHandler) PayOrder(ctx context.Context, in *orderpb.PayOrderRequest) (*orderpb.PayOrderResponse, error) {
	if in.GetOrderId() <= 0 || in.GetUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "order_id and user_id must be positive")
	}

	o, err := h.service.Pay(ctx, in.GetOrderId(), in.GetUserId())
	switch {
	case err == nil:
		h.publishPaid(ctx, o)
		return &orderpb.PayOrderResponse{Order: orderToPB(o)}, nil

	case errors.Is(err, service.ErrOrderNotFound), errors.Is(err, service.ErrNotOwner):
		// Same code for both. "Not yours" and "does not exist" must be
		// indistinguishable or the API confirms which ids are live.
		return nil, status.Error(codes.NotFound, "order not found")

	case errors.Is(err, service.ErrNotUnpaid):
		return nil, status.Error(codes.FailedPrecondition, "order is not awaiting payment")

	case errors.Is(err, saga.ErrRolledBack):
		// S2 ran and unwound. The overwhelmingly common cause is an
		// insufficient balance, and the money and the status are both back
		// where they started -- so this is a clean, retryable refusal rather
		// than a half-finished payment.
		//
		// It is NOT codes.Internal: nothing is broken.
		slog.Info("S2 compensated", "order_id", in.GetOrderId(), "reason", err)
		return nil, status.Error(codes.Aborted, "payment failed and was rolled back")

	default:
		slog.Error("pay order", "order_id", in.GetOrderId(), "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
}

// publishPaid tells shop-service an order was paid, so it can bump shop.sold.
//
// Best effort, AFTER S2 committed: a failed publish is logged, not returned --
// the money has moved and the user must not see an error for it. The cost is
// that a crash or Kafka outage in this window loses one increment; duplicates
// are harmless because shop-service dedups on order_id+status.
func (h *GRPCHandler) publishPaid(ctx context.Context, o repository.VoucherOrder) {
	payload, err := json.Marshal(map[string]any{
		"order_id":   o.ID,
		"user_id":    o.UserID,
		"voucher_id": o.VoucherID,
		"shop_id":    o.ShopID,
		"status":     string(o.Status),
	})
	if err != nil {
		slog.Error("marshal order.state_changed", "order_id", o.ID, "error", err)
		return
	}
	// WithoutCancel: the caller hanging up after the payment succeeded must not
	// cancel the publish too.
	pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := h.producer.Publish(pctx, kafkax.TopicOrderState,
		strconv.FormatInt(o.ID, 10), payload, kafkax.Header2TraceID(ctx)); err != nil {
		slog.Error("publish order.state_changed", "order_id", o.ID, "error", err)
	}
}
