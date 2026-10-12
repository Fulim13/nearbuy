// cmd/sagademo submits either Saga by hand.
//
// Same purpose as cmd/kafkagen in lesson 09: a Saga you cannot start by hand is
// a Saga you cannot debug. This lets you drive one branch sequence at a time,
// with a gid you chose, and watch trans_global -- without a seckill, without
// Kafka, and without waiting for a consumer to rebalance.
//
//	go run ./cmd/sagademo -saga s1 -order-id 900001 -user 5 -voucher-id 2 -wait
//	go run ./cmd/sagademo -saga s1 -order-id 900001 -user 5 -voucher-id 2 -wait   # same gid: refused
//	go run ./cmd/sagademo -saga s1 -order-id 900002 -user 6 -voucher-id 2 -qty 100 -wait  # out of stock
//	go run ./cmd/sagademo -saga s2 -order-id 900001 -user 5 -wait                 # pay
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/dtm-labs/client/dtmcli"
)

func main() {
	var (
		which     = flag.String("saga", "s1", "which saga: s1 (seckill), s2 (pay)")
		dtmServer = flag.String("dtm", env("DTM_SERVER", "http://localhost:36789/api/dtmsvr"), "dtm server api base")
		orderBase = flag.String("order", env("ORDER_BRANCH_BASE", "http://localhost:8000/branch"), "order-service branch base url")
		vouchBase = flag.String("voucher", env("VOUCHER_BRANCH_BASE", "http://localhost:8000/branch"), "voucher-service branch base url")
		userBase  = flag.String("user-base", env("USER_BRANCH_BASE", "http://localhost:8000/branch"), "user-service branch base url")
		orderID   = flag.Int64("order-id", 0, "order id (also the gid)")
		userID    = flag.Int64("user", 0, "user id")
		voucherID = flag.Int64("voucher-id", 0, "voucher id")
		payValue  = flag.Int64("pay", 2000, "pay value in cents")
		qty       = flag.Int("qty", 1, "stock quantity to deduct")
		gidFlag   = flag.String("gid", "", "override the gid (default <saga>-<order-id>)")
		wait      = flag.Bool("wait", false, "block until the saga reaches a terminal state")
	)
	flag.Parse()

	if *orderID == 0 || *userID == 0 {
		log.Fatal("-order-id and -user are required")
	}
	if *which != "s2" && *voucherID == 0 {
		log.Fatal("-voucher-id is required for s1")
	}

	// Payloads as maps, not as the services' own structs: cmd/ sits outside
	// services/*/internal/ and Go will not let it import them. Which is the
	// right constraint -- a branch payload is a JSON contract, and a tool that
	// shared the server's struct would stop noticing when the two drift.
	var (
		gid  string
		saga *dtmcli.Saga
	)
	switch *which {
	case "s1":
		gid = defaultGid(*gidFlag, "seckill", *orderID)
		saga = dtmcli.NewSaga(*dtmServer, gid).
			Add(*orderBase+"/create_order", *orderBase+"/create_order_comp",
				map[string]any{
					"order_id": *orderID, "user_id": *userID,
					"voucher_id": *voucherID, "pay_value": *payValue,
				}).
			Add(*vouchBase+"/deduct_stock", *vouchBase+"/deduct_stock_comp",
				map[string]any{"voucher_id": *voucherID, "qty": *qty})

	case "s2":
		gid = defaultGid(*gidFlag, "pay", *orderID)
		pay := map[string]any{
			"order_id": *orderID, "user_id": *userID, "amount": *payValue,
		}
		// Balance FIRST. See internal/saga for why this order and not the other.
		saga = dtmcli.NewSaga(*dtmServer, gid).
			Add(*userBase+"/deduct_balance", *userBase+"/deduct_balance_comp", pay).
			Add(*orderBase+"/pay_order", *orderBase+"/pay_order_comp", pay)

	default:
		log.Fatalf("unknown -saga %q: want s1 or s2", *which)
	}

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

func defaultGid(override, prefix string, orderID int64) string {
	if override != "" {
		return override
	}
	return fmt.Sprintf("%s-%d", prefix, orderID)
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
