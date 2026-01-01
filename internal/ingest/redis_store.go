package ingest

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLocationStore stores latest locations in Redis and appends to a stream.
type RedisLocationStore struct {
	client    RedisPipelineClient
	stream    string
	keyPrefix string
	ttl       time.Duration
	maxLen    int64
}

// RedisPipelineClient is the minimal client surface used by RedisLocationStore.
type RedisPipelineClient interface {
	Pipeline() RedisPipeliner
}

// RedisPipeliner is the subset of commands used within a pipeline.
type RedisPipeliner interface {
	HSet(ctx context.Context, key string, values ...any) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd
	Exec(ctx context.Context) ([]redis.Cmder, error)
}

// NewRedisLocationStore constructs a Redis-backed location store.
func NewRedisLocationStore(client RedisPipelineClient, stream string, ttl time.Duration, maxLen int64) *RedisLocationStore {
	if stream == "" {
		stream = "location_events"
	}
	return &RedisLocationStore{
		client:    client,
		stream:    stream,
		keyPrefix: "driver:",
		ttl:       ttl,
		maxLen:    maxLen,
	}
}

// Update writes the latest location and appends to the stream.
func (r *RedisLocationStore) Update(ctx context.Context, loc Location) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	key := r.keyPrefix + loc.DriverID
	timestamp := loc.Timestamp.UTC().Format(time.RFC3339Nano)

	pipe := r.client.Pipeline()
	pipe.HSet(ctx, key, map[string]any{
		"driver_id": loc.DriverID,
		"lat":       loc.Lat,
		"long":      loc.Long,
		"timestamp": timestamp,
	})
	if r.ttl > 0 {
		pipe.Expire(ctx, key, r.ttl)
	}

	args := &redis.XAddArgs{
		Stream: r.stream,
		Values: map[string]any{
			"driver_id": loc.DriverID,
			"lat":       loc.Lat,
			"long":      loc.Long,
			"timestamp": timestamp,
		},
	}
	if r.maxLen > 0 {
		args.MaxLen = r.maxLen
		args.Approx = true
	}
	pipe.XAdd(ctx, args)

	_, err := pipe.Exec(ctx)
	return err
}
