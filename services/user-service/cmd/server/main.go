package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Fulim13/nearbuy/pkg/dtmx"
	"github.com/Fulim13/nearbuy/pkg/grpx"
	"github.com/Fulim13/nearbuy/pkg/util"
	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"

	"github.com/Fulim13/nearbuy/services/user-service/internal/branch"
	"github.com/Fulim13/nearbuy/services/user-service/internal/handler"
	"github.com/Fulim13/nearbuy/services/user-service/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, os.Getenv("POSTGRES_DSN"))
	if err != nil {
		slog.Error("postgres pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		slog.Error("postgres unreachable", "error", err)
		os.Exit(1)
	}

	sqlDB := dtmx.SQLDB(pool)
	defer sqlDB.Close()

	rdb := redis.NewClient(&redis.Options{Addr: util.Env("REDIS_ADDR", "localhost:6379")})
	defer rdb.Close()

	port := util.Env("GRPC_PORT", "9000")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("listen", "port", port, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpx.UnaryServerInterceptor()),
	)
	handler.NewGRPCHandler(
		grpcServer,
		service.NewUserService(pool),
		service.NewFollowService(pool, rdb),
	)

	// Backs the k8s grpc: probe. Registered last so it only reports SERVING once
	// every real service is registered.
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Lets grpcurl work without -proto files. Dev convenience; drop it in prod.
	reflection.Register(grpcServer)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("user-service listening", "addr", listener.Addr().String())
		if err := grpcServer.Serve(listener); err != nil {
			slog.Error("serve", "error", err)
			stop()
		}
	}()

	// Second listener: S2's deduct_balance branch. user-service is the third
	// service to grow one, and the pattern is identical in all three.
	e := echo.New()
	e.GET("/healthz", branch.Health)
	branch.Register(e, branch.New(sqlDB, service.SQLStore{}))

	branchPort := util.Env("BRANCH_PORT", "8000")
	wg.Add(1)
	go func() {
		defer wg.Done()
		sc := echo.StartConfig{
			Address:         ":" + branchPort,
			GracefulTimeout: 10 * time.Second,
			HideBanner:      true,
			OnShutdownError: func(err error) {
				slog.Error("branch listener shutdown failed", "error", err)
			},
		}
		slog.Info("user-service branch listener", "port", branchPort)
		if err := sc.Start(ctx, e); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("branch serve", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	// Mark NOT_SERVING first so the readiness probe fails and kube-proxy stops
	// sending new work before we stop accepting it.
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	grpcServer.GracefulStop()
	wg.Wait()
	slog.Info("stopped")
}
