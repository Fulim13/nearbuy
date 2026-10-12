// Package branch holds voucher-service's DTM Saga branch endpoints.
//
//	S1  deduct_stock        / deduct_stock_comp         stock - 1
//	S3  restore_for_expiry  / restore_for_expiry_comp   stock + 1, both stores
package branch

import (
	"context"
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

// ExpiryReq is Saga S3's second branch: give one unit of stock back, in BOTH
// stores. order_id is carried for the log, since this service owns no orders.
type ExpiryReq struct {
	VoucherID int64 `json:"voucher_id"`
	UserID    int64 `json:"user_id"`
	OrderID   int64 `json:"order_id"`
}

type Store interface {
	DeductStock(tx *sql.Tx, voucherID int64, qty int32) error
	RestoreStock(tx *sql.Tx, voucherID int64, qty int32) error
}

// Releaser is the Redis half of the expiry restore, injected as an interface so
// this package does not import internal/seckill. Implemented by
// *seckill.Admitter.
type Releaser interface {
	Release(ctx context.Context, voucherID, userID int64) (bool, error)
}

type Handler struct {
	db       *sql.DB
	store    Store
	releaser Releaser
}

func New(db *sql.DB, store Store, releaser Releaser) *Handler {
	return &Handler{db: db, store: store, releaser: releaser}
}

func Register(e *echo.Echo, h *Handler) {
	g := e.Group("/branch")

	g.POST("/deduct_stock", dtmx.WrapBranch(h.deductStock))
	g.POST("/deduct_stock_comp", dtmx.WrapBranch(h.deductStockComp))

	g.POST("/restore_for_expiry", dtmx.WrapBranch(h.restoreForExpiry))
	g.POST("/restore_for_expiry_comp", dtmx.WrapBranch(h.restoreForExpiryComp))
}

// run is generic for the same reason order-service's is: two Sagas, two payload
// types, and Go does not allow type parameters on methods.
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

// ---------------------------------------------------------------- S3

// restoreForExpiry is the only branch in the project that writes to two
// different STORES, and the order inside it is the whole point.
//
//  1. Postgres  stock + 1     -- inside the barrier's transaction
//  2. Redis     release.lua   -- after the SQL, before the commit
//
// Redis is not transactional with Postgres and never can be. Two separate
// properties are what make this safe:
//
//   - Redis LAST. If release.lua fails, returning the error rolls the
//     transaction back -- barrier row and stock increment together -- and DTM
//     retries the whole branch. Had Redis gone first, a Postgres failure would
//     leave Redis ahead with nothing recording that it happened.
//   - release.lua is IDEMPOTENT. The remaining window is "Redis succeeded,
//     COMMIT failed": the barrier row is gone with the rollback, so the retry
//     runs the body again. A bare SREM + INCRBY would double the Redis stock
//     there. release.lua checks SISMEMBER first, so the second call returns 0
//     and changes nothing.
//
// That second point is why lesson 13 wrote release.lua with a membership check
// instead of two bare commands. It did not matter then. It matters here.
func (h *Handler) restoreForExpiry(c *echo.Context) any {
	return run(h, c, func(tx *sql.Tx, r ExpiryReq) error {
		if err := h.store.RestoreStock(tx, r.VoucherID, 1); err != nil {
			// Even ErrNotSeckill is ErrOngoing here. S3 is a cancellation and a
			// cancellation is not allowed to give up: if this voucher's row is
			// genuinely gone, the right outcome is a Saga a human can see in
			// trans_global, not a silent abandonment.
			slog.Error("expiry stock restore failed, will retry",
				"order_id", r.OrderID, "voucher_id", r.VoucherID, "error", err)
			return dtmcli.ErrOngoing
		}

		// Why S3 must reach Redis at all: returning the Postgres stock while
		// leaving the user in seckill:order:{vid} means they can NEVER buy this
		// voucher again -- admit.lua's SISMEMBER check rejects them forever --
		// while the two stock counts diverge silently. A compensation is only
		// complete if it covers every store the forward path touched.
		released, err := h.releaser.Release(c.Request().Context(), r.VoucherID, r.UserID)
		if err != nil {
			slog.Error("expiry redis release failed, rolling back and retrying",
				"order_id", r.OrderID, "voucher_id", r.VoucherID, "error", err)
			return dtmcli.ErrOngoing
		}
		if released {
			slog.Info("expiry released the admission",
				"order_id", r.OrderID, "voucher_id", r.VoucherID, "user_id", r.UserID)
		} else {
			// Not an error. The user was not in seckill:order:{vid} -- the key
			// was flushed, or a previous attempt of this branch already
			// released. Postgres is authoritative either way.
			slog.Warn("expiry found no admission to release",
				"order_id", r.OrderID, "voucher_id", r.VoucherID, "user_id", r.UserID)
		}
		return nil
	})
}

// restoreForExpiryComp is UNREACHABLE by design, and exists anyway.
//
// S3 has exactly one failure point -- cancel_order, its FIRST branch -- so
// nothing after it can trigger an unwind. DTM still wants a URL per branch, and
// an endpoint that genuinely un-restored the stock would be code that can only
// ever be wrong if it ran: the order is cancelled by then, so taking the stock
// back would strand it.
//
// So it succeeds and says so loudly. Leaving the stock restored errs toward the
// user being able to buy again, which is the right direction to be wrong in.
func (h *Handler) restoreForExpiryComp(c *echo.Context) any {
	return run(h, c, func(tx *sql.Tx, r ExpiryReq) error {
		slog.Error("S3 compensated restore_for_expiry; this should be unreachable",
			"order_id", r.OrderID, "voucher_id", r.VoucherID, "user_id", r.UserID)
		return nil
	})
}

func Health(c *echo.Context) error { return c.String(http.StatusOK, "ok") }
