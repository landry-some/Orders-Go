package config

import (
	"testing"
	"time"
)

func TestLoadGRPC(t *testing.T) {
	t.Setenv("GRPC_RATE_LIMIT_INTERVAL", "5ms")
	t.Setenv("GRPC_RATE_LIMIT_BURST", "10")

	cfg, err := LoadGRPC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RateLimitInterval != 5*time.Millisecond || cfg.RateLimitBurst != 10 {
		t.Fatalf("unexpected grpc cfg: %+v", cfg)
	}
}

func TestLoadObservability(t *testing.T) {
	t.Setenv("OBS_ADDR", ":9999")

	cfg, err := LoadObservability()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Addr != ":9999" {
		t.Fatalf("unexpected observability addr: %+v", cfg)
	}
}

func TestLoadRedis(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("REDIS_STREAM", "s")
	t.Setenv("REDIS_HEALTHCHECK_TIMEOUT", "2s")
	t.Setenv("REDIS_LOCATION_TTL", "10m")
	t.Setenv("REDIS_STREAM_MAXLEN", "1000")

	cfg, err := LoadRedis()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.URL != "redis://localhost:6379/0" {
		t.Fatalf("unexpected redis url: %s", cfg.URL)
	}
	if cfg.Stream != "s" {
		t.Fatalf("unexpected stream: %s", cfg.Stream)
	}
	if cfg.HealthcheckTimeout != 2*time.Second {
		t.Fatalf("unexpected healthcheck timeout: %v", cfg.HealthcheckTimeout)
	}
	if cfg.LocationTTL != 10*time.Minute {
		t.Fatalf("unexpected location ttl: %v", cfg.LocationTTL)
	}
	if cfg.StreamMaxLen != 1000 {
		t.Fatalf("unexpected stream maxlen: %d", cfg.StreamMaxLen)
	}
}

func TestLoadGRPCMissingEnv(t *testing.T) {
	t.Setenv("GRPC_RATE_LIMIT_INTERVAL", "")
	if _, err := LoadGRPC(); err == nil {
		t.Fatalf("expected error for missing interval")
	}
}
