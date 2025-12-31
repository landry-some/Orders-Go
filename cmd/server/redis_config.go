package main

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

type redisConfig struct {
	url                string
	stream             string
	dialTimeout        *time.Duration
	readTimeout        *time.Duration
	writeTimeout       *time.Duration
	poolSize           *int
	minIdleConns       *int
	maxRetries         *int
	healthcheckTimeout time.Duration
	locationTTL        time.Duration
	streamMaxLen       int64
	enableOTel         bool
	tlsConfig          *tls.Config
}

func loadRedisConfigFromEnv() (redisConfig, error) {
	cfg := redisConfig{
		url:    strings.TrimSpace(os.Getenv("REDIS_URL")),
		stream: strings.TrimSpace(os.Getenv("REDIS_STREAM")),
	}
	if cfg.url == "" {
		return cfg, errors.New("REDIS_URL is required")
	}

	var err error
	if cfg.dialTimeout, err = parseOptionalDuration("REDIS_DIAL_TIMEOUT"); err != nil {
		return cfg, err
	}
	if cfg.readTimeout, err = parseOptionalDuration("REDIS_READ_TIMEOUT"); err != nil {
		return cfg, err
	}
	if cfg.writeTimeout, err = parseOptionalDuration("REDIS_WRITE_TIMEOUT"); err != nil {
		return cfg, err
	}
	if cfg.poolSize, err = parseOptionalInt("REDIS_POOL_SIZE"); err != nil {
		return cfg, err
	}
	if cfg.minIdleConns, err = parseOptionalInt("REDIS_MIN_IDLE_CONNS"); err != nil {
		return cfg, err
	}
	if cfg.maxRetries, err = parseOptionalInt("REDIS_MAX_RETRIES"); err != nil {
		return cfg, err
	}

	if cfg.healthcheckTimeout, err = parseRequiredDuration("REDIS_HEALTHCHECK_TIMEOUT"); err != nil {
		return cfg, err
	}

	if cfg.locationTTL, err = parseRequiredDuration("REDIS_LOCATION_TTL"); err != nil {
		return cfg, err
	}

	if cfg.streamMaxLen, err = parseRequiredInt64("REDIS_STREAM_MAXLEN"); err != nil {
		return cfg, err
	}

	if cfg.enableOTel, err = parseOptionalBool("REDIS_OTEL"); err != nil {
		return cfg, err
	}

	cfg.tlsConfig, err = loadRedisTLSConfigFromEnv()
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

func loadRedisTLSConfigFromEnv() (*tls.Config, error) {
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

func parseOptionalDuration(name string) (*time.Duration, error) {
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

func parseOptionalInt(name string) (*int, error) {
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
		return 0, fmt.Errorf("%s must be >= 0", name)
	}
	return val, nil
}

func parseRequiredInt64(name string) (int64, error) {
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

func parseOptionalBool(name string) (bool, error) {
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
