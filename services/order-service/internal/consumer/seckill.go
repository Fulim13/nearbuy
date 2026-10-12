// Package consumer turns an admitted seckill into a Saga.
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Fulim13/nearbuy/pkg/dtmx"
	"github.com/Fulim13/nearbuy/pkg/kafkax"
	"github.com/Fulim13/nearbuy/services/order-service/internal/branch"
	"github.com/Fulim13/nearbuy/services/order-service/internal/saga"
	"github.com/dtm-labs/client/dtmcli"
	"github.com/segmentio/kafka-go"
)

// SeckillAdmitted mirrors what voucher-service publishes. Duplicated
// rather than shared: an event is a contract between services, and a shared Go
// struct would let one side change it without the other noticing at build time.
type SeckillAdmitted struct {
	OrderID   int64 `json:"order_id"`
	UserID    int64 `json:"user_id"`
	VoucherID int64 `json:"voucher_id"`
	ShopID    int64 `json:"shop_id"`
	PayValue  int64 `json:"pay_value"`
}

type SagaStarter struct {
	e saga.Endpoints
}

func NewSagaStarter(e saga.Endpoints) *SagaStarter { return &SagaStarter{e: e} }

// Handle starts Saga S1 for one admitted seckill.
func (s *SagaStarter) Handle(ctx context.Context, m kafka.Message) error {
	var ev SeckillAdmitted
	if err := json.Unmarshal(m.Value, &ev); err != nil {
		return fmt.Errorf("%w: unmarshal: %v", kafkax.ErrPermanent, err)
	}
	if ev.OrderID == 0 || ev.UserID == 0 || ev.VoucherID == 0 {
		return fmt.Errorf("%w: order_id, user_id and voucher_id are required", kafkax.ErrPermanent)
	}

	// The gid comes from the order ID, NOT a random ID.
	//
	// Why: Kafka delivers at least once, so this same event can arrive twice
	// (e.g. we submit the Saga, then crash before committing the offset).
	//
	// With a random gid, each delivery starts a NEW Saga:
	//   delivery 1 -> gid=abc -> stock -1
	//   delivery 2 -> gid=xyz -> stock -1 again   <- one order, two deductions
	//
	// The DTM barrier can't stop this. It dedupes on (gid, branch_id, op,
	// barrier_id), so a different gid looks like a different transaction.
	//
	// With gid = "seckill-<order_id>", both deliveries share one gid and DTM
	// rejects the second Submit(). (Handled below as success, not an error.)
	gid := fmt.Sprintf("seckill-%d", ev.OrderID)

	payload := branch.CreateOrderReq{
		OrderID:   ev.OrderID,
		UserID:    ev.UserID,
		VoucherID: ev.VoucherID,
		PayValue:  ev.PayValue,
	}
	deduct := map[string]any{"voucher_id": ev.VoucherID, "qty": 1}

	// Step order matters: DTM runs branches in the order they are Added(), one after another.
	// 1. create_order  - cheap, easy to cancel
	// 2. deduct_stock  - takes the scarce resource, so it goes LAST
	//
	// Sold out? deduct_stock fails, DTM cancels the order, and stock was
	// never touched.
	//
	// If stock went first and create_order hit a transient error, DTM would
	// retry the insert every RetryInterval while the stock sat spent with no
	// order behind it. Other buyers would see "sold out" for an item nobody
	// bought.
	//
	// Do NOT use SetConcurrent(): running both in parallel loses this ordering.
	saga := dtmcli.NewSaga(s.e.DTMServer, gid).
		Add(s.e.OrderBase+"/create_order", s.e.OrderBase+"/create_order_comp", payload).
		Add(s.e.VoucherBase+"/deduct_stock", s.e.VoucherBase+"/deduct_stock_comp", deduct)

	// Wait 10s between retries when a branch fails with a transient error (timeout, 5xx, network). 10 is the minimum DTM accepts.
	//
	// Side effect: compensation can't start until a retry gets a final
	// FAILURE, so it begins at least 10s after a transient error:
	//
	//   00s  deduct_stock -> timeout   (transient, DTM waits)
	//   10s  deduct_stock -> FAILURE   (sold out)
	//   10s  create_order_comp runs    (order cancelled)
	//
	// So a Saga that sits unfinished for ~10s is probably just waiting for
	// its next retry, not stuck. Minutes of no progress is a real problem.
	saga.RetryInterval = 10

	// We do NOT wait for the Saga to finish.
	// Why: Kafka handles messages one by one. If we wait here, all other
	// messages must wait too. And the user already got a reply (202),
	// so nobody needs the result.
	if err := saga.Submit(); err != nil {

		// Submit can fail for two reasons. We check which one.

		// Reason 1: we already submitted this Saga before.
		// Kafka can send the same message twice. The second time, DTM says
		// "I already have this gid" and returns an error.
		// But the work is already done, so this is OK. Return nil.
		// (If we return the error, Kafka keeps retrying and finally puts
		// this good message in the dead-letter queue by mistake.)
		if st, qerr := dtmx.QueryTrans(ctx, s.e.DTMServer, gid); qerr == nil && st != nil {
			slog.Info("saga already exists, acking redelivery",
				"gid", gid, "status", st.Status)
			return nil
		}

		// Reason 2: the submit really failed (for example, DTM is down).
		// DTM never got the Saga, so nothing happened yet.
		// Return the error so Kafka tries again later.
		slog.Error("saga submit failed", "gid", gid, "error", err)
		return err
	}

	slog.Info("saga submitted", "gid", gid, "order_id", ev.OrderID,
		"voucher_id", ev.VoucherID, "trace_id", kafkax.Header(m, kafkax.HeaderTraceID))
	return nil
}
