package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fulim13/nearbuy/services/order-service/internal/branch"
	"github.com/Fulim13/nearbuy/services/order-service/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrOrderNotFound = errors.New("order not found")

type OrderService struct {
	pool *pgxpool.Pool
	repo *repository.Queries
}

func NewOrderService(pool *pgxpool.Pool) *OrderService {
	return &OrderService{pool: pool, repo: repository.New(pool)}
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
		INSERT INTO voucher_order (id, user_id, voucher_id, pay_value, status)
		VALUES ($1, $2, $3, $4, 'unpaid')
		ON CONFLICT (id) DO NOTHING`,
		r.OrderID, r.UserID, r.VoucherID, r.PayValue)
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
