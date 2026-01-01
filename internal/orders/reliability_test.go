package orders

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

type stubPayment struct {
	errs  []error
	calls int
}

func (s *stubPayment) Charge(ctx context.Context, orderID string, amount float64) error {
	s.calls++
	if s.calls <= len(s.errs) {
		return s.errs[s.calls-1]
	}
	return nil
}

func (s *stubPayment) Refund(ctx context.Context, orderID string, amount float64) error {
	s.calls++
	if s.calls <= len(s.errs) {
		return s.errs[s.calls-1]
	}
	return nil
}

type stubDriver struct {
	err   error
	calls int
}

type stubLimiter struct {
	err    error
	called bool
}

func (s *stubLimiter) Wait(context.Context) error {
	s.called = true
	return s.err
}

func (s *stubDriver) Assign(ctx context.Context, orderID string, driverID string) error {
	s.calls++
	return s.err
}

func TestRetryPolicy_RetriesWithBackoff(t *testing.T) {
	attempts := 0
	var delays []time.Duration

	policy := RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    50 * time.Millisecond,
		Jitter:      func(d time.Duration) time.Duration { return d },
		Sleep: func(ctx context.Context, d time.Duration) error {
			delays = append(delays, d)
			return nil
		},
		ShouldRetry: func(error) bool { return true },
	}

	err := policy.Do(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return errors.New("fail")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if len(delays) != 2 || delays[0] != 10*time.Millisecond || delays[1] != 20*time.Millisecond {
		t.Fatalf("unexpected delays: %v", delays)
	}
}

func TestRetryPolicy_StopsOnNonRetryable(t *testing.T) {
	attempts := 0
	var delays []time.Duration
	expected := errors.New("nope")

	policy := RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		Sleep: func(ctx context.Context, d time.Duration) error {
			delays = append(delays, d)
			return nil
		},
		ShouldRetry: func(error) bool { return false },
	}

	err := policy.Do(context.Background(), func() error {
		attempts++
		return expected
	})
	if err != expected {
		t.Fatalf("expected %v, got %v", expected, err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
	if len(delays) != 0 {
		t.Fatalf("expected no delay, got %v", delays)
	}
}

func TestCircuitBreaker_OpensAndResets(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	calls := 0

	breaker := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:  2,
		ResetTimeout: time.Second,
		Now:          func() time.Time { return now },
	})

	fail := func() error {
		calls++
		return errors.New("fail")
	}

	if err := breaker.Execute(fail); err == nil {
		t.Fatalf("expected failure")
	}
	if err := breaker.Execute(fail); err == nil {
		t.Fatalf("expected failure")
	}

	if err := breaker.Execute(func() error { return nil }); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected circuit open error, got %v", err)
	}

	now = now.Add(2 * time.Second)

	if err := breaker.Execute(func() error { return nil }); err != nil {
		t.Fatalf("expected breaker to allow trial, got %v", err)
	}
	if err := breaker.Execute(func() error { return nil }); err != nil {
		t.Fatalf("expected breaker to close, got %v", err)
	}

	if calls != 2 {
		t.Fatalf("expected 2 failed calls, got %d", calls)
	}
}

func TestRateLimiter_WaitsWhenExhausted(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	var waits []time.Duration

	limiter := NewRateLimiter(100*time.Millisecond, 1)
	limiter.now = func() time.Time { return now }
	limiter.last = now
	limiter.sleep = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		now = now.Add(d)
		return nil
	}

	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(waits) != 1 || waits[0] != 100*time.Millisecond {
		t.Fatalf("expected one wait of 100ms, got %v", waits)
	}
}

func TestReliablePaymentClient_ChargeRetries(t *testing.T) {
	base := &stubPayment{errs: []error{errors.New("fail"), nil}}
	policy := RetryPolicy{
		MaxAttempts: 2,
		BaseDelay:   1 * time.Millisecond,
		Jitter:      func(d time.Duration) time.Duration { return d },
		Sleep:       func(context.Context, time.Duration) error { return nil },
		ShouldRetry: func(error) bool { return true },
	}

	client := NewReliablePaymentClient(base, nil, nil, policy)
	if err := client.Charge(context.Background(), "order-1", 9.99); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if base.calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", base.calls)
	}
}

