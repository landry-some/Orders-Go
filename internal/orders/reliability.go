package orders

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"
)

// ErrCircuitOpen indicates the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker open")

// RetryPolicy controls retry behavior for outbound calls.
type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      func(time.Duration) time.Duration
	Sleep       func(context.Context, time.Duration) error
	ShouldRetry func(error) bool
}

// Do executes the function with retries according to the policy.
func (p RetryPolicy) Do(ctx context.Context, fn func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}

	attempts := p.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	sleep := p.Sleep
	if sleep == nil {
		sleep = sleepWithContext
	}
	shouldRetry := p.ShouldRetry
	if shouldRetry == nil {
		shouldRetry = func(err error) bool {
			return !errors.Is(err, context.Canceled) &&
				!errors.Is(err, context.DeadlineExceeded) &&
				!errors.Is(err, ErrCircuitOpen)
		}
	}
	jitter := p.Jitter
	if jitter == nil {
		jitter = defaultJitter
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := fn()
		if err == nil {
			return nil
		}
		if attempt == attempts || !shouldRetry(err) {
			return err
		}

		delay := p.BaseDelay
		if delay > 0 {
			delay = delay << (attempt - 1)
		}
		if p.MaxDelay > 0 && delay > p.MaxDelay {
			delay = p.MaxDelay
		}
		delay = jitter(delay)
		if delay > 0 {
			if err := sleep(ctx, delay); err != nil {
				return err
			}
		}
	}
	return nil
}

// CircuitBreakerConfig configures a circuit breaker.
type CircuitBreakerConfig struct {
	MaxFailures  int
	ResetTimeout time.Duration
	Now          func() time.Time
}

type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

// CircuitBreaker stops calls after repeated failures.
type CircuitBreaker struct {
	mu         sync.Mutex
	maxFails   int
	resetAfter time.Duration
	now        func() time.Time

	state          circuitState
	failures       int
	openedAt       time.Time
	halfOpenFlight bool
}

// NewCircuitBreaker constructs a circuit breaker with sane defaults.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	maxFails := cfg.MaxFailures
	if maxFails < 1 {
		maxFails = 1
	}
	resetAfter := cfg.ResetTimeout
	if resetAfter <= 0 {
		resetAfter = 2 * time.Second
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &CircuitBreaker{
		maxFails:   maxFails,
		resetAfter: resetAfter,
		now:        now,
		state:      circuitClosed,
	}
}

// Execute runs the given function while enforcing breaker state.
func (c *CircuitBreaker) Execute(fn func() error) error {
	if c == nil {
		return fn()
	}

	now := c.now()

	c.mu.Lock()
	switch c.state {
	case circuitOpen:
		if now.Sub(c.openedAt) < c.resetAfter {
			c.mu.Unlock()
			return ErrCircuitOpen
		}
		c.state = circuitHalfOpen
	case circuitHalfOpen:
		if c.halfOpenFlight {
			c.mu.Unlock()
			return ErrCircuitOpen
		}
	}
	if c.state == circuitHalfOpen {
		c.halfOpenFlight = true
	}
	c.mu.Unlock()

	err := fn()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state == circuitHalfOpen {
		c.halfOpenFlight = false
	}

	if err == nil {
		c.state = circuitClosed
		c.failures = 0
		return nil
	}

	if c.state == circuitHalfOpen {
		c.state = circuitOpen
		c.openedAt = now
		c.failures = 0
		return err
	}

	c.failures++
	if c.failures >= c.maxFails {
		c.state = circuitOpen
		c.openedAt = now
	}
	return err
}

// RateLimiter is a token-bucket limiter.
type RateLimiter struct {
	mu    sync.Mutex
	rate  time.Duration
	burst int
	now   func() time.Time
	sleep func(context.Context, time.Duration) error

	tokens int
	last   time.Time
}

// NewRateLimiter constructs a limiter that refills one token every rate.
func NewRateLimiter(rate time.Duration, burst int) *RateLimiter {
	limiter := &RateLimiter{
		rate:  rate,
		burst: burst,
		now:   time.Now,
		sleep: sleepWithContext,
	}
	limiter.tokens = burst
	limiter.last = limiter.now()
	return limiter
}

