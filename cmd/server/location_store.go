package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"wayfinder/cmd/server/config"
	ingestdb "wayfinder/internal/db/ingest"
	"wayfinder/internal/ingest"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

var openLocationDB = func(driver, dsn string) (*sql.DB, error) {
	return sql.Open(driver, dsn)
}

func buildLocationStore(ctx context.Context) (ingest.LocationStore, func(), error) {
	cfg, err := config.LoadRedis()
	if err != nil {
		return nil, nil, err
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return nil, nil, errors.New("DATABASE_URL is required")
	}

	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, nil, err
	}
	if cfg.DialTimeout != nil {
		opts.DialTimeout = *cfg.DialTimeout
	}
	if cfg.ReadTimeout != nil {
		opts.ReadTimeout = *cfg.ReadTimeout
	}
	if cfg.WriteTimeout != nil {
		opts.WriteTimeout = *cfg.WriteTimeout
	}
	if cfg.PoolSize != nil {
		opts.PoolSize = *cfg.PoolSize
	}
	if cfg.MinIdleConns != nil {
		opts.MinIdleConns = *cfg.MinIdleConns
	}
	if cfg.MaxRetries != nil {
		opts.MaxRetries = *cfg.MaxRetries
	}
	if cfg.TLSConfig != nil {
		opts.TLSConfig = cfg.TLSConfig
	}

	client := redis.NewClient(opts)
	if cfg.EnableOTel {
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
	if cfg.HealthcheckTimeout > 0 {
		var cancel context.CancelFunc
		pingCtx, cancel = context.WithTimeout(pingCtx, cfg.HealthcheckTimeout)
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

	latestStore := ingest.NewRedisLocationStore(redisClientAdapter{client: client}, cfg.Stream, time.Duration(cfg.LocationTTL), cfg.StreamMaxLen)
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

type redisClientAdapter struct {
	client *redis.Client
}

func (a redisClientAdapter) Pipeline() ingest.RedisPipeliner {
	return redisPipelineAdapter{pipe: a.client.Pipeline()}
}

type redisPipelineAdapter struct {
	pipe redis.Pipeliner
}

func (p redisPipelineAdapter) HSet(ctx context.Context, key string, values ...any) *redis.IntCmd {
	return p.pipe.HSet(ctx, key, values...)
}

func (p redisPipelineAdapter) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	return p.pipe.Expire(ctx, key, expiration)
}

func (p redisPipelineAdapter) XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd {
	return p.pipe.XAdd(ctx, a)
}

func (p redisPipelineAdapter) Exec(ctx context.Context) ([]redis.Cmder, error) {
	return p.pipe.Exec(ctx)
}
