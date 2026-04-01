package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRealIP_FromXForwardedFor(t *testing.T) {
	var gotIP string
	handler := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = GetRealIP(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotIP != "203.0.113.50" {
		t.Errorf("expected 203.0.113.50, got %q", gotIP)
	}
}

func TestRealIP_FromXForwardedFor_MultipleIPs(t *testing.T) {
	var gotIP string
	handler := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = GetRealIP(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18, 150.172.238.178")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	// Should take the first IP (client IP)
	if gotIP != "203.0.113.50" {
		t.Errorf("expected first IP 203.0.113.50, got %q", gotIP)
	}
}

func TestRealIP_FromXRealIP(t *testing.T) {
	var gotIP string
	handler := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = GetRealIP(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "198.51.100.10")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotIP != "198.51.100.10" {
		t.Errorf("expected 198.51.100.10, got %q", gotIP)
	}
}

func TestRealIP_XForwardedForTakesPrecedence(t *testing.T) {
	var gotIP string
	handler := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = GetRealIP(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	req.Header.Set("X-Real-IP", "10.0.0.2")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotIP != "10.0.0.1" {
		t.Errorf("expected X-Forwarded-For to take precedence, got %q", gotIP)
	}
}

func TestRealIP_FallbackToRemoteAddr(t *testing.T) {
	var gotIP string
	handler := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = GetRealIP(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// httptest.NewRequest sets RemoteAddr to "192.0.2.1:1234"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotIP != "192.0.2.1" {
		t.Errorf("expected RemoteAddr IP 192.0.2.1, got %q", gotIP)
	}
}

func TestRealIP_TrimWhitespace(t *testing.T) {
	var gotIP string
	handler := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = GetRealIP(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "  10.0.0.1 , 10.0.0.2 ")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotIP != "10.0.0.1" {
		t.Errorf("expected trimmed IP, got %q", gotIP)
	}
}

func TestGetRealIP_EmptyContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if ip := GetRealIP(req.Context()); ip != "" {
		t.Errorf("expected empty string for bare context, got %q", ip)
	}
}

func TestRealIP_RemoteAddrWithoutPort(t *testing.T) {
	var gotIP string
	handler := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = GetRealIP(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5" // no port
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotIP != "10.0.0.5" {
		t.Errorf("expected 10.0.0.5, got %q", gotIP)
	}
}

func TestRealIP_IPv6(t *testing.T) {
	var gotIP string
	handler := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = GetRealIP(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[::1]:8080"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotIP != "::1" {
		t.Errorf("expected ::1, got %q", gotIP)
	}
}

func TestRealIP_MalformedXForwardedFor(t *testing.T) {
	var gotIP string
	handler := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = GetRealIP(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "invalid-ip, 203.0.113.50")
	req.RemoteAddr = "192.0.2.1:1234"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	// Should skip invalid IP and take the valid one
	if gotIP != "203.0.113.50" {
		t.Errorf("expected valid IP 203.0.113.50, got %q", gotIP)
	}
}

func TestRealIP_AllInvalidXForwardedFor(t *testing.T) {
	var gotIP string
	handler := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = GetRealIP(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "invalid, also-invalid, not-an-ip")
	req.RemoteAddr = "192.0.2.1:1234"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	// Should fall back to RemoteAddr
	if gotIP != "192.0.2.1" {
		t.Errorf("expected fallback to RemoteAddr 192.0.2.1, got %q", gotIP)
	}
}

func TestRealIP_EmptyXForwardedFor(t *testing.T) {
	var gotIP string
	handler := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = GetRealIP(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "")
	req.RemoteAddr = "192.0.2.1:1234"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	// Should fall back to RemoteAddr
	if gotIP != "192.0.2.1" {
		t.Errorf("expected fallback to RemoteAddr 192.0.2.1, got %q", gotIP)
	}
}
