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

	port := util.Env("GRPC_PORT", "9000")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("listen", "port", port, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpx.UnaryServerInterceptor()),
	)
	handler.NewGRPCHandler(grpcServer, service.NewUserService(pool))

	// Backs the k8s grpc: probe. Registered last so it only reports SERVING once
	// every real service is registered.
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Lets grpcurl work without -proto files. Dev convenience; drop it in prod.
	reflection.Register(grpcServer)

	go func() {
		slog.Info("user-service listening", "addr", listener.Addr().String())
		if err := grpcServer.Serve(listener); err != nil {
			slog.Error("serve", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	// Mark NOT_SERVING first so the readiness probe fails and kube-proxy stops
	// sending new work before we stop accepting it.
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	grpcServer.GracefulStop()
}
