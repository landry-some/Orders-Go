package orders

import (
	"context"
	"database/sql"
	"log"
	"time"

	ordersdb "wayfinder/internal/orders/db"
)

// BuildOrderService wires an OrderService from config (Postgres DSN and logger).
// If the DSN is empty or initialization fails, it falls back to in-memory payments.
// The returned cleanup closes any external resources (e.g., DB connections).
func BuildOrderService(ctx context.Context, dsn string, logf func(format string, args ...any)) (*OrderService, func()) {
	if logf == nil {
		logf = log.Printf
	}

	cleanup := func() {}
	var payments PaymentClient = NewInMemoryPaymentClient()

	if dsn != "" {
		sqlDB, err := sql.Open("pgx", dsn)
		if err != nil {
			logf("postgres open failed, falling back to in-memory payments: %v", err)
		} else {
			setupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			client, err := ordersdb.NewPostgresPaymentClientWithSchema(setupCtx, sqlDB)
			if err != nil {
				logf("postgres init failed, falling back to in-memory payments: %v", err)
				_ = sqlDB.Close()
			} else {
				logf("postgres payments enabled")
				payments = client
				cleanup = func() {
					if err := sqlDB.Close(); err != nil {
						logf("close postgres: %v", err)
					}
				}
			}
		}
	}

	return NewOrderService(
			payments,
			NewInMemoryDriverClient(),
			func() string { return "order-" + time.Now().Format("20060102150405.000000000") },
			func() string { return "driver-" + time.Now().Format("150405") },
		),
		cleanup
}
