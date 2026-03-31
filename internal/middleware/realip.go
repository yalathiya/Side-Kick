package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
)

const RealIPKey ctxKey = "realIP"

// RealIP extracts the client's real IP from X-Forwarded-For or X-Real-IP headers,
// falling back to the remote address.
func RealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		ctx := context.WithValue(r.Context(), RealIPKey, ip)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRealIP extracts the real IP from the context.
func GetRealIP(ctx context.Context) string {
	if ip, ok := ctx.Value(RealIPKey).(string); ok {
		return ip
	}
	return ""
}

func extractIP(r *http.Request) string {
	// Check X-Forwarded-For first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
