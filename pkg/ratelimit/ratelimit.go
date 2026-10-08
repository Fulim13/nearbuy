package ratelimit

import (
	"context"
	"time"

	_ "embed"

	"github.com/redis/go-redis/v9"
)

// The algorithm lives in Lua because the read-modify-write on the bucket has to
// be atomic. Do it in Go -- GET, compute, SET -- and the limiter leaks under
// exactly the concurrency it exists to stop.

//go:embed token_bucket.lua
var tokenBucketSrc string

var tokenBucketScript = redis.NewScript(tokenBucketSrc)

type Decision struct {
	Allowed    bool
	Remaining  int64
	RetryAfter time.Duration
}

type Limiter struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb}
}

func (l *Limiter) TokenBucket(ctx context.Context, key string, capacity, ratePerSec float64) (Decision, error) {
	res, err := tokenBucketScript.Run(ctx, l.rdb, []string{key}, capacity, ratePerSec, 1).Int64Slice()
	if err != nil {
		return Decision{}, err
	}
	return Decision{
		Allowed:    res[0] == 1,
		Remaining:  res[1],
		RetryAfter: time.Duration(res[2]) * time.Second,
	}, nil
}
