package ordersdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// PostgresPaymentClient persists charges and refunds in Postgres.
type PostgresPaymentClient struct {
	db *sql.DB
}

// NewPostgresPaymentClient constructs a PaymentClient backed by Postgres.
func NewPostgresPaymentClient(db *sql.DB) *PostgresPaymentClient {
	return &PostgresPaymentClient{db: db}
}

// NewPostgresPaymentClientWithSchema initializes the schema then returns the client.
func NewPostgresPaymentClientWithSchema(ctx context.Context, db *sql.DB) (*PostgresPaymentClient, error) {
	client := NewPostgresPaymentClient(db)
	if err := client.InitSchema(ctx); err != nil {
		return nil, err
	}
	return client, nil
}

// InitSchema creates the payments table if it does not exist.
func (p *PostgresPaymentClient) InitSchema(ctx context.Context) error {
	_, err := p.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS payments (
			order_id TEXT PRIMARY KEY,
			amount DOUBLE PRECISION NOT NULL,
			charged_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			refunded_at TIMESTAMPTZ,
			refund_amount DOUBLE PRECISION
		)
	`)
	return err
}

// ErrAlreadyCharged signals an order has already been charged.
var ErrAlreadyCharged = errors.New("order already charged")

// ErrNotCharged signals an order has no recorded charge.
var ErrNotCharged = errors.New("order not charged")

// ErrAlreadyRefunded signals an order has already been refunded.
var ErrAlreadyRefunded = errors.New("order already refunded")

func (p *PostgresPaymentClient) Charge(orderID string, amount float64) error {
	if orderID == "" {
		return fmt.Errorf("order id required")
	}

	res, err := p.db.Exec(`INSERT INTO payments (order_id, amount) VALUES ($1, $2) ON CONFLICT (order_id) DO NOTHING`, orderID, amount)
	if err != nil {
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrAlreadyCharged
	}

	return nil
}

func (p *PostgresPaymentClient) Refund(orderID string, amount float64) error {
	if orderID == "" {
		return fmt.Errorf("order id required")
	}

	res, err := p.db.Exec(`UPDATE payments SET refund_amount = $2, refunded_at = NOW() WHERE order_id = $1 AND refunded_at IS NULL`, orderID, amount)
	if err != nil {
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}

	var refunded bool
	row := p.db.QueryRow(`SELECT refunded_at IS NOT NULL FROM payments WHERE order_id = $1`, orderID)
	switch scanErr := row.Scan(&refunded); scanErr {
	case nil:
		if refunded {
			return ErrAlreadyRefunded
		}
		return ErrNotCharged
	case sql.ErrNoRows:
		return ErrNotCharged
	default:
		return scanErr
	}
}
