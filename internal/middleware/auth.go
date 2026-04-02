package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

const ClaimsKey ctxKey = "claims"

// Claims represents the decoded JWT claims forwarded to upstream services.
type Claims struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
}

// AuthConfig holds JWT authentication configuration.
type AuthConfig struct {
	Enabled   bool
	Secret    []byte         // HMAC secret
	PublicKey *rsa.PublicKey  // RSA public key (optional, used if Secret is nil)
	Issuer    string         // Expected issuer (optional)
}

// Auth validates JWT tokens and attaches claims to the request context.
func Auth(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Keep dashboard and metrics public (for monitoring/config) even when auth is enabled.
			if strings.HasPrefix(r.URL.Path, "/dashboard") || strings.HasPrefix(r.URL.Path, "/metrics") || r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			tokenStr := extractBearerToken(r)
			if tokenStr == "" {
				writeAuthError(w, http.StatusUnauthorized, "missing or invalid authorization header")
				return
			}

			claims, err := validateToken(tokenStr, cfg)
			if err != nil {
				status := http.StatusUnauthorized
				msg := "invalid token"

				switch {
				case isSignatureError(err):
					status = http.StatusForbidden
					msg = "invalid token signature"
				case isIssuerError(err):
					status = http.StatusForbidden
					msg = "invalid token issuer"
				case isExpiredError(err):
					msg = "token expired"
				}

				writeAuthError(w, status, msg)
				return
			}

			// Attach claims to context
			ctx := context.WithValue(r.Context(), ClaimsKey, claims)

			// Forward claims as headers to upstream
			r = r.WithContext(ctx)
			r.Header.Set("X-User-ID", claims.UserID)
			if len(claims.Roles) > 0 {
				r.Header.Set("X-User-Roles", strings.Join(claims.Roles, ","))
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetClaims extracts the JWT claims from the request context.
func GetClaims(ctx context.Context) *Claims {
	if c, ok := ctx.Value(ClaimsKey).(*Claims); ok {
		return c
	}
	return nil
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func validateToken(tokenStr string, cfg AuthConfig) (*Claims, error) {
	parserOpts := []jwt.ParserOption{jwt.WithExpirationRequired()}
	if cfg.Issuer != "" {
		parserOpts = append(parserOpts, jwt.WithIssuer(cfg.Issuer))
	}

	keyFunc := func(token *jwt.Token) (any, error) {
		// If RSA public key is set and token uses RSA, use it
		if cfg.PublicKey != nil {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); ok {
				return cfg.PublicKey, nil
			}
		}
		// Otherwise use HMAC secret
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); ok {
			return cfg.Secret, nil
		}
		return nil, jwt.ErrSignatureInvalid
	}

	token, err := jwt.Parse(tokenStr, keyFunc, parserOpts...)
	if err != nil {
		return nil, err
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, jwt.ErrTokenInvalidClaims
	}

	claims := &Claims{}
	if uid, ok := mapClaims["user_id"].(string); ok {
		claims.UserID = uid
	}
	if sub, ok := mapClaims["sub"].(string); ok && claims.UserID == "" {
		claims.UserID = sub
	}
	if roles, ok := mapClaims["roles"].([]any); ok {
		for _, role := range roles {
			if s, ok := role.(string); ok {
				claims.Roles = append(claims.Roles, s)
			}
		}
	}

	return claims, nil
}

func isSignatureError(err error) bool {
	return strings.Contains(err.Error(), "signature")
}

func isExpiredError(err error) bool {
	return strings.Contains(err.Error(), "expired")
}

func isIssuerError(err error) bool {
	return strings.Contains(err.Error(), "issuer")
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