// Wait blocks until a token is available or the context ends.
func (r *RateLimiter) Wait(ctx context.Context) error {
	if r == nil || r.rate <= 0 || r.burst <= 0 {
		if ctx == nil {
			return nil
		}
		return ctx.Err()
	}
	if ctx == nil {
		ctx = context.Background()
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		r.mu.Lock()
		now := r.now()
		r.refill(now)
		if r.tokens > 0 {
			r.tokens--
			r.mu.Unlock()
			return nil
		}
		wait := r.rate - now.Sub(r.last)
		r.mu.Unlock()
		if wait <= 0 {
			continue
		}
		if err := r.sleep(ctx, wait); err != nil {
			return err
		}
	}
}

func (r *RateLimiter) refill(now time.Time) {
	if r.rate <= 0 {
		r.tokens = r.burst
		r.last = now
		return
	}
	elapsed := now.Sub(r.last)
	if elapsed < r.rate {
		return
	}
	add := int(elapsed / r.rate)
	if add <= 0 {
		return
	}
	r.tokens += add
	if r.tokens > r.burst {
		r.tokens = r.burst
	}
	r.last = r.last.Add(time.Duration(add) * r.rate)
}

// ReliablePaymentClient wraps a PaymentClient with reliability controls.
type ReliablePaymentClient struct {
	base    PaymentClient
	limiter *RateLimiter
	breaker *CircuitBreaker
	retry   RetryPolicy
}

// NewReliablePaymentClient constructs a reliability-wrapped payment client.
func NewReliablePaymentClient(base PaymentClient, limiter *RateLimiter, breaker *CircuitBreaker, retry RetryPolicy) *ReliablePaymentClient {
	return &ReliablePaymentClient{
		base:    base,
		limiter: limiter,
		breaker: breaker,
		retry:   retry,
	}
}

func (c *ReliablePaymentClient) Charge(ctx context.Context, orderID string, amount float64) error {
	return c.do(ctx, func() error {
		return c.base.Charge(ctx, orderID, amount)
	})
}

func (c *ReliablePaymentClient) Refund(ctx context.Context, orderID string, amount float64) error {
	return c.do(ctx, func() error {
		return c.base.Refund(ctx, orderID, amount)
	})
}

func (c *ReliablePaymentClient) do(ctx context.Context, fn func() error) error {
	attempt := func() error {
		if c.limiter != nil {
			if err := c.limiter.Wait(ctx); err != nil {
				return err
			}
		}
		if c.breaker != nil {
			return c.breaker.Execute(fn)
		}
		return fn()
	}
	return c.retry.Do(ctx, attempt)
}

// ReliableDriverClient wraps a DriverClient with reliability controls.
type ReliableDriverClient struct {
	base    DriverClient
	limiter *RateLimiter
	breaker *CircuitBreaker
	retry   RetryPolicy
}

// NewReliableDriverClient constructs a reliability-wrapped driver client.
func NewReliableDriverClient(base DriverClient, limiter *RateLimiter, breaker *CircuitBreaker, retry RetryPolicy) *ReliableDriverClient {
	return &ReliableDriverClient{
		base:    base,
		limiter: limiter,
		breaker: breaker,
		retry:   retry,
	}
}

func (c *ReliableDriverClient) Assign(ctx context.Context, orderID string, driverID string) error {
	return c.do(ctx, func() error {
		return c.base.Assign(ctx, orderID, driverID)
	})
}

func (c *ReliableDriverClient) do(ctx context.Context, fn func() error) error {
	attempt := func() error {
		if c.limiter != nil {
			if err := c.limiter.Wait(ctx); err != nil {
				return err
			}
		}
		if c.breaker != nil {
			return c.breaker.Execute(fn)
		}
		return fn()
	}
	return c.retry.Do(ctx, attempt)
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func defaultJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	half := d / 2
	return half + time.Duration(rand.Int63n(int64(half)+1))
}
