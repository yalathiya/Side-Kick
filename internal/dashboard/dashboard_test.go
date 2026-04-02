package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yash/sidekick/internal/capture"
	"github.com/yash/sidekick/internal/config"
	"github.com/yash/sidekick/internal/metrics"
)

func testSetup() Deps {
	cfg := &config.Config{
		Port:           8081,
		UpstreamURL:    "http://localhost:8080",
		RateLimitRate:  10,
		RateLimitBurst: 20,
	}
	m := metrics.NewWithRegistry(prometheus.NewRegistry())
	return Deps{
		Cfg:       cfg,
		Metrics:   m,
		StartTime: time.Now().Add(-5 * time.Minute),
	}
}

func TestStatsAPI_ReturnsJSON(t *testing.T) {
	deps := testSetup()
	router := New(deps)

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
	deps := testSetup()
	router := New(deps)

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
	if snap.UptimeSeconds < 300 {
		t.Errorf("expected uptime >= 300s, got %f", snap.UptimeSeconds)
	}
}

func TestStatsAPI_ReflectsMetrics(t *testing.T) {
	deps := testSetup()
	deps.Metrics.RequestsTotal.WithLabelValues("GET", "/api", "200").Add(5)
	deps.Metrics.RateLimitHits.Add(2)

	router := New(deps)

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
	deps := testSetup()
	router := New(deps)

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
	deps := testSetup()
	router := New(deps)

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

func TestConfigAPI_HasPhase2Fields(t *testing.T) {
	deps := testSetup()
	router := New(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var result map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&result)

	phase2Keys := []string{"auth_enabled", "redis_enabled", "configdb_enabled", "capture_bodies"}
	for _, key := range phase2Keys {
		if _, ok := result[key]; !ok {
			t.Errorf("missing Phase 2 config key %q", key)
		}
	}
}

func TestDashboard_ServesHTML(t *testing.T) {
	deps := testSetup()
	router := New(deps)

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
	deps := testSetup()
	router := New(deps)

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
		if !strings.Contains(body, s) {
			t.Errorf("dashboard HTML missing expected content: %q", s)
		}
	}
}

func TestRequestsAPI_EmptyWhenDisabled(t *testing.T) {
	deps := testSetup()
	router := New(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/requests", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var result []any
	json.NewDecoder(rr.Body).Decode(&result)
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d entries", len(result))
	}
}

func TestRequestsAPI_ReturnsEntries(t *testing.T) {
	deps := testSetup()
	deps.ReqLog = capture.NewRingBuffer(10)
	deps.ReqLog.Add(capture.Entry{RequestID: "req-1", Method: "GET", Path: "/test"})
	deps.ReqLog.Add(capture.Entry{RequestID: "req-2", Method: "POST", Path: "/data"})

	router := New(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/requests", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var result []capture.Entry
	json.NewDecoder(rr.Body).Decode(&result)
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	// Recent returns newest first
	if result[0].RequestID != "req-2" {
		t.Errorf("expected newest first, got %s", result[0].RequestID)
	}
}

func TestRequestDetailAPI_Found(t *testing.T) {
	deps := testSetup()
	deps.ReqLog = capture.NewRingBuffer(10)
	deps.ReqLog.Add(capture.Entry{RequestID: "detail-1", Method: "GET", Path: "/found"})

	router := New(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/request/detail-1", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	var entry capture.Entry
	json.NewDecoder(rr.Body).Decode(&entry)
	if entry.Path != "/found" {
		t.Errorf("expected path /found, got %s", entry.Path)
	}
}

func TestRequestDetailAPI_NotFound(t *testing.T) {
	deps := testSetup()
	deps.ReqLog = capture.NewRingBuffer(10)

	router := New(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/request/nonexistent", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestRequestsAPI_FilterByStatus(t *testing.T) {
	deps := testSetup()
	deps.ReqLog = capture.NewRingBuffer(10)
	deps.ReqLog.Add(capture.Entry{RequestID: "ok", Method: "GET", Path: "/a", Status: 200})
	deps.ReqLog.Add(capture.Entry{RequestID: "err", Method: "GET", Path: "/b", Status: 500})
	deps.ReqLog.Add(capture.Entry{RequestID: "notfound", Method: "GET", Path: "/c", Status: 404})

	router := New(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/requests?status=4xx,5xx", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var result []capture.Entry
	json.NewDecoder(rr.Body).Decode(&result)
	if len(result) != 2 {
		t.Fatalf("expected 2 error entries, got %d", len(result))
	}
	for _, e := range result {
		if e.Status < 400 {
			t.Errorf("expected only errors, got status %d", e.Status)
		}
	}
}

func TestRequestsAPI_SortByDuration(t *testing.T) {
	deps := testSetup()
	deps.ReqLog = capture.NewRingBuffer(10)
	deps.ReqLog.Add(capture.Entry{RequestID: "fast", DurationMs: 5, Status: 200, Method: "GET", Path: "/"})
	deps.ReqLog.Add(capture.Entry{RequestID: "slow", DurationMs: 500, Status: 200, Method: "GET", Path: "/"})
	deps.ReqLog.Add(capture.Entry{RequestID: "mid", DurationMs: 50, Status: 200, Method: "GET", Path: "/"})

	router := New(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/requests?sort=duration", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var result []capture.Entry
	json.NewDecoder(rr.Body).Decode(&result)
	if len(result) != 3 {
		t.Fatalf("expected 3, got %d", len(result))
	}
	if result[0].RequestID != "slow" {
		t.Errorf("expected slowest first, got %s", result[0].RequestID)
	}
}

func TestRequestsAPI_LimitAndMethod(t *testing.T) {
	deps := testSetup()
	deps.ReqLog = capture.NewRingBuffer(10)
	for i := 0; i < 5; i++ {
		deps.ReqLog.Add(capture.Entry{RequestID: fmt.Sprintf("g%d", i), Method: "GET", Status: 200, Path: "/"})
	}
	deps.ReqLog.Add(capture.Entry{RequestID: "p1", Method: "POST", Status: 201, Path: "/"})

	router := New(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/requests?method=GET&limit=3", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var result []capture.Entry
	json.NewDecoder(rr.Body).Decode(&result)
	if len(result) != 3 {
		t.Errorf("expected 3, got %d", len(result))
	}
	for _, e := range result {
		if e.Method != "GET" {
			t.Errorf("expected only GET, got %s", e.Method)
		}
	}
}

func TestConfigDBAPI_DisabledReturnsEmpty(t *testing.T) {
	deps := testSetup()
	router := New(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/configdb", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var result map[string]any
	json.NewDecoder(rr.Body).Decode(&result)
	if result["enabled"] != false {
		t.Error("expected enabled=false when configdb is nil")
	}
}
