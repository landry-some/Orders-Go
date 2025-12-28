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
	refundErr       error
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
	return s.refundErr
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

func TestCreateOrder_RefundFailureReported(t *testing.T) {
	callSeq = 0
	refundErr := errors.New("refund failed")
	driverErr := errors.New("assign failed")
	payment := &spyPayment{refundErr: refundErr}
	driver := &spyDriver{err: driverErr}
	service := &OrderService{payments: payment, drivers: driver}

	orderID := "order-789"
	amount := 29.99
	driverID := "driver-ghi"

	err := service.CreateOrder(orderID, amount, driverID)
	if err == nil {
		t.Fatalf("expected error due to driver and refund failure, got nil")
	}

	if !errors.Is(err, driverErr) {
		t.Fatalf("expected driver error to be present: %v", err)
	}

	if !payment.refundCalled {
		t.Fatalf("expected refund to be attempted")
	}
}

func TestCreateOrder_PaymentFailureStopsFlow(t *testing.T) {
	callSeq = 0
	paymentErr := errors.New("charge failed")
	payment := &spyPayment{err: paymentErr}
	driver := &spyDriver{}
	service := &OrderService{payments: payment, drivers: driver}

	orderID := "order-999"
	amount := 49.99
	driverID := "driver-jkl"

	err := service.CreateOrder(orderID, amount, driverID)
	if err == nil {
		t.Fatalf("expected error due to payment failure, got nil")
	}

	if !errors.Is(err, paymentErr) {
		t.Fatalf("unexpected error: %v", err)
	}

	if driver.called {
		t.Fatalf("driver.Assign should not be called when payment fails")
	}
}
