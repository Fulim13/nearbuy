// Package scheduler holds order-service's single-runner background jobs.
package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Fulim13/nearbuy/services/order-service/internal/repository"
	"github.com/Fulim13/nearbuy/services/order-service/internal/saga"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ExpiryLockID is the advisory lock this job holds while it selects.
//
// A plain int64, chosen once and written down, NOT hashtext('order-expiry').
// hashtext is not documented as stable across major Postgres versions, and a
// lock id that changes under an upgrade is a lock two versions of the same
// deployment would not share -- which is the one thing it exists to prevent.
const ExpiryLockID int64 = 20261014

type Config struct {
	// Interval between ticks.
	Interval time.Duration
	// Age at which an unpaid order is expired. 15m in production; seconds in
	// the demo.
	Expiry time.Duration
	// Rows per tick. The sweep is bounded so that a backlog drains over several
	// ticks instead of submitting tens of thousands of Sagas at once.
	Batch int32
}

type Expiry struct {
	pool      *pgxpool.Pool
	repo      *repository.Queries
	endpoints saga.Endpoints
	cfg       Config
}

func NewExpiry(pool *pgxpool.Pool, e saga.Endpoints, cfg Config) *Expiry {
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.Expiry <= 0 {
		cfg.Expiry = 15 * time.Minute
	}
	if cfg.Batch <= 0 {
		cfg.Batch = 100
	}
	return &Expiry{pool: pool, repo: repository.New(pool), endpoints: e, cfg: cfg}
}

// Run ticks until ctx is cancelled.
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
	rows, got, err := x.claim(ctx)
	if err != nil {
		return err
	}
	if !got {
		// Another replica is sweeping. Nothing to log at anything above Debug:
		// with three replicas this is the normal outcome two times in three.
		slog.Debug("expiry tick skipped, lock held elsewhere")
		return nil
	}
	if len(rows) == 0 {
		return nil
	}

	slog.Info("expiring orders", "count", len(rows))
	for _, r := range rows {
		// Submitted AFTER the lock was released, deliberately -- see claim().
		if err := saga.SubmitExpire(ctx, x.endpoints, r.ID, r.UserID, r.VoucherID); err != nil {
			slog.Error("submit S3 failed", "order_id", r.ID, "error", err)
			// Keep going. One unreachable DTM should not stop the other 99
			// orders, and this one is re-selected next tick.
			continue
		}
	}
	return nil
}

// claim takes the advisory lock, selects a batch, and releases the lock by
// committing -- all before a single Saga is submitted.
//
// Two things here are easy to get wrong.
//
// 1. pg_try_advisory_XACT_lock, not pg_try_advisory_lock.
//
// Both exclude other connections correctly -- that is not the difference. The
// difference is RELEASE. A session-level advisory lock is held by the
// connection that took it until an explicit pg_advisory_unlock on that same
// connection, or until the connection closes. Handing the connection back to
// the pool does not release it.
//
// So through a pgxpool the session variant fails like this: the first tick
// takes the lock and returns the connection to the pool still holding it. Every
// later tick -- on that connection or any other -- gets false, because the lock
// is held by a session nobody is going to unlock. The sweeper runs exactly once
// in the lifetime of the pod and then silently never again. Nothing errors.
//
// Making it work would mean pool.Acquire()ing a connection, keeping it for the
// whole sweep, and unlocking on every exit path including panics. The xact
// variant is scoped to the transaction -- and pgx guarantees a transaction uses
// one connection -- so COMMIT releases it, unconditionally.
//
// 2. The lock covers the SELECT, not the submissions.
//
// Submitting N Sagas is N HTTP round-trips to DTM. Holding a Postgres
// transaction open across them pins a pooled connection for seconds at a time,
// for no benefit: the deterministic gid from lesson 14 is what makes a double
// sweep harmless. Two replicas selecting the same order submit expire-{id}
// twice and DTM runs it once.
//
// Which makes this lock an OPTIMISATION, not the correctness mechanism. Worth
// being honest about -- it is easy to write a lock like this and then believe
// the gid does not matter.
func (x *Expiry) claim(ctx context.Context) ([]repository.ListExpiredUnpaidRow, bool, error) {
	tx, err := x.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful Commit

	var locked bool
	if err := tx.QueryRow(ctx,
		`SELECT pg_try_advisory_xact_lock($1)`, ExpiryLockID).Scan(&locked); err != nil {
		return nil, false, err
	}
	if !locked {
		return nil, false, nil
	}

	rows, err := repository.New(tx).ListExpiredUnpaid(ctx, repository.ListExpiredUnpaidParams{
		// sqlc maps a Postgres interval to pgtype.Interval, so a Go duration has
		// to be handed over in microseconds. Months and Days stay zero: a
		// duration is an exact span, and "1 month" is not.
		OlderThan: pgtype.Interval{Microseconds: x.cfg.Expiry.Microseconds(), Valid: true},
		MaxRows:   x.cfg.Batch,
	})
	if err != nil {
		return nil, false, err
	}

	// COMMIT is what releases the advisory lock.
	return rows, true, tx.Commit(ctx)
}
