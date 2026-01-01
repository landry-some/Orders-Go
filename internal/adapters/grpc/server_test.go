package grpc

import (
	"testing"

	"context"
	"errors"
	"net"

	driverpb "wayfinder/api/proto/driver"
	"wayfinder/internal/ingest"

	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
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

type errIngestService struct {
	err error
}

func (s *errIngestService) Ingest(_ context.Context, _ ingest.Location) error {
	return s.err
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

func TestUpdateLocation_InvalidLocationReturnsError(t *testing.T) {
	t.Parallel()

	lis := bufconn.Listen(1024 * 1024)
	s := grpcpkg.NewServer()
	driverpb.RegisterDriverServiceServer(s, NewServer(&spyIngestService{}))
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(func() {
		s.Stop()
		_ = lis.Close()
	})

	ctx := context.Background()
	conn, err := grpcpkg.DialContext(ctx, "bufnet",
		grpcpkg.WithContextDialer(bufDialer(lis)),
		grpcpkg.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufnet: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := driverpb.NewDriverServiceClient(conn)
	stream, err := client.UpdateLocation(ctx)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}

	// Missing driver id triggers invalid argument from ingest.NewLocation.
	if sendErr := stream.Send(&driverpb.Location{DriverId: ""}); sendErr != nil {
		t.Fatalf("send: %v", sendErr)
	}

	_, err = stream.CloseAndRecv()
	if err == nil {
		t.Fatalf("expected error for invalid location")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestUpdateLocation_IngestFailurePropagates(t *testing.T) {
	t.Parallel()

	lis := bufconn.Listen(1024 * 1024)
	svcErr := errors.New("ingest failed")
	s := grpcpkg.NewServer()
	driverpb.RegisterDriverServiceServer(s, NewServer(&errIngestService{err: svcErr}))
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(func() {
		s.Stop()
		_ = lis.Close()
	})

	ctx := context.Background()
	conn, err := grpcpkg.DialContext(ctx, "bufnet",
		grpcpkg.WithContextDialer(bufDialer(lis)),
		grpcpkg.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufnet: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := driverpb.NewDriverServiceClient(conn)
	stream, err := client.UpdateLocation(ctx)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}

	msg := &driverpb.Location{
		DriverId:  "driver-err",
		Latitude:  1,
		Longitude: 2,
	}
	if sendErr := stream.Send(msg); sendErr != nil {
		t.Fatalf("send: %v", sendErr)
	}
	_, err = stream.CloseAndRecv()
	if err == nil {
		t.Fatalf("expected error due to ingest failure")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
}
