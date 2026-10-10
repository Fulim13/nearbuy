package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Fulim13/nearbuy/pkg/kafkax"
	"github.com/Fulim13/nearbuy/services/shop-service/internal/service"
	"github.com/segmentio/kafka-go"
)

// OrderStateChanged is the event order-service publishes on every order
// transition. It arrives on topic order.state_changed keyed by order_id, so all
// transitions for one order land on one partition and cannot be reordered.
//
// Note shop_id: shop-service cannot look up which shop a voucher belongs to --
// voucher lives in voucher_db and this is shop_db. Database-per-service means an
// event has to CARRY what its consumers need; it cannot expect them to join.
type OrderStateChanged struct {
	OrderID   int64  `json:"order_id"`
	UserID    int64  `json:"user_id"`
	VoucherID int64  `json:"voucher_id"`
	ShopID    int64  `json:"shop_id"`
	Status    string `json:"status"`
}

type OrderStateHandler struct {
	shops *service.ShopService
}

func NewOrderStateHandler(s *service.ShopService) *OrderStateHandler {
	return &OrderStateHandler{shops: s}
}

func (h *OrderStateHandler) Handle(ctx context.Context, m kafka.Message) error {
	var ev OrderStateChanged
	if err := json.Unmarshal(m.Value, &ev); err != nil {
		// Malformed JSON will never parse, however many times we retry.
		return fmt.Errorf("%w: unmarshal: %v", kafkax.ErrPermanent, err)
	}
	if ev.OrderID == 0 || ev.ShopID == 0 {
		return fmt.Errorf("%w: order_id and shop_id are required", kafkax.ErrPermanent)
	}

	if ev.Status != "paid" {
		slog.Info("ignoring order state", "order_id", ev.OrderID, "status", ev.Status)
		return nil
	}

	key := fmt.Sprintf("order.state_changed:%d:%s", ev.OrderID, ev.Status)
	applied, err := h.shops.ApplySoldIncrement(ctx, key, m.Topic, ev.ShopID)
	if err != nil {
		return err // transient: let kafkax retry, then dead-letter
	}
	if !applied {
		slog.Info("duplicate event ignored", "event_key", key,
			"trace_id", kafkax.Header(m, kafkax.HeaderTraceID))
		return nil
	}
	slog.Info("shop.sold incremented", "shop_id", ev.ShopID, "order_id", ev.OrderID,
		"trace_id", kafkax.Header(m, kafkax.HeaderTraceID))
	return nil
}
