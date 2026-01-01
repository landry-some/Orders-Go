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

func TestLoadRedis_WithOptionalFields(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("REDIS_HEALTHCHECK_TIMEOUT", "1s")
	t.Setenv("REDIS_LOCATION_TTL", "1m")
	t.Setenv("REDIS_STREAM_MAXLEN", "10")
	t.Setenv("REDIS_DIAL_TIMEOUT", "3s")
	t.Setenv("REDIS_READ_TIMEOUT", "4s")
	t.Setenv("REDIS_WRITE_TIMEOUT", "5s")
	t.Setenv("REDIS_POOL_SIZE", "9")
	t.Setenv("REDIS_MIN_IDLE_CONNS", "2")
	t.Setenv("REDIS_MAX_RETRIES", "3")
	t.Setenv("REDIS_OTEL", "true")

	cfg, err := LoadRedis()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DialTimeout == nil || *cfg.DialTimeout != 3*time.Second {
		t.Fatalf("unexpected dial timeout: %v", cfg.DialTimeout)
	}
	if cfg.ReadTimeout == nil || *cfg.ReadTimeout != 4*time.Second {
		t.Fatalf("unexpected read timeout: %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout == nil || *cfg.WriteTimeout != 5*time.Second {
		t.Fatalf("unexpected write timeout: %v", cfg.WriteTimeout)
	}
	if cfg.PoolSize == nil || *cfg.PoolSize != 9 {
		t.Fatalf("unexpected pool size: %v", cfg.PoolSize)
	}
	if cfg.MinIdleConns == nil || *cfg.MinIdleConns != 2 {
		t.Fatalf("unexpected min idle: %v", cfg.MinIdleConns)
	}
	if cfg.MaxRetries == nil || *cfg.MaxRetries != 3 {
		t.Fatalf("unexpected max retries: %v", cfg.MaxRetries)
	}
	if !cfg.EnableOTel {
		t.Fatalf("expected otel enabled")
	}
}

func TestLoadGRPCMissingEnv(t *testing.T) {
	t.Setenv("GRPC_RATE_LIMIT_INTERVAL", "")
	if _, err := LoadGRPC(); err == nil {
		t.Fatalf("expected error for missing interval")
	}
}

func TestGetRedisURL(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://example")
	url, err := GetRedisURL()
	if err != nil {
		t.Fatalf("expected url, got %v", err)
	}
	if url != "redis://example" {
		t.Fatalf("unexpected url: %s", url)
	}
	t.Setenv("REDIS_URL", "")
	if _, err := GetRedisURL(); err == nil {
		t.Fatalf("expected error when missing")
	}
}

func TestLoadRedis_MissingURL(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	if _, err := LoadRedis(); err == nil {
		t.Fatalf("expected missing url error")
	}
}

func TestLoadRedisTLS_NoSettingsReturnsNil(t *testing.T) {
	if cfg, err := loadRedisTLSFromEnv(); err != nil || cfg != nil {
		t.Fatalf("expected nil tls config, got %#v err %v", cfg, err)
	}
}

func TestLoadRedisTLS_MismatchedKeyPair(t *testing.T) {
	t.Setenv("REDIS_TLS_CERT_FILE", "cert")
	if _, err := loadRedisTLSFromEnv(); err == nil {
		t.Fatalf("expected cert/key mismatch error")
	}
}

func TestLoadRedisTLS_InvalidInsecureFlag(t *testing.T) {
	t.Setenv("REDIS_TLS_INSECURE_SKIP_VERIFY", "notabool")
	if _, err := loadRedisTLSFromEnv(); err == nil {
		t.Fatalf("expected parse bool error")
	}
}

func TestLoadRedisTLS_InsecureTrue(t *testing.T) {
	t.Setenv("REDIS_TLS_INSECURE_SKIP_VERIFY", "true")
	cfg, err := loadRedisTLSFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil || !cfg.InsecureSkipVerify {
		t.Fatalf("expected insecure tls config, got %#v", cfg)
	}
}

func TestLoadRedisTLS_ReadCAError(t *testing.T) {
	t.Setenv("REDIS_TLS_CA_FILE", "/no/such/file")
	if _, err := loadRedisTLSFromEnv(); err == nil {
		t.Fatalf("expected read error for missing CA file")
	}
}

func TestOptionalAndRequiredHelpers(t *testing.T) {
	t.Setenv("X_OPT_DUR", "-1ms")
	if _, err := optionalDuration("X_OPT_DUR"); err == nil {
		t.Fatalf("expected negative duration error")
	}
	t.Setenv("X_OPT_INT", "-1")
	if _, err := optionalInt("X_OPT_INT"); err == nil {
		t.Fatalf("expected negative int error")
	}
	t.Setenv("X_OPT_BOOL", "notbool")
	if _, err := optionalBool("X_OPT_BOOL"); err == nil {
		t.Fatalf("expected bool parse error")
	}

	t.Setenv("X_REQ_INT64", "notint")
	if _, err := requiredInt64("X_REQ_INT64"); err == nil {
		t.Fatalf("expected int64 parse error")
	}
	t.Setenv("X_REQ_INT64", "-1")
	if _, err := requiredInt64("X_REQ_INT64"); err == nil {
		t.Fatalf("expected negative int64 error")
	}

	t.Setenv("X_REQ_INT", "-1")
	if _, err := requiredInt("X_REQ_INT"); err == nil {
		t.Fatalf("expected negative int error")
	}

	t.Setenv("X_REQ_DUR", "bad")
	if _, err := requiredDuration("X_REQ_DUR"); err == nil {
		t.Fatalf("expected bad duration error")
	}
}

func TestLoadRedis_InvalidRequiredFields(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("REDIS_HEALTHCHECK_TIMEOUT", "bad")
	t.Setenv("REDIS_LOCATION_TTL", "10m")
	t.Setenv("REDIS_STREAM_MAXLEN", "1000")
	if _, err := LoadRedis(); err == nil {
		t.Fatalf("expected error for bad healthcheck timeout")
	}

	t.Setenv("REDIS_HEALTHCHECK_TIMEOUT", "1s")
	t.Setenv("REDIS_LOCATION_TTL", "bad")
	if _, err := LoadRedis(); err == nil {
		t.Fatalf("expected error for bad location ttl")
	}

	t.Setenv("REDIS_LOCATION_TTL", "1s")
	t.Setenv("REDIS_STREAM_MAXLEN", "notint")
	if _, err := LoadRedis(); err == nil {
		t.Fatalf("expected error for bad stream maxlen")
	}
}
