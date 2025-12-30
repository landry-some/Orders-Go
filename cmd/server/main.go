package main

import (
	"log"
	"net"
	"os"

	driverpb "wayfinder/api/proto"
	"wayfinder/internal/adapters/grpc"
	"wayfinder/internal/courier"
	"wayfinder/internal/grid"

	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	wal, err := grid.NewFileWAL("courier.log")
	if err != nil {
		log.Fatalf("create WAL: %v", err)
	}
	defer wal.Close()

	g, err := grid.NewGridServiceWithRecovery(wal)
	if err != nil {
		log.Fatalf("init grid: %v", err)
	}

	publisher := courier.NewLocalGridPublisher(g)
	ingest := courier.NewIngestService(publisher)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	server := grpcpkg.NewServer()
	driverpb.RegisterDriverServiceServer(server, grpc.NewServer(ingest))

	if env := os.Getenv("APP_ENV"); env != "production" {
		reflection.Register(server)
		log.Println("gRPC reflection enabled (APP_ENV=", env, ")")
	}

	log.Println("Server running on :50051...")
	if err := server.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
