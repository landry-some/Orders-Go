package saga

import (
	"context"
	"errors"
)

// SagaStatus captures the current state of an order saga.
type SagaStatus string

const (
	SagaStatusStarted   SagaStatus = "started"
	SagaStatusSucceeded SagaStatus = "succeeded"
	SagaStatusFailed    SagaStatus = "failed"
	SagaStatusRefunded  SagaStatus = "refunded"
)

// SagaRecord represents a stored saga entry.
type SagaRecord struct {
	OrderID string
	UserID  string
	Amount  float64
	Status  SagaStatus
}

// SagaStore persists idempotency keys and saga steps.
type SagaStore interface {
	Start(ctx context.Context, idempotencyKey, orderID, userID string, amount float64) (SagaRecord, bool, error)
	UpdateStatus(ctx context.Context, orderID string, status SagaStatus) error
	AddStep(ctx context.Context, orderID, step, status, detail string) error
}

var ErrIdempotencyConflict = errors.New("idempotency key reused with different payload")
