package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// RedisConfig holds Redis connection and behavior settings.
type RedisConfig struct {
	URL                string
	Stream             string
	DialTimeout        *time.Duration
	ReadTimeout        *time.Duration
	WriteTimeout       *time.Duration
	PoolSize           *int
	MinIdleConns       *int
	MaxRetries         *int
	HealthcheckTimeout time.Duration
	LocationTTL        time.Duration
	StreamMaxLen       int64
	EnableOTel         bool
	TLSConfig          *tls.Config
}

// GRPCConfig holds ingress rate limiting settings.
type GRPCConfig struct {
	RateLimitInterval time.Duration
	RateLimitBurst    int
}

// ObservabilityConfig holds the HTTP address for the metrics endpoint.
type ObservabilityConfig struct {
	Addr string
}

// LoadRedis reads Redis config from env.
func LoadRedis() (RedisConfig, error) {
	cfg := RedisConfig{
		Stream: strings.TrimSpace(os.Getenv("REDIS_STREAM")),
	}

	url, err := requiredString("REDIS_URL")
	if err != nil {
		return cfg, err
	}
	cfg.URL = url

	if cfg.DialTimeout, err = optionalDuration("REDIS_DIAL_TIMEOUT"); err != nil {
		return cfg, err
	}
	if cfg.ReadTimeout, err = optionalDuration("REDIS_READ_TIMEOUT"); err != nil {
		return cfg, err
	}
	if cfg.WriteTimeout, err = optionalDuration("REDIS_WRITE_TIMEOUT"); err != nil {
		return cfg, err
	}
	if cfg.PoolSize, err = optionalInt("REDIS_POOL_SIZE"); err != nil {
		return cfg, err
	}
	if cfg.MinIdleConns, err = optionalInt("REDIS_MIN_IDLE_CONNS"); err != nil {
		return cfg, err
	}
	if cfg.MaxRetries, err = optionalInt("REDIS_MAX_RETRIES"); err != nil {
		return cfg, err
	}

	if cfg.HealthcheckTimeout, err = requiredDuration("REDIS_HEALTHCHECK_TIMEOUT"); err != nil {
		return cfg, err
	}
	if cfg.LocationTTL, err = requiredDuration("REDIS_LOCATION_TTL"); err != nil {
		return cfg, err
	}
	if cfg.StreamMaxLen, err = requiredInt64("REDIS_STREAM_MAXLEN"); err != nil {
		return cfg, err
	}

	if cfg.EnableOTel, err = optionalBool("REDIS_OTEL"); err != nil {
		return cfg, err
	}

	if cfg.TLSConfig, err = loadRedisTLSFromEnv(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// GetRedisURL returns the required Redis URL from env.
func GetRedisURL() (string, error) {
	return requiredString("REDIS_URL")
}

// LoadGRPC reads gRPC ingress rate limit settings from env.
func LoadGRPC() (GRPCConfig, error) {
	interval, err := requiredDuration("GRPC_RATE_LIMIT_INTERVAL")
	if err != nil {
		return GRPCConfig{}, err
	}
	burst, err := requiredInt("GRPC_RATE_LIMIT_BURST")
	if err != nil {
		return GRPCConfig{}, err
	}
	return GRPCConfig{
		RateLimitInterval: interval,
		RateLimitBurst:    burst,
	}, nil
}

// LoadObservability reads metrics HTTP server address from env.
func LoadObservability() (ObservabilityConfig, error) {
	addr, err := requiredString("OBS_ADDR")
	if err != nil {
		return ObservabilityConfig{}, err
	}
	return ObservabilityConfig{Addr: addr}, nil
}

func loadRedisTLSFromEnv() (*tls.Config, error) {
	caFile := strings.TrimSpace(os.Getenv("REDIS_TLS_CA_FILE"))
	certFile := strings.TrimSpace(os.Getenv("REDIS_TLS_CERT_FILE"))
	keyFile := strings.TrimSpace(os.Getenv("REDIS_TLS_KEY_FILE"))
	serverName := strings.TrimSpace(os.Getenv("REDIS_TLS_SERVER_NAME"))
	insecureStr := strings.TrimSpace(os.Getenv("REDIS_TLS_INSECURE_SKIP_VERIFY"))

	if caFile == "" && certFile == "" && keyFile == "" && serverName == "" && insecureStr == "" {
		return nil, nil
	}
	if (certFile == "") != (keyFile == "") {
		return nil, errors.New("REDIS_TLS_CERT_FILE and REDIS_TLS_KEY_FILE must be set together")
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: serverName,
	}

	if insecureStr != "" {
		insecure, err := strconv.ParseBool(insecureStr)
		if err != nil {
			return nil, fmt.Errorf("REDIS_TLS_INSECURE_SKIP_VERIFY: %w", err)
		}
		tlsConfig.InsecureSkipVerify = insecure
	}

	if caFile != "" {
		pemData, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read REDIS_TLS_CA_FILE: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemData) {
			return nil, errors.New("REDIS_TLS_CA_FILE contains no valid certificates")
		}
		tlsConfig.RootCAs = pool
	}

	if certFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load redis TLS keypair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

func optionalDuration(name string) (*time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil, nil
	}
	val, err := time.ParseDuration(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if val < 0 {
		return nil, fmt.Errorf("%s must be >= 0", name)
	}
	return &val, nil
}

func optionalInt(name string) (*int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil, nil
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if val < 0 {
		return nil, fmt.Errorf("%s must be >= 0", name)
	}
	return &val, nil
}

func optionalBool(name string) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return false, nil
	}
	val, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}
	return val, nil
}

func requiredString(name string) (string, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return raw, nil
}

func requiredInt(name string) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if val < 0 {
		return 0, fmt.Errorf("%s must be >= 0", name)
	}
	return val, nil
}

func requiredDuration(name string) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	val, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if val < 0 {
		return 0, fmt.Errorf("%s must be >= 0", name)
	}
	return val, nil
}

func requiredInt64(name string) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	val, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if val < 0 {
		return 0, fmt.Errorf("%s must be >= 0", name)
	}
	return val, nil
}
