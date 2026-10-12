// Package saga builds the global transactions order-service starts.
//
// S1 lives in internal/consumer because Kafka starts it. S2 and S3 are here,
// because the things that start them -- an RPC and a scheduler -- have nothing
// else in common.
package saga

import (
	"context"
	"errors"
	"fmt"

	"github.com/Fulim13/nearbuy/pkg/dtmx"
	"github.com/dtm-labs/client/dtmcli"
)

// Endpoints is where DTM will call back.
type Endpoints struct {
	DTMServer   string // http://dtm:36789/api/dtmsvr
	OrderBase   string // http://order-service:8000/branch
	VoucherBase string // http://voucher-service:8000/branch
	UserBase    string // http://user-service:8000/branch
}

// RetryInterval is the floor DTM enforces. Named so the three Sagas cannot drift apart by accident.
const RetryInterval = 10

// ErrRolledBack means the Saga RAN and unwound: every branch that committed has
// been compensated, so the stores are back where they started.
//
// It exists because dtmcli cannot tell you this. Submit() ends in
//
//	dtmimp.TransCallDtmExt -> errors.New(resp.String())
//
// a PLAIN error, so errors.Is(err, dtmcli.ErrFailure) is false for every error
// Submit ever returns -- the sentinel only ever wraps BRANCH results, never
// submission results. Which leaves two options: string-match "FAILURE" in the
// response body, or ask DTM what happened. classify() asks.
var ErrRolledBack = errors.New("saga rolled back")

// classify turns a Submit error into one of three answers, by asking DTM for the
// transaction's terminal status rather than parsing its error text.
//
//	nil            the Saga actually succeeded (a lost response, a redelivery)
//	ErrRolledBack  it ran and compensated -- a business refusal, safe to retry
//	err            DTM never accepted it, or we cannot tell: transient
func classify(ctx context.Context, server, gid string, submitErr error) error {
	st, qerr := dtmx.QueryTrans(ctx, server, gid)
	if qerr != nil || st == nil {
		// Either DTM is unreachable (so the submit failure was transient) or it
		// has never heard of this gid (so nothing ran). Both are the caller's
		// error to retry.
		return submitErr
	}
	switch st.Status {
	case dtmcli.StatusSucceed:
		return nil
	case dtmcli.StatusFailed:
		return fmt.Errorf("%w: %v", ErrRolledBack, submitErr)
	default:
		// prepared, submitted, aborting: still moving. Not terminal, so not an
		// answer yet.
		return submitErr
	}
}

// ---------------------------------------------------------------- S2, pay

// PayGID is deterministic for the same reason S1's is, with a different
// duplicate to defeat: a double-clicked Pay button, or a client that retries on
// a timeout. Two Submit()s of pay-{orderID} and DTM runs the branches once.
func PayGID(orderID int64) string { return fmt.Sprintf("pay-%d", orderID) }

// SubmitPay runs S2: take the money, then mark the order paid.
//
// THE ORDERING IS THE DECISION. Balance first, order second -- deliberately the
// riskier-looking way round.
//
// Consider being killed between the two branches:
//
//	balance first: money gone, order still 'unpaid'. The user sees an unpaid
//	               order and a smaller balance. Visible, and DTM will either
//	               finish it or refund.
//	order first:   order says 'paid', money still in the account. The voucher is
//	               redeemable and nobody paid for it.
//
// The second is the one you cannot recover by waiting, so the money moves
// first. "Which inconsistency would you rather be caught in" is the question
// branch order answers.
//
// WaitResult is true here and false in S1, and that is not an inconsistency: a
// human is holding a browser open on this one.
func SubmitPay(ctx context.Context, e Endpoints, orderID, userID, amount int64) error {
	saga := dtmcli.NewSaga(e.DTMServer, PayGID(orderID)).
		Add(e.UserBase+"/deduct_balance", e.UserBase+"/deduct_balance_comp",
			map[string]any{"user_id": userID, "amount": amount, "order_id": orderID}).
		Add(e.OrderBase+"/pay_order", e.OrderBase+"/pay_order_comp",
			map[string]any{"order_id": orderID, "user_id": userID, "amount": amount})

	saga.RetryInterval = RetryInterval
	saga.WaitResult = true

	if err := saga.Submit(); err != nil {
		return classify(ctx, e.DTMServer, PayGID(orderID), err)
	}
	return nil
}

// ---------------------------------------------------------------- S3, expire

func ExpireGID(orderID int64) string { return fmt.Sprintf("expire-%d", orderID) }

// SubmitExpire runs S3: cancel the order, then give the stock back in Postgres
// AND Redis.
//
// Forward-only in the sense that matters: no branch after the first can fail,
// so nothing after the first can be unwound. The first CAN fail -- the order
// stopped being 'unpaid' while the sweep was deciding -- and unwinding the
// first branch of a Saga is an empty compensation, which the barrier handles
// without the branch knowing.
//
// WaitResult is FALSE. The sweeper submits a batch of these; blocking on each
// would make one slow Saga stall the whole tick, and nothing is waiting for the
// answer.
func SubmitExpire(ctx context.Context, e Endpoints, orderID, userID, voucherID int64) error {
	gid := ExpireGID(orderID)
	payload := map[string]any{
		"order_id": orderID, "user_id": userID, "voucher_id": voucherID,
	}

	saga := dtmcli.NewSaga(e.DTMServer, gid).
		Add(e.OrderBase+"/cancel_order", e.OrderBase+"/cancel_order_comp", payload).
		Add(e.VoucherBase+"/restore_for_expiry", e.VoucherBase+"/restore_for_expiry_comp", payload)

	saga.RetryInterval = RetryInterval

	if err := saga.Submit(); err != nil {
		// The sweeper re-selects any order still 'unpaid' on its next tick, so a
		// gid DTM already knows is the NORMAL case, not an error -- the previous
		// S3 is still working through its retries, or finished.
		//
		// ErrRolledBack is normal here too: it means cancel_order found the
		// order no longer 'unpaid' because the user paid it. Nothing to report.
		if cerr := classify(ctx, e.DTMServer, gid, err); cerr == nil || errors.Is(cerr, ErrRolledBack) {
			return nil
		}
		return err
	}
	return nil
}
