package middleware

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Fulim13/nearbuy/pkg/ratelimit"
	"github.com/labstack/echo/v5"
)

type Tier struct {
	Name       string
	Key        func(c *echo.Context) string // "" means this tier does not apply
	Capacity   float64
	RatePerSec float64
}

func RateLimit(l *ratelimit.Limiter, tiers ...Tier) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			for _, t := range tiers {
				key := t.Key(c)
				if key == "" {
					continue
				}

				d, err := l.TokenBucket(c.Request().Context(), key, t.Capacity, t.RatePerSec)
				if err != nil {
					slog.Warn("rate limiter unavailable, failing open", "tier", t.Name, "key", key, "error", err)
					continue
				}
				c.Response().Header().Set("X-RateLimit-Limit", strconv.FormatFloat(t.Capacity, 'f', -1, 64))
				c.Response().Header().Set("X-RateLimit-Remaining", strconv.FormatInt(d.Remaining, 10))

				if !d.Allowed {
					retry := int(d.RetryAfter.Seconds())
					if retry < 1 {
						retry = 1
					}
					c.Response().Header().Set("Retry-After", strconv.Itoa(retry))
					return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded: "+t.Name)
				}
			}
			return next(c)
		}
	}
}

// Sometimes you can't rate-limit in middleware because the key is only known inside the handler.
// A common case is "max 1 SMS code per phone number per minute": the phone number is in the request body, which the handler parses.
func TooManyRequests(c *echo.Context, d ratelimit.Decision, name string, capacity float64) error {
	retry := int(d.RetryAfter.Seconds())
	if retry < 1 {
		retry = 1
	}
	c.Response().Header().Set("Retry-After", strconv.Itoa(retry))
	c.Response().Header().Set("X-RateLimit-Limit", strconv.FormatFloat(capacity, 'f', -1, 64))
	c.Response().Header().Set("X-RateLimit-Remaining", strconv.FormatInt(0, 10))
	return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded: "+name)
}
