package handler

import (
	"net/http"

	"github.com/Fulim13/nearbuy/pkg/ratelimit"
	"github.com/Fulim13/nearbuy/pkg/util"
	mw "github.com/Fulim13/nearbuy/services/api-gateway/internal/middleware"
	"github.com/labstack/echo/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Routes(e *echo.Echo, limiter *ratelimit.Limiter, user *UserHandler, auth *AuthHandler) {
	e.GET("/healthz", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	perIP := mw.Tier{
		Name:       "per-ip",
		Key:        func(c *echo.Context) string { return "rl:ip:" + c.RealIP() },
		Capacity:   100,
		RatePerSec: 100.0 / 60.0,
	}

	api := e.Group("/api", mw.RateLimit(limiter, perIP))
	api.GET("/ping", ping)
	api.GET("/user/:id", user.getUser)
	api.POST("/user/code", auth.sendCode)
	api.POST("/user/login", auth.login)
}

func ping(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"message": "pong",
		"pod":     util.Env("POD_NAME", "unknown"),
	})
}

func grpcError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
	switch st.Code() {
	case codes.NotFound:
		return echo.NewHTTPError(http.StatusNotFound, st.Message())
	case codes.InvalidArgument:
		return echo.NewHTTPError(http.StatusBadRequest, st.Message())
	case codes.PermissionDenied:
		return echo.NewHTTPError(http.StatusForbidden, st.Message())
	case codes.Unauthenticated:
		return echo.NewHTTPError(http.StatusUnauthorized, st.Message())
	case codes.DeadlineExceeded:
		return echo.NewHTTPError(http.StatusGatewayTimeout, "upstream timeout")
	case codes.Unavailable:
		return echo.NewHTTPError(http.StatusServiceUnavailable, "upstream unavailable")
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
}
