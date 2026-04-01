package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_GeneratesNewID(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		if id == "" {
			t.Error("expected non-empty request ID in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	responseID := rr.Header().Get("X-Request-ID")
	if responseID == "" {
		t.Error("expected X-Request-ID header in response")
	}
}

func TestRequestID_PreservesExistingID(t *testing.T) {
	existingID := "my-custom-request-id-12345"

	var ctxID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", existingID)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if ctxID != existingID {
		t.Errorf("expected context ID=%q, got %q", existingID, ctxID)
	}
	if got := rr.Header().Get("X-Request-ID"); got != existingID {
		t.Errorf("expected response header ID=%q, got %q", existingID, got)
	}
}

func TestRequestID_UniquePerRequest(t *testing.T) {
	ids := make(map[string]bool)
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		id := rr.Header().Get("X-Request-ID")
		if ids[id] {
			t.Fatalf("duplicate request ID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestRequestID_UUIDFormat(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	id := rr.Header().Get("X-Request-ID")
	// UUID format: 8-4-4-4-12 hex characters
	if len(id) != 36 {
		t.Errorf("expected UUID length 36, got %d (%q)", len(id), id)
	}
	if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		t.Errorf("expected UUID format (8-4-4-4-12), got %q", id)
	}
}

func TestGetRequestID_EmptyContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if id := GetRequestID(req.Context()); id != "" {
		t.Errorf("expected empty ID for bare context, got %q", id)
	}
}

func TestRequestID_MultipleHeaders(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		w.Header().Set("X-Test-ID", id)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Add("X-Request-ID", "first-id")
	req.Header.Add("X-Request-ID", "second-id") // Should take first
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Test-ID"); got != "first-id" {
		t.Errorf("expected first header value 'first-id', got %q", got)
	}
}

func TestRequestID_EmptyHeader(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		w.Header().Set("X-Test-ID", id)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should generate new ID when header is empty
	got := rr.Header().Get("X-Test-ID")
	if got == "" {
		t.Error("expected generated ID when header is empty")
	}
	if len(got) != 36 {
		t.Errorf("expected UUID length 36, got %d", len(got))
	}
}

func TestRequestID_TrimWhitespace(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		w.Header().Set("X-Test-ID", id)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "  trimmed-id  ")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Test-ID"); got != "trimmed-id" {
		t.Errorf("expected trimmed ID 'trimmed-id', got %q", got)
	}
}
