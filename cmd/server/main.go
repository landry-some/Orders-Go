package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	driverpb "wayfinder/api/proto/driver"
	orderpb "wayfinder/api/proto/order"
	"wayfinder/internal/adapters/grpc"
	"wayfinder/internal/courier"
	"wayfinder/internal/grid"
	"wayfinder/internal/orders"
	"wayfinder/internal/realtime"

	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"github.com/gorilla/websocket"
)

func main() {
	wal, err := grid.NewFileWAL("courier.log")
	if err != nil {
		log.Fatalf("create WAL: %v", err)
	}
	defer func() {
		if err := wal.Close(); err != nil {
			log.Printf("close WAL: %v", err)
		}
	}()

	g, err := grid.NewGridServiceWithRecovery(wal)
	if err != nil {
		log.Fatalf("init grid: %v", err)
	}

	hub := realtime.NewHub()
	go hub.Run()
	startWebsocketServer(hub)

	gridPublisher := courier.NewGridPublisher(g)
	broadcaster := realtime.NewHubBroadcaster(hub)
	publisher := courier.NewFanoutPublisher(gridPublisher, broadcaster)
	ingest := courier.NewIngestService(publisher)

	orderService := orders.NewOrderService(
		orders.NewInMemoryPaymentClient(),
		orders.NewInMemoryDriverClient(),
		func() string { return "order-" + strconv.FormatInt(time.Now().UnixNano(), 10) },
		func() string { return "driver-" + time.Now().Format("150405") },
	)
	orderAdapter := grpc.NewOrderServer(orderService)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	server := grpcpkg.NewServer()
	driverpb.RegisterDriverServiceServer(server, grpc.NewServer(ingest))
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

func startWebsocketServer(hub *realtime.Hub) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade websocket: %v", err)
			return
		}
		hub.Register <- conn

		go func(c *websocket.Conn) {
			defer func() {
				hub.Unregister <- c
			}()
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}(conn)
	})

	go func() {
		addr := ":8080"
		log.Printf("WebSocket server running on %s/ws", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("ws server: %v", err)
		}
	}()
}
