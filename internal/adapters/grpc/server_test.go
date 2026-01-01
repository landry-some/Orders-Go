package grpc

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"

	driverpb "wayfinder/api/proto/driver"
	"wayfinder/internal/ingest"

	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	conn, err := grpcpkg.NewClient(
		"passthrough:///bufnet",
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
	conn, err := grpcpkg.NewClient(
		"passthrough:///bufnet",
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
	conn, err := grpcpkg.NewClient(
		"passthrough:///bufnet",
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

type stubUpdateLocationStream struct {
	grpcpkg.ServerStream
	msgs    []*driverpb.Location
	idx     int
	recvErr error
	ack     *driverpb.UpdateLocationAck
	ctx     context.Context
}

func (s *stubUpdateLocationStream) Recv() (*driverpb.Location, error) {
	if s.idx < len(s.msgs) {
		m := s.msgs[s.idx]
		s.idx++
		return m, nil
	}
	if s.recvErr != nil {
		return nil, s.recvErr
	}
	return nil, io.EOF
}

func (s *stubUpdateLocationStream) SendAndClose(ack *driverpb.UpdateLocationAck) error {
	s.ack = ack
	return nil
}

func (s *stubUpdateLocationStream) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

func (s *stubUpdateLocationStream) SetHeader(md metadata.MD) error  { return nil }
func (s *stubUpdateLocationStream) SendHeader(md metadata.MD) error { return nil }
func (s *stubUpdateLocationStream) SetTrailer(md metadata.MD)       {}

func TestUpdateLocation_InvalidTimestamp(t *testing.T) {
	t.Parallel()

	stream := &stubUpdateLocationStream{
		msgs: []*driverpb.Location{
			{
				DriverId: "driver-1",
				Timestamp: &timestamppb.Timestamp{
					Seconds: -1,
					Nanos:   -1,
				},
			},
		},
	}

	err := NewServer(&spyIngestService{}).UpdateLocation(stream)
	if err == nil {
		t.Fatalf("expected error")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if stream.ack != nil {
		t.Fatalf("expected no ack on failure")
	}
}

func TestUpdateLocation_RecvError(t *testing.T) {
	t.Parallel()

	stream := &stubUpdateLocationStream{
		recvErr: errors.New("recv boom"),
	}

	err := NewServer(&spyIngestService{}).UpdateLocation(stream)
	if err == nil {
		t.Fatalf("expected error")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
}