func TestReliableDriverClient_AssignCircuitOpen(t *testing.T) {
	base := &stubDriver{err: errors.New("fail")}
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	breaker := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:  1,
		ResetTimeout: time.Second,
		Now:          func() time.Time { return now },
	})
	policy := RetryPolicy{
		MaxAttempts: 1,
		ShouldRetry: func(error) bool { return false },
	}

	client := NewReliableDriverClient(base, nil, breaker, policy)
	if err := client.Assign(context.Background(), "order-1", "driver-1"); err == nil {
		t.Fatalf("expected failure")
	}
	if err := client.Assign(context.Background(), "order-1", "driver-1"); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected circuit open, got %v", err)
	}
	if base.calls != 1 {
		t.Fatalf("expected 1 call, got %d", base.calls)
	}
}

func TestRetryPolicy_RespectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	policy := RetryPolicy{
		MaxAttempts: 3,
		ShouldRetry: func(error) bool { called = true; return true },
	}
	if err := policy.Do(ctx, func() error { return errors.New("boom") }); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled, got %v", err)
	}
	if called {
		t.Fatalf("should not evaluate ShouldRetry when context canceled")
	}
}

func TestRetryPolicy_CapsMaxDelayAndDefaultShouldRetry(t *testing.T) {
	var delays []time.Duration
	policy := RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    15 * time.Millisecond,
		Jitter:      func(d time.Duration) time.Duration { return d },
		Sleep: func(ctx context.Context, d time.Duration) error {
			delays = append(delays, d)
			return nil
		},
	}

	err := policy.Do(context.Background(), func() error { return ErrCircuitOpen })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected circuit open to stop retries, got %v", err)
	}

	policy.ShouldRetry = func(error) bool { return true }
	if err := policy.Do(context.Background(), func() error { return errors.New("fail") }); err == nil {
		t.Fatalf("expected failure")
	}
	if len(delays) != 2 || delays[0] != 10*time.Millisecond || delays[1] != 15*time.Millisecond {
		t.Fatalf("expected capped delays, got %v", delays)
	}
}

func TestRateLimiter_RefillsAfterElapsed(t *testing.T) {
	rate := 10 * time.Millisecond
	limiter := NewRateLimiter(rate, 2)
	start := time.Unix(0, 0)
	limiter.last = start
	limiter.tokens = 0
	limiter.now = func() time.Time { return start.Add(2 * rate) }
	limiter.sleep = func(context.Context, time.Duration) error { return nil }

	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("expected wait to succeed: %v", err)
	}
	if limiter.tokens != 1 {
		t.Fatalf("expected tokens to refill to 1 after consume, got %d", limiter.tokens)
	}
}

func TestRateLimiter_RefillNoAdvanceAndZeroRate(t *testing.T) {
	limiter := NewRateLimiter(10*time.Millisecond, 3)
	limiter.tokens = 1
	limiter.last = time.Unix(0, 0)
	before := limiter.tokens
	limiter.refill(time.Unix(0, 0).Add(5 * time.Millisecond))
	if limiter.tokens != before {
		t.Fatalf("expected no refill when elapsed < rate, got %d", limiter.tokens)
	}

	limiter.rate = 0
	limiter.tokens = 0
	limiter.refill(time.Unix(0, 1))
	if limiter.tokens != limiter.burst {
		t.Fatalf("expected tokens reset to burst on zero rate, got %d", limiter.tokens)
	}
}

