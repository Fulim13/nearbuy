// Package branch holds voucher-service's DTM Saga branch endpoints.
//
//	S1  deduct_stock  / deduct_stock_comp  stock - 1
//
// Expired orders give their stock back through the RestoreStock gRPC call, not
// a Saga: see internal/service.VoucherService.RestoreStock.
package branch

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Fulim13/nearbuy/pkg/dtmx"
	"github.com/Fulim13/nearbuy/services/voucher-service/internal/service"
	"github.com/dtm-labs/client/dtmcli"
	"github.com/labstack/echo/v5"
)

// DeductStockReq is the payload Saga S1's second branch carries. qty is in the
// payload rather than hard-coded to 1 so the compensation and the forward action
// are symmetric by construction -- they read the same number from the same JSON.
type DeductStockReq struct {
	VoucherID int64 `json:"voucher_id"`
	Qty       int32 `json:"qty"`
}

type Store interface {
	DeductStock(tx *sql.Tx, voucherID int64, qty int32) error
	RestoreStock(tx *sql.Tx, voucherID int64, qty int32) error
}

type Handler struct {
	db    *sql.DB
	store Store
}

func New(db *sql.DB, store Store) *Handler {
	return &Handler{db: db, store: store}
}

func Register(e *echo.Echo, h *Handler) {
	g := e.Group("/branch")

	g.POST("/deduct_stock", dtmx.WrapBranch(h.deductStock))
	g.POST("/deduct_stock_comp", dtmx.WrapBranch(h.deductStockComp))
}

// run is generic so another Saga's payload type can reuse it; Go does not allow
// type parameters on methods.
func run[T any](h *Handler, c *echo.Context, busi func(tx *sql.Tx, r T) error) any {
	bb, err := dtmx.BarrierFromEcho(c)
	if err != nil {
		return err
	}
	slog.Info("saga branch", "path", c.Request().URL.Path, "barrier", bb.String())

	return bb.CallWithDB(h.db, func(tx *sql.Tx) error {
		var r T
		if err := json.NewDecoder(c.Request().Body).Decode(&r); err != nil {
			return dtmcli.ErrFailure
		}
		return busi(tx, r)
	})
}

// ---------------------------------------------------------------- S1

// deductStock returns ErrFailure on a business condition -- the only S1 branch
// that does, and the entire point of S1.
//
// Out of stock is not transient: no amount of retrying creates stock. Returning
// ErrOngoing here would retry until a human intervened while the order sat
// 'unpaid' forever. ErrFailure tells DTM to compensate, which deletes the order
// the first branch inserted.
func (h *Handler) deductStock(c *echo.Context) any {
	return run(h, c, func(tx *sql.Tx, r DeductStockReq) error {
		if r.Qty <= 0 {
			r.Qty = 1
		}
		err := h.store.DeductStock(tx, r.VoucherID, r.Qty)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, service.ErrOutOfStock), errors.Is(err, service.ErrNotSeckill):
			slog.Warn("deduct stock refused, saga will compensate",
				"voucher_id", r.VoucherID, "reason", err)
			return dtmcli.ErrFailure
		default:
			slog.Error("deduct stock branch failed", "voucher_id", r.VoucherID, "error", err)
			return dtmcli.ErrOngoing
		}
	})
}

// deductStockComp returns the stock.
//
// It never returns ErrFailure. It is also the branch that most needs the
// barrier's EMPTY COMPENSATION detection: DTM compensates branches in reverse
// order and will call this one even when the forward deduct never ran -- because
// the forward call timed out, say, and DTM cannot know whether it landed. The
// barrier sees no action key for this branch, skips the body, and the stock is
// not incremented for a deduction that never happened.
//
// Without that, every timed-out branch would hand out free stock.
func (h *Handler) deductStockComp(c *echo.Context) any {
	return run(h, c, func(tx *sql.Tx, r DeductStockReq) error {
		if r.Qty <= 0 {
			r.Qty = 1
		}
		if err := h.store.RestoreStock(tx, r.VoucherID, r.Qty); err != nil {
			slog.Error("restore stock compensation failed, will retry",
				"voucher_id", r.VoucherID, "error", err)
			return dtmcli.ErrOngoing
		}
		return nil
	})
}

func Health(c *echo.Context) error { return c.String(http.StatusOK, "ok") }
