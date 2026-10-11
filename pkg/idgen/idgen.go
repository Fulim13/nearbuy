package idgen

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Package idgen produces globally unique, roughly time-ordered int64 ids.
//
// Why not a Postgres IDENTITY column:
//
//   - it LEAKS VOLUME. Buy two vouchers an hour apart, subtract the ids, and you
//     know how many orders the business took in that hour.
//   - it cannot be generated OUTSIDE a transaction,we need to hands the
//     client an orderId in a 202 before any row exists. This is the decisive
//     one -- IDENTITY was never an option, independent of taste.
//   - it does not survive sharding: two shards both counting from 1 collide.
//
// The layout is the original's, and it is a reasonable one:
//
//	 63        62 .............. 32  31 ................ 0
//	┌──┬──────────────────────────┬────────────────────────┐
//	│ 0│ seconds since the epoch  │ counter within the day │
//	└──┴──────────────────────────┴────────────────────────┘
//	   sign      31 bits                   32 bits
//
// Sortable by time, unique by the counter, and the counter resets daily so it
// never approaches its 32-bit ceiling in practice.

// beginEpoch is 2025-01-01T00:00:00Z.
const beginEpoch int64 = 1735689600

// counterBits is how much of the id the per-day counter occupies.
const counterBits = 32

// maxCounter is the ceiling before the counter would overflow into the timestamp
// and produce an id in the future. 4.29 billion ids in one day.
const maxCounter int64 = 1<<counterBits - 1

type Generator struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Generator {
	return &Generator{rdb: rdb}
}

// NextID returns the next id for a namespace ("order", "voucher", ...).
func (g *Generator) NextID(ctx context.Context, namespace string) (int64, error) {
	now := time.Now().UTC()

	// The counter key is per DAY, so it resets before it can grow large. UTC, not
	// local time: a server in a different zone must not get a different key for
	// the same instant, or two of them can hand out the same counter value.
	key := fmt.Sprintf("icr:%s:%s", namespace, now.Format("2006:01:02"))

	count, err := g.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("idgen incr: %w", err)
	}

	// Expire the counter two days out. Two days rather than one so a
	// clock skew or a long-running request near midnight cannot expire a counter
	// that is still in use.
	if count == 1 {
		if err := g.rdb.Expire(ctx, key, 48*time.Hour).Err(); err != nil {
			return 0, fmt.Errorf("idgen expire: %w", err)
		}
	}

	if count > maxCounter {
		// Refusing is the only safe answer: continuing would silently produce an
		// id whose timestamp bits are wrong, i.e. an id from the future that
		// sorts incorrectly and may collide tomorrow.
		return 0, fmt.Errorf("idgen: counter overflow for %s (%d > %d)", key, count, maxCounter)
	}

	timestamp := now.Unix() - beginEpoch
	return timestamp<<counterBits | count, nil
}

// Decode splits an id back into its parts.
func Decode(id int64) (issuedAt time.Time, counter int64) {
	counter = id & maxCounter
	seconds := id >> counterBits
	return time.Unix(seconds+beginEpoch, 0).UTC(), counter
}


// WORKED EXAMPLE
//  ┌─────────────────┬─────────────────┐
//  │  timestamp (4)  │   counter (4)   │
//  └─────────────────┴─────────────────┘
//    bit 7 ...... 4    bit 3 ...... 0

// Inputs: timestamp = 12 (1100), count = 6 (0110)

// Encode: timestamp<<counterBits | count
// Step 1: Shift the timestamp left by 4
// timestamp       =           1100      (12)
//                             ↓ << 4
// timestamp << 4  =  1100 0000          (192)
//                    ▲▲▲▲ ▲▲▲▲
//                    time  empty (all zeros, ready for the counter)
// Step 2: OR the counter into the empty low bits
//                    1100 0000          (192)
//           OR       0000 0110          (6)
//                    ─────────
// id              =  1100 0110          (198)
//                    ╰─┬─╯╰─┬─╯
//                   time=12  count=6


// Decode
// Get the counter: id & maxCounter
// id               =  1100 0110         (198)
//           AND       0000 1111         (maxCounter = 15, the "mask")
//                     ─────────
// counter          =  0000 0110         (6)  ✅

// AND keeps a bit only where both sides are 1. The mask has 1s only in the counter slot, so it erases the timestamp and leaves the counter.

// Get the timestamp: id >> counterBits
// id               =  1100 0110         (198)
//                     ↓ >> 4   (the low 4 bits fall off the right end)
// seconds          =       1100         (12)  ✅

// Then time.Unix(seconds + beginEpoch, 0) turns 12 back into a real date.
