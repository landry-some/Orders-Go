package ordersdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"wayfinder/internal/orders/saga"
)

// SagaStore persists idempotency keys and saga steps in Postgres.
type SagaStore struct {
	db *sql.DB
}

// NewSagaStore constructs a SagaStore backed by Postgres.
func NewSagaStore(db *sql.DB) *SagaStore {
	return &SagaStore{db: db}
}

// NewSagaStoreWithSchema initializes the schema then returns the store.
func NewSagaStoreWithSchema(ctx context.Context, db *sql.DB) (*SagaStore, error) {
	store := NewSagaStore(db)
	if err := store.InitSchema(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

// InitSchema creates saga tables if they do not exist.
func (s *SagaStore) InitSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS order_sagas (
			order_id TEXT PRIMARY KEY,
			idempotency_key TEXT UNIQUE NOT NULL,
			user_id TEXT NOT NULL,
			amount DOUBLE PRECISION NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS order_saga_steps (
			id BIGSERIAL PRIMARY KEY,
			order_id TEXT NOT NULL,
			step TEXT NOT NULL,
			status TEXT NOT NULL,
			detail TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			FOREIGN KEY (order_id) REFERENCES order_sagas(order_id) ON DELETE CASCADE
		)`,
	}

	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

// Start inserts a new saga or returns the existing one for the idempotency key.
func (s *SagaStore) Start(ctx context.Context, idempotencyKey, orderID, userID string, amount float64) (saga.SagaRecord, bool, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO order_sagas (order_id, idempotency_key, user_id, amount, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (idempotency_key) DO NOTHING`,
		orderID, idempotencyKey, userID, amount, saga.SagaStatusStarted,
	)
	if err != nil {
		return saga.SagaRecord{}, false, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return saga.SagaRecord{}, false, err
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT order_id, user_id, amount, status
		FROM order_sagas
		WHERE idempotency_key = $1`,
		idempotencyKey,
	)

	var record saga.SagaRecord
	var status string
	if err := row.Scan(&record.OrderID, &record.UserID, &record.Amount, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return saga.SagaRecord{}, false, fmt.Errorf("saga not found after insert")
		}
		return saga.SagaRecord{}, false, err
	}
	record.Status = saga.SagaStatus(status)

	if record.UserID != userID || record.Amount != amount {
		return saga.SagaRecord{}, false, saga.ErrIdempotencyConflict
	}

	return record, affected == 1, nil
}

// UpdateStatus updates the saga's status and timestamp.
func (s *SagaStore) UpdateStatus(ctx context.Context, orderID string, status saga.SagaStatus) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE order_sagas
		SET status = $2, updated_at = NOW()
		WHERE order_id = $1`,
		orderID, status,
	)
	return err
}

// AddStep appends a saga step row.
func (s *SagaStore) AddStep(ctx context.Context, orderID, step, status, detail string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO order_saga_steps (order_id, step, status, detail)
		VALUES ($1, $2, $3, $4)`,
		orderID, step, status, detail,
	)
	return err
}
