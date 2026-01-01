package orders

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	ordersdb "wayfinder/internal/db/orders"
)

func BuildOrderService(ctx context.Context, dsn string, logf func(format string, args ...any)) (*OrderService, func(), error) {
	if logf == nil {
		logf = log.Printf
	}

	if dsn == "" {
		return nil, nil, fmt.Errorf("DATABASE_URL is required")
	}

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("postgres open failed: %w", err)
	}

	setupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	payments, err := ordersdb.NewPostgresPaymentClientWithSchema(setupCtx, sqlDB)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("postgres init failed: %w", err)
	}

	sagas, err := ordersdb.NewSagaStoreWithSchema(setupCtx, sqlDB)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("saga store init failed: %w", err)
	}

	drivers, err := ordersdb.NewPostgresDriverClientWithSchema(setupCtx, sqlDB)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("driver store init failed: %w", err)
	}

	reliabilityCfg, err := loadReliabilityConfigFromEnv()
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("reliability config: %w", err)
	}

	retryPolicy := RetryPolicy{
		MaxAttempts: reliabilityCfg.RetryMaxAttempts,
		BaseDelay:   reliabilityCfg.RetryBaseDelay,
		MaxDelay:    reliabilityCfg.RetryMaxDelay,
	}
	paymentLimiter := NewRateLimiter(reliabilityCfg.RateLimitInterval, reliabilityCfg.RateLimitBurst)
	driverLimiter := NewRateLimiter(reliabilityCfg.RateLimitInterval, reliabilityCfg.RateLimitBurst)
	paymentBreaker := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:  reliabilityCfg.BreakerMaxFailures,
		ResetTimeout: reliabilityCfg.BreakerResetTimeout,
	})
	driverBreaker := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:  reliabilityCfg.BreakerMaxFailures,
		ResetTimeout: reliabilityCfg.BreakerResetTimeout,
	})

	reliablePayments := NewReliablePaymentClient(payments, paymentLimiter, paymentBreaker, retryPolicy)
	reliableDrivers := NewReliableDriverClient(drivers, driverLimiter, driverBreaker, retryPolicy)

	cleanup := func() {
		if err := sqlDB.Close(); err != nil {
			logf("close postgres: %v", err)
		}
	}

	return NewOrderService(
		reliablePayments,
		reliableDrivers,
		sagas,
		newOrderID,
		newDriverID,
	), cleanup, nil
}
