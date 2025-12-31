package orders

import (
	"testing"
	"time"
)

func TestLoadReliabilityConfigFromEnv_Parses(t *testing.T) {
	t.Setenv("ORDER_RETRY_MAX_ATTEMPTS", "3")
	t.Setenv("ORDER_RETRY_BASE_DELAY", "50ms")
	t.Setenv("ORDER_RETRY_MAX_DELAY", "500ms")
	t.Setenv("ORDER_BREAKER_MAX_FAILURES", "4")
	t.Setenv("ORDER_BREAKER_RESET_TIMEOUT", "2s")
	t.Setenv("ORDER_RATE_LIMIT_INTERVAL", "1ms")
	t.Setenv("ORDER_RATE_LIMIT_BURST", "100")

	cfg, err := loadReliabilityConfigFromEnv()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.RetryMaxAttempts != 3 {
		t.Fatalf("expected retry attempts 3, got %d", cfg.RetryMaxAttempts)
	}
	if cfg.RetryBaseDelay != 50*time.Millisecond {
		t.Fatalf("expected retry base delay 50ms, got %v", cfg.RetryBaseDelay)
	}
	if cfg.RetryMaxDelay != 500*time.Millisecond {
		t.Fatalf("expected retry max delay 500ms, got %v", cfg.RetryMaxDelay)
	}
	if cfg.BreakerMaxFailures != 4 {
		t.Fatalf("expected breaker failures 4, got %d", cfg.BreakerMaxFailures)
	}
	if cfg.BreakerResetTimeout != 2*time.Second {
		t.Fatalf("expected breaker reset 2s, got %v", cfg.BreakerResetTimeout)
	}
	if cfg.RateLimitInterval != time.Millisecond {
		t.Fatalf("expected rate interval 1ms, got %v", cfg.RateLimitInterval)
	}
	if cfg.RateLimitBurst != 100 {
		t.Fatalf("expected rate burst 100, got %d", cfg.RateLimitBurst)
	}
}

func TestLoadReliabilityConfigFromEnv_Missing(t *testing.T) {
	if _, err := loadReliabilityConfigFromEnv(); err == nil {
		t.Fatalf("expected missing env error")
	}
}
