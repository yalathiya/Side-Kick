package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yash/sidekick/internal/metrics"
	"github.com/yash/sidekick/internal/ratelimit"
)

func TestRateLimit_AllowsWithinBurst(t *testing.T) {
	limiter := ratelimit.New(10, 5)
	m := metrics.NewWithRegistry(prometheus.NewRegistry())
	called := 0

	handler := RateLimit(limiter, m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(req.Context(), RealIPKey, "10.0.0.1")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req.WithContext(ctx))

		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}

	if called != 5 {
		t.Errorf("expected 5 calls to next handler, got %d", called)
	}
}

func TestRateLimit_Returns429WhenExceeded(t *testing.T) {
	limiter := ratelimit.New(1, 2) // burst of 2
	m := metrics.NewWithRegistry(prometheus.NewRegistry())

	handler := RateLimit(limiter, m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust burst
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(req.Context(), RealIPKey, "10.0.0.1")
		handler.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))
	}

	// This should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), RealIPKey, "10.0.0.1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rr.Code)
	}
}

func TestRateLimit_429ResponseBody(t *testing.T) {
	limiter := ratelimit.New(1, 1)
	m := metrics.NewWithRegistry(prometheus.NewRegistry())

	handler := RateLimit(limiter, m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust burst
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), RealIPKey, "10.0.0.1")
	handler.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))

	// Get the 429 response
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	ctx = context.WithValue(req.Context(), RealIPKey, "10.0.0.1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode 429 body: %v", err)
	}

	if body["error"] != "rate limit exceeded" {
		t.Errorf("expected error='rate limit exceeded', got %q", body["error"])
	}
}

func TestRateLimit_SetsRetryAfterHeader(t *testing.T) {
	limiter := ratelimit.New(1, 1)
	m := metrics.NewWithRegistry(prometheus.NewRegistry())

	handler := RateLimit(limiter, m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust burst
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), RealIPKey, "10.0.0.1")
	handler.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))

	// Get the 429 response
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	ctx = context.WithValue(req.Context(), RealIPKey, "10.0.0.1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if got := rr.Header().Get("Retry-After"); got != "1" {
		t.Errorf("expected Retry-After=1, got %q", got)
	}
}

func TestRateLimit_SetsContentTypeJSON(t *testing.T) {
	limiter := ratelimit.New(1, 1)
	m := metrics.NewWithRegistry(prometheus.NewRegistry())

	handler := RateLimit(limiter, m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust + trigger 429
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), RealIPKey, "10.0.0.1")
	handler.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	ctx = context.WithValue(req.Context(), RealIPKey, "10.0.0.1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("expected Content-Type=application/json, got %q", got)
	}
}

func TestRateLimit_IncrementsMetricOnReject(t *testing.T) {
	limiter := ratelimit.New(1, 1)
	m := metrics.NewWithRegistry(prometheus.NewRegistry())

	handler := RateLimit(limiter, m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust burst then trigger 3 rejections
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(req.Context(), RealIPKey, "10.0.0.1")
		handler.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))
	}

	snap := m.Snapshot()
	if snap.RateLimitHits != 3 {
		t.Errorf("expected 3 rate limit hits in metrics, got %f", snap.RateLimitHits)
	}
}

func TestRateLimit_DifferentIPsIndependent(t *testing.T) {
	limiter := ratelimit.New(1, 1)
	m := metrics.NewWithRegistry(prometheus.NewRegistry())

	handler := RateLimit(limiter, m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust IP-1
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), RealIPKey, "10.0.0.1")
	handler.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))

	// IP-1 should be blocked
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	ctx = context.WithValue(req.Context(), RealIPKey, "10.0.0.1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("IP-1 should be rate limited, got %d", rr.Code)
	}

	// IP-2 should still be allowed
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	ctx = context.WithValue(req.Context(), RealIPKey, "10.0.0.2")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))
	if rr.Code != http.StatusOK {
		t.Errorf("IP-2 should be allowed, got %d", rr.Code)
	}
}

func TestRateLimit_FallbackToRemoteAddr(t *testing.T) {
	limiter := ratelimit.New(1, 1)
	m := metrics.NewWithRegistry(prometheus.NewRegistry())

	handler := RateLimit(limiter, m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// No RealIP in context — should fall back to RemoteAddr
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("first request should be allowed, got %d", rr.Code)
	}
}

func TestRateLimit_DoesNotCallNextOn429(t *testing.T) {
	limiter := ratelimit.New(1, 1)
	m := metrics.NewWithRegistry(prometheus.NewRegistry())
	callCount := 0

	handler := RateLimit(limiter, m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))

	// First: allowed
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), RealIPKey, "10.0.0.1")
	handler.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))

	// Second: rate limited — should NOT call next
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	ctx = context.WithValue(req.Context(), RealIPKey, "10.0.0.1")
	handler.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))

	if callCount != 1 {
		t.Errorf("expected next called once, got %d", callCount)
	}
}
