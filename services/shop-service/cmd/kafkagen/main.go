package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Fulim13/nearbuy/pkg/kafkax"
	"github.com/Fulim13/nearbuy/pkg/util"
	"github.com/google/uuid"
)

// cmd/kafkagen publishes order.state_changed events by hand.
//
//
//	go run ./cmd/kafkagen -order 1001 -shop 1 -status paid
//	go run ./cmd/kafkagen -order 1001 -shop 1 -status paid        # duplicate
//	go run ./cmd/kafkagen -raw 'not json'                         # poison

func main() {
	var (
		brokers = flag.String("brokers", util.Env("KAFKA_BROKERS", "localhost:9092"), "comma-separated brokers")
		order   = flag.Int64("order", 0, "order id")
		user    = flag.Int64("user", 1, "user id")
		voucher = flag.Int64("voucher", 1, "voucher id")
		shop    = flag.Int64("shop", 1, "shop id")
		status  = flag.String("status", "paid", "order status")
		raw     = flag.String("raw", "", "send this literal payload instead (to test the DLQ)")
		count   = flag.Int("count", 1, "how many to send")
		trace   = flag.String("trace", "", "trace id (generated when empty)")
	)
	flag.Parse()

	p := kafkax.NewProducer(strings.Split(*brokers, ","))
	defer p.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	traceID := *trace
	if traceID == "" {
		traceID = uuid.NewString()
	}

	for i := 0; i < *count; i++ {
		id := *order
		if id == 0 {
			id = time.Now().UnixNano()
		} else if *count > 1 {
			id += int64(i)
		}

		var payload []byte
		key := fmt.Sprint(id)
		if *raw != "" {
			payload = []byte(*raw)
		} else {
			payload, _ = json.Marshal(map[string]any{
				"order_id":   id,
				"user_id":    *user,
				"voucher_id": *voucher,
				"shop_id":    *shop,
				"status":     *status,
			})
		}

		// Keyed by order_id: every transition for one order lands on one
		// partition, so paid-then-refunded can never be observed out of order.
		if err := p.Publish(ctx, kafkax.TopicOrderState, key, payload, traceID); err != nil {
			fmt.Fprintln(os.Stderr, "publish failed:", err)
			os.Exit(1)
		}
		fmt.Printf("published order_id=%s status=%s trace_id=%s\n", key, *status, traceID)
	}
}
