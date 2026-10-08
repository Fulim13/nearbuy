package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Fulim13/nearbuy/pkg/grpx"
	"github.com/Fulim13/nearbuy/services/api-gateway/internal/session"
	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"
)

const (
	ctxSessionUser  = "session.user"
	ctxSessionToken = "session.token"
)

func SessionUser(c *echo.Context) (session.User, bool) {
	u, ok := c.Get(ctxSessionUser).(session.User)
	return u, ok
}

func SessionToken(c *echo.Context) string {
	t, _ := c.Get(ctxSessionToken).(string)
	return t
}

func Session(store *session.Store) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			token := bearer(c)
			if token == "" {
				return next(c)
			}

			u, err := store.Get(c.Request().Context(), token)
			if err != nil {
				if !errors.Is(err, redis.Nil) {
					slog.Error("session lookup", "error", err)
				}
				return next(c)
			}

			c.Set(ctxSessionUser, u)
			c.Set(ctxSessionToken, token)

			ctx := grpx.ContextWithUserID(c.Request().Context(), u.ID)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

func RequireAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if _, ok := SessionUser(c); !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}
			return next(c)
		}
	}
}

func bearer(c *echo.Context) string {
	h := c.Request().Header.Get("Authorization")
	if h == "" {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
}
