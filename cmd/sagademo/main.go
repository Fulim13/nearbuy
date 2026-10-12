// cmd/sagademo submits Saga S1 by hand.
//
// a Saga you cannot start by hand is
// a Saga you cannot debug. This lets you drive one branch sequence at a time,
// with a gid you chose, and watch trans_global -- without a seckill, without
// Kafka, and without waiting for a consumer to rebalance.
//
//	go run ./cmd/sagademo -order-id 900001 -user 5 -voucher-id 2 -wait
//	go run ./cmd/sagademo -order-id 900001 -user 5 -voucher-id 2 -wait   # same gid: refused
//	go run ./cmd/sagademo -order-id 900002 -user 6 -voucher-id 2 -qty 100 -wait  # out of stock
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Fulim13/nearbuy/pkg/util"
	"github.com/dtm-labs/client/dtmcli"
)

func main() {
	var (
		dtmServer = flag.String("dtm", util.Env("DTM_SERVER", "http://localhost:36789/api/dtmsvr"), "dtm server api base")
		orderBase = flag.String("order", util.Env("ORDER_BRANCH_BASE", "http://localhost:8000/branch"), "order-service branch base url")
		vouchBase = flag.String("voucher", util.Env("VOUCHER_BRANCH_BASE", "http://localhost:8000/branch"), "voucher-service branch base url")
		orderID   = flag.Int64("order-id", 0, "order id (also the gid)")
		userID    = flag.Int64("user", 0, "user id")
		voucherID = flag.Int64("voucher-id", 0, "voucher id")
		payValue  = flag.Int64("pay", 2000, "pay value in cents")
		qty       = flag.Int("qty", 1, "stock quantity to deduct")
		gidFlag   = flag.String("gid", "", "override the gid (default seckill-<order-id>)")
		wait      = flag.Bool("wait", false, "block until the saga reaches a terminal state")
	)
	flag.Parse()

	if *orderID == 0 || *userID == 0 || *voucherID == 0 {
		log.Fatal("-order-id, -user and -voucher-id are required")
	}

	gid := *gidFlag
	if gid == "" {
		gid = fmt.Sprintf("seckill-%d", *orderID)
	}

	saga := dtmcli.NewSaga(*dtmServer, gid).
		// Payloads as maps, not as the services' own structs: cmd/ sits outside
		// services/*/internal/ and Go will not let it import them. Which is the
		// right constraint -- a branch payload is a JSON contract, and a tool
		// that shares the server's struct would stop noticing when the two
		// drift.
		Add(*orderBase+"/create_order", *orderBase+"/create_order_comp",
			map[string]any{
				"order_id": *orderID, "user_id": *userID,
				"voucher_id": *voucherID, "pay_value": *payValue,
			}).
		Add(*vouchBase+"/deduct_stock", *vouchBase+"/deduct_stock_comp",
			map[string]any{"voucher_id": *voucherID, "qty": *qty})

	saga.WaitResult = *wait

	fmt.Printf("submitting gid=%s\n", gid)
	if err := saga.Submit(); err != nil {
		// A duplicate gid lands here, and that is the point of the demo: DTM
		// refuses the second submission instead of running the branches again.
		fmt.Printf("submit error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("submitted")
}
