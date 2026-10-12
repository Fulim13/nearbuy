// Package seckill is the admission path: decide, atomically, whether one user
// gets one of a limited number of vouchers.
//
// It touches NO DATABASE. That is the entire design: at 10,000 requests a second
// against 100 items of stock, 9,900 of those requests are rejections, and making
// a rejection cost a Postgres round trip is what takes the database down. Redis
// decides; Postgres finds out later, from a Kafka consumer.
package seckill

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed admit.lua
var admitSrc string

//go:embed release.lua
var releaseSrc string

var (
	admitScript   = redis.NewScript(admitSrc)
	releaseScript = redis.NewScript(releaseSrc)
)

// Code is the admission outcome. The numbers are the script's return values.
type Code int

const (
	Admitted       Code = 0
	OutOfStock     Code = 1
	AlreadyOrdered Code = 2
	NotStarted     Code = 3
	Finished       Code = 4
	UnknownVoucher Code = 5
)

func (c Code) String() string {
	switch c {
	case Admitted:
		return "admitted"
	case OutOfStock:
		return "out of stock"
	case AlreadyOrdered:
		return "already ordered"
	case NotStarted:
		return "sale has not started"
	case Finished:
		return "sale has ended"
	case UnknownVoucher:
		return "unknown voucher"
	default:
		return "unknown code " + strconv.Itoa(int(c))
	}
}

func StockKey(voucherID int64) string {
	return "seckill:stock:" + strconv.FormatInt(voucherID, 10)
}

// OrderKey holds the user ids that have already been admitted. A SET, so
// SISMEMBER is O(1) inside the script.
func OrderKey(voucherID int64) string {
	return "seckill:order:" + strconv.FormatInt(voucherID, 10)
}

// Window is a sale's open and close instants.
type Window struct {
	Begin time.Time
	End   time.Time
}

// WindowLoader fetches a sale window from the authoritative store. Injected so
// this package does not import the repository.
type WindowLoader func(ctx context.Context, voucherID int64) (Window, error)

// Admitter runs the admission script.
//
// It caches sale windows in process and never invalidates them, because
// seckill_voucher.begin_time and end_time do not change once the sale is created
// -- the valid_window CHECK is on an immutable row. Caching
// something mutable this way would be a bug; caching this is just correct.
type Admitter struct {
	rdb    *redis.Client
	load   WindowLoader
	mu     sync.RWMutex
	cached map[int64]Window
}

func NewAdmitter(rdb *redis.Client, load WindowLoader) *Admitter {
	return &Admitter{rdb: rdb, load: load, cached: make(map[int64]Window)}
}

// Admit is the hot path: at most one Redis round trip once the window is cached.
func (a *Admitter) Admit(ctx context.Context, voucherID, userID int64) (Code, error) {
	w, err := a.window(ctx, voucherID)
	if err != nil {
		return UnknownVoucher, err
	}

	res, err := admitScript.Run(ctx, a.rdb,
		[]string{StockKey(voucherID), OrderKey(voucherID)},
		userID, w.Begin.UnixMilli(), w.End.UnixMilli(),
	).Int64()
	if err != nil {
		return 0, fmt.Errorf("seckill admit: %w", err)
	}
	return Code(res), nil
}

// It returns whether it ACTUALLY released. release.lua checks SISMEMBER first,
// so calling it twice releases once -- the set membership IS the idempotency
// token, which is why lesson 14's Saga branches can call this from inside a
// transaction that may be retried. A bare SREM + INCRBY would double the stock.
func (a *Admitter) Release(ctx context.Context, voucherID, userID int64) (bool, error) {
	n, err := releaseScript.Run(ctx, a.rdb,
		[]string{StockKey(voucherID), OrderKey(voucherID)}, userID).Int64()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func (a *Admitter) window(ctx context.Context, voucherID int64) (Window, error) {
	a.mu.RLock()
	w, ok := a.cached[voucherID]
	a.mu.RUnlock()
	if ok {
		return w, nil
	}

	w, err := a.load(ctx, voucherID)
	if err != nil {
		return Window{}, err
	}

	a.mu.Lock()
	a.cached[voucherID] = w
	a.mu.Unlock()
	return w, nil
}

// Remember lets the creation path seed the cache, so the very first request of a
// flash sale does not pay for a Postgres read.
func (a *Admitter) Remember(voucherID int64, w Window) {
	a.mu.Lock()
	a.cached[voucherID] = w
	a.mu.Unlock()
}
