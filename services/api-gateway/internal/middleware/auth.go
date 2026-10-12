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
			ctx = grpx.ContextWithUserRole(ctx, u.Role)
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

// RequireAdmin guards the catalogue writes: shops and vouchers.
func RequireAdmin() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			u, ok := SessionUser(c)
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}
			if !u.IsAdmin() {
				return echo.NewHTTPError(http.StatusForbidden, "admin only")
			}
			return next(c)
		}
	}
}

// RequireCustomer keeps admins out of the shopper actions: grabbing, paying,
// following. An admin account runs the catalogue; it does not shop.
func RequireCustomer() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			u, ok := SessionUser(c)
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}
			if u.IsAdmin() {
				return echo.NewHTTPError(http.StatusForbidden, "admins cannot do this")
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
