package orders

import (
	"context"
	"errors"
	"testing"
)

type spyPayment struct {
	called        bool
	orderID       string
	amount        float64
	err           error
	refundCalled  bool
	refundOrderID string
	refundAmount  float64
	refundErr     error
	callLog       *[]string
}

func (s *spyPayment) Charge(orderID string, amount float64) error {
	s.called = true
	s.orderID = orderID
	s.amount = amount
	if s.callLog != nil {
		*s.callLog = append(*s.callLog, "charge")
	}
	return s.err
}

func (s *spyPayment) Refund(orderID string, amount float64) error {
	s.refundCalled = true
	s.refundOrderID = orderID
	s.refundAmount = amount
	if s.callLog != nil {
		*s.callLog = append(*s.callLog, "refund")
	}
	return s.refundErr
}

type spyDriver struct {
	called   bool
	orderID  string
	driverID string
	err      error
	callLog  *[]string
}

func (s *spyDriver) Assign(orderID string, driverID string) error {
	s.called = true
	s.orderID = orderID
	s.driverID = driverID
	if s.callLog != nil {
		*s.callLog = append(*s.callLog, "assign")
	}
	return s.err
}

func TestCreateOrder_Success(t *testing.T) {
	t.Parallel()

	callLog := []string{}
	payment := &spyPayment{callLog: &callLog}
	driver := &spyDriver{callLog: &callLog}
	idGen := func() string { return "order-123" }
	driverSel := func() string { return "driver-abc" }
	service := NewOrderService(payment, driver, idGen, driverSel)

	amount := 9.99

	orderID, err := service.CreateOrder(context.Background(), "user-1", amount)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if orderID != "order-123" {
		t.Fatalf("expected orderID order-123, got %s", orderID)
	}

	if !payment.called {
		t.Fatalf("expected payment.Charge to be called")
	}

	if !driver.called {
		t.Fatalf("expected driver.Assign to be called")
	}

	if len(callLog) < 2 || callLog[0] != "charge" || callLog[1] != "assign" {
		t.Fatalf("expected call order [charge assign], got %v", callLog)
	}

	if payment.orderID != orderID || driver.orderID != orderID || driver.driverID != "driver-abc" {
		t.Fatalf("order or driver IDs mismatch payment=%s driver=%s/%s", payment.orderID, driver.orderID, driver.driverID)
	}
}

func TestCreateOrder_Compensates_On_DriverFailure(t *testing.T) {
	t.Parallel()

	callLog := []string{}
	payment := &spyPayment{callLog: &callLog}
	driver := &spyDriver{err: errors.New("assign failed"), callLog: &callLog}
	service := NewOrderService(payment, driver, func() string { return "order-456" }, func() string { return "driver-def" })

	amount := 19.99

	_, err := service.CreateOrder(context.Background(), "user-1", amount)
	if err == nil {
		t.Fatalf("expected error due to driver failure, got nil")
	}

	if !payment.refundCalled {
		t.Fatalf("expected payment.Refund to be called")
	}

	if payment.refundOrderID != "order-456" || payment.refundAmount != amount {
		t.Fatalf("refund called with wrong args: id=%s amount=%f", payment.refundOrderID, payment.refundAmount)
	}

	if len(callLog) < 3 || callLog[0] != "charge" || callLog[1] != "assign" || callLog[2] != "refund" {
		t.Fatalf("expected call order [charge assign refund], got %v", callLog)
	}
}

func TestCreateOrder_RefundFailureReported(t *testing.T) {
	t.Parallel()

	callLog := []string{}
	refundErr := errors.New("refund failed")
	driverErr := errors.New("assign failed")
	payment := &spyPayment{refundErr: refundErr, callLog: &callLog}
	driver := &spyDriver{err: driverErr, callLog: &callLog}
	service := NewOrderService(payment, driver, func() string { return "order-789" }, func() string { return "driver-ghi" })

	amount := 29.99

	_, err := service.CreateOrder(context.Background(), "user-1", amount)
	if err == nil {
		t.Fatalf("expected error due to driver and refund failure, got nil")
	}

	if !errors.Is(err, driverErr) {
		t.Fatalf("expected driver error to be present: %v", err)
	}

	if !payment.refundCalled {
		t.Fatalf("expected refund to be attempted")
	}

	if len(callLog) < 3 || callLog[0] != "charge" || callLog[1] != "assign" || callLog[2] != "refund" {
		t.Fatalf("expected call order [charge assign refund], got %v", callLog)
	}
}

func TestCreateOrder_PaymentFailureStopsFlow(t *testing.T) {
	t.Parallel()

	paymentErr := errors.New("charge failed")
	callLog := []string{}
	payment := &spyPayment{err: paymentErr, callLog: &callLog}
	driver := &spyDriver{callLog: &callLog}
	service := NewOrderService(payment, driver, func() string { return "order-999" }, func() string { return "driver-jkl" })

	amount := 49.99

	_, err := service.CreateOrder(context.Background(), "user-1", amount)
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
