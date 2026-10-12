package service

import (
	"database/sql"
	"fmt"
	"log/slog"
)


type SQLStore struct{}

// DeductStock is Saga S1's forward action.
//
// No "AND stock > 0" guard: CHECK (stock >= 0) does that in the
// database, so an oversell surfaces as a named constraint violation the branch
// can map to ErrFailure. A WHERE guard would instead return 0 rows, which is
// indistinguishable from "that voucher is not a seckill voucher".
func (SQLStore) DeductStock(tx *sql.Tx, voucherID int64, qty int32) error {
	res, err := tx.Exec(
		`UPDATE seckill_voucher SET stock = stock - $1 WHERE voucher_id = $2`,
		qty, voucherID)
	if err != nil {
		if isCheckViolation(err, "stock_not_negative") {
			// Redis said yes and Postgres says no. Postgres wins; the Saga
			// unwinds. This is the disagreement lesson 16 provokes on purpose.
			return ErrOutOfStock
		}
		return fmt.Errorf("deduct stock: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("deduct stock rows: %w", err)
	}
	if n == 0 {
		return ErrNotSeckill
	}
	return nil
}

// RestoreStock is DeductStock's compensation.
func (SQLStore) RestoreStock(tx *sql.Tx, voucherID int64, qty int32) error {
	res, err := tx.Exec(
		`UPDATE seckill_voucher SET stock = stock + $1 WHERE voucher_id = $2`,
		qty, voucherID)
	if err != nil {
		return fmt.Errorf("restore stock: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("restore stock rows: %w", err)
	}
	if n == 0 {
		slog.Error("restore stock found no row; stock cannot be returned",
			"voucher_id", voucherID, "qty", qty)
	}
	return nil
}
