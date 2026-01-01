package observability

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	metrics.AddRateLimitWait(0)

	snap := metrics.Snapshot()
	if snap.RateLimitWaits != 2 {
		t.Fatalf("expected 2 waits, got %d", snap.RateLimitWaits)
	}
	if snap.RateLimitWaitMs != 75 {
		t.Fatalf("expected 75ms, got %d", snap.RateLimitWaitMs)
	}
}

func TestMetricsMarkShutdown(t *testing.T) {
	metrics := NewMetrics()
	metrics.MarkShutdown(5)
	snap := metrics.Snapshot()
	if snap.Lifecycle == nil {
		t.Fatalf("expected lifecycle snapshot")
	}
	if snap.Lifecycle.InFlightAtShutdown != 5 {
		t.Fatalf("expected inflight 5, got %d", snap.Lifecycle.InFlightAtShutdown)
	}
	if snap.Lifecycle.ShutdownAt.IsZero() {
		t.Fatalf("expected shutdown timestamp")
	}
}

func TestHandlerReturnsJSON(t *testing.T) {
	metrics := NewMetrics()
	span := metrics.Start("/test")
	span.End(errors.New("fail"))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	Handler(metrics).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var snap Snapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &snap); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if snap.TotalErrors != 1 {
		t.Fatalf("expected total errors 1, got %d", snap.TotalErrors)
	}
	if len(snap.Methods) == 0 {
		t.Fatalf("expected methods in snapshot")
	}
}

func TestMetricsNilSafePaths(t *testing.T) {
	var m *Metrics
	span := m.Start("ignored") // nil-safe
	span.End(nil)              // should not panic

	m.MarkShutdown(10) // nil-safe
}
