package handler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	orderpb "github.com/Fulim13/nearbuy/gen/golang/v1/order"
	"github.com/Fulim13/nearbuy/services/order-service/internal/repository"
	"github.com/Fulim13/nearbuy/services/order-service/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCHandler struct {
	orderpb.UnimplementedOrderServer
	service *service.OrderService
}

func NewGRPCHandler(s *grpc.Server, svc *service.OrderService) {
	orderpb.RegisterOrderServer(s, &GRPCHandler{service: svc})
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
