package orders

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type ReliabilityConfig struct {
	RetryMaxAttempts    int
	RetryBaseDelay      time.Duration
	RetryMaxDelay       time.Duration
	BreakerMaxFailures  int
	BreakerResetTimeout time.Duration
	RateLimitInterval   time.Duration
	RateLimitBurst      int
}

func loadReliabilityConfigFromEnv() (ReliabilityConfig, error) {
	cfg := ReliabilityConfig{}
	var err error

	if cfg.RetryMaxAttempts, err = parseRequiredInt("ORDER_RETRY_MAX_ATTEMPTS"); err != nil {
		return cfg, err
	}
	if cfg.RetryBaseDelay, err = parseRequiredDuration("ORDER_RETRY_BASE_DELAY"); err != nil {
		return cfg, err
	}
	if cfg.RetryMaxDelay, err = parseRequiredDuration("ORDER_RETRY_MAX_DELAY"); err != nil {
		return cfg, err
	}
	if cfg.BreakerMaxFailures, err = parseRequiredInt("ORDER_BREAKER_MAX_FAILURES"); err != nil {
		return cfg, err
	}
	if cfg.BreakerResetTimeout, err = parseRequiredDuration("ORDER_BREAKER_RESET_TIMEOUT"); err != nil {
		return cfg, err
	}
	if cfg.RateLimitInterval, err = parseRequiredDuration("ORDER_RATE_LIMIT_INTERVAL"); err != nil {
		return cfg, err
	}
	if cfg.RateLimitBurst, err = parseRequiredInt("ORDER_RATE_LIMIT_BURST"); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func parseRequiredDuration(name string) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	val, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if val < 0 {
		return 0, errors.New(name + " must be >= 0")
	}
	return val, nil
}

func parseRequiredInt(name string) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if val < 0 {
		return 0, errors.New(name + " must be >= 0")
	}
	return val, nil
}
