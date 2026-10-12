package dtmx

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/dtm-labs/client/dtmcli"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v5"
)

const BarrierTable = "dtm_barrier.barrier"

// SQLDB bridges a pgx pool into the *sql.DB dtmcli requires, sharing the pool
// rather than opening a second one.
func SQLDB(pool *pgxpool.Pool) *sql.DB {
	return stdlib.OpenDBFromPool(pool)
}

// BarrierFromEcho builds the barrier from the query string DTM appends to every
// branch callback:
//
//	/branch/create_order?gid=...&trans_type=saga&branch_id=01&op=action
//
// Those four fields ARE the barrier key. The insert of that key inside the
// business transaction is what makes a branch idempotent, and what detects an
// empty compensation (a compensate whose action never ran).
func BarrierFromEcho(c *echo.Context) (*dtmcli.BranchBarrier, error) {
	bb, err := dtmcli.BarrierFromQuery(c.Request().URL.Query())
	if err != nil {
		return nil, err
	}
	// Without this the barrier SQL is built with MySQL placeholders and every
	// branch fails with a syntax error that mentions "$1" vs "?".
	bb.DBType = dtmcli.DBTypePostgres
	bb.BarrierTableName = BarrierTable
	return bb, nil
}

// WrapBranch turns a branch handler's return value into the response DTM reads.
//
// A branch returns exactly one of:
//
//	nil                -> 200, the branch succeeded, DTM moves to the next
//	dtmcli.ErrFailure  -> 409, failed for good; DTM compensates the whole saga
//	dtmcli.ErrOngoing  -> 425, not done yet; DTM retries after RetryInterval
//	anything else      -> 500, which DTM also treats as "retry later"
//
// The distinction between ErrFailure and ErrOngoing is the single most important
// decision in a branch: the first unwinds the transaction, the second keeps
// trying. Returning ErrFailure for a transient error throws away work that would
// have succeeded; returning ErrOngoing for a permanent one retries forever.
func WrapBranch(fn func(*echo.Context) any) echo.HandlerFunc {
	return func(c *echo.Context) error {
		result := fn(c)
		if result == nil {
			result = dtmcli.MapSuccess
		}
		return c.JSON(dtmcli.Result2HttpJSON(result))
	}
}

// BranchQuery is the query string DTM appends when it calls a branch. Exposed so
// a test or a manual curl can build one.
func BranchQuery(transType, gid, branchID, op string) string {
	return url.Values{
		"trans_type": {transType},
		"gid":        {gid},
		"branch_id":  {branchID},
		"op":         {op},
	}.Encode()
}

// ---------------------------------------------------------------- query

// TransStatus is a global transaction's status as DTM reports it: prepared,
// submitted, succeed, failed or aborting.
type TransStatus struct {
	Gid    string `json:"gid"`
	Status string `json:"status"`
}

type queryResponse struct {
	// null when DTM has never heard of the gid, which is the fact we are after.
	Transaction *TransStatus `json:"transaction"`
}

// QueryTrans asks DTM whether it knows a global transaction (gid), and if so,
// what state it is in.
//
// Returns:
//
//	status, nil  -> DTM has this gid; status.Status is e.g. "submitted" or "succeed"
//	nil, nil     -> DTM has never seen this gid (not an error)
//	nil, err     -> DTM could not be asked (network error, bad response)
//
// # Why this exists
//
// It has one caller: the Kafka consumer, after Submit() returns an error.
//
// The consumer builds each saga's gid from the Kafka event, so the same event
// always produces the same gid. Kafka redelivers an event if the consumer did
// not ACK it, for example after a crash between Submit() succeeding and the
// ACK being sent. On redelivery the consumer submits the same gid again, and
// DTM rejects it as a duplicate. That rejection is correct: it stops one event
// from starting two sagas.
//
// But Submit() also fails when the saga was never created at all (DTM down,
// network error). The consumer must tell the two apart, because they need
// opposite handling:
//
//	gid already exists in DTM  -> the first attempt worked; ACK the event
//	gid not in DTM             -> nothing was created; do not ACK, retry
//	query itself failed        -> unknown; do not ACK, retry
//
// Getting it wrong either loses the event (ACK when nothing was created) or
// blocks the consumer forever (retrying a duplicate DTM will always reject).
//
// # Why not inspect the Submit() error
//
// For a duplicate, DTM returns an error wrapping dtmcli.ErrFailure with a
// message like "current status 'succeed', cannot sumbmit" (the typo is
// DTM's). But ErrFailure alone does not prove a duplicate, and matching the
// message text would silently stop working if DTM ever changed the wording.
// Asking DTM directly gives the answer without depending on error text.
func QueryTrans(ctx context.Context, server, gid string) (*TransStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server+"/query", nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("gid", gid)
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dtm query %s: status %d", gid, resp.StatusCode)
	}

	var out queryResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("dtm query %s: %w", gid, err)
	}
	// nil, nil means "DTM has no such transaction" -- not an error.
	return out.Transaction, nil
}
