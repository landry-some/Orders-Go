package ingestdb

import (
	"context"
	"database/sql"

	"wayfinder/internal/ingest"
)

// PostgresLocationStore persists location history in Postgres.
type PostgresLocationStore struct {
	db *sql.DB
}

// NewPostgresLocationStore constructs a location store backed by Postgres.
func NewPostgresLocationStore(db *sql.DB) *PostgresLocationStore {
	return &PostgresLocationStore{db: db}
}

// NewPostgresLocationStoreWithSchema initializes the schema then returns the store.
func NewPostgresLocationStoreWithSchema(ctx context.Context, db *sql.DB) (*PostgresLocationStore, error) {
	store := NewPostgresLocationStore(db)
	if err := store.InitSchema(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

// InitSchema creates the driver_locations table if it does not exist.
func (s *PostgresLocationStore) InitSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS driver_locations (
			id BIGSERIAL PRIMARY KEY,
			driver_id TEXT NOT NULL,
			lat DOUBLE PRECISION NOT NULL,
			long DOUBLE PRECISION NOT NULL,
			recorded_at TIMESTAMPTZ NOT NULL
		)
	`)
	return err
}

// Update inserts a new location history row.
func (s *PostgresLocationStore) Update(ctx context.Context, loc ingest.Location) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO driver_locations (driver_id, lat, long, recorded_at)
		VALUES ($1, $2, $3, $4)
	`, loc.DriverID, loc.Lat, loc.Long, loc.Timestamp.UTC())
	return err
}
