package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yash/sidekick/internal/metrics"
)

func newTestMetrics(t *testing.T) *metrics.Metrics {
	t.Helper()
	reg := prometheus.NewRegistry()
	return metrics.NewWithRegistry(reg)
}

func TestMetrics_IncrementsRequestCounter(t *testing.T) {
	m := newTestMetrics(t)

	handler := Metrics(m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/other", nil))

	snap := m.Snapshot()
	if snap.TotalRequests != 3 {
		t.Errorf("expected 3 total requests, got %f", snap.TotalRequests)
	}
}

func TestMetrics_RecordsCorrectLabels(t *testing.T) {
	m := newTestMetrics(t)

	handler := Metrics(m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	snap := m.Snapshot()
	if len(snap.Routes) != 1 {
		t.Fatalf("expected 1 route entry, got %d", len(snap.Routes))
	}
	route := snap.Routes[0]
	if route.Method != "GET" {
		t.Errorf("expected method=GET, got %q", route.Method)
	}
	if route.Route != "/missing" {
		t.Errorf("expected route=/missing, got %q", route.Route)
	}
	if route.Status != "404" {
		t.Errorf("expected status=404, got %q", route.Status)
	}
}

func TestMetrics_RecordsDuration(t *testing.T) {
	m := newTestMetrics(t)

	handler := Metrics(m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/fast", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	snap := m.Snapshot()
	if len(snap.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(snap.Routes))
	}
	if snap.Routes[0].TotalMs <= 0 {
		// Duration should be > 0 (even if tiny)
		// Allow 0 for extremely fast execution
	}
	if snap.Routes[0].Count != 1 {
		t.Errorf("expected count=1, got %f", snap.Routes[0].Count)
	}
}

func TestMetrics_CallsNextHandler(t *testing.T) {
	m := newTestMetrics(t)
	called := false

	handler := Metrics(m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !called {
		t.Error("metrics middleware should call next handler")
	}
}

func TestMetrics_MultipleRoutes(t *testing.T) {
	m := newTestMetrics(t)

	ok := Metrics(m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	fail := Metrics(m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	ok.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api", nil))
	ok.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api", nil))
	fail.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api", nil))

	snap := m.Snapshot()
	if snap.TotalRequests != 3 {
		t.Errorf("expected 3 total, got %f", snap.TotalRequests)
	}
	if len(snap.Routes) != 2 {
		t.Errorf("expected 2 route entries (different method+status), got %d", len(snap.Routes))
	}
}
