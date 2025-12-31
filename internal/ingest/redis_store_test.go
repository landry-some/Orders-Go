package ingest

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisLocationStore_UpdatesHashAndStream(t *testing.T) {
	t.Parallel()

	srv, err := miniredis.Run()
	if err != nil {
		t.Skipf("miniredis unavailable: %v", err)
	}
	t.Cleanup(srv.Close)

	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })

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

	if got := srv.HGet("driver:driver-1", "driver_id"); got != "driver-1" {
		t.Fatalf("expected driver_id to be stored, got %q", got)
	}
	if got := srv.HGet("driver:driver-1", "lat"); got != "12.34" {
		t.Fatalf("expected lat to be stored, got %q", got)
	}
	if got := srv.HGet("driver:driver-1", "long"); got != "56.78" {
		t.Fatalf("expected long to be stored, got %q", got)
	}

	entries, err := srv.Stream("location_events")
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 stream entry, got %d", len(entries))
	}

	values := map[string]string{}
	for i := 0; i+1 < len(entries[0].Values); i += 2 {
		values[entries[0].Values[i]] = entries[0].Values[i+1]
	}
	if values["driver_id"] != "driver-1" {
		t.Fatalf("expected stream driver_id to be stored, got %q", values["driver_id"])
	}
}
