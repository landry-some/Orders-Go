package observability

import (
	"errors"
	"testing"
	"time"
)

func TestMetricsTracksCalls(t *testing.T) {
	metrics := NewMetrics()
	span := metrics.Start("svc.Method")
	time.Sleep(1 * time.Millisecond)
	span.End(nil)

	span = metrics.Start("svc.Method")
	span.End(errors.New("fail"))

	snap := metrics.Snapshot()
	stats := snap.Methods["svc.Method"]
	if stats.Count != 2 {
		t.Fatalf("expected 2 calls, got %d", stats.Count)
	}
	if stats.Errors != 1 {
		t.Fatalf("expected 1 error, got %d", stats.Errors)
	}
	if stats.InFlight != 0 {
		t.Fatalf("expected 0 inflight, got %d", stats.InFlight)
	}
	if snap.TotalRequests != 2 || snap.TotalErrors != 1 {
		t.Fatalf("unexpected totals: %+v", snap)
	}
}

func TestMetricsTracksRateLimitWait(t *testing.T) {
	metrics := NewMetrics()
	metrics.AddRateLimitWait(50 * time.Millisecond)
	metrics.AddRateLimitWait(25 * time.Millisecond)

	snap := metrics.Snapshot()
	if snap.RateLimitWaits != 2 {
		t.Fatalf("expected 2 waits, got %d", snap.RateLimitWaits)
	}
	if snap.RateLimitWaitMs != 75 {
		t.Fatalf("expected 75ms, got %d", snap.RateLimitWaitMs)
	}
}
