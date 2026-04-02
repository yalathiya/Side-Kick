package middleware

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/yash/sidekick/internal/capture"
)

const maxBodyCapture = 10 * 1024 // 10KB max per request/response body

// CaptureConfig controls what gets recorded.
type CaptureConfig struct {
	CaptureBodies bool
}

// Capture records request/response data into a ring buffer for the live log.
func Capture(buf *capture.RingBuffer, cfg CaptureConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			entry := capture.Entry{
				RequestID: GetRequestID(r.Context()),
				Method:    r.Method,
				Path:      r.URL.Path,
				ClientIP:  GetRealIP(r.Context()),
				Timestamp: start,
				Headers:   captureHeaders(r),
			}

			// Capture request body if enabled
			if cfg.CaptureBodies && r.Body != nil {
				body, _ := io.ReadAll(io.LimitReader(r.Body, maxBodyCapture))
				r.Body = io.NopCloser(bytes.NewReader(body))
				entry.ReqBody = string(body)
			}

			// Get user ID from auth claims if present
			if claims := GetClaims(r.Context()); claims != nil {
				entry.UserID = claims.UserID
			}

			// Wrap response writer to capture status
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			entry.Status = sw.status
			entry.DurationMs = float64(time.Since(start).Microseconds()) / 1000.0

			buf.Add(entry)
		})
	}
}

func captureHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	return headers
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.wroteHeader {
		sw.status = code
		sw.wroteHeader = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.wroteHeader {
		sw.wroteHeader = true
	}
	return sw.ResponseWriter.Write(b)
}
