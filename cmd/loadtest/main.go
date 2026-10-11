// cmd/loadtest fires N simultaneous seckill attempts at one voucher and asserts
// the properties that make a seckill correct.
//
//	go run ./cmd/loadtest -voucher 2 -users 1000 -stock 100
//
// It talks gRPC to voucher-service directly, NOT HTTP through the gateway. That
// is deliberate: the gateway's tier-1 limiter  caps one IP at 100
// requests a minute, so a 1000-request burst through it measures the limiter, not
// the admission.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	voucherpb "github.com/Fulim13/nearbuy/gen/golang/v1/voucher"
	"github.com/Fulim13/nearbuy/pkg/grpx"
	"github.com/Fulim13/nearbuy/pkg/util"
)

func main() {
	var (
		target  = flag.String("addr", util.Env("VOUCHER_SERVICE_ADDR", "localhost:9000"), "voucher-service address")
		voucher = flag.Int64("voucher", 0, "seckill voucher id")
		users   = flag.Int("users", 1000, "distinct users, one attempt each")
		stock   = flag.Int("stock", 100, "expected stock, for the assertions")
		dupes   = flag.Int("dupes", 0, "extra attempts by user 1, to test one-per-user")
	)
	flag.Parse()
	if *voucher <= 0 {
		fmt.Fprintln(os.Stderr, "-voucher is required")
		os.Exit(2)
	}

	conn, err := grpx.Dial(*target)
	must(err)
	defer conn.Close()
	client := voucherpb.NewVoucherClient(conn)

	total := *users + *dupes
	type result struct {
		admitted bool
		orderID  int64
		code     voucherpb.SeckillCode
		err      error
	}
	results := make([]result, total)

	// Every goroutine blocks on one channel, so the burst is genuinely
	// simultaneous rather than ramping up as goroutines get scheduled.
	var ready, done sync.WaitGroup
	gun := make(chan struct{})
	ready.Add(total)
	done.Add(total)

	for i := 0; i < total; i++ {
		go func(i int) {
			defer done.Done()
			// user ids 1..users; the dupes all reuse user 1
			userID := int64(i + 1)
			if i >= *users {
				userID = 1
			}
			ready.Done()
			<-gun

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			resp, err := client.Seckill(ctx, &voucherpb.SeckillRequest{
				VoucherId: *voucher, UserId: userID,
			})
			if err != nil {
				results[i] = result{err: err}
				return
			}
			results[i] = result{
				admitted: resp.GetAdmitted(),
				orderID:  resp.GetOrderId(),
				code:     resp.GetCode(),
			}
		}(i)
	}

	ready.Wait()
	start := time.Now()
	close(gun)
	done.Wait()
	elapsed := time.Since(start)

	var admitted, failed atomic.Int64
	byCode := map[voucherpb.SeckillCode]int{}
	orderIDs := map[int64]int{}
	for _, r := range results {
		if r.err != nil {
			failed.Add(1)
			continue
		}
		byCode[r.code]++
		if r.admitted {
			admitted.Add(1)
			orderIDs[r.orderID]++
		}
	}

	dupOrders := 0
	for _, n := range orderIDs {
		if n > 1 {
			dupOrders++
		}
	}

	fmt.Printf("  attempts:        %d in %v (%.0f/s)\n", total,
		elapsed.Round(time.Millisecond), float64(total)/elapsed.Seconds())
	fmt.Printf("  rpc errors:      %d\n", failed.Load())
	fmt.Printf("  admitted:        %d   (expected %d)\n", admitted.Load(), *stock)
	fmt.Printf("  distinct orders: %d\n", len(orderIDs))
	fmt.Printf("  duplicate ids:   %d   <- must be 0\n", dupOrders)
	for code, n := range byCode {
		fmt.Printf("  %-32s %d\n", shortCode(code)+":", n)
	}

	ok := failed.Load() == 0 &&
		int(admitted.Load()) == *stock &&
		len(orderIDs) == *stock &&
		dupOrders == 0
	if ok {
		fmt.Println("  RESULT: PASS -- exactly the stock was admitted, no oversell, no duplicate ids")
	} else {
		fmt.Println("  RESULT: FAIL")
		os.Exit(1)
	}
}

func shortCode(c voucherpb.SeckillCode) string {
	return c.String()[len("SECKILL_CODE_"):]
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}
