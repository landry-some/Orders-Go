package orders

import (
	"context"
	"errors"
	"testing"
)

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

	seq *int
}

func (s *spyPayment) Charge(orderID string, amount float64) error {
	s.called = true
	s.orderID = orderID
	s.amount = amount
	s.callOrder = *s.seq
	*s.seq++
	return s.err
}

func (s *spyPayment) Refund(orderID string, amount float64) error {
	s.refundCalled = true
	s.refundOrderID = orderID
	s.refundAmount = amount
	s.refundCallOrder = *s.seq
	*s.seq++
	return s.refundErr
}

type spyDriver struct {
	called    bool
	orderID   string
	driverID  string
	callOrder int
	err       error
	seq       *int
}

func (s *spyDriver) Assign(orderID string, driverID string) error {
	s.called = true
	s.orderID = orderID
	s.driverID = driverID
	s.callOrder = *s.seq
	*s.seq++
	return s.err
}

func TestCreateOrder_Success(t *testing.T) {
	seq := 0
	payment := &spyPayment{seq: &seq}
	driver := &spyDriver{seq: &seq}
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

	if payment.callOrder >= driver.callOrder {
		t.Fatalf("expected payment.Charge to be called before driver.Assign; got payment=%d driver=%d", payment.callOrder, driver.callOrder)
	}

	if payment.orderID != orderID || driver.orderID != orderID || driver.driverID != "driver-abc" {
		t.Fatalf("order or driver IDs mismatch payment=%s driver=%s/%s", payment.orderID, driver.orderID, driver.driverID)
	}
}

func TestCreateOrder_Compensates_On_DriverFailure(t *testing.T) {
	seq := 0
	payment := &spyPayment{seq: &seq}
	driver := &spyDriver{err: errors.New("assign failed"), seq: &seq}
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
}

func TestCreateOrder_RefundFailureReported(t *testing.T) {
	seq := 0
	refundErr := errors.New("refund failed")
	driverErr := errors.New("assign failed")
	payment := &spyPayment{refundErr: refundErr, seq: &seq}
	driver := &spyDriver{err: driverErr, seq: &seq}
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
}

func TestCreateOrder_PaymentFailureStopsFlow(t *testing.T) {
	seq := 0
	paymentErr := errors.New("charge failed")
	payment := &spyPayment{err: paymentErr, seq: &seq}
	driver := &spyDriver{seq: &seq}
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
