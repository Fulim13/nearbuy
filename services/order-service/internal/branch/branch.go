// Package branch holds order-service's DTM Saga branch endpoints.
//
// These are HTTP, not gRPC, and they are reached on a SECOND listener (:8000)
// alongside the gRPC one (:9000). DTM calls them by URL, so the barrier key
// arrives in the query string and a stuck transaction can be curl'd by hand.
//
// order-service appears in all three Sagas, which is why this file has six
// endpoints rather than two:
//
//	S1  create_order  / create_order_comp   insert the order   (lesson 14)
//	S2  pay_order     / pay_order_comp      unpaid -> paid
//	S3  cancel_order  / cancel_order_comp   unpaid -> cancelled
package branch

import (
	"database/sql"
	"encoding/json"
	"errors"
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

// PayOrderReq is S2's second branch. It carries user_id and amount it does not
// strictly need, because the COMPENSATION needs them and DTM replays one payload
// to both.
type PayOrderReq struct {
	OrderID int64 `json:"order_id"`
	UserID  int64 `json:"user_id"`
	Amount  int64 `json:"amount"`
}

// CancelOrderReq is S3's first branch.
type CancelOrderReq struct {
	OrderID   int64 `json:"order_id"`
	UserID    int64 `json:"user_id"`
	VoucherID int64 `json:"voucher_id"`
}

// ErrNotPayable and ErrNotCancellable are the two conditions that are NOT
// transient: the order has already left 'unpaid', so no retry will help.
var (
	ErrNotPayable     = errors.New("order is not payable")
	ErrNotCancellable = errors.New("order is not cancellable")
)

// Store is the subset of order-service's data layer the branches need, in
// database/sql terms because that is what the barrier speaks.
type Store interface {
	CreateOrder(tx *sql.Tx, r CreateOrderReq) error
	DeleteOrder(tx *sql.Tx, orderID int64) error

	PayOrder(tx *sql.Tx, orderID int64) error
	UnpayOrder(tx *sql.Tx, orderID int64) error

	CancelOrder(tx *sql.Tx, orderID int64) error
	UncancelOrder(tx *sql.Tx, orderID int64) error
}

type Handler struct {
	db    *sql.DB
	store Store
}

func New(db *sql.DB, store Store) *Handler { return &Handler{db: db, store: store} }

// Register mounts every branch. The paths are what Saga.Add() passes to DTM, so
// changing one here means changing it there -- and DTM stores the URL per
// branch at Submit() time, so a rename breaks transactions already in flight.
func Register(e *echo.Echo, h *Handler) {
	g := e.Group("/branch")

	g.POST("/create_order", dtmx.WrapBranch(h.createOrder))
	g.POST("/create_order_comp", dtmx.WrapBranch(h.createOrderComp))

	g.POST("/pay_order", dtmx.WrapBranch(h.payOrder))
	g.POST("/pay_order_comp", dtmx.WrapBranch(h.payOrderComp))

	g.POST("/cancel_order", dtmx.WrapBranch(h.cancelOrder))
	g.POST("/cancel_order_comp", dtmx.WrapBranch(h.cancelOrderComp))
}

// run is the body every branch shares: build the barrier, then do the work
// INSIDE the transaction the barrier opens.
//
// It is a generic FREE FUNCTION rather than a method because Go does not allow
// type parameters on methods, and three Sagas means three payload types. The
// alternative -- passing raw []byte and unmarshalling in each branch -- works
// and loses the compile-time check that the branch and its compensation agree
// on a payload shape.
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
func run[T any](h *Handler, c *echo.Context, busi func(tx *sql.Tx, r T) error) any {
	bb, err := dtmx.BarrierFromEcho(c)
	if err != nil {
		// No DTM query parameters: someone called this endpoint by hand.
		return err
	}
	slog.Info("saga branch", "path", c.Request().URL.Path, "barrier", bb.String())

	return bb.CallWithDB(h.db, func(tx *sql.Tx) error {
		var r T
		// DTM always posts the branch payload as a JSON body.
		if err := json.NewDecoder(c.Request().Body).Decode(&r); err != nil {
			// Unparseable payload will never parse. ErrFailure, not ErrOngoing.
			return dtmcli.ErrFailure
		}
		return busi(tx, r)
	})
}

// ---------------------------------------------------------------- S1

func (h *Handler) createOrder(c *echo.Context) any {
	return run(h, c, func(tx *sql.Tx, r CreateOrderReq) error {
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
	return run(h, c, func(tx *sql.Tx, r CreateOrderReq) error {
		if err := h.store.DeleteOrder(tx, r.OrderID); err != nil {
			slog.Error("create order compensation failed, will retry",
				"order_id", r.OrderID, "error", err)
			return dtmcli.ErrOngoing
		}
		return nil
	})
}

// ---------------------------------------------------------------- S2

// payOrder is S2's SECOND branch: the balance has already left the user's
// account by the time this runs.
//
// The UPDATE carries its own guard -- WHERE status = 'unpaid' -- and zero rows
// affected is ErrFailure, not ErrOngoing. The order has left 'unpaid' since the
// Saga was submitted (paid by a second tab, or cancelled by the expiry sweep),
// and no amount of retrying puts it back. ErrFailure unwinds, which refunds.
func (h *Handler) payOrder(c *echo.Context) any {
	return run(h, c, func(tx *sql.Tx, r PayOrderReq) error {
		err := h.store.PayOrder(tx, r.OrderID)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, ErrNotPayable):
			slog.Warn("pay order refused, saga will compensate", "order_id", r.OrderID)
			return dtmcli.ErrFailure
		default:
			slog.Error("pay order branch failed", "order_id", r.OrderID, "error", err)
			return dtmcli.ErrOngoing
		}
	})
}

// payOrderComp puts the order back to 'unpaid' and clears pay_time.
//
// Reached when a LATER branch fails -- except there is no later branch in S2, so
// in practice it is reached only when DTM could not confirm this branch and
// unwound. Which is exactly the empty-compensation case the barrier handles.
func (h *Handler) payOrderComp(c *echo.Context) any {
	return run(h, c, func(tx *sql.Tx, r PayOrderReq) error {
		if err := h.store.UnpayOrder(tx, r.OrderID); err != nil {
			slog.Error("pay order compensation failed, will retry",
				"order_id", r.OrderID, "error", err)
			return dtmcli.ErrOngoing
		}
		return nil
	})
}

// ---------------------------------------------------------------- S3

// cancelOrder is S3's FIRST branch, and the only place S3 is allowed to fail.
//
// WHERE status = 'unpaid' again. Zero rows means the sweep selected this order
// and the user paid it in the seconds before the branch ran -- a race the sweep
// cannot avoid and must not lose. ErrFailure stops S3 before branch 2 gives the
// stock back on an order that is now paid.
//
// Because this is the first branch, unwinding it is an EMPTY compensation: the
// barrier sees no action key, skips the body, and nothing is touched.
func (h *Handler) cancelOrder(c *echo.Context) any {
	return run(h, c, func(tx *sql.Tx, r CancelOrderReq) error {
		err := h.store.CancelOrder(tx, r.OrderID)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, ErrNotCancellable):
			slog.Info("expiry skipped, order is no longer unpaid", "order_id", r.OrderID)
			return dtmcli.ErrFailure
		default:
			slog.Error("cancel order branch failed", "order_id", r.OrderID, "error", err)
			return dtmcli.ErrOngoing
		}
	})
}

func (h *Handler) cancelOrderComp(c *echo.Context) any {
	return run(h, c, func(tx *sql.Tx, r CancelOrderReq) error {
		if err := h.store.UncancelOrder(tx, r.OrderID); err != nil {
			slog.Error("cancel order compensation failed, will retry",
				"order_id", r.OrderID, "error", err)
			return dtmcli.ErrOngoing
		}
		return nil
	})
}

// Health is a plain liveness endpoint for the branch listener, so k8s can probe
// the HTTP port as well as the gRPC one.
func Health(c *echo.Context) error { return c.String(http.StatusOK, "ok") }
