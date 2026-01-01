package ordersdb

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func newDriverMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
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

func TestPostgresDriverClient_InitSchema(t *testing.T) {
	db, mock, cleanup := newDriverMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS order_assignments").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectClose()

	client := NewPostgresDriverClient(db)
	if err := client.InitSchema(context.Background()); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
}

func TestPostgresDriverClient_Assign_Inserts(t *testing.T) {
	db, mock, cleanup := newDriverMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("INSERT INTO order_assignments").
		WithArgs("order-1", "driver-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectClose()

	client := NewPostgresDriverClient(db)
	if err := client.Assign(context.Background(), "order-1", "driver-1"); err != nil {
		t.Fatalf("Assign: %v", err)
	}
}

func TestPostgresDriverClient_Assign_Idempotent(t *testing.T) {
	db, mock, cleanup := newDriverMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("INSERT INTO order_assignments").
		WithArgs("order-1", "driver-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery("SELECT driver_id FROM order_assignments").
		WithArgs("order-1").
		WillReturnRows(sqlmock.NewRows([]string{"driver_id"}).AddRow("driver-1"))
	mock.ExpectClose()

	client := NewPostgresDriverClient(db)
	if err := client.Assign(context.Background(), "order-1", "driver-1"); err != nil {
		t.Fatalf("Assign: %v", err)
	}
}

func TestPostgresDriverClient_WithSchema(t *testing.T) {
	db, mock, cleanup := newDriverMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS order_assignments").
		WillReturnError(errors.New("boom"))
	mock.ExpectClose()

	client, err := NewPostgresDriverClientWithSchema(context.Background(), db)
	if err == nil {
		t.Fatalf("expected error")
	}
	if client != nil {
		t.Fatalf("expected nil client on error")
	}
}

func TestPostgresDriverClient_WithSchema_Success(t *testing.T) {
	db, mock, cleanup := newDriverMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS order_assignments").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectClose()

	client, err := NewPostgresDriverClientWithSchema(context.Background(), db)
	if err != nil {
		t.Fatalf("WithSchema: %v", err)
	}
	if client == nil {
		t.Fatalf("expected client")
	}
}

func TestPostgresDriverClient_Assign_EmptyIDs(t *testing.T) {
	client := NewPostgresDriverClient(nil)
	if err := client.Assign(context.Background(), "", "driver"); err == nil {
		t.Fatalf("expected error for empty order id")
	}
	if err := client.Assign(context.Background(), "order", ""); err == nil {
		t.Fatalf("expected error for empty driver id")
	}
}

func TestPostgresDriverClient_Assign_ConflictDifferentDriver(t *testing.T) {
	db, mock, cleanup := newDriverMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("INSERT INTO order_assignments").
		WithArgs("order-1", "driver-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery("SELECT driver_id FROM order_assignments").
		WithArgs("order-1").
		WillReturnRows(sqlmock.NewRows([]string{"driver_id"}).AddRow("driver-2"))
	mock.ExpectClose()

	client := NewPostgresDriverClient(db)
	if err := client.Assign(context.Background(), "order-1", "driver-1"); err == nil {
		t.Fatalf("expected conflict error")
	}
}

func TestPostgresDriverClient_Assign_NotFoundAfterInsert(t *testing.T) {
	db, mock, cleanup := newDriverMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("INSERT INTO order_assignments").
		WithArgs("order-1", "driver-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery("SELECT driver_id FROM order_assignments").
		WithArgs("order-1").
		WillReturnRows(sqlmock.NewRows([]string{"driver_id"}))
	mock.ExpectClose()

	client := NewPostgresDriverClient(db)
	if err := client.Assign(context.Background(), "order-1", "driver-1"); err == nil {
		t.Fatalf("expected not found error")
	}
}

func TestPostgresDriverClient_Assign_RowsAffectedError(t *testing.T) {
	db, mock, cleanup := newDriverMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("INSERT INTO order_assignments").
		WithArgs("order-err", "driver-err").
		WillReturnResult(sqlmock.NewErrorResult(errors.New("rows affected boom")))
	mock.ExpectClose()

	client := NewPostgresDriverClient(db)
	if err := client.Assign(context.Background(), "order-err", "driver-err"); err == nil {
		t.Fatalf("expected rows affected error")
	}
}
