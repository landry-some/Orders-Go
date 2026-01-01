package ingest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisLocationStore_UpdatesHashAndStream(t *testing.T) {
	t.Parallel()

	pipe := &stubPipeline{}
	client := &stubRedisClient{pipe: pipe}
	store := NewRedisLocationStore(client, "location_events", 0, 0)

	loc := Location{
		DriverID:  "driver-1",
		Lat:       12.34,
		Long:      56.78,
		Timestamp: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
	}

	if err := store.Update(context.Background(), loc); err != nil {
		t.Fatalf("update: %v", err)
	}

	if len(pipe.hsets) != 1 {
		t.Fatalf("expected 1 HSET, got %d", len(pipe.hsets))
	}
	if pipe.hsets[0].key != "driver:driver-1" {
		t.Fatalf("unexpected hash key %q", pipe.hsets[0].key)
	}

	hash := toMap(pipe.hsets[0].values)
	if hash["driver_id"] != "driver-1" || hash["lat"] != 12.34 || hash["long"] != 56.78 {
		t.Fatalf("unexpected hash values: %+v", hash)
	}

	if len(pipe.xadds) != 1 {
		t.Fatalf("expected 1 XADD, got %d", len(pipe.xadds))
	}
	if pipe.xadds[0].Stream != "location_events" {
		t.Fatalf("unexpected stream %q", pipe.xadds[0].Stream)
	}

	if !pipe.execCalled {
		t.Fatalf("expected Exec to be called")
	}
}

func TestRedisLocationStore_TTLMaxLenAndDefaultStream(t *testing.T) {
	t.Parallel()

	pipe := &stubPipeline{}
	client := &stubRedisClient{pipe: pipe}
	store := NewRedisLocationStore(client, "", time.Second, 1)

	loc1 := Location{
		DriverID:  "driver-ttl",
		Lat:       1,
		Long:      2,
		Timestamp: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	loc2 := Location{
		DriverID:  "driver-ttl",
		Lat:       3,
		Long:      4,
		Timestamp: time.Date(2024, 1, 2, 3, 4, 6, 0, time.UTC),
	}

	if err := store.Update(context.Background(), loc1); err != nil {
		t.Fatalf("update loc1: %v", err)
	}
	if err := store.Update(context.Background(), loc2); err != nil {
		t.Fatalf("update loc2: %v", err)
	}

	if pipe.expirationCalls != 2 {
		t.Fatalf("expected expiration to be set twice (once per Update)")
	}
	if pipe.expirations["driver:driver-ttl"] != time.Second {
		t.Fatalf("unexpected ttl: %v", pipe.expirations["driver:driver-ttl"])
	}

	if len(pipe.xadds) != 2 {
		t.Fatalf("expected 2 XADDs, got %d", len(pipe.xadds))
	}
	for _, xa := range pipe.xadds {
		if xa.Stream != "location_events" {
			t.Fatalf("expected default stream, got %q", xa.Stream)
		}
		if xa.MaxLen != 1 || !xa.Approx {
			t.Fatalf("expected maxlen settings applied, got %+v", xa)
		}
	}
}

func TestRedisLocationStore_RespectsCanceledContext(t *testing.T) {
	t.Parallel()

	pipe := &stubPipeline{}
	client := &stubRedisClient{pipe: pipe}
	store := NewRedisLocationStore(client, "location_events", 0, 0)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.Update(ctx, Location{DriverID: "driver-cancel"})
	if err == nil {
		t.Fatalf("expected context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if pipe.execCalled || len(pipe.hsets) > 0 || len(pipe.xadds) > 0 {
		t.Fatalf("expected no writes when context canceled")
	}
}

type stubRedisClient struct {
	pipe *stubPipeline
}

func (s *stubRedisClient) Pipeline() RedisPipeliner { return s.pipe }

type stubPipeline struct {
	hsets []struct {
		key    string
		values []any
	}
	expirations     map[string]time.Duration
	expirationCalls int
	xadds           []redis.XAddArgs
	execCalled      bool
	execErr         error
}

func (s *stubPipeline) HSet(_ context.Context, key string, values ...any) *redis.IntCmd {
	s.hsets = append(s.hsets, struct {
		key    string
		values []any
	}{key: key, values: values})
	return redis.NewIntCmd(context.Background())
}

func (s *stubPipeline) Expire(_ context.Context, key string, ttl time.Duration) *redis.BoolCmd {
	if s.expirations == nil {
		s.expirations = map[string]time.Duration{}
	}
	s.expirations[key] = ttl
	s.expirationCalls++
	return redis.NewBoolCmd(context.Background())
}

func (s *stubPipeline) XAdd(_ context.Context, a *redis.XAddArgs) *redis.StringCmd {
	s.xadds = append(s.xadds, *a)
	return redis.NewStringCmd(context.Background())
}

func (s *stubPipeline) Exec(_ context.Context) ([]redis.Cmder, error) {
	s.execCalled = true
	return nil, s.execErr
}

func toMap(args []any) map[string]any {
	if len(args) == 0 {
		return map[string]any{}
	}
	if m, ok := args[0].(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
