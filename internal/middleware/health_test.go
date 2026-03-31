package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheck_StatusOK(t *testing.T) {
	handler := HealthCheck()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHealthCheck_ContentTypeJSON(t *testing.T) {
	handler := HealthCheck()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestHealthCheck_ResponseBody(t *testing.T) {
	handler := HealthCheck()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	var resp HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status=healthy, got %q", resp.Status)
	}
	if resp.Service != "sidekick" {
		t.Errorf("expected service=sidekick, got %q", resp.Service)
	}
}

func TestHealthCheck_ResponseStructure(t *testing.T) {
	handler := HealthCheck()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	var raw map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	// Should have exactly 2 keys
	if len(raw) != 2 {
		t.Errorf("expected 2 JSON fields, got %d: %v", len(raw), raw)
	}

	if _, ok := raw["status"]; !ok {
		t.Error("missing 'status' field in response")
	}
	if _, ok := raw["service"]; !ok {
		t.Error("missing 'service' field in response")
	}
}
