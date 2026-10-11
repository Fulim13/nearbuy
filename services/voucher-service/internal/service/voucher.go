package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/Fulim13/nearbuy/pkg/idgen"
	"github.com/Fulim13/nearbuy/services/voucher-service/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// StockKey is the Redis counter the seckill's Lua script decrements.
// Exported because the seckill admission runs in the gateway, not here.
func StockKey(voucherID int64) string {
	return "seckill:stock:" + strconv.FormatInt(voucherID, 10)
}

var (
	ErrVoucherNotFound = errors.New("voucher not found")
	ErrNotSeckill      = errors.New("voucher is not a seckill voucher")
	ErrInvalidWindow   = errors.New("end_time must be after begin_time")
	ErrOutOfStock      = errors.New("out of stock")
)

type VoucherService struct {
	pool *pgxpool.Pool
	repo *repository.Queries
	rdb  *redis.Client
	ids  *idgen.Generator
}

func NewVoucherService(pool *pgxpool.Pool, rdb *redis.Client) *VoucherService {
	return &VoucherService{
		pool: pool,
		repo: repository.New(pool),
		rdb:  rdb,
		ids:  idgen.New(rdb),
	}
}

func (s *VoucherService) withTx(ctx context.Context, fn func(q *repository.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(s.repo.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *VoucherService) CreateVoucher(ctx context.Context, arg repository.CreateVoucherParams) (repository.Voucher, error) {
	return s.repo.CreateVoucher(ctx, arg)
}

// CreateSeckillVoucher writes both halves in ONE transaction and only then warms
// Redis.
//
// Order matters. Postgres is the source of truth for stock; the Redis counter is
// a fast copy that Lua decrements. Warming Redis first would let a
// seckill start against stock for a voucher whose rows never committed.
func (s *VoucherService) CreateSeckillVoucher(
	ctx context.Context,
	arg repository.CreateSeckillVoucherRowParams,
	stock int32,
	beginTime, endTime time.Time,
) (repository.Voucher, error) {
	if !endTime.After(beginTime) {
		// Also enforced by the valid_window CHECK; caught here so the caller
		// gets a clean error instead of a constraint violation.
		return repository.Voucher{}, ErrInvalidWindow
	}

	var created repository.Voucher
	err := s.withTx(ctx, func(q *repository.Queries) error {
		v, err := q.CreateSeckillVoucherRow(ctx, arg)
		if err != nil {
			return fmt.Errorf("insert voucher: %w", err)
		}
		if err := q.InsertSeckillStock(ctx, repository.InsertSeckillStockParams{
			VoucherID: v.ID,
			Stock:     stock,
			BeginTime: beginTime,
			EndTime:   endTime,
		}); err != nil {
			return fmt.Errorf("insert seckill stock: %w", err)
		}
		created = v
		return nil
	})
	if err != nil {
		return repository.Voucher{}, err
	}

	if err := s.warmOne(ctx, created.ID, stock); err != nil {
		// Not fatal: WarmStock at startup will pick it up, and until then the
		// Lua admission simply finds no key and admits nobody -- which fails
		// CLOSED, the safe direction for a seckill.
		slog.Warn("could not warm seckill stock", "voucher_id", created.ID, "error", err)
	}
	return created, nil
}

func (s *VoucherService) GetVoucher(ctx context.Context, id int64) (repository.GetVoucherRow, error) {
	v, err := s.repo.GetVoucher(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return repository.GetVoucherRow{}, ErrVoucherNotFound
	}
	return v, err
}

func (s *VoucherService) ListVouchersByShop(ctx context.Context, shopID int64) ([]repository.ListVouchersByShopRow, error) {
	rows, err := s.repo.ListVouchersByShop(ctx, shopID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []repository.ListVouchersByShopRow{}
	}
	return rows, nil
}

// NextOrderID is here rather than in order-service because the gateway needs an
// order id BEFORE order-service has seen the request (returns it in a
// 202). voucher-service is already on the gateway's hot path for the admission
// check, so it is the cheapest place to ask.
func (s *VoucherService) NextOrderID(ctx context.Context) (int64, error) {
	return s.ids.NextID(ctx, "order")
}

// WarmStock copies the stock of every live seckill into Redis. Called at startup,
// so a Redis restart does not permanently break every running seckill.
//
// Redis holds a COPY. Postgres is authoritative, which is what makes this
// rebuildable -- and what makes Saga able to reject an admission that
// Redis wrongly allowed.
func (s *VoucherService) WarmStock(ctx context.Context) (int, error) {
	rows, err := s.repo.ListSeckillVouchers(ctx)
	if err != nil {
		return 0, err
	}
	for _, r := range rows {
		if err := s.warmOne(ctx, r.VoucherID, r.Stock); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

// warmOne sets the counter only if it is ABSENT.
//
// the Redis counter and the Postgres stock are not always equal, and Redis is usually the more up-to-date of the two during a live seckill.
// When someone grabs a voucher:
// 1. The gateway’s Lua script decrements the Redis counter immediately.
// 2. The order goes off to the Saga, and only later does DeductStock decrement Postgres.
// What goes wrong with plain SET
// Say the voucher has 100 stock:
// Moment						Redis	Postgres
// Seckill starts					100	100
// 60 people admitted, Saga has processed 10 of them	40	90
// voucher-service restarts, WarmStock runs	?	90
// If warmOne used SET, Redis would be overwritten with 90
// SETNX means “set only if the key does not exist.”
func (s *VoucherService) warmOne(ctx context.Context, voucherID int64, stock int32) error {
	return s.rdb.SetNX(ctx, StockKey(voucherID), stock, 0).Err()
}

// DeductStock is Saga S1's forward action.
//
// It reports a constraint violation as ErrOutOfStock, which is the whole point:
// Redis is an optimistic pre-filter, and when the two disagree Postgres wins and
// the Saga compensates.
func (s *VoucherService) DeductStock(ctx context.Context, voucherID int64, qty int32) error {
	rows, err := s.repo.DeductStock(ctx, repository.DeductStockParams{
		VoucherID: voucherID, Qty: qty,
	})
	if err != nil {
		if isCheckViolation(err, "stock_not_negative") {
			return ErrOutOfStock
		}
		return err
	}
	if rows == 0 {
		return ErrNotSeckill
	}
	return nil
}

// RestoreStock is DeductStock's compensation.
func (s *VoucherService) RestoreStock(ctx context.Context, voucherID int64, qty int32) error {
	rows, err := s.repo.RestoreStock(ctx, repository.RestoreStockParams{
		VoucherID: voucherID, Qty: qty,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotSeckill
	}
	return nil
}
