package main

import (
	"context"
	"log"
	"net"
	"os"

	driverpb "wayfinder/api/proto/driver"
	orderpb "wayfinder/api/proto/order"
	"wayfinder/internal/adapters/grpc"
	"wayfinder/internal/ingest"
	"wayfinder/internal/orders"

	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	locationStore, cleanupStore, err := buildLocationStore(context.Background())
	if err != nil {
		log.Fatalf("location store init: %v", err)
	}
	defer cleanupStore()

	publisher := ingest.NewFanoutPublisher(ingest.NewGridPublisher(locationStore), nil)
	ingestService := ingest.NewIngestService(publisher)

	orderService, cleanup, err := orders.BuildOrderService(context.Background(), os.Getenv("DATABASE_URL"), log.Printf)
	if err != nil {
		log.Fatalf("order service init: %v", err)
	}
	defer cleanup()
	orderAdapter := grpc.NewOrderServer(orderService)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	server := grpcpkg.NewServer()
	driverpb.RegisterDriverServiceServer(server, grpc.NewServer(ingestService))
	orderpb.RegisterOrderServiceServer(server, orderAdapter)

	if env := os.Getenv("APP_ENV"); env != "production" {
		reflection.Register(server)
		log.Println("gRPC reflection enabled (APP_ENV=", env, ")")
	}

	log.Println("Server running on :50051...")
	if err := server.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
