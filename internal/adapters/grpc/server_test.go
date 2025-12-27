package grpc

import (
	"testing"

	driverpb "wayfinder/api/proto"
)

func TestServerImplementsDriverServiceServer(t *testing.T) {
	var _ driverpb.DriverServiceServer = (*Server)(nil)
}
