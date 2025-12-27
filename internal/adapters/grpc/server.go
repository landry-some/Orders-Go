package grpc

import driverpb "wayfinder/api/proto"

// Server adapts DriverService to gRPC.
type Server struct {
	driverpb.UnimplementedDriverServiceServer
}
