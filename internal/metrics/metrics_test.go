package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func newTestMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	return NewWithRegistry(reg)
}

func TestNew_CreatesAllMetrics(t *testing.T) {
	m := newTestMetrics()

	if m.RequestsTotal == nil {
		t.Error("RequestsTotal should not be nil")
	}
	if m.RequestDuration == nil {
		t.Error("RequestDuration should not be nil")
	}
	if m.RateLimitHits == nil {
		t.Error("RateLimitHits should not be nil")
	}
}

func TestSnapshot_Empty(t *testing.T) {
	m := newTestMetrics()

	snap := m.Snapshot()

	if snap.TotalRequests != 0 {
		t.Errorf("expected 0 total requests, got %f", snap.TotalRequests)
	}
	if snap.RateLimitHits != 0 {
		t.Errorf("expected 0 rate limit hits, got %f", snap.RateLimitHits)
	}
	if len(snap.Routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(snap.Routes))
	}
}

func TestSnapshot_CountsRequests(t *testing.T) {
	m := newTestMetrics()

	m.RequestsTotal.WithLabelValues("GET", "/api", "200").Inc()
	m.RequestsTotal.WithLabelValues("GET", "/api", "200").Inc()
	m.RequestsTotal.WithLabelValues("POST", "/api", "201").Inc()

	snap := m.Snapshot()

	if snap.TotalRequests != 3 {
		t.Errorf("expected 3 total requests, got %f", snap.TotalRequests)
	}
	if len(snap.Routes) != 2 {
		t.Errorf("expected 2 route entries, got %d", len(snap.Routes))
	}
}

func TestSnapshot_RecordsRateLimitHits(t *testing.T) {
	m := newTestMetrics()

	m.RateLimitHits.Inc()
	m.RateLimitHits.Inc()
	m.RateLimitHits.Inc()

	snap := m.Snapshot()

	if snap.RateLimitHits != 3 {
		t.Errorf("expected 3 rate limit hits, got %f", snap.RateLimitHits)
	}
}

func TestSnapshot_CalculatesAvgLatency(t *testing.T) {
	m := newTestMetrics()

	m.RequestsTotal.WithLabelValues("GET", "/slow", "200").Add(2)
	m.RequestDuration.WithLabelValues("GET", "/slow", "200").Observe(0.1) // 100ms
	m.RequestDuration.WithLabelValues("GET", "/slow", "200").Observe(0.3) // 300ms

	snap := m.Snapshot()

	if len(snap.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(snap.Routes))
	}

	route := snap.Routes[0]
	if route.Count != 2 {
		t.Errorf("expected count=2, got %f", route.Count)
	}
	// Total: 400ms, Avg: 200ms
	if route.TotalMs < 390 || route.TotalMs > 410 {
		t.Errorf("expected TotalMs ~400, got %f", route.TotalMs)
	}
	if route.AvgMs < 190 || route.AvgMs > 210 {
		t.Errorf("expected AvgMs ~200, got %f", route.AvgMs)
	}
}

func TestSnapshot_RouteLabels(t *testing.T) {
	m := newTestMetrics()

	m.RequestsTotal.WithLabelValues("DELETE", "/users/1", "204").Inc()
	m.RequestDuration.WithLabelValues("DELETE", "/users/1", "204").Observe(0.05)

	snap := m.Snapshot()

	if len(snap.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(snap.Routes))
	}

	r := snap.Routes[0]
	if r.Method != "DELETE" {
		t.Errorf("expected method=DELETE, got %q", r.Method)
	}
	if r.Route != "/users/1" {
		t.Errorf("expected route=/users/1, got %q", r.Route)
	}
	if r.Status != "204" {
		t.Errorf("expected status=204, got %q", r.Status)
	}
}

func TestSplitKey(t *testing.T) {
	tests := []struct {
		input string
		want  [3]string
	}{
		{"GET|/api|200", [3]string{"GET", "/api", "200"}},
		{"POST|/users|201", [3]string{"POST", "/users", "201"}},
		{"DELETE|/|204", [3]string{"DELETE", "/", "204"}},
		{"GET||500", [3]string{"GET", "", "500"}},
	}

	for _, tt := range tests {
		got := splitKey(tt.input)
		if got != tt.want {
			t.Errorf("splitKey(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestNewWithRegistry_IsolatedFromDefault(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewWithRegistry(reg)

	m.RequestsTotal.WithLabelValues("GET", "/test", "200").Inc()

	// Gather from the custom registry — should have our metric
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather error: %v", err)
	}

	found := false
	for _, f := range families {
		if f.GetName() == "sidekick_requests_total" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected sidekick_requests_total in custom registry")
	}
}
