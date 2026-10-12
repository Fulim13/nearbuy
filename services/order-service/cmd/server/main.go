package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Fulim13/nearbuy/gen/golang/v1/voucher"
	"github.com/Fulim13/nearbuy/pkg/dtmx"
	"github.com/Fulim13/nearbuy/pkg/grpx"
	"github.com/Fulim13/nearbuy/pkg/kafkax"
	"github.com/Fulim13/nearbuy/pkg/util"
	"github.com/Fulim13/nearbuy/services/order-service/internal/branch"
	"github.com/Fulim13/nearbuy/services/order-service/internal/consumer"
	"github.com/Fulim13/nearbuy/services/order-service/internal/handler"
	"github.com/Fulim13/nearbuy/services/order-service/internal/saga"
	"github.com/Fulim13/nearbuy/services/order-service/internal/scheduler"
	"github.com/Fulim13/nearbuy/services/order-service/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// order-service is the first service in this project with THREE listeners:
//
//	:9000  gRPC   -- reads, for the gateway
//	:8000  HTTP   -- DTM's Saga branch callbacks
//	(none) Kafka  -- the seckill.admitted consumer that starts the Saga
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

	// ONE pool, two APIs. dtmcli's barrier is written against database/sql, the
	// repository against pgx. OpenDBFromPool wraps the pool we already have
	// rather than dialling again
	sqlDB := dtmx.SQLDB(pool)
	defer sqlDB.Close()

	// One Endpoints value, built once, shared by all three Sagas. The branch
	// URLs are stored per branch in trans_branch_op at Submit() time, so a
	// transaction in flight keeps calling the URL it was submitted with --
	// changing one of these does not retarget work already running.
	endpoints := saga.Endpoints{
		DTMServer:   util.Env("DTM_SERVER", "http://dtm:36789/api/dtmsvr"),
		OrderBase:   util.Env("ORDER_BRANCH_BASE", "http://order-service:8000/branch"),
		VoucherBase: util.Env("VOUCHER_BRANCH_BASE", "http://voucher-service:8000/branch"),
		UserBase:    util.Env("USER_BRANCH_BASE", "http://user-service:8000/branch"),
	}
	starter := consumer.NewSagaStarter(endpoints)

	svc := service.NewOrderService(pool, endpoints)

	var wg sync.WaitGroup

	// ------------------------------------------------------------- gRPC :9000
	grpcPort := util.Env("GRPC_PORT", "9000")
	listener, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		slog.Error("listen", "port", grpcPort, "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpx.UnaryServerInterceptor()),
	)
	brokers := strings.Split(util.Env("KAFKA_BROKERS", "localhost:9092"), ",")
	producer := kafkax.NewProducer(brokers)
	defer producer.Close()
	handler.NewGRPCHandler(grpcServer, svc, producer)

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	reflection.Register(grpcServer)

	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("order-service grpc listening", "addr", listener.Addr().String())
		if err := grpcServer.Serve(listener); err != nil {
			slog.Error("grpc serve", "error", err)
			stop()
		}
	}()

	// ------------------------------------------- DTM branch callbacks :8000
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
		slog.Info("order-service branch listener", "port", branchPort)
		if err := sc.Start(ctx, e); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("branch serve", "error", err)
			stop()
		}
	}()

	// -------------------------------------------- seckill.admitted consumer
	dlq := kafkax.NewProducer(brokers)
	defer dlq.Close()

	cons := kafkax.NewConsumer(kafkax.ConsumerConfig{
		Brokers: brokers,
		Topic:   kafkax.TopicSeckillAdmitted,
		// (service, purpose).
		GroupID:     "order-service.saga-starter",
		MaxAttempts: 3,
	}, dlq)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := cons.Run(ctx, starter.Handle); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("consumer stopped", "error", err)
			stop()
		}
	}()

	// ------------------------------------------------------ expiry scheduler
	//
	// Every replica runs this goroutine. Cancelling skips rows another replica
	// has locked, and RestoreStock is idempotent per order, so running it
	// everywhere is safe.
	voucherConn, err := grpx.Dial(util.Env("VOUCHER_SERVICE_ADDR", "voucher-service:9000"))
	if err != nil {
		slog.Error("voucher client", "error", err)
		os.Exit(1)
	}
	defer voucherConn.Close()

	expiry := scheduler.NewExpiry(pool, voucher.NewVoucherClient(voucherConn), scheduler.Config{
		Interval: envDuration("EXPIRY_INTERVAL", 30*time.Second),
		Expiry:   envDuration("ORDER_EXPIRY", 15*time.Minute),
		Batch:    int32(envInt("EXPIRY_BATCH", 100)),
	})
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := expiry.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("expiry scheduler stopped", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	grpcServer.GracefulStop()
	wg.Wait()
	slog.Info("stopped")
}

// envDuration takes a Go duration string ("15m", "30s"), so the demo can set
// ORDER_EXPIRY=20s without a second unit-carrying variable.
func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		// Loud, and then carry on with the default. A typo in ORDER_EXPIRY
		// should not stop the service from booting, but it must not silently
		// become 0 either -- a zero expiry cancels every order instantly.
		slog.Error("bad duration, using default", "key", key, "value", v, "default", fallback)
		return fallback
	}
	return d
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		slog.Error("bad int, using default", "key", key, "value", v, "default", fallback)
		return fallback
	}
	return n
}
