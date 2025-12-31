package orders

import (
	"context"
	"errors"
	"testing"
	"time"
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
