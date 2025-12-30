package orders

import (
	"context"
	"fmt"
	"time"
)

// PaymentClient charges a payment instrument for an order.
type PaymentClient interface {
	Charge(orderID string, amount float64) error
	Refund(orderID string, amount float64) error
}

// DriverClient assigns a driver to an order.
type DriverClient interface {
	Assign(orderID string, driverID string) error
}

// IDGenerator returns a new order ID.
type IDGenerator func() string

// DriverSelector returns a driver ID to assign.
type DriverSelector func() string

// OrderService coordinates payment and driver assignment.
type OrderService struct {
	payments  PaymentClient
	drivers   DriverClient
	idGen     IDGenerator
	driverSel DriverSelector
}

// NewOrderService constructs an OrderService.
func NewOrderService(payments PaymentClient, drivers DriverClient, idGen IDGenerator, driverSel DriverSelector) *OrderService {
	if idGen == nil {
		idGen = func() string { return "order-" + time.Now().Format("20060102150405.000000000") }
	}
	if driverSel == nil {
		driverSel = func() string { return "driver-" + time.Now().Format("150405") }
	}

	return &OrderService{
		payments:  payments,
		drivers:   drivers,
		idGen:     idGen,
		driverSel: driverSel,
	}
}

// CreateOrder orchestrates the payment and driver assignment steps.
func (s *OrderService) CreateOrder(ctx context.Context, userID string, amount float64) (string, error) {
	orderID := s.idGen()
	driverID := s.driverSel()

	if err := s.payments.Charge(orderID, amount); err != nil {
		return "", err
	}

	if err := s.drivers.Assign(orderID, driverID); err != nil {
		// Compensate by refunding the payment if driver assignment fails.
		if refundErr := s.payments.Refund(orderID, amount); refundErr != nil {
			return "", fmt.Errorf("driver assignment failed: %w; refund failed: %v", err, refundErr)
		}
		return "", err
	}

	return orderID, nil
}
