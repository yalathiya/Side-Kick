package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNew_ValidUpstream(t *testing.T) {
	handler, err := New("http://localhost:8080")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestNew_InvalidUpstream(t *testing.T) {
	_, err := New("://invalid-url")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestProxy_ForwardsToUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("upstream-response"))
	}))
	defer upstream.Close()

	handler, err := New(upstream.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test-path", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if string(body) != "upstream-response" {
		t.Errorf("expected body 'upstream-response', got %q", string(body))
	}
}

func TestProxy_SetsXForwardedHost(t *testing.T) {
	var receivedHeaders http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler, _ := New(upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "original-host.com"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := receivedHeaders.Get("X-Forwarded-Host"); got != "original-host.com" {
		t.Errorf("expected X-Forwarded-Host=original-host.com, got %q", got)
	}
}

func TestProxy_SetsXSidekickHeader(t *testing.T) {
	var receivedHeaders http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler, _ := New(upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := receivedHeaders.Get("X-Sidekick"); got != "true" {
		t.Errorf("expected X-Sidekick=true, got %q", got)
	}
}

func TestProxy_SetsXProxiedByResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler, _ := New(upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Proxied-By"); got != "sidekick" {
		t.Errorf("expected X-Proxied-By=sidekick, got %q", got)
	}
}

func TestProxy_PreservesPath(t *testing.T) {
	var receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler, _ := New(upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users?page=1", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if receivedPath != "/api/v1/users" {
		t.Errorf("expected path /api/v1/users, got %q", receivedPath)
	}
}

func TestProxy_PreservesQueryParams(t *testing.T) {
	var receivedQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler, _ := New(upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/search?q=hello&page=2", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if receivedQuery != "q=hello&page=2" {
		t.Errorf("expected query q=hello&page=2, got %q", receivedQuery)
	}
}

func TestProxy_ForwardsRequestBody(t *testing.T) {
	var receivedBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		receivedBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler, _ := New(upstream.URL)
	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if receivedBody != body {
		t.Errorf("expected body %q, got %q", body, receivedBody)
	}
}

func TestProxy_HandlesUpstreamError(t *testing.T) {
	// Point to a non-existent upstream
	handler, _ := New("http://127.0.0.1:1") // port 1 — should fail to connect

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected 502 Bad Gateway for unreachable upstream, got %d", rr.Code)
	}
}

func TestProxy_ForwardsUpstream4xxError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer upstream.Close()

	handler, _ := New(upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if string(body) != "not found" {
		t.Errorf("expected body 'not found', got %q", string(body))
	}
}

func TestProxy_ForwardsUpstream5xxError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer upstream.Close()

	handler, _ := New(upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if string(body) != "server error" {
		t.Errorf("expected body 'server error', got %q", string(body))
	}
}
