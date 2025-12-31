package ingestdb

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"wayfinder/internal/ingest"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func newLocationMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
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

func TestPostgresLocationStore_InitSchema(t *testing.T) {
	db, mock, cleanup := newLocationMockDB(t)
	t.Cleanup(cleanup)

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS driver_locations").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectClose()

	store := NewPostgresLocationStore(db)
	if err := store.InitSchema(context.Background()); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
}

func TestPostgresLocationStore_Update_InsertsRow(t *testing.T) {
	db, mock, cleanup := newLocationMockDB(t)
	t.Cleanup(cleanup)

	ts := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	mock.ExpectExec("INSERT INTO driver_locations").
		WithArgs("driver-1", 1.23, 4.56, ts).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectClose()

	store := NewPostgresLocationStore(db)
	loc := ingest.Location{
		DriverID:  "driver-1",
		Lat:       1.23,
		Long:      4.56,
		Timestamp: ts,
	}
	if err := store.Update(context.Background(), loc); err != nil {
		t.Fatalf("Update: %v", err)
	}
}
