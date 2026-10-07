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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/redis/go-redis/v9"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rdb := redis.NewClient(&redis.Options{
		Addr: env("REDIS_ADDR", "localhost:6379"),
	})
	defer rdb.Close()

	pool, err := pgxpool.New(ctx, os.Getenv("POSTGRES_DSN"))
	if err != nil {
		slog.Error("pogtres pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	e.GET("/healthz", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	api := e.Group("/api")

	api.GET("/ping", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "pong v2",
			"pod":     env("POD_NAME", "unknown"),
		})
	})

	api.GET("/health/deps", func(c *echo.Context) error {
		reqCtx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()

		out := map[string]string{"redis": "ok", "postgres": "ok"}
		if err := rdb.Ping(reqCtx).Err(); err != nil {
			out["redis"] = err.Error()
		}

		if err := pool.Ping(reqCtx); err != nil {
			out["postgres"] = err.Error()
		}
		if out["redis"] != "ok" || out["postgres"] != "ok" {
			return c.JSON(http.StatusServiceUnavailable, out)
		}
		return c.JSON(http.StatusOK, out)
	})

	sc := echo.StartConfig{
		Address:         ":" + env("PORT", "8080"),
		GracefulTimeout: 10 * time.Second,
	}

	slog.Info("api-gateway listening", "port", env("PORT", "8080"))

	// Blocks until ctx is cancelled, then drains in-flight requests for up to GracefulTimeout.
	if err := sc.Start(ctx, e); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "error", err)
		stop()
		pool.Close()
		_ = rdb.Close()
		os.Exit(1)
	}

	slog.Info("shut down cleanly")
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