func TestRateLimiter_WaitCancelledDuringSleep(t *testing.T) {
	limiter := NewRateLimiterWithHook(10*time.Millisecond, 1, nil)
	limiter.tokens = 0
	limiter.last = time.Unix(0, 0)
	limiter.now = func() time.Time { return time.Unix(0, 0) }
	limiter.sleep = func(ctx context.Context, d time.Duration) error { return context.DeadlineExceeded }

	if err := limiter.Wait(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestRateLimiter_RefillCapsBurst(t *testing.T) {
	limiter := NewRateLimiter(10*time.Millisecond, 2)
	limiter.tokens = 1
	limiter.last = time.Unix(0, 0)
	limiter.refill(time.Unix(0, 0).Add(10 * time.Millisecond * 5))
	if limiter.tokens != 2 {
		t.Fatalf("expected tokens capped at burst, got %d", limiter.tokens)
	}
}

func TestReliablePaymentClient_RefundRetries(t *testing.T) {
	base := &stubPayment{errs: []error{errors.New("fail"), nil}}
	policy := RetryPolicy{
		MaxAttempts: 2,
		BaseDelay:   1 * time.Millisecond,
		Jitter:      func(d time.Duration) time.Duration { return d },
		Sleep:       func(context.Context, time.Duration) error { return nil },
		ShouldRetry: func(error) bool { return true },
	}

	client := NewReliablePaymentClient(base, nil, nil, policy)
	if err := client.Refund(context.Background(), "order-1", 9.99); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if base.calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", base.calls)
	}
}

func TestReliableDriverClient_LimiterErrorStops(t *testing.T) {
	base := &stubDriver{}
	limiter := NewRateLimiter(10*time.Millisecond, 1)
	limiter.tokens = 0
	limiter.last = time.Unix(0, 0)
	limiter.now = func() time.Time { return time.Unix(0, 0) }
	limiter.sleep = func(context.Context, time.Duration) error { return context.DeadlineExceeded }
	client := ReliableDriverClient{
		base:    base,
		limiter: limiter,
		retry: RetryPolicy{
			MaxAttempts: 1,
		},
	}

	if err := client.Assign(context.Background(), "order-1", "driver-1"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected limiter error, got %v", err)
	}
	if base.calls != 0 {
		t.Fatalf("expected no driver call when limiter fails, got %d", base.calls)
	}
}

func TestRateLimiter_NilOrZeroRate(t *testing.T) {
	var limiter *RateLimiter
	if err := limiter.Wait(nil); err != nil {
		t.Fatalf("nil limiter with nil ctx should be nil, got %v", err)
	}

	limiter = NewRateLimiter(0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := limiter.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled, got %v", err)
	}
}

func TestSleepWithContext(t *testing.T) {
	if err := sleepWithContext(context.Background(), 0); err != nil {
		t.Fatalf("expected nil for zero duration, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepWithContext(ctx, 10*time.Millisecond); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled, got %v", err)
	}
}

func TestDefaultJitterBounds(t *testing.T) {
	d := 100 * time.Millisecond
	got := defaultJitter(d)
	if got < d/2 || got > d {
		t.Fatalf("expected jitter in [%v,%v], got %v", d/2, d, got)
	}
	if defaultJitter(0) != 0 {
		t.Fatalf("expected zero jitter for non-positive delay")
	}
}

func TestLoadReliabilityConfigFromEnv(t *testing.T) {
	t.Setenv("ORDER_RETRY_MAX_ATTEMPTS", "3")
	t.Setenv("ORDER_RETRY_BASE_DELAY", "10ms")
	t.Setenv("ORDER_RETRY_MAX_DELAY", "1s")
	t.Setenv("ORDER_BREAKER_MAX_FAILURES", "2")
	t.Setenv("ORDER_BREAKER_RESET_TIMEOUT", "500ms")
	t.Setenv("ORDER_RATE_LIMIT_INTERVAL", "5ms")
	t.Setenv("ORDER_RATE_LIMIT_BURST", "10")

	cfg, err := loadReliabilityConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RetryMaxAttempts != 3 || cfg.RateLimitBurst != 10 {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoadReliabilityConfigFromEnv_Missing(t *testing.T) {
	t.Setenv("ORDER_RETRY_MAX_ATTEMPTS", "")
	if _, err := loadReliabilityConfigFromEnv(); err == nil {
		t.Fatalf("expected error on missing env")
	}
	t.Setenv("ORDER_RETRY_MAX_ATTEMPTS", "3")
	t.Setenv("ORDER_RETRY_BASE_DELAY", "")
	if _, err := loadReliabilityConfigFromEnv(); err == nil {
		t.Fatalf("expected error on missing base delay")
	}
}

func TestParseRequiredHelpers(t *testing.T) {
	t.Setenv("ORDER_RETRY_BASE_DELAY", "-1ms")
	if _, err := parseRequiredDuration("ORDER_RETRY_BASE_DELAY"); err == nil {
		t.Fatalf("expected negative duration error")
	}
	t.Setenv("ORDER_RATE_LIMIT_BURST", "-1")
	if _, err := parseRequiredInt("ORDER_RATE_LIMIT_BURST"); err == nil {
		t.Fatalf("expected negative int error")
	}
}

func TestBuildOrderServiceRequiresDSN(t *testing.T) {
	if _, _, err := BuildOrderService(context.Background(), "", nil); err == nil {
		t.Fatalf("expected error for missing dsn")
	}
}

func TestBuildOrderServiceOpenError(t *testing.T) {
	origOpen := openOrderDB
	defer func() { openOrderDB = origOpen }()

	openOrderDB = func(driver, dsn string) (*sql.DB, error) {
		return nil, errors.New("open fail")
	}

	if _, _, err := BuildOrderService(context.Background(), "dsn", nil); err == nil {
		t.Fatalf("expected open failure")
	}
}

func TestBuildOrderServiceSuccessWithMocks(t *testing.T) {
	origOpen := openOrderDB
	defer func() { openOrderDB = origOpen }()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	// Close expectation handled via cleanup from BuildOrderService.

	openOrderDB = func(driver, dsn string) (*sql.DB, error) {
		return db, nil
	}

	// Schema expectations.
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS payments").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS order_sagas").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS order_saga_steps").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS order_assignments").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Required reliability env.
	t.Setenv("ORDER_RETRY_MAX_ATTEMPTS", "1")
	t.Setenv("ORDER_RETRY_BASE_DELAY", "1ms")
	t.Setenv("ORDER_RETRY_MAX_DELAY", "1ms")
	t.Setenv("ORDER_BREAKER_MAX_FAILURES", "1")
	t.Setenv("ORDER_BREAKER_RESET_TIMEOUT", "1s")
	t.Setenv("ORDER_RATE_LIMIT_INTERVAL", "1ms")
	t.Setenv("ORDER_RATE_LIMIT_BURST", "1")

	svc, cleanup, err := BuildOrderService(context.Background(), "dsn", nil)
	if err != nil {
		t.Fatalf("BuildOrderService: %v", err)
	}
	if svc == nil {
		t.Fatalf("expected service")
	}
	if cleanup == nil {
		t.Fatalf("expected cleanup func")
	}
	cleanup()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestBuildOrderServiceSchemaErrorClosesDB(t *testing.T) {
	origOpen := openOrderDB
	defer func() { openOrderDB = origOpen }()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}

	openOrderDB = func(driver, dsn string) (*sql.DB, error) {
		return db, nil
	}

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS payments").
		WillReturnError(errors.New("schema fail"))
	mock.ExpectClose()

	t.Setenv("ORDER_RETRY_MAX_ATTEMPTS", "1")
	t.Setenv("ORDER_RETRY_BASE_DELAY", "1ms")
	t.Setenv("ORDER_RETRY_MAX_DELAY", "1ms")
	t.Setenv("ORDER_BREAKER_MAX_FAILURES", "1")
	t.Setenv("ORDER_BREAKER_RESET_TIMEOUT", "1s")
	t.Setenv("ORDER_RATE_LIMIT_INTERVAL", "1ms")
	t.Setenv("ORDER_RATE_LIMIT_BURST", "1")

	if _, _, err := BuildOrderService(context.Background(), "dsn", nil); err == nil {
		t.Fatalf("expected schema failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
