package grpc

import (
	"testing"

	"context"
	"net"

	driverpb "wayfinder/api/proto"
	"wayfinder/internal/courier"

	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestServerImplementsDriverServiceServer(t *testing.T) {
	var _ driverpb.DriverServiceServer = (*Server)(nil)
}

type spyIngestService struct {
	received []courier.Location
}

func (s *spyIngestService) Ingest(ctx context.Context, loc courier.Location) error {
	s.received = append(s.received, loc)
	return nil
}

func bufDialer(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.Dial()
	}
}

func TestUpdateLocationStreamsToIngestService(t *testing.T) {
	t.Parallel()

	lis := bufconn.Listen(1024 * 1024)
	ingest := &spyIngestService{}
	s := grpcpkg.NewServer()
	driverpb.RegisterDriverServiceServer(s, NewServer(ingest))
	go func() {
		_ = s.Serve(lis)
	}()
	t.Cleanup(func() {
		s.Stop()
		lis.Close()
	})

	ctx := context.Background()
	conn, err := grpcpkg.DialContext(ctx, "bufnet",
		grpcpkg.WithContextDialer(bufDialer(lis)),
		grpcpkg.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufnet: %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
	})

	client := driverpb.NewDriverServiceClient(conn)
	stream, err := client.UpdateLocation(ctx)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}

	msg := &driverpb.Location{
		DriverId:  "driver-123",
		Latitude:  37.7749,
		Longitude: -122.4194,
	}

	if err := stream.Send(msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	_, err = stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("close and recv: %v", err)
	}

	if len(ingest.received) != 1 {
		t.Fatalf("expected 1 ingested location, got %d", len(ingest.received))
	}

	got := ingest.received[0]
	if got.DriverID != msg.DriverId || got.Lat != msg.Latitude || got.Long != msg.Longitude {
		t.Fatalf("ingested location mismatch: %+v", got)
	}
}
