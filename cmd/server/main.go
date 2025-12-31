package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	driverpb "wayfinder/api/proto/driver"
	orderpb "wayfinder/api/proto/order"
	"wayfinder/cmd/server/config"
	"wayfinder/internal/adapters/grpc"
	"wayfinder/internal/ingest"
	"wayfinder/internal/observability"
	"wayfinder/internal/orders"

	"github.com/joho/godotenv"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

func main() {
	_ = godotenv.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func run(ctx context.Context) error {
	locationStore, cleanupStore, err := buildLocationStore(ctx)
	if err != nil {
		return err
	}
	defer cleanupStore()

	publisher := ingest.NewFanoutPublisher(ingest.NewGridPublisher(locationStore), nil)
	ingestService := ingest.NewIngestService(publisher)

	orderService, cleanup, err := orders.BuildOrderService(ctx, os.Getenv("DATABASE_URL"), log.Printf)
	if err != nil {
		return err
	}
	defer cleanup()
	orderAdapter := grpc.NewOrderServer(orderService)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		return err
	}

	grpcCfg, err := config.LoadGRPC()
	if err != nil {
		return err
	}
	metrics := observability.NewMetrics()
	limiter := newGrpcRateLimiter(grpcCfg.RateLimitInterval, grpcCfg.RateLimitBurst, metrics.AddRateLimitWait)

	server := grpcpkg.NewServer(
		grpcpkg.UnaryInterceptor(rateLimitUnaryInterceptor(limiter, metrics)),
		grpcpkg.StreamInterceptor(rateLimitStreamInterceptor(limiter, metrics)),
	)
	driverpb.RegisterDriverServiceServer(server, grpc.NewServer(ingestService))
	orderpb.RegisterOrderServiceServer(server, orderAdapter)

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus(driverpb.DriverService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(orderpb.OrderService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	if env := os.Getenv("APP_ENV"); env != "production" {
		reflection.Register(server)
		log.Println("gRPC reflection enabled (APP_ENV=", env, ")")
	}

	log.Println("Server running on :50051...")
	obsSrv, obsErr := startObservabilityServer(ctx, metrics)
	if obsErr != nil {
		return obsErr
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(lis)
	}()

	select {
	case <-ctx.Done():
		healthServer.SetServingStatus(driverpb.DriverService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
		healthServer.SetServingStatus(orderpb.OrderService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		server.GracefulStop()
		if obsSrv != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = obsSrv.Shutdown(shutdownCtx)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

func startObservabilityServer(ctx context.Context, metrics *observability.Metrics) (*http.Server, error) {
	cfg, err := config.LoadObservability()
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", observability.Handler(metrics))
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := readinessCheck(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("observability server error: %v", err)
		}
	}()

	return srv, nil
}

func readinessCheck(ctx context.Context) error {
	redisURL, err := config.GetRedisURL()
	if err != nil {
		return err
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		return err
	}
	rdb := redis.NewClient(redisOpts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return err
	}
	_ = rdb.Close()

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "SELECT 1"); err != nil {
		return err
	}
	return nil
}
