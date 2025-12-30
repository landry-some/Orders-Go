package grpc

import (
	"context"
	"errors"
	"testing"

	orderpb "wayfinder/api/proto/order"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestOrderServerImplementsOrderServiceServer(t *testing.T) {
	var _ orderpb.OrderServiceServer = (*OrderServer)(nil)
}

type spyOrderService struct {
	orderID string
	err     error
}

func (s *spyOrderService) CreateOrder(ctx context.Context, userID string, amount float64) (string, error) {
	return s.orderID, s.err
}

func TestCreateOrder_Success(t *testing.T) {
	svc := &spyOrderService{orderID: "order-123"}
	server := NewOrderServer(svc)

	resp, err := server.CreateOrder(context.Background(), &orderpb.CreateOrderRequest{
		UserId: "user-1",
		Amount: 12.34,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.OrderId != "order-123" || resp.Status != "ok" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestCreateOrder_PaymentFailureMapsToFailedPrecondition(t *testing.T) {
	svc := &spyOrderService{err: errors.New("payment failed: card declined")}
	server := NewOrderServer(svc)

	_, err := server.CreateOrder(context.Background(), &orderpb.CreateOrderRequest{
		UserId: "user-1",
		Amount: 12.34,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("unexpected status code: %v", status.Code(err))
	}
}

func TestCreateOrder_GenericErrorMapsToInternal(t *testing.T) {
	svc := &spyOrderService{err: errors.New("boom")}
	server := NewOrderServer(svc)

	_, err := server.CreateOrder(context.Background(), &orderpb.CreateOrderRequest{
		UserId: "user-1",
		Amount: 12.34,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if status.Code(err) != codes.Internal {
		t.Fatalf("unexpected status code: %v", status.Code(err))
	}
}
