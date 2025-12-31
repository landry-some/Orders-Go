package orders

import (
	"context"
	"errors"
	"testing"

	"wayfinder/internal/orders/saga"
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

type sagaStep struct {
	orderID string
	step    string
	status  string
	detail  string
}

type spySagaStore struct {
	startCalled bool
	startKey    string
	startOrder  string
	startUser   string
	startAmount float64
	created     bool
	record      saga.SagaRecord
	err         error
	steps       []sagaStep
	statuses    []saga.SagaStatus
}

func (s *spySagaStore) Start(ctx context.Context, idempotencyKey, orderID, userID string, amount float64) (saga.SagaRecord, bool, error) {
	s.startCalled = true
	s.startKey = idempotencyKey
	s.startOrder = orderID
	s.startUser = userID
	s.startAmount = amount
	if s.err != nil {
		return saga.SagaRecord{}, false, s.err
	}
	record := s.record
	if record.OrderID == "" {
		record = saga.SagaRecord{OrderID: orderID, UserID: userID, Amount: amount, Status: saga.SagaStatusStarted}
	}
	return record, s.created, nil
}

func (s *spySagaStore) UpdateStatus(ctx context.Context, orderID string, status saga.SagaStatus) error {
	s.statuses = append(s.statuses, status)
	return nil
}

func (s *spySagaStore) AddStep(ctx context.Context, orderID, step, status, detail string) error {
	s.steps = append(s.steps, sagaStep{orderID: orderID, step: step, status: status, detail: detail})
	return nil
}

func TestCreateOrder_Success(t *testing.T) {
	t.Parallel()

	callLog := []string{}
	payment := &spyPayment{callLog: &callLog}
	driver := &spyDriver{callLog: &callLog}
	sagas := &spySagaStore{created: true}
	idGen := func() string { return "order-123" }
	driverSel := func() string { return "driver-abc" }
	service := NewOrderService(payment, driver, sagas, idGen, driverSel)

	amount := 9.99

	orderID, err := service.CreateOrder(context.Background(), "user-1", amount, "idem-1")
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

	if len(sagas.statuses) == 0 || sagas.statuses[len(sagas.statuses)-1] != saga.SagaStatusSucceeded {
		t.Fatalf("expected saga to end in succeeded status, got %v", sagas.statuses)
	}
}

func TestCreateOrder_Compensates_On_DriverFailure(t *testing.T) {
	t.Parallel()

	callLog := []string{}
	payment := &spyPayment{callLog: &callLog}
	driver := &spyDriver{err: errors.New("assign failed"), callLog: &callLog}
	sagas := &spySagaStore{created: true}
	service := NewOrderService(payment, driver, sagas, func() string { return "order-456" }, func() string { return "driver-def" })

	amount := 19.99

	_, err := service.CreateOrder(context.Background(), "user-1", amount, "idem-2")
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

	if len(sagas.statuses) == 0 || sagas.statuses[len(sagas.statuses)-1] != saga.SagaStatusRefunded {
		t.Fatalf("expected saga to end in refunded status, got %v", sagas.statuses)
	}
}

func TestCreateOrder_RefundFailureReported(t *testing.T) {
	t.Parallel()

	callLog := []string{}
	refundErr := errors.New("refund failed")
	driverErr := errors.New("assign failed")
	payment := &spyPayment{refundErr: refundErr, callLog: &callLog}
	driver := &spyDriver{err: driverErr, callLog: &callLog}
	sagas := &spySagaStore{created: true}
	service := NewOrderService(payment, driver, sagas, func() string { return "order-789" }, func() string { return "driver-ghi" })

	amount := 29.99

	_, err := service.CreateOrder(context.Background(), "user-1", amount, "idem-3")
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

	if len(sagas.statuses) == 0 || sagas.statuses[len(sagas.statuses)-1] != saga.SagaStatusFailed {
		t.Fatalf("expected saga to end in failed status, got %v", sagas.statuses)
	}
}

func TestCreateOrder_PaymentFailureStopsFlow(t *testing.T) {
	t.Parallel()

	paymentErr := errors.New("charge failed")
	callLog := []string{}
	payment := &spyPayment{err: paymentErr, callLog: &callLog}
	driver := &spyDriver{callLog: &callLog}
	sagas := &spySagaStore{created: true}
	service := NewOrderService(payment, driver, sagas, func() string { return "order-999" }, func() string { return "driver-jkl" })

	amount := 49.99

	_, err := service.CreateOrder(context.Background(), "user-1", amount, "idem-4")
	if err == nil {
		t.Fatalf("expected error due to payment failure, got nil")
	}

	if !errors.Is(err, paymentErr) {
		t.Fatalf("unexpected error: %v", err)
	}

	if driver.called {
		t.Fatalf("driver.Assign should not be called when payment fails")
	}

	if len(sagas.statuses) == 0 || sagas.statuses[len(sagas.statuses)-1] != saga.SagaStatusFailed {
		t.Fatalf("expected saga to end in failed status, got %v", sagas.statuses)
	}
}

func TestCreateOrder_IdempotencyReturnsExisting(t *testing.T) {
	t.Parallel()

	payment := &spyPayment{}
	driver := &spyDriver{}
	sagas := &spySagaStore{
		created: false,
		record: saga.SagaRecord{
			OrderID: "order-123",
			UserID:  "user-1",
			Amount:  10,
			Status:  saga.SagaStatusSucceeded,
		},
	}

	service := NewOrderService(payment, driver, sagas, func() string { return "order-999" }, func() string { return "driver-zzz" })

	orderID, err := service.CreateOrder(context.Background(), "user-1", 10, "idem-5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if orderID != "order-123" {
		t.Fatalf("expected idempotent order id order-123, got %s", orderID)
	}
	if payment.called || driver.called {
		t.Fatalf("expected no payment/driver calls on idempotent replay")
	}
}

func TestCreateOrder_RequiresIdempotencyKey(t *testing.T) {
	t.Parallel()

	payment := &spyPayment{}
	driver := &spyDriver{}
	sagas := &spySagaStore{created: true}
	service := NewOrderService(payment, driver, sagas, func() string { return "order-1" }, func() string { return "driver-1" })

	_, err := service.CreateOrder(context.Background(), "user-1", 1.0, "")
	if !errors.Is(err, ErrIdempotencyKeyRequired) {
		t.Fatalf("expected ErrIdempotencyKeyRequired, got %v", err)
	}
}

func TestCreateOrder_IdempotencyConflict(t *testing.T) {
	t.Parallel()

	payment := &spyPayment{}
	driver := &spyDriver{}
	sagas := &spySagaStore{err: ErrIdempotencyConflict}
	service := NewOrderService(payment, driver, sagas, func() string { return "order-1" }, func() string { return "driver-1" })

	_, err := service.CreateOrder(context.Background(), "user-1", 1.0, "idem-x")
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected ErrIdempotencyConflict, got %v", err)
	}
}
