package observability

import (
	"sync"
	"time"
)

type MethodSnapshot struct {
	Count         int64   `json:"count"`
	Errors        int64   `json:"errors"`
	InFlight      int64   `json:"in_flight"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	MaxLatencyMs  float64 `json:"max_latency_ms"`
	LastLatencyMs float64 `json:"last_latency_ms"`
}

type Snapshot struct {
	UptimeSec       int64                     `json:"uptime_sec"`
	TotalRequests   int64                     `json:"total_requests"`
	TotalErrors     int64                     `json:"total_errors"`
	InFlight        int64                     `json:"in_flight"`
	RateLimitWaits  int64                     `json:"rate_limit_waits"`
	RateLimitWaitMs int64                     `json:"rate_limit_wait_ms"`
	Lifecycle       *LifecycleSnapshot        `json:"lifecycle,omitempty"`
	Methods         map[string]MethodSnapshot `json:"methods"`
}

type methodStats struct {
	count        int64
	errors       int64
	inFlight     int64
	totalLatency time.Duration
	maxLatency   time.Duration
	lastLatency  time.Duration
}

type Metrics struct {
	mu             sync.Mutex
	start          time.Time
	methods        map[string]*methodStats
	rateLimitWaits int64
	rateLimitWait  time.Duration
	lifecycle      lifecycleStats
}

type CallSpan struct {
	metrics *Metrics
	method  string
	start   time.Time
}

type lifecycleStats struct {
	shutdownAt time.Time
	inflight   int64
}

type LifecycleSnapshot struct {
	ShutdownAt         time.Time `json:"shutdown_at"`
	InFlightAtShutdown int64     `json:"inflight_at_shutdown"`
}

func NewMetrics() *Metrics {
	return &Metrics{
		start:   time.Now(),
		methods: make(map[string]*methodStats),
	}
}

func (m *Metrics) Start(method string) *CallSpan {
	if m == nil {
		return &CallSpan{}
	}
	m.mu.Lock()
	stats := m.ensureMethod(method)
	stats.inFlight++
	m.mu.Unlock()
	return &CallSpan{
		metrics: m,
		method:  method,
		start:   time.Now(),
	}
}

func (s *CallSpan) End(err error) {
	if s == nil || s.metrics == nil {
		return
	}
	dur := time.Since(s.start)
	s.metrics.finish(s.method, dur, err != nil)
}

func (m *Metrics) AddRateLimitWait(d time.Duration) {
	if m == nil || d <= 0 {
		return
	}
	m.mu.Lock()
	m.rateLimitWaits++
	m.rateLimitWait += d
	m.mu.Unlock()
}

func (m *Metrics) Snapshot() Snapshot {
	if m == nil {
		return Snapshot{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	snap := Snapshot{
		UptimeSec:       int64(now.Sub(m.start).Seconds()),
		Methods:         make(map[string]MethodSnapshot),
		RateLimitWaits:  m.rateLimitWaits,
		RateLimitWaitMs: int64(m.rateLimitWait / time.Millisecond),
	}

	for method, stats := range m.methods {
		avg := 0.0
		if stats.count > 0 {
			avg = float64(stats.totalLatency.Milliseconds()) / float64(stats.count)
		}
		snap.Methods[method] = MethodSnapshot{
			Count:         stats.count,
			Errors:        stats.errors,
			InFlight:      stats.inFlight,
			AvgLatencyMs:  avg,
			MaxLatencyMs:  float64(stats.maxLatency.Milliseconds()),
			LastLatencyMs: float64(stats.lastLatency.Milliseconds()),
		}
		snap.TotalRequests += stats.count
		snap.TotalErrors += stats.errors
		snap.InFlight += stats.inFlight
	}

	if !m.lifecycle.shutdownAt.IsZero() {
		snap.Lifecycle = &LifecycleSnapshot{
			ShutdownAt:         m.lifecycle.shutdownAt,
			InFlightAtShutdown: m.lifecycle.inflight,
		}
	}

	return snap
}

func (m *Metrics) ensureMethod(method string) *methodStats {
	stats, ok := m.methods[method]
	if !ok {
		stats = &methodStats{}
		m.methods[method] = stats
	}
	return stats
}

func (m *Metrics) finish(method string, dur time.Duration, failed bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	stats := m.ensureMethod(method)
	stats.inFlight--
	stats.count++
	if failed {
		stats.errors++
	}
	stats.totalLatency += dur
	if dur > stats.maxLatency {
		stats.maxLatency = dur
	}
	stats.lastLatency = dur
	m.mu.Unlock()
}

func (m *Metrics) MarkShutdown(inflight int64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.lifecycle.shutdownAt = time.Now()
	m.lifecycle.inflight = inflight
	m.mu.Unlock()
}
