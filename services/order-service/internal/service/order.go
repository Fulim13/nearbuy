package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fulim13/nearbuy/services/order-service/internal/branch"
	"github.com/Fulim13/nearbuy/services/order-service/internal/repository"
	"github.com/Fulim13/nearbuy/services/order-service/internal/saga"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrOrderNotFound = errors.New("order not found")
	ErrNotOwner      = errors.New("order belongs to another user")
	ErrNotUnpaid     = errors.New("order is not awaiting payment")
)

type OrderService struct {
	pool      *pgxpool.Pool
	repo      *repository.Queries
	endpoints saga.Endpoints
}

func NewOrderService(pool *pgxpool.Pool, e saga.Endpoints) *OrderService {
	return &OrderService{pool: pool, repo: repository.New(pool), endpoints: e}
}


func (s *OrderService) GetOrder(ctx context.Context, id int64) (repository.VoucherOrder, error) {
	o, err := s.repo.GetOrder(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return repository.VoucherOrder{}, ErrOrderNotFound
	}
	return o, err
}

func (s *OrderService) ListOrdersByUser(ctx context.Context, userID int64, limit, offset int32) ([]repository.VoucherOrder, error) {
	rows, err := s.repo.ListOrdersByUser(ctx, repository.ListOrdersByUserParams{
		UserID: userID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []repository.VoucherOrder{}
	}
	return rows, nil
}

// ---------------------------------------------------------------- saga branches
//
// These take a *sql.Tx, not a pgx.Tx, because they run inside the transaction
// DTM's barrier opened -- and dtmcli is built on database/sql. The sqlc-generated
// repository is pgx, so these two methods write their SQL by hand.

// SQLStore implements branch.Store over the barrier's transaction.
// package branch
//
//	type Store interface {
//	    CreateOrder(tx *sql.Tx, r CreateOrderReq) error
//	    DeleteOrder(tx *sql.Tx, orderID int64) error
//	}
type SQLStore struct{}

func (SQLStore) CreateOrder(tx *sql.Tx, r branch.CreateOrderReq) error {
	_, err := tx.Exec(`
		INSERT INTO voucher_order (id, user_id, voucher_id, shop_id, pay_value, status)
		VALUES ($1, $2, $3, $4, $5, 'unpaid')
		ON CONFLICT (id) DO NOTHING`,
		r.OrderID, r.UserID, r.VoucherID, r.ShopID, r.PayValue)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}
	return nil
}

func (SQLStore) DeleteOrder(tx *sql.Tx, orderID int64) error {
	if _, err := tx.Exec(`DELETE FROM voucher_order WHERE id = $1`, orderID); err != nil {
		return fmt.Errorf("delete order: %w", err)
	}
	return nil
}

// PayOrder is S2's second branch.
//
// The guard is in the WHERE clause, not in a SELECT beforehand. A
// read-then-write would be two statements with a gap between them, and the
// REPEATABLE READ isolation this transaction does not use would not close it --
// a concurrent cancel could land in the gap. One UPDATE whose WHERE carries the
// precondition is atomic by construction, and rows-affected is the answer.
func (SQLStore) PayOrder(tx *sql.Tx, orderID int64) error {
	res, err := tx.Exec(`
		UPDATE voucher_order
		SET status = 'paid', pay_type = 'balance', pay_time = now()
		WHERE id = $1 AND status = 'unpaid'`, orderID)
	if err != nil {
		return fmt.Errorf("pay order: %w", err)
	}
	return oneRow(res, branch.ErrNotPayable, "pay order")
}

func (SQLStore) UnpayOrder(tx *sql.Tx, orderID int64) error {
	// No status guard on the compensation. It must converge, and a guard that
	// does not match is a guard that makes it loop forever.
	_, err := tx.Exec(`
		UPDATE voucher_order
		SET status = 'unpaid', pay_time = NULL
		WHERE id = $1`, orderID)
	if err != nil {
		return fmt.Errorf("unpay order: %w", err)
	}
	return nil
}

// CancelOrder is S3's first branch. Same WHERE-clause guard, different reason:
// here zero rows means the user paid between the sweep's SELECT and this UPDATE.
func (SQLStore) CancelOrder(tx *sql.Tx, orderID int64) error {
	res, err := tx.Exec(`
		UPDATE voucher_order
		SET status = 'cancelled'
		WHERE id = $1 AND status = 'unpaid'`, orderID)
	if err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}
	return oneRow(res, branch.ErrNotCancellable, "cancel order")
}

func (SQLStore) UncancelOrder(tx *sql.Tx, orderID int64) error {
	_, err := tx.Exec(`
		UPDATE voucher_order SET status = 'unpaid' WHERE id = $1`, orderID)
	if err != nil {
		return fmt.Errorf("uncancel order: %w", err)
	}
	return nil
}

// oneRow turns "the WHERE clause matched nothing" into a named error the branch
// can map to dtmcli.ErrFailure, and keeps a genuine driver error transient.
func oneRow(res sql.Result, mismatch error, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows: %w", what, err)
	}
	if n == 0 {
		return mismatch
	}
	return nil
}

// Pay starts Saga S2 and waits for it.
//
// The three checks below are a COURTESY, not the authority. Each one is
// re-asserted inside the branch that depends on it -- pay_order's WHERE clause
// re-checks 'unpaid', deduct_balance's CHECK constraint re-checks the balance --
// because between this function and those branches is a network. What the checks
// buy is a useful error: "order is not awaiting payment" instead of a Saga that
// fails somewhere the caller cannot see.
//
// Skipping them would be correct and unkind. Trusting them would be neither.
func (s *OrderService) Pay(ctx context.Context, orderID, callerID int64) (repository.VoucherOrder, error) {
	o, err := s.GetOrder(ctx, orderID)
	if err != nil {
		return repository.VoucherOrder{}, err
	}
	if o.UserID != callerID {
		return repository.VoucherOrder{}, ErrNotOwner
	}
	if o.Status != repository.OrderStatusUnpaid {
		return repository.VoucherOrder{}, ErrNotUnpaid
	}

	if err := saga.SubmitPay(ctx, s.endpoints, o.ID, o.UserID, o.PayValue); err != nil {
		return repository.VoucherOrder{}, err
	}

	// WaitResult was true, so by here the Saga is terminal and the row is
	// current. Re-read rather than synthesising the response: the branch did the
	// write and the branch's view is the one the user should see.
	return s.GetOrder(ctx, orderID)
}
