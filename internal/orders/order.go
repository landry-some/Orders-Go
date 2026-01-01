package orders

import (
	"context"
	"errors"
	"fmt"

	"wayfinder/internal/orders/saga"

	"github.com/google/uuid"
)

// PaymentClient charges a payment instrument for an order.
type PaymentClient interface {
	Charge(ctx context.Context, orderID string, amount float64) error
	Refund(ctx context.Context, orderID string, amount float64) error
}

// DriverClient assigns a driver to an order.
type DriverClient interface {
	Assign(ctx context.Context, orderID string, driverID string) error
}

// IDGenerator returns a new order ID.
type IDGenerator func() string

// DriverSelector returns a driver ID to assign.
type DriverSelector func() string

// OrderService coordinates payment and driver assignment.
type OrderService struct {
	payments  PaymentClient
	drivers   DriverClient
	sagas     saga.SagaStore
	idGen     IDGenerator
	driverSel DriverSelector
}

// NewOrderService constructs an OrderService.
func NewOrderService(payments PaymentClient, drivers DriverClient, sagas saga.SagaStore, idGen IDGenerator, driverSel DriverSelector) *OrderService {
	if idGen == nil {
		idGen = newOrderID
	}
	if driverSel == nil {
		driverSel = newDriverID
	}

	return &OrderService{
		payments:  payments,
		drivers:   drivers,
		sagas:     sagas,
		idGen:     idGen,
		driverSel: driverSel,
	}
}

var (
	ErrIdempotencyKeyRequired = errors.New("idempotency key required")
	ErrIdempotencyConflict    = saga.ErrIdempotencyConflict
)

// CreateOrder orchestrates the payment and driver assignment steps.
func (s *OrderService) CreateOrder(ctx context.Context, userID string, amount float64, idempotencyKey string) (string, error) {
	if idempotencyKey == "" {
		return "", ErrIdempotencyKeyRequired
	}

	orderID := s.idGen()
	driverID := s.driverSel()

	record, created, err := s.sagas.Start(ctx, idempotencyKey, orderID, userID, amount)
	if err != nil {
		if errors.Is(err, ErrIdempotencyConflict) {
			return "", err
		}
		return "", err
	}
	if !created {
		if record.Status == saga.SagaStatusSucceeded {
			return record.OrderID, nil
		}
		return record.OrderID, fmt.Errorf("order already processed with status %s", record.Status)
	}

	_ = s.sagas.AddStep(ctx, orderID, "charge", "started", "")
	if err := s.payments.Charge(ctx, orderID, amount); err != nil {
		_ = s.sagas.AddStep(ctx, orderID, "charge", "failed", err.Error())
		_ = s.sagas.UpdateStatus(ctx, orderID, saga.SagaStatusFailed)
		return "", err
	}
	_ = s.sagas.AddStep(ctx, orderID, "charge", "succeeded", "")

	_ = s.sagas.AddStep(ctx, orderID, "assign", "started", "")
	if err := s.drivers.Assign(ctx, orderID, driverID); err != nil {
		_ = s.sagas.AddStep(ctx, orderID, "assign", "failed", err.Error())
		// Compensate by refunding the payment if driver assignment fails.
		_ = s.sagas.AddStep(ctx, orderID, "refund", "started", "")
		if refundErr := s.payments.Refund(ctx, orderID, amount); refundErr != nil {
			_ = s.sagas.AddStep(ctx, orderID, "refund", "failed", refundErr.Error())
			_ = s.sagas.UpdateStatus(ctx, orderID, saga.SagaStatusFailed)
			return "", fmt.Errorf("driver assignment failed: %w; refund failed: %v", err, refundErr)
		}
		_ = s.sagas.AddStep(ctx, orderID, "refund", "succeeded", "")
		_ = s.sagas.UpdateStatus(ctx, orderID, saga.SagaStatusRefunded)
		return "", err
	}

	_ = s.sagas.AddStep(ctx, orderID, "assign", "succeeded", "")
	_ = s.sagas.UpdateStatus(ctx, orderID, saga.SagaStatusSucceeded)
	return orderID, nil
}

func newOrderID() string  { return newUUIDString() }
func newDriverID() string { return newUUIDString() }

func newUUIDString() string {
	return uuid.Must(uuid.NewV7()).String()
}
