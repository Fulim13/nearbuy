// Package scheduler holds order-service's background jobs.
package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Fulim13/nearbuy/gen/golang/v1/voucher"
	"github.com/Fulim13/nearbuy/services/order-service/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
)

type Config struct {
	// Interval between ticks.
	Interval time.Duration
	// Age at which an unpaid order is expired. 15m in production; seconds in
	// the demo.
	Expiry time.Duration
	// Rows per step per tick. Bounded so that a backlog after an outage drains
	// over several ticks instead of all at once.
	Batch int32
}

// StockRestorer is voucher-service's RestoreStock RPC, as an interface so a test
// can fake it. Implemented by voucher.VoucherClient.
type StockRestorer interface {
	RestoreStock(ctx context.Context, in *voucher.RestoreStockRequest, opts ...grpc.CallOption) (*voucher.RestoreStockResponse, error)
}

// restoreTimeout bounds one RestoreStock call, so a hung voucher-service stalls
// one order for a few seconds rather than the whole tick.
const restoreTimeout = 3 * time.Second

// Expiry cancels orders left unpaid past the payment window and gives their
// stock back. It used to be Saga S3; it is two plain steps now, because nothing
// after the cancel can fail -- giving stock back only ever needs retrying:
//
//  1. cancel   one local UPDATE in order_db (CancelExpiredOrders)
//  2. restore  voucher-service RestoreStock for every cancelled order not yet
//     marked stock_restored_at, then mark it
//
// A failed restore leaves stock_restored_at NULL, so the next tick tries again.
// RestoreStock is idempotent per order, so a retry -- or two replicas restoring
// the same order -- gives the stock back once.
type Expiry struct {
	repo     *repository.Queries
	vouchers StockRestorer
	cfg      Config
}

func NewExpiry(pool *pgxpool.Pool, vouchers StockRestorer, cfg Config) *Expiry {
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.Expiry <= 0 {
		cfg.Expiry = 15 * time.Minute
	}
	if cfg.Batch <= 0 {
		cfg.Batch = 100
	}
	return &Expiry{repo: repository.New(pool), vouchers: vouchers, cfg: cfg}
}

// Run ticks until ctx is cancelled. Every replica runs it; no lock is needed,
// see CancelExpiredOrders and RestoreStock.
func (x *Expiry) Run(ctx context.Context) error {
	t := time.NewTicker(x.cfg.Interval)
	defer t.Stop()

	slog.Info("expiry scheduler started",
		"interval", x.cfg.Interval, "expiry", x.cfg.Expiry, "batch", x.cfg.Batch)
	for {
		select {
		case <-ctx.Done():
			slog.Info("expiry scheduler stopping")
			return ctx.Err()
		case <-t.C:
			if err := x.tick(ctx); err != nil && !errors.Is(err, context.Canceled) {
				// Never fatal. A failed tick is retried by the next tick, and a
				// scheduler that exits on a transient database error is a
				// scheduler that stops sweeping for the rest of the pod's life.
				slog.Error("expiry tick failed", "error", err)
			}
		}
	}
}

func (x *Expiry) tick(ctx context.Context) error {
	cancelled, err := x.repo.CancelExpiredOrders(ctx, repository.CancelExpiredOrdersParams{
		// sqlc maps a Postgres interval to pgtype.Interval, so a Go duration has
		// to be handed over in microseconds. Months and Days stay zero: a
		// duration is an exact span, and "1 month" is not.
		OlderThan: pgtype.Interval{Microseconds: x.cfg.Expiry.Microseconds(), Valid: true},
		MaxRows:   x.cfg.Batch,
	})
	if err != nil {
		return err
	}
	if len(cancelled) > 0 {
		slog.Info("cancelled expired orders", "count", len(cancelled))
	}

	// Restore runs every tick, not only after a cancel: it is also what picks up
	// orders whose restore failed on an earlier tick.
	return x.restorePending(ctx)
}

func (x *Expiry) restorePending(ctx context.Context) error {
	pending, err := x.repo.ListRestorePending(ctx, x.cfg.Batch)
	if err != nil {
		return err
	}
	for _, o := range pending {
		callCtx, cancel := context.WithTimeout(ctx, restoreTimeout)
		resp, err := x.vouchers.RestoreStock(callCtx, &voucher.RestoreStockRequest{
			OrderId: o.ID, VoucherId: o.VoucherID, UserId: o.UserID,
		})
		cancel()
		if err != nil {
			// Keep going: one unreachable voucher must not hold up the rest, and
			// this order is listed again next tick.
			slog.Warn("restore stock failed, will retry next tick",
				"order_id", o.ID, "voucher_id", o.VoucherID, "error", err)
			continue
		}
		if err := x.repo.MarkStockRestored(ctx, o.ID); err != nil {
			// The stock IS back; only the mark is missing. Next tick calls
			// RestoreStock again, which is a no-op for this order, and retries
			// the mark.
			slog.Error("mark stock restored failed", "order_id", o.ID, "error", err)
			continue
		}
		slog.Info("expired order stock restored",
			"order_id", o.ID, "voucher_id", o.VoucherID, "restored_now", resp.GetRestored())
	}
	return nil
}
