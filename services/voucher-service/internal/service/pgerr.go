package service

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// isCheckViolation reports whether err is a Postgres CHECK violation for a
// NAMED constraint.
//
// Matching on the constraint name rather than on the error text is what makes
// habit of naming every constraint pay off: "stock cannot go
// negative" becomes a condition the Go code can branch on, instead of a string
// comparison that breaks when the message changes.
func isCheckViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	// 23514 = check_violation
	return pgErr.Code == "23514" && pgErr.ConstraintName == constraint
}
