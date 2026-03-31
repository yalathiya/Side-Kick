package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yash/sidekick/internal/logging"
)

func TestResponseWriter_CapturesStatusCode(t *testing.T) {
	rw := newResponseWriter(httptest.NewRecorder())

	rw.WriteHeader(http.StatusNotFound)

	if rw.statusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rw.statusCode)
	}
}

func TestResponseWriter_DefaultsTo200(t *testing.T) {
	rw := newResponseWriter(httptest.NewRecorder())

	if rw.statusCode != http.StatusOK {
		t.Errorf("expected default status 200, got %d", rw.statusCode)
	}
}

func TestResponseWriter_WriteHeaderOnce(t *testing.T) {
	rw := newResponseWriter(httptest.NewRecorder())

	rw.WriteHeader(http.StatusCreated)
	rw.WriteHeader(http.StatusInternalServerError) // should be ignored

	if rw.statusCode != http.StatusCreated {
		t.Errorf("expected first status 201, got %d", rw.statusCode)
	}
}

func TestResponseWriter_WriteMarksWritten(t *testing.T) {
	rw := newResponseWriter(httptest.NewRecorder())

	if rw.written {
		t.Error("should not be marked written initially")
	}

	rw.Write([]byte("hello"))

	if !rw.written {
		t.Error("should be marked written after Write()")
	}
}

func TestResponseWriter_WritePassesThrough(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := newResponseWriter(rec)

	n, err := rw.Write([]byte("hello world"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 11 {
		t.Errorf("expected 11 bytes written, got %d", n)
	}
	if rec.Body.String() != "hello world" {
		t.Errorf("expected body 'hello world', got %q", rec.Body.String())
	}
}

func TestLogging_MiddlewareCallsNext(t *testing.T) {
	logger := logging.New()
	called := false

	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !called {
		t.Error("logging middleware should call next handler")
	}
}

func TestLogging_PreservesStatusCode(t *testing.T) {
	logger := logging.New()

	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Errorf("expected status 418, got %d", rr.Code)
	}
}

func TestLogging_PreservesResponseBody(t *testing.T) {
	logger := logging.New()

	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("response-body"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Body.String() != "response-body" {
		t.Errorf("expected body 'response-body', got %q", rr.Body.String())
	}
}
