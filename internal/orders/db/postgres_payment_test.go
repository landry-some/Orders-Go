package db

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
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

func TestPostgresPayment_InitSchema(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS payments").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectClose()

	client := NewPostgresPaymentClient(db)
	if err := client.InitSchema(context.Background()); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
}

func TestPostgresPayment_WithSchemaHelper(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS payments").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectClose()

	client, err := NewPostgresPaymentClientWithSchema(context.Background(), db)
	if err != nil {
		t.Fatalf("helper: %v", err)
	}
	if client == nil {
		t.Fatalf("expected client")
	}
}

func TestPostgresPayment_Charge_SucceedsOnce(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("INSERT INTO payments").
		WithArgs("order-1", 9.99).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec("INSERT INTO payments").
		WithArgs("order-1", 9.99).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectClose()

	client := NewPostgresPaymentClient(db)

	if err := client.Charge("order-1", 9.99); err != nil {
		t.Fatalf("first charge: %v", err)
	}

	err := client.Charge("order-1", 9.99)
	if err == nil {
		t.Fatalf("expected duplicate charge error")
	}
	if err != ErrAlreadyCharged {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPostgresPayment_Refund_Succeeds(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("UPDATE payments SET refund_amount").
		WithArgs("order-1", 9.99).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectClose()

	client := NewPostgresPaymentClient(db)

	if err := client.Refund("order-1", 9.99); err != nil {
		t.Fatalf("refund: %v", err)
	}
}

func TestPostgresPayment_Refund_NotCharged(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("UPDATE payments SET refund_amount").
		WithArgs("order-404", 5.0).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery("SELECT refunded_at").
		WithArgs("order-404").
		WillReturnRows(sqlmock.NewRows([]string{"refunded"}))
	mock.ExpectClose()

	client := NewPostgresPaymentClient(db)

	err := client.Refund("order-404", 5.0)
	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrNotCharged {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPostgresPayment_Refund_AlreadyRefunded(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("UPDATE payments SET refund_amount").
		WithArgs("order-1", 9.99).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery("SELECT refunded_at").
		WithArgs("order-1").
		WillReturnRows(sqlmock.NewRows([]string{"refunded"}).AddRow(true))
	mock.ExpectClose()

	client := NewPostgresPaymentClient(db)

	err := client.Refund("order-1", 9.99)
	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrAlreadyRefunded {
		t.Fatalf("unexpected error: %v", err)
	}
}
