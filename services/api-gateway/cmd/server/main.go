package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Fulim13/nearbuy/pkg/util"
	"github.com/Fulim13/nearbuy/services/api-gateway/internal/client"
	"github.com/Fulim13/nearbuy/services/api-gateway/internal/handler"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/redis/go-redis/v9"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rdb := redis.NewClient(&redis.Options{
		Addr: util.Env("REDIS_ADDR", "localhost:6379"),
	})
	defer rdb.Close()

	userClient, err := client.NewUserClient(util.Env("USER_SERVICE_ADDR", "user-service:9000"))
	if err != nil {
		slog.Error("user client", "error", err)
		os.Exit(1)
	}
	defer userClient.Close()

	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	handler.Routes(e, handler.NewUserHandler(userClient))

	e.GET("/api/health/deps", func(c *echo.Context) error {
		reqCtx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()

		out := map[string]string{"redis": "ok"}
		if err := rdb.Ping(reqCtx).Err(); err != nil {
			out["redis"] = err.Error()
			return c.JSON(http.StatusServiceUnavailable, out)
		}
		return c.JSON(http.StatusOK, out)
	})

	sc := echo.StartConfig{
		Address:         ":" + util.Env("PORT", "8080"),
		GracefulTimeout: 10 * time.Second,
		HideBanner:      true,
		OnShutdownError: func(err error) {
			slog.Error("graceful shutdown failed", "error", err)
		},
	}

	slog.Info("api-gateway listening", "port", util.Env("PORT", "8080"))
	if err := sc.Start(ctx, e); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
	slog.Info("stopped")
}
