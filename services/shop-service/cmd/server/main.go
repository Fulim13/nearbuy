package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Fulim13/nearbuy/pkg/cache"
	"github.com/Fulim13/nearbuy/pkg/grpx"
	"github.com/Fulim13/nearbuy/pkg/kafkax"
	"github.com/Fulim13/nearbuy/pkg/util"
	"github.com/Fulim13/nearbuy/services/shop-service/internal/consumer"
	"github.com/Fulim13/nearbuy/services/shop-service/internal/handler"
	"github.com/Fulim13/nearbuy/services/shop-service/internal/service"
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

	port := util.Env("GRPC_PORT", "9001")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("listen", "port", port, "error", err)
		os.Exit(1)
	}

	cacheStore := cache.New(redis.NewClient(&redis.Options{
		Addr: util.Env("REDIS_ADDR", "localhost:6379"),
	}))

	brokers := strings.Split(util.Env("KAFKA_BROKERS", "localhost:9092"), ",")
	dlq := kafkax.NewProducer(brokers)
	defer dlq.Close()

	shopSvc := service.NewShopService(pool, cacheStore)
	orderStates := kafkax.NewConsumer(kafkax.ConsumerConfig{
		Brokers: brokers,
		Topic:   kafkax.TopicOrderState,
		// The group id is (service, purpose)
		GroupID: "shop-service.shop-stats",
	}, dlq)

	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		h := consumer.NewOrderStateHandler(shopSvc)
		if err := orderStates.Run(ctx, h.Handle); err != nil {
			slog.Error("order state consumer stopped", "error", err)
			stop()
		}
	}()

	grpcServer := grpc.NewServer(
		// Lifts x-user-id off the wire into the context, so handlers call
		// grpcx.UserIDFromContext and never touch metadata.
		grpc.ChainUnaryInterceptor(grpx.UnaryServerInterceptor()),
	)
	handler.NewGRPCHandler(grpcServer, service.NewShopService(pool, cacheStore))

	// Backs the k8s grpc: probe. Registered last so it only reports SERVING once
	// every real service is registered.
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Lets grpcurl work without -proto files. Dev convenience; drop it in prod.
	reflection.Register(grpcServer)

	go func() {
		slog.Info("shop-service listening", "addr", listener.Addr().String())
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

	// Wait for the consumer to finish its in-flight message and commit. Without
	// this the process can exit mid-handler, and that message redelivers on the
	// next start -- correct, thanks to applied_event, but noisy and avoidable.
	<-consumerDone
	slog.Info("stopped")
}
