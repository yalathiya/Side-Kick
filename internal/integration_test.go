package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/yash/sidekick/internal/logging"
	"github.com/yash/sidekick/internal/metrics"
	"github.com/yash/sidekick/internal/middleware"
	"github.com/yash/sidekick/internal/ratelimit"
)

// buildStack creates a full middleware chain matching the production setup.
func buildStack(m *metrics.Metrics, limiter *ratelimit.TokenBucket, upstream http.Handler) http.Handler {
	logger := logging.New()
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Metrics(m))
	r.Use(middleware.RateLimit(limiter, m))

	r.Get("/health", middleware.HealthCheck())
	r.Handle("/*", upstream)

	return r
}

func TestIntegration_FullChain_HealthEndpoint(t *testing.T) {
	m := metrics.NewWithRegistry(prometheus.NewRegistry())
	limiter := ratelimit.New(100, 100)
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("health endpoint should not hit upstream")
	})

	stack := buildStack(m, limiter, upstream)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)

	// Verify status
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	// Verify X-Request-ID was added
	if rr.Header().Get("X-Request-ID") == "" {
		t.Error("missing X-Request-ID header")
	}

	// Verify response body
	var body map[string]string
	json.NewDecoder(rr.Body).Decode(&body)
	if body["status"] != "healthy" {
		t.Errorf("expected healthy, got %q", body["status"])
	}

	// Verify metrics recorded
	snap := m.Snapshot()
	if snap.TotalRequests != 1 {
		t.Errorf("expected 1 total request in metrics, got %f", snap.TotalRequests)
	}
}

func TestIntegration_FullChain_ProxyPassthrough(t *testing.T) {
	m := metrics.NewWithRegistry(prometheus.NewRegistry())
	limiter := ratelimit.New(100, 100)

	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("upstream-ok"))
	})

	stack := buildStack(m, limiter, upstream)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	// Request ID should be propagated to upstream
	responseID := rr.Header().Get("X-Request-ID")
	if responseID == "" {
		t.Error("missing X-Request-ID in response")
	}
}

func TestIntegration_FullChain_RateLimitKicksIn(t *testing.T) {
	m := metrics.NewWithRegistry(prometheus.NewRegistry())
	limiter := ratelimit.New(1, 3) // burst of 3

	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	stack := buildStack(m, limiter, upstream)

	// Send 3 requests (within burst)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rr := httptest.NewRecorder()
		stack.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}

	// 4th request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("4th request: expected 429, got %d", rr.Code)
	}

	// Verify 429 still has X-Request-ID
	if rr.Header().Get("X-Request-ID") == "" {
		t.Error("rate limited response should still have X-Request-ID")
	}

	// Verify rate limit hits metric
	snap := m.Snapshot()
	if snap.RateLimitHits != 1 {
		t.Errorf("expected 1 rate limit hit, got %f", snap.RateLimitHits)
	}
}

func TestIntegration_FullChain_DifferentIPsGetOwnBudget(t *testing.T) {
	m := metrics.NewWithRegistry(prometheus.NewRegistry())
	limiter := ratelimit.New(1, 1) // burst of 1 per IP

	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	stack := buildStack(m, limiter, upstream)

	ips := []string{"10.0.0.1:1234", "10.0.0.2:1234", "10.0.0.3:1234"}
	for _, ip := range ips {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		stack.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("first request from %s should be allowed, got %d", ip, rr.Code)
		}
	}

	snap := m.Snapshot()
	if snap.TotalRequests != 3 {
		t.Errorf("expected 3 total requests, got %f", snap.TotalRequests)
	}
}

func TestIntegration_FullChain_MetricsAccumulateAcrossRoutes(t *testing.T) {
	m := metrics.NewWithRegistry(prometheus.NewRegistry())
	limiter := ratelimit.New(100, 100)

	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	stack := buildStack(m, limiter, upstream)

	// Health
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	stack.ServeHTTP(httptest.NewRecorder(), req)

	// OK upstream
	req = httptest.NewRequest(http.MethodGet, "/ok", nil)
	stack.ServeHTTP(httptest.NewRecorder(), req)
	req = httptest.NewRequest(http.MethodGet, "/ok", nil)
	stack.ServeHTTP(httptest.NewRecorder(), req)

	// Failed upstream
	req = httptest.NewRequest(http.MethodGet, "/fail", nil)
	stack.ServeHTTP(httptest.NewRecorder(), req)

	snap := m.Snapshot()
	if snap.TotalRequests != 4 {
		t.Errorf("expected 4 total requests, got %f", snap.TotalRequests)
	}
	if len(snap.Routes) < 3 {
		t.Errorf("expected at least 3 route entries, got %d", len(snap.Routes))
	}
}

func TestIntegration_FullChain_RequestIDPropagated(t *testing.T) {
	m := metrics.NewWithRegistry(prometheus.NewRegistry())
	limiter := ratelimit.New(100, 100)

	customID := "trace-abc-123"
	var ctxID string
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = middleware.GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	stack := buildStack(m, limiter, upstream)

	req := httptest.NewRequest(http.MethodGet, "/proxy", nil)
	req.Header.Set("X-Request-ID", customID)
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)

	// Context should have the custom ID
	if ctxID != customID {
		t.Errorf("expected context ID=%q, got %q", customID, ctxID)
	}

	// Response header should echo it back
	if rr.Header().Get("X-Request-ID") != customID {
		t.Errorf("expected response X-Request-ID=%q, got %q", customID, rr.Header().Get("X-Request-ID"))
	}
}

func TestIntegration_FullChain_RealIPExtracted(t *testing.T) {
	m := metrics.NewWithRegistry(prometheus.NewRegistry())
	limiter := ratelimit.New(100, 100)

	var gotIP string
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = middleware.GetRealIP(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	stack := buildStack(m, limiter, upstream)

	req := httptest.NewRequest(http.MethodGet, "/proxy", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)

	if gotIP != "203.0.113.50" {
		t.Errorf("expected real IP 203.0.113.50, got %q", gotIP)
	}
}

func TestIntegration_FullChain_MultipleMethodsTracked(t *testing.T) {
	m := metrics.NewWithRegistry(prometheus.NewRegistry())
	limiter := ratelimit.New(100, 100)

	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	stack := buildStack(m, limiter, upstream)

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/api", nil)
		stack.ServeHTTP(httptest.NewRecorder(), req)
	}

	snap := m.Snapshot()
	if snap.TotalRequests != 4 {
		t.Errorf("expected 4 requests, got %f", snap.TotalRequests)
	}
}
