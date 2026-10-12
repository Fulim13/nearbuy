// Package branch holds voucher-service's DTM Saga branch endpoints.
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

func (h *Handler) run(c *echo.Context, busi func(tx *sql.Tx, r DeductStockReq) error) any {
	bb, err := dtmx.BarrierFromEcho(c)
	if err != nil {
		return err
	}
	slog.Info("saga branch", "path", c.Request().URL.Path, "barrier", bb.String())

	return bb.CallWithDB(h.db, func(tx *sql.Tx) error {
		var r DeductStockReq
		if err := json.NewDecoder(c.Request().Body).Decode(&r); err != nil {
			return dtmcli.ErrFailure
		}
		if r.Qty <= 0 {
			r.Qty = 1
		}
		return busi(tx, r)
	})
}

// deductStock is the only branch in the project that returns ErrFailure on a
// business condition -- and that is the entire point of Saga S1.
//
// Out of stock is not a transient error: no amount of retrying creates stock.
// Returning ErrOngoing here would retry until a human intervened while the
// order sat 'unpaid' forever. ErrFailure tells DTM to compensate, which deletes
// the order the first branch inserted.
func (h *Handler) deductStock(c *echo.Context) any {
	return h.run(c, func(tx *sql.Tx, r DeductStockReq) error {
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
// DTM may call this even if deduct_stock never committed: the forward call
// timed out (DTM can't tell whether it ran) or it returned ErrFailure (its
// transaction rolled back). The barrier handles this: if no "action" record
// exists for this branch, it skips the body, so we never restore stock that
// was never deducted. It also blocks a late-arriving forward call from
// running after compensation.
func (h *Handler) deductStockComp(c *echo.Context) any {
	return h.run(c, func(tx *sql.Tx, r DeductStockReq) error {
		if err := h.store.RestoreStock(tx, r.VoucherID, r.Qty); err != nil {
			slog.Error("restore stock compensation failed, will retry",
				"voucher_id", r.VoucherID, "error", err)
			return dtmcli.ErrOngoing
		}
		return nil
	})
}

func Health(c *echo.Context) error {
	return c.String(http.StatusOK, "ok")
}
