package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadRedisConfigFromEnv_ParsesOverrides(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("REDIS_STREAM", "stream-x")
	t.Setenv("REDIS_DIAL_TIMEOUT", "150ms")
	t.Setenv("REDIS_READ_TIMEOUT", "250ms")
	t.Setenv("REDIS_WRITE_TIMEOUT", "300ms")
	t.Setenv("REDIS_POOL_SIZE", "12")
	t.Setenv("REDIS_MIN_IDLE_CONNS", "3")
	t.Setenv("REDIS_MAX_RETRIES", "4")
	t.Setenv("REDIS_HEALTHCHECK_TIMEOUT", "400ms")
	t.Setenv("REDIS_LOCATION_TTL", "30m")
	t.Setenv("REDIS_STREAM_MAXLEN", "250")
	t.Setenv("REDIS_OTEL", "true")

	cfg, err := loadRedisConfigFromEnv()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.url != "redis://localhost:6379/0" {
		t.Fatalf("expected url to match, got %q", cfg.url)
	}
	if cfg.stream != "stream-x" {
		t.Fatalf("expected stream to match, got %q", cfg.stream)
	}
	if cfg.dialTimeout == nil || *cfg.dialTimeout != 150*time.Millisecond {
		t.Fatalf("expected dial timeout to be set, got %+v", cfg.dialTimeout)
	}
	if cfg.readTimeout == nil || *cfg.readTimeout != 250*time.Millisecond {
		t.Fatalf("expected read timeout to be set, got %+v", cfg.readTimeout)
	}
	if cfg.writeTimeout == nil || *cfg.writeTimeout != 300*time.Millisecond {
		t.Fatalf("expected write timeout to be set, got %+v", cfg.writeTimeout)
	}
	if cfg.poolSize == nil || *cfg.poolSize != 12 {
		t.Fatalf("expected pool size to be set, got %+v", cfg.poolSize)
	}
	if cfg.minIdleConns == nil || *cfg.minIdleConns != 3 {
		t.Fatalf("expected min idle conns to be set, got %+v", cfg.minIdleConns)
	}
	if cfg.maxRetries == nil || *cfg.maxRetries != 4 {
		t.Fatalf("expected max retries to be set, got %+v", cfg.maxRetries)
	}
	if cfg.healthcheckTimeout != 400*time.Millisecond {
		t.Fatalf("expected healthcheck timeout to be set, got %+v", cfg.healthcheckTimeout)
	}
	if cfg.locationTTL != 30*time.Minute {
		t.Fatalf("expected location TTL to be set, got %+v", cfg.locationTTL)
	}
	if cfg.streamMaxLen != 250 {
		t.Fatalf("expected stream max len to be set, got %+v", cfg.streamMaxLen)
	}
	if !cfg.enableOTel {
		t.Fatalf("expected otel to be enabled")
	}
}

func TestLoadRedisConfigFromEnv_InvalidDuration(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("REDIS_DIAL_TIMEOUT", "bad")
	t.Setenv("REDIS_HEALTHCHECK_TIMEOUT", "400ms")
	t.Setenv("REDIS_LOCATION_TTL", "30m")
	t.Setenv("REDIS_STREAM_MAXLEN", "250")

	if _, err := loadRedisConfigFromEnv(); err == nil {
		t.Fatalf("expected invalid duration error")
	}
}

func TestLoadRedisTLSConfigFromEnv_MissingKey(t *testing.T) {
	t.Setenv("REDIS_TLS_CERT_FILE", "/tmp/cert.pem")
	t.Setenv("REDIS_TLS_KEY_FILE", "")

	if _, err := loadRedisTLSConfigFromEnv(); err == nil {
		t.Fatalf("expected error when cert is set without key")
	}
}

func TestLoadRedisTLSConfigFromEnv_LoadsFiles(t *testing.T) {
	certFile, keyFile, caFile := writeTempTLSFiles(t)

	t.Setenv("REDIS_TLS_CERT_FILE", certFile)
	t.Setenv("REDIS_TLS_KEY_FILE", keyFile)
	t.Setenv("REDIS_TLS_CA_FILE", caFile)
	t.Setenv("REDIS_TLS_INSECURE_SKIP_VERIFY", "true")

	cfg, err := loadRedisTLSConfigFromEnv()
	if err != nil {
		t.Fatalf("load tls config: %v", err)
	}
	if cfg == nil || len(cfg.Certificates) == 0 {
		t.Fatalf("expected certificates to be loaded")
	}
	if cfg.RootCAs == nil {
		t.Fatalf("expected root CAs to be loaded")
	}
	if !cfg.InsecureSkipVerify {
		t.Fatalf("expected insecure skip verify to be set")
	}
}

func TestBuildLocationStore_PingFails(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://127.0.0.1:1/0")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/dbname?sslmode=disable")
	t.Setenv("REDIS_DIAL_TIMEOUT", "20ms")
	t.Setenv("REDIS_HEALTHCHECK_TIMEOUT", "30ms")
	t.Setenv("REDIS_LOCATION_TTL", "1h")
	t.Setenv("REDIS_STREAM_MAXLEN", "10")

	if _, _, err := buildLocationStore(context.Background()); err == nil {
		t.Fatalf("expected ping failure")
	}
}

func writeTempTLSFiles(t *testing.T) (string, string, string) {
	t.Helper()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	dir := t.TempDir()
	certFile := filepath.Join(dir, "redis_cert.pem")
	keyFile := filepath.Join(dir, "redis_key.pem")
	caFile := filepath.Join(dir, "redis_ca.pem")

	writePEMFile(t, certFile, "CERTIFICATE", certDER, 0644)
	writePEMFile(t, caFile, "CERTIFICATE", certDER, 0644)

	keyDER := x509.MarshalPKCS1PrivateKey(privKey)
	writePEMFile(t, keyFile, "RSA PRIVATE KEY", keyDER, 0600)

	return certFile, keyFile, caFile
}

func writePEMFile(t *testing.T, path, blockType string, derBytes []byte, perm os.FileMode) {
	t.Helper()

	block := &pem.Block{Type: blockType, Bytes: derBytes}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Fatalf("close file: %v", err)
		}
	}()
	if err := pem.Encode(file, block); err != nil {
		t.Fatalf("write pem: %v", err)
	}
}
