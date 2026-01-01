package grpc

import (
	"context"
	"io"
	"log"
	"time"

	driverpb "wayfinder/api/proto/driver"
	"wayfinder/internal/ingest"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IngestService exposes the ingest behavior needed by the gRPC adapter.
type IngestService interface {
	Ingest(ctx context.Context, loc ingest.Location) error
}

// Server adapts DriverService to gRPC.
type Server struct {
	driverpb.UnimplementedDriverServiceServer
	ingest IngestService
}

// NewServer constructs a Server with the given ingest service.
func NewServer(ingest IngestService) *Server {
	return &Server{ingest: ingest}
}

// UpdateLocation receives streamed locations and forwards them to the ingest service.
func (s *Server) UpdateLocation(stream driverpb.DriverService_UpdateLocationServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&driverpb.UpdateLocationAck{Message: "ok"})
		}
		if err != nil {
			log.Printf("UpdateLocation recv error: %v", err)
			return status.Errorf(codes.Internal, "recv: %v", err)
		}

		ts := time.Time{}
		if msg.GetTimestamp() != nil {
			if !msg.GetTimestamp().IsValid() {
				log.Printf("UpdateLocation invalid timestamp: %v", msg.GetTimestamp())
				return status.Errorf(codes.InvalidArgument, "invalid timestamp")
			}
			ts = msg.GetTimestamp().AsTime()
		}

		loc, err := ingest.NewLocation(msg.GetDriverId(), msg.GetLatitude(), msg.GetLongitude(), ts)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "invalid location: %v", err)
		}

		if err := s.ingest.Ingest(stream.Context(), loc); err != nil {
			return status.Errorf(codes.Internal, "ingest: %v", err)
		}
	}
}
