package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"strings"

	ingestdb "wayfinder/internal/db/ingest"
	"wayfinder/internal/ingest"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

var openLocationDB = func(driver, dsn string) (*sql.DB, error) {
	return sql.Open(driver, dsn)
}

func buildLocationStore(ctx context.Context) (ingest.LocationStore, func(), error) {
	cfg, err := loadRedisConfigFromEnv()
	if err != nil {
		return nil, nil, err
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return nil, nil, errors.New("DATABASE_URL is required")
	}

	opts, err := redis.ParseURL(cfg.url)
	if err != nil {
		return nil, nil, err
	}
	if cfg.dialTimeout != nil {
		opts.DialTimeout = *cfg.dialTimeout
	}
	if cfg.readTimeout != nil {
		opts.ReadTimeout = *cfg.readTimeout
	}
	if cfg.writeTimeout != nil {
		opts.WriteTimeout = *cfg.writeTimeout
	}
	if cfg.poolSize != nil {
		opts.PoolSize = *cfg.poolSize
	}
	if cfg.minIdleConns != nil {
		opts.MinIdleConns = *cfg.minIdleConns
	}
	if cfg.maxRetries != nil {
		opts.MaxRetries = *cfg.maxRetries
	}
	if cfg.tlsConfig != nil {
		opts.TLSConfig = cfg.tlsConfig
	}

	client := redis.NewClient(opts)
	if cfg.enableOTel {
		if err := redisotel.InstrumentTracing(client); err != nil {
			_ = client.Close()
			return nil, nil, err
		}
		if err := redisotel.InstrumentMetrics(client); err != nil {
			_ = client.Close()
			return nil, nil, err
		}
	}

	pingCtx := ctx
	if pingCtx == nil {
		pingCtx = context.Background()
	}
	if cfg.healthcheckTimeout > 0 {
		var cancel context.CancelFunc
		pingCtx, cancel = context.WithTimeout(pingCtx, cfg.healthcheckTimeout)
		defer cancel()
	}
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, nil, err
	}

	db, err := openLocationDB("pgx", databaseURL)
	if err != nil {
		_ = client.Close()
		return nil, nil, err
	}

	historyStore, err := ingestdb.NewPostgresLocationStoreWithSchema(ctx, db)
	if err != nil {
		_ = client.Close()
		_ = db.Close()
		return nil, nil, err
	}

	latestStore := ingest.NewRedisLocationStore(client, cfg.stream, cfg.locationTTL, cfg.streamMaxLen)
	store := ingest.NewMultiLocationStore(historyStore, latestStore)
	cleanup := func() {
		if err := client.Close(); err != nil {
			log.Printf("close redis: %v", err)
		}
		if err := db.Close(); err != nil {
			log.Printf("close locations db: %v", err)
		}
	}
	return store, cleanup, nil
}
