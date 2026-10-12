// Package branch holds user-service's DTM Saga branch endpoints.
//
// user-service appears in exactly one Saga -- S2, the payment -- and owns its
// first branch. The money leaves here before the order says 'paid', which is a
// deliberate ordering choice; see the lesson.
package branch

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Fulim13/nearbuy/pkg/dtmx"
	"github.com/Fulim13/nearbuy/services/user-service/internal/service"
	"github.com/dtm-labs/client/dtmcli"
	"github.com/labstack/echo/v5"
)

// DeductBalanceReq is replayed verbatim to the compensation, which is why the
// amount is in the payload rather than re-read from the order.
type DeductBalanceReq struct {
	UserID  int64 `json:"user_id"`
	Amount  int64 `json:"amount"`
	OrderID int64 `json:"order_id"` // for the log only; this service owns no orders
}

type Store interface {
	DeductBalance(tx *sql.Tx, userID, amount int64) error
	RefundBalance(tx *sql.Tx, userID, amount int64) error
}

type Handler struct {
	db    *sql.DB
	store Store
}

func New(db *sql.DB, store Store) *Handler { return &Handler{db: db, store: store} }

func Register(e *echo.Echo, h *Handler) {
	g := e.Group("/branch")
	g.POST("/deduct_balance", dtmx.WrapBranch(h.deductBalance))
	g.POST("/deduct_balance_comp", dtmx.WrapBranch(h.deductBalanceComp))
}

func (h *Handler) run(c *echo.Context, busi func(tx *sql.Tx, r DeductBalanceReq) error) any {
	bb, err := dtmx.BarrierFromEcho(c)
	if err != nil {
		return err
	}
	slog.Info("saga branch", "path", c.Request().URL.Path, "barrier", bb.String())

	return bb.CallWithDB(h.db, func(tx *sql.Tx) error {
		var r DeductBalanceReq
		if err := json.NewDecoder(c.Request().Body).Decode(&r); err != nil {
			return dtmcli.ErrFailure
		}
		if r.Amount <= 0 || r.UserID <= 0 {
			// A zero or negative amount is not a transient condition.
			return dtmcli.ErrFailure
		}
		return busi(tx, r)
	})
}

// deductBalance maps an overdraft to ErrFailure and everything else to
// ErrOngoing.
//
// "Insufficient balance" is the clearest ErrFailure in the whole project:
// retrying cannot create money. The user has to top up and try again, which is
// a NEW Saga with a new gid, not a retry of this one.
func (h *Handler) deductBalance(c *echo.Context) any {
	return h.run(c, func(tx *sql.Tx, r DeductBalanceReq) error {
		err := h.store.DeductBalance(tx, r.UserID, r.Amount)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, service.ErrInsufficientBalance), errors.Is(err, service.ErrUserNotFound):
			slog.Warn("deduct balance refused, saga will compensate",
				"user_id", r.UserID, "amount", r.Amount, "order_id", r.OrderID, "reason", err)
			return dtmcli.ErrFailure
		default:
			slog.Error("deduct balance branch failed",
				"user_id", r.UserID, "order_id", r.OrderID, "error", err)
			return dtmcli.ErrOngoing
		}
	})
}

// deductBalanceComp refunds. It never returns ErrFailure -- money that has left
// an account has to be able to come back, unconditionally.
func (h *Handler) deductBalanceComp(c *echo.Context) any {
	return h.run(c, func(tx *sql.Tx, r DeductBalanceReq) error {
		if err := h.store.RefundBalance(tx, r.UserID, r.Amount); err != nil {
			slog.Error("refund compensation failed, will retry",
				"user_id", r.UserID, "amount", r.Amount, "error", err)
			return dtmcli.ErrOngoing
		}
		return nil
	})
}

func Health(c *echo.Context) error { return c.String(http.StatusOK, "ok") }
