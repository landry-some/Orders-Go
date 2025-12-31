package grpc

import (
	"context"
	"errors"
	"strings"

	orderpb "wayfinder/api/proto/order"
	"wayfinder/internal/orders"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OrderService defines the behavior needed by the gRPC adapter.
type OrderService interface {
	CreateOrder(ctx context.Context, userID string, amount float64, idempotencyKey string) (string, error)
}

// OrderServer adapts OrderService to gRPC.
type OrderServer struct {
	orderpb.UnimplementedOrderServiceServer
	service OrderService
}

// NewOrderServer constructs an OrderServer.
func NewOrderServer(svc OrderService) *OrderServer {
	return &OrderServer{service: svc}
}

// CreateOrder handles the gRPC request and maps domain errors to gRPC status codes.
func (s *OrderServer) CreateOrder(ctx context.Context, req *orderpb.CreateOrderRequest) (*orderpb.CreateOrderResponse, error) {
	orderID, err := s.service.CreateOrder(ctx, req.GetUserId(), req.GetAmount(), req.GetIdempotencyKey())
	if err != nil {
		return nil, mapOrderError(err)
	}

	return &orderpb.CreateOrderResponse{
		OrderId: orderID,
		Status:  "ok",
		Message: "order created",
	}, nil
}

func mapOrderError(err error) error {
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, err.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, err.Error())
	}
	if errors.Is(err, orders.ErrIdempotencyKeyRequired) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	if errors.Is(err, orders.ErrIdempotencyConflict) {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	if strings.Contains(strings.ToLower(err.Error()), "payment failed") {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}
