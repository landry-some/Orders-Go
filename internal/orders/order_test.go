package orders

import (
	"errors"
	"testing"
)

var callSeq int

type spyPayment struct {
	called    bool
	orderID   string
	amount    float64
	callOrder int
	err       error

	refundCalled    bool
	refundOrderID   string
	refundAmount    float64
	refundCallOrder int
}

func (s *spyPayment) Charge(orderID string, amount float64) error {
	s.called = true
	s.orderID = orderID
	s.amount = amount
	s.callOrder = callSeq
	callSeq++
	return s.err
}

func (s *spyPayment) Refund(orderID string, amount float64) error {
	s.refundCalled = true
	s.refundOrderID = orderID
	s.refundAmount = amount
	s.refundCallOrder = callSeq
	callSeq++
	return nil
}

type spyDriver struct {
	called    bool
	orderID   string
	driverID  string
	callOrder int
	err       error
}

func (s *spyDriver) Assign(orderID string, driverID string) error {
	s.called = true
	s.orderID = orderID
	s.driverID = driverID
	s.callOrder = callSeq
	callSeq++
	return s.err
}

func TestCreateOrder_Success(t *testing.T) {
	callSeq = 0
	payment := &spyPayment{}
	driver := &spyDriver{}
	service := &OrderService{payments: payment, drivers: driver}

	orderID := "order-123"
	amount := 9.99
	driverID := "driver-abc"

	err := service.CreateOrder(orderID, amount, driverID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !payment.called {
		t.Fatalf("expected payment.Charge to be called")
	}

	if !driver.called {
		t.Fatalf("expected driver.Assign to be called")
	}

	if payment.callOrder >= driver.callOrder {
		t.Fatalf("expected payment.Charge to be called before driver.Assign; got payment=%d driver=%d", payment.callOrder, driver.callOrder)
	}
}

func TestCreateOrder_Compensates_On_DriverFailure(t *testing.T) {
	callSeq = 0
	payment := &spyPayment{}
	driver := &spyDriver{err: errors.New("assign failed")}
	service := &OrderService{payments: payment, drivers: driver}

	orderID := "order-456"
	amount := 19.99
	driverID := "driver-def"

	err := service.CreateOrder(orderID, amount, driverID)
	if err == nil {
		t.Fatalf("expected error due to driver failure, got nil")
	}

	if !payment.refundCalled {
		t.Fatalf("expected payment.Refund to be called")
	}

	if payment.refundOrderID != orderID || payment.refundAmount != amount {
		t.Fatalf("refund called with wrong args: id=%s amount=%f", payment.refundOrderID, payment.refundAmount)
	}
}
