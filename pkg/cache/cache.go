// Package cache implements cache-aside over Redis with negative caching.
//
// The problem it solves is CACHE PENETRATION: requests for ids that do not exist
// find nothing in the cache, so every one of them reaches the database. A single
// script walking /api/shops/1..1000000 becomes a million Postgres queries, and no
// amount of caching real rows helps, because the rows are not real.
//
// The fix is to cache the ABSENCE as deliberately as the presence. That requires
// three states where a naive implementation has two:
//
//	key missing            -> not cached, go and look
//	key holds the sentinel -> cached MISS, the row does not exist, do not look
//	key holds JSON         -> cached HIT
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrNotFound = errors.New("not found")

// sentinel marks "this id does not exist".
const sentinel = "__nil__"

type Cache struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Cache {
	return &Cache{rdb: rdb}
}

func (c *Cache) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// Loader fetches the authoritative value from db. It must return ErrNotFound (not a zero
// value, not a nil error) when the row genuinely does not exist -- that is the
// signal that makes negative caching possible.
type Loader[T any] func(ctx context.Context) (T, error)

func GetOrLoad[T any](ctx context.Context, c *Cache, key string, ttl, nilTTL time.Duration, load Loader[T]) (T, error) {
	var zero T

	raw, err := c.rdb.Get(ctx, key).Result()
	switch {
	case err == nil && raw == sentinel:
		return zero, ErrNotFound
	case err == nil:
		var v T
		if jsonErr := json.Unmarshal([]byte(raw), &v); jsonErr == nil {
			return v, nil
		}
		// Corrupt payload: drop it and fall through to the loader rather than
		// serving garbage or failing the request.
		_ = c.rdb.Del(ctx, key).Err()
	case !errors.Is(err, redis.Nil):
		// Redis is down. Fall through to the database -
	}

	v, err := load(ctx)
	if errors.Is(err, ErrNotFound) {
		_ = c.rdb.Set(ctx, key, sentinel, nilTTL).Err()
		return zero, ErrNotFound
	}

	if err != nil {
		return zero, err
	}

	if payload, marshalErr := json.Marshal(v); marshalErr == nil {
		_ = c.rdb.Set(ctx, key, payload, ttl).Err()
	}
	return v, nil
}
