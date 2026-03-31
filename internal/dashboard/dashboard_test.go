package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yash/sidekick/internal/config"
	"github.com/yash/sidekick/internal/metrics"
)

func testSetup() (*config.Config, *metrics.Metrics, time.Time) {
	cfg := &config.Config{
		Port:           8081,
		UpstreamURL:    "http://localhost:8080",
		RateLimitRate:  10,
		RateLimitBurst: 20,
	}
	m := metrics.NewWithRegistry(prometheus.NewRegistry())
	startTime := time.Now().Add(-5 * time.Minute) // 5 min ago
	return cfg, m, startTime
}

func TestStatsAPI_ReturnsJSON(t *testing.T) {
	cfg, m, start := testSetup()
	router := New(cfg, m, start)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestStatsAPI_SnapshotStructure(t *testing.T) {
	cfg, m, start := testSetup()
	router := New(cfg, m, start)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var snap metrics.Snapshot
	if err := json.NewDecoder(rr.Body).Decode(&snap); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if snap.TotalRequests != 0 {
		t.Errorf("expected 0 total requests, got %f", snap.TotalRequests)
	}
	if snap.UptimeSeconds < 300 { // should be ~5 minutes
		t.Errorf("expected uptime >= 300s, got %f", snap.UptimeSeconds)
	}
}

func TestStatsAPI_ReflectsMetrics(t *testing.T) {
	cfg, m, start := testSetup()

	// Record some metrics before creating the router
	m.RequestsTotal.WithLabelValues("GET", "/api", "200").Add(5)
	m.RateLimitHits.Add(2)

	router := New(cfg, m, start)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var snap metrics.Snapshot
	json.NewDecoder(rr.Body).Decode(&snap)

	if snap.TotalRequests != 5 {
		t.Errorf("expected 5 total requests, got %f", snap.TotalRequests)
	}
	if snap.RateLimitHits != 2 {
		t.Errorf("expected 2 rate limit hits, got %f", snap.RateLimitHits)
	}
}

func TestConfigAPI_ReturnsJSON(t *testing.T) {
	cfg, m, start := testSetup()
	router := New(cfg, m, start)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestConfigAPI_ReturnsCorrectValues(t *testing.T) {
	cfg, m, start := testSetup()
	router := New(cfg, m, start)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var result map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if result["port"].(float64) != 8081 {
		t.Errorf("expected port=8081, got %v", result["port"])
	}
	if result["upstream_url"] != "http://localhost:8080" {
		t.Errorf("expected upstream_url=http://localhost:8080, got %v", result["upstream_url"])
	}
	if result["rate_limit_rate"].(float64) != 10 {
		t.Errorf("expected rate_limit_rate=10, got %v", result["rate_limit_rate"])
	}
	if result["rate_limit_burst"].(float64) != 20 {
		t.Errorf("expected rate_limit_burst=20, got %v", result["rate_limit_burst"])
	}
}

func TestConfigAPI_HasAllFields(t *testing.T) {
	cfg, m, start := testSetup()
	router := New(cfg, m, start)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var result map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&result)

	expected := []string{"port", "upstream_url", "rate_limit_rate", "rate_limit_burst"}
	for _, key := range expected {
		if _, ok := result[key]; !ok {
			t.Errorf("missing config key %q", key)
		}
	}
	if len(result) != len(expected) {
		t.Errorf("expected %d config fields, got %d", len(expected), len(result))
	}
}

func TestDashboard_ServesHTML(t *testing.T) {
	cfg, m, start := testSetup()
	router := New(cfg, m, start)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html content type, got %q", ct)
	}

	body := rr.Body.String()
	if len(body) < 100 {
		t.Error("expected substantial HTML content")
	}
	if body[:15] != "<!DOCTYPE html>" {
		t.Errorf("expected HTML doctype, got %q", body[:15])
	}
}

func TestDashboard_HTMLContainsDashboardElements(t *testing.T) {
	cfg, m, start := testSetup()
	router := New(cfg, m, start)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	body := rr.Body.String()

	mustContain := []string{
		"Sidekick",
		"total-requests",
		"chart-routes",
		"chart-status",
		"chart-timeline",
		"/api/stats",
		"/api/config",
	}

	for _, s := range mustContain {
		if !contains(body, s) {
			t.Errorf("dashboard HTML missing expected content: %q", s)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
