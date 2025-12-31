package ordersdb

import (
	"context"
	"database/sql"
	"fmt"
)

// PostgresDriverClient persists driver assignments in Postgres.
type PostgresDriverClient struct {
	db *sql.DB
}

// NewPostgresDriverClient constructs a driver client backed by Postgres.
func NewPostgresDriverClient(db *sql.DB) *PostgresDriverClient {
	return &PostgresDriverClient{db: db}
}

// NewPostgresDriverClientWithSchema initializes the schema then returns the client.
func NewPostgresDriverClientWithSchema(ctx context.Context, db *sql.DB) (*PostgresDriverClient, error) {
	client := NewPostgresDriverClient(db)
	if err := client.InitSchema(ctx); err != nil {
		return nil, err
	}
	return client, nil
}

// InitSchema creates the order_assignments table if it does not exist.
func (c *PostgresDriverClient) InitSchema(ctx context.Context) error {
	_, err := c.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS order_assignments (
			order_id TEXT PRIMARY KEY,
			driver_id TEXT NOT NULL,
			assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			FOREIGN KEY (order_id) REFERENCES order_sagas(order_id) ON DELETE CASCADE
		)
	`)
	return err
}

// Assign stores a driver assignment for an order.
func (c *PostgresDriverClient) Assign(orderID string, driverID string) error {
	if orderID == "" || driverID == "" {
		return fmt.Errorf("order and driver ids are required")
	}

	res, err := c.db.Exec(`INSERT INTO order_assignments (order_id, driver_id) VALUES ($1, $2) ON CONFLICT (order_id) DO NOTHING`, orderID, driverID)
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

	var existing string
	row := c.db.QueryRow(`SELECT driver_id FROM order_assignments WHERE order_id = $1`, orderID)
	switch scanErr := row.Scan(&existing); scanErr {
	case nil:
		if existing == driverID {
			return nil
		}
		return fmt.Errorf("order already assigned to different driver")
	case sql.ErrNoRows:
		return fmt.Errorf("assignment not found after insert")
	default:
		return scanErr
	}
}
