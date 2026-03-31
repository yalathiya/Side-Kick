package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/yash/sidekick/internal/metrics"
)

// Metrics records request count and latency per route/method/status.
func Metrics(m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := newResponseWriter(w)

			next.ServeHTTP(rw, r)

			duration := time.Since(start).Seconds()
			status := strconv.Itoa(rw.statusCode)

			m.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
			m.RequestDuration.WithLabelValues(r.Method, r.URL.Path, status).Observe(duration)
		})
	}
}
