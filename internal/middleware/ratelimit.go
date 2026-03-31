package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/yash/sidekick/internal/metrics"
	"github.com/yash/sidekick/internal/ratelimit"
)

// RateLimit applies per-IP rate limiting using a token bucket algorithm.
func RateLimit(limiter *ratelimit.TokenBucket, m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := GetRealIP(r.Context())
			if ip == "" {
				ip = r.RemoteAddr
			}

			if !limiter.Allow(ip) {
				m.RateLimitHits.Inc()
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "rate limit exceeded",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
