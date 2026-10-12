// Package branch holds order-service's DTM Saga branch endpoints.
//
// These are HTTP, not gRPC, and they are reached on a SECOND listener (:8000)
// alongside the gRPC one (:9000). DTM calls them by URL, so the barrier key
// arrives in the query string and a stuck transaction can be curl'd by hand.
package branch

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Fulim13/nearbuy/pkg/dtmx"
	"github.com/dtm-labs/client/dtmcli"
	"github.com/labstack/echo/v5"
)

// CreateOrderReq is the payload DTM stores at Submit() time and replays verbatim
// on every retry and on the compensation. A branch therefore needs no state of
// its own -- which is what lets DTM, rather than the caller, own the retrying.
type CreateOrderReq struct {
	OrderID   int64 `json:"order_id"`
	UserID    int64 `json:"user_id"`
	VoucherID int64 `json:"voucher_id"`
	PayValue  int64 `json:"pay_value"`
}

// Store is the subset of order-service's data layer the branches need, in
// database/sql terms because that is what the barrier speaks.
type Store interface {
	CreateOrder(tx *sql.Tx, r CreateOrderReq) error
	DeleteOrder(tx *sql.Tx, orderID int64) error
}

type Handler struct {
	db    *sql.DB
	store Store
}

func New(db *sql.DB, store Store) *Handler {
	return &Handler{db: db, store: store}
}

// Register mounts the branches. The paths are what Saga.Add() passes to DTM, so
// changing one here means changing it there.
func Register(e *echo.Echo, h *Handler) {
	g := e.Group("/branch")
	g.POST("/create_order", dtmx.WrapBranch(h.createOrder))
	g.POST("/create_order_comp", dtmx.WrapBranch(h.createOrderComp))
}

// run is the body every branch shares: build the barrier, then do the work
// INSIDE the transaction the barrier opens.
//
// CallWithDB inserts the barrier key and only calls busi if that insert affected
// a row. That single fact gives three properties at once:
//
//	retry of a completed branch  -> key exists, busi skipped       (idempotency)
//	compensate with no action    -> action key absent, busi skipped (empty compensation)
//	action arriving after its own compensation -> key left by the
//	                                compensation blocks it         (anti-hanging)
//
// None of which the business code has to know about.
func (h *Handler) run(c *echo.Context, busi func(tx *sql.Tx, r CreateOrderReq) error) any {
	bb, err := dtmx.BarrierFromEcho(c)
	if err != nil {
		// No DTM query parameters: someone called this endpoint by hand.
		return err
	}
	slog.Info("saga branch", "path", c.Request().URL.Path, "barrier", bb.String())

	return bb.CallWithDB(h.db, func(tx *sql.Tx) error {
		var r CreateOrderReq
		// DTM always posts the branch payload as a JSON body.
		if err := json.NewDecoder(c.Request().Body).Decode(&r); err != nil {
			// Unparseable payload will never parse. ErrFailure, not ErrOngoing.
			return dtmcli.ErrFailure
		}
		return busi(tx, r)
	})
}

func (h *Handler) createOrder(c *echo.Context) any {
	return h.run(c, func(tx *sql.Tx, r CreateOrderReq) error {
		if err := h.store.CreateOrder(tx, r); err != nil {
			// A database error here is transient as far as we can tell -- a lost
			// connection, a lock timeout. ErrOngoing so DTM retries rather than
			// unwinding a transaction that would succeed a second later.
			slog.Error("create order branch failed", "order_id", r.OrderID, "error", err)
			return dtmcli.ErrOngoing
		}
		return nil
	})
}

// createOrderComp removes the order again.
//
// A compensation must eventually succeed, so it NEVER returns ErrFailure --
// there is nothing to compensate a compensation with. ErrOngoing keeps DTM
// knocking.
func (h *Handler) createOrderComp(c *echo.Context) any {
	return h.run(c, func(tx *sql.Tx, r CreateOrderReq) error {
		if err := h.store.DeleteOrder(tx, r.OrderID); err != nil {
			slog.Error("create order compensation failed, will retry",
				"order_id", r.OrderID, "error", err)
			return dtmcli.ErrOngoing
		}
		return nil
	})
}

// Health is a plain liveness endpoint for the branch listener, so k8s can probe
// the HTTP port as well as the gRPC one.
func Health(c *echo.Context) error {
	return c.String(http.StatusOK, "ok")
}
