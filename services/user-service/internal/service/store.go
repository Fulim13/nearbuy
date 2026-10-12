package service

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrInsufficientBalance is S2's one business failure, and the whole reason S2
// is interesting: it is a condition the database detects, not one Go checked.
var ErrInsufficientBalance = errors.New("insufficient balance")

// SQLStore is user-service's Saga branch data layer, over the transaction DTM's
// barrier opened.
type SQLStore struct{}

// DeductBalance is S2's FIRST branch.
//
// There is no "AND balance >= $1" guard, on purpose. because
//
//	CONSTRAINT balance_positive CHECK (balance >= 0)
//
// so an overdraft is a NAMED constraint violation -- a fact the branch can map
// to dtmcli.ErrFailure. A WHERE guard would instead return zero rows, which is
// indistinguishable from "no such user", and the two need different handling.
//
// It is also the difference between a check that holds under concurrency and one
// that does not: the CHECK is evaluated by Postgres against the row it just
// wrote, inside the same transaction, holding the row lock. Nothing can
// interleave.
func (SQLStore) DeductBalance(tx *sql.Tx, userID, amount int64) error {
	res, err := tx.Exec(
		`UPDATE app_user SET balance = balance - $1 WHERE id = $2`,
		amount, userID)
	if err != nil {
		if isCheckViolation(err, "balance_positive") {
			return ErrInsufficientBalance
		}
		return fmt.Errorf("deduct balance: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("deduct balance rows: %w", err)
	}
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// RefundBalance is DeductBalance's compensation.
//
// Note what it does NOT have: a CHECK that could fail. balance + amount can
// never violate balance_positive, so this compensation has no business failure
// mode at all -- which is exactly the property a compensation needs.
func (SQLStore) RefundBalance(tx *sql.Tx, userID, amount int64) error {
	if _, err := tx.Exec(
		`UPDATE app_user SET balance = balance + $1 WHERE id = $2`,
		amount, userID); err != nil {
		return fmt.Errorf("refund balance: %w", err)
	}
	return nil
}

// isCheckViolation reports whether err is a Postgres CHECK violation for a
// NAMED constraint. Same helper as voucher-service's, and for the same reason:
// branching on a constraint name is stable, branching on an error message is
// not.
func isCheckViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	// 23514 = check_violation
	return pgErr.Code == "23514" && pgErr.ConstraintName == constraint
}
