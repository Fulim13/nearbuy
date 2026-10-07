package handler

import (
	"net/http"

	"github.com/Fulim13/nearbuy/pkg/util"
	"github.com/labstack/echo/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Routes(e *echo.Echo, user *UserHandler) {
	e.GET("/healthz", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	api := e.Group("/api")
	api.GET("/ping", ping)
	api.GET("/user/:id", user.getUser)
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
