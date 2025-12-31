package ordersdb

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"wayfinder/internal/orders/saga"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func newSagaMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}

	cleanup := func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	}

	return db, mock, cleanup
}

func TestSagaStore_InitSchema(t *testing.T) {
	db, mock, cleanup := newSagaMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS order_sagas").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS order_saga_steps").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectClose()

	store := NewSagaStore(db)
	if err := store.InitSchema(context.Background()); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
}

func TestSagaStore_Start_New(t *testing.T) {
	db, mock, cleanup := newSagaMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("INSERT INTO order_sagas").
		WithArgs("order-1", "idem-1", "user-1", 10.0, "started").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT order_id, user_id, amount, status").
		WithArgs("idem-1").
		WillReturnRows(sqlmock.NewRows([]string{"order_id", "user_id", "amount", "status"}).
			AddRow("order-1", "user-1", 10.0, "started"))
	mock.ExpectClose()

	store := NewSagaStore(db)
	record, created, err := store.Start(context.Background(), "idem-1", "order-1", "user-1", 10.0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !created {
		t.Fatalf("expected created saga")
	}
	if record.OrderID != "order-1" {
		t.Fatalf("unexpected order id: %s", record.OrderID)
	}
}

func TestSagaStore_Start_IdempotencyConflict(t *testing.T) {
	db, mock, cleanup := newSagaMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("INSERT INTO order_sagas").
		WithArgs("order-1", "idem-1", "user-1", 10.0, "started").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT order_id, user_id, amount, status").
		WithArgs("idem-1").
		WillReturnRows(sqlmock.NewRows([]string{"order_id", "user_id", "amount", "status"}).
			AddRow("order-99", "user-1", 11.0, "started"))
	mock.ExpectClose()

	store := NewSagaStore(db)
	_, _, err := store.Start(context.Background(), "idem-1", "order-1", "user-1", 10.0)
	if err == nil {
		t.Fatalf("expected conflict error")
	}
	if !errors.Is(err, saga.ErrIdempotencyConflict) {
		t.Fatalf("unexpected error: %v", err)
	}
}
