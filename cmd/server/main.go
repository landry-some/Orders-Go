package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	driverpb "wayfinder/api/proto/driver"
	orderpb "wayfinder/api/proto/order"
	"wayfinder/internal/adapters/grpc"
	"wayfinder/internal/ingest"
	"wayfinder/internal/orders"

	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
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

	grpcCfg, err := loadGrpcConfigFromEnv()
	if err != nil {
		return err
	}
	limiter := newGrpcRateLimiter(grpcCfg.rateLimitInterval, grpcCfg.rateLimitBurst)

	server := grpcpkg.NewServer(
		grpcpkg.UnaryInterceptor(rateLimitUnaryInterceptor(limiter)),
		grpcpkg.StreamInterceptor(rateLimitStreamInterceptor(limiter)),
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
		return nil
	case err := <-errCh:
		return err
	}
}
