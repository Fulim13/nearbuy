package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/Fulim13/nearbuy/pkg/grpx"
	"github.com/Fulim13/nearbuy/pkg/util"
	"github.com/Fulim13/nearbuy/services/voucher-service/internal/handler"
	"github.com/Fulim13/nearbuy/services/voucher-service/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
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

	rdb := redis.NewClient(&redis.Options{Addr: util.Env("REDIS_ADDR", "localhost:6379")})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		// Unlike shop-service, where Redis is only a cache, here it holds the
		// seckill counters and generates order ids. Without it this service
		// cannot do its job, so fail fast rather than report Ready.
		slog.Error("redis unreachable", "error", err)
		os.Exit(1)
	}

	svc := service.NewVoucherService(pool, rdb)

	// Warm the seckill counters from Postgres before accepting traffic. A Redis
	// restart would otherwise leave every running seckill with no stock key, and
	// the Lua admission would admit nobody until someone created a new voucher.
	if n, err := svc.WarmStock(ctx); err != nil {
		slog.Error("warm seckill stock", "error", err)
		os.Exit(1)
	} else {
		slog.Info("seckill stock warmed", "vouchers", n)
	}

	port := util.Env("GRPC_PORT", "9000")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("listen", "port", port, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpx.UnaryServerInterceptor()),
	)
	handler.NewGRPCHandler(grpcServer, svc)

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	reflection.Register(grpcServer)

	go func() {
		slog.Info("voucher-service listening", "addr", listener.Addr().String())
		if err := grpcServer.Serve(listener); err != nil {
			slog.Error("serve", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	grpcServer.GracefulStop()
}
