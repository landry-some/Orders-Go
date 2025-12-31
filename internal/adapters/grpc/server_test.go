package grpc

import (
	"testing"

	"context"
	"net"

	driverpb "wayfinder/api/proto/driver"
	"wayfinder/internal/ingest"

	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestServerImplementsDriverServiceServer(t *testing.T) {
	var _ driverpb.DriverServiceServer = (*Server)(nil)
}

type spyIngestService struct {
	received []ingest.Location
}

func (s *spyIngestService) Ingest(ctx context.Context, loc ingest.Location) error {
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
		if err := lis.Close(); err != nil {
			t.Fatalf("close listener: %v", err)
		}
	})

	ctx := context.Background()
	//nolint:staticcheck // grpc.DialContext works with bufconn; deprecated in favor of NewClient.
	conn, err := grpcpkg.DialContext(ctx, "bufnet",
		grpcpkg.WithContextDialer(bufDialer(lis)),
		grpcpkg.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufnet: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("close conn: %v", err)
		}
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
