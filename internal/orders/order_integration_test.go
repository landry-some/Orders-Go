package orders_test

import (
	"context"
	"errors"
	"net"
	"testing"

	orderpb "wayfinder/api/proto/order"
	ordersdb "wayfinder/internal/db/orders"
	"wayfinder/internal/orders"

	grpcadapter "wayfinder/internal/adapters/grpc"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"time"
)

type failingDriver struct {
	err error
}

func (f failingDriver) Assign(orderID string, driverID string) error {
	return f.err
}

func bufDialer(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.Dial()
	}
}

func TestOrderService_RefundsOnDriverFailure_WithPostgresPayments(t *testing.T) {
	ctx := context.Background()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	}()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS payments").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS order_sagas").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS order_saga_steps").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO order_sagas").
		WithArgs("order-1", "idem-1", "u1", 9.99, "started").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT order_id, user_id, amount, status").
		WithArgs("idem-1").
		WillReturnRows(sqlmock.NewRows([]string{"order_id", "user_id", "amount", "status"}).AddRow("order-1", "u1", 9.99, "started"))
	mock.ExpectExec("INSERT INTO order_saga_steps").
		WithArgs("order-1", "charge", "started", "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO payments").
		WithArgs("order-1", 9.99).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO order_saga_steps").
		WithArgs("order-1", "charge", "succeeded", "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO order_saga_steps").
		WithArgs("order-1", "assign", "started", "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO order_saga_steps").
		WithArgs("order-1", "assign", "failed", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO order_saga_steps").
		WithArgs("order-1", "refund", "started", "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE payments SET refund_amount").
		WithArgs("order-1", 9.99).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO order_saga_steps").
		WithArgs("order-1", "refund", "succeeded", "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE order_sagas").
		WithArgs("order-1", "refunded").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectClose()

	payments, err := ordersdb.NewPostgresPaymentClientWithSchema(ctx, sqlDB)
	if err != nil {
		t.Fatalf("init postgres payments: %v", err)
	}
	sagas, err := ordersdb.NewSagaStoreWithSchema(ctx, sqlDB)
	if err != nil {
		t.Fatalf("init saga store: %v", err)
	}

	driver := failingDriver{err: errors.New("assign failed")}
	idGen := func() string { return "order-1" }
	driverSel := func() string { return "driver-x" }
	service := orders.NewOrderService(payments, driver, sagas, idGen, driverSel)

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	orderpb.RegisterOrderServiceServer(srv, grpcadapter.NewOrderServer(service))
	go func() {
		_ = srv.Serve(lis)
	}()
	defer srv.Stop()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(bufDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufnet: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("close conn: %v", err)
		}
	})
	conn.Connect()
	connectCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	for state := conn.GetState(); state != connectivity.Ready; state = conn.GetState() {
		if !conn.WaitForStateChange(connectCtx, state) {
			t.Fatalf("grpc connection did not become ready")
		}
	}

	client := orderpb.NewOrderServiceClient(conn)
	_, err = client.CreateOrder(ctx, &orderpb.CreateOrderRequest{
		UserId:         "u1",
		Amount:         9.99,
		IdempotencyKey: "idem-1",
	})
	if err == nil {
		t.Fatalf("expected error due to driver failure")
	}
}
