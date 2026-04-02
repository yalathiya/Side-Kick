package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testHMACSecret = []byte("test-secret-key-for-sidekick")

func makeHMACToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString(testHMACSecret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func authMiddleware(cfg AuthConfig) http.Handler {
	handler := Auth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaims(r.Context())
		w.Header().Set("Content-Type", "application/json")
		if claims != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"user_id":      claims.UserID,
				"roles":        claims.Roles,
				"x_user_id":    r.Header.Get("X-User-ID"),
				"x_user_roles": r.Header.Get("X-User-Roles"),
			})
		} else {
			json.NewEncoder(w).Encode(map[string]string{"status": "no claims"})
		}
	}))
	return handler
}

func TestAuth_ValidToken(t *testing.T) {
	cfg := AuthConfig{Enabled: true, Secret: testHMACSecret}
	token := makeHMACToken(t, jwt.MapClaims{
		"user_id": "user-123",
		"roles":   []any{"admin", "editor"},
		"exp":     time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	authMiddleware(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if body["user_id"] != "user-123" {
		t.Errorf("expected user_id=user-123, got %v", body["user_id"])
	}
	if body["x_user_id"] != "user-123" {
		t.Errorf("expected X-User-ID header forwarded, got %v", body["x_user_id"])
	}
	if body["x_user_roles"] != "admin,editor" {
		t.Errorf("expected X-User-Roles=admin,editor, got %v", body["x_user_roles"])
	}
}

func TestAuth_ValidTokenWithSubClaim(t *testing.T) {
	cfg := AuthConfig{Enabled: true, Secret: testHMACSecret}
	token := makeHMACToken(t, jwt.MapClaims{
		"sub": "user-456",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	authMiddleware(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if body["user_id"] != "user-456" {
		t.Errorf("expected user_id from sub claim, got %v", body["user_id"])
	}
}

func TestAuth_ExpiredToken(t *testing.T) {
	cfg := AuthConfig{Enabled: true, Secret: testHMACSecret}
	token := makeHMACToken(t, jwt.MapClaims{
		"user_id": "user-123",
		"exp":     time.Now().Add(-time.Hour).Unix(),
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	authMiddleware(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_MissingToken(t *testing.T) {
	cfg := AuthConfig{Enabled: true, Secret: testHMACSecret}

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	authMiddleware(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_MissingBearerPrefix(t *testing.T) {
	cfg := AuthConfig{Enabled: true, Secret: testHMACSecret}
	token := makeHMACToken(t, jwt.MapClaims{
		"user_id": "user-123",
		"exp":     time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", token) // no "Bearer " prefix
	rec := httptest.NewRecorder()

	authMiddleware(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_InvalidSignature(t *testing.T) {
	cfg := AuthConfig{Enabled: true, Secret: testHMACSecret}

	// Sign with a different key
	wrongKey := []byte("wrong-secret-key")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "user-123",
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString(wrongKey)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	authMiddleware(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestAuth_InvalidIssuer(t *testing.T) {
	cfg := AuthConfig{Enabled: true, Secret: testHMACSecret, Issuer: "my-service"}
	token := makeHMACToken(t, jwt.MapClaims{
		"user_id": "user-123",
		"iss":     "other-service",
		"exp":     time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	authMiddleware(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for wrong issuer, got %d", rec.Code)
	}
}

func TestAuth_ValidIssuer(t *testing.T) {
	cfg := AuthConfig{Enabled: true, Secret: testHMACSecret, Issuer: "my-service"}
	token := makeHMACToken(t, jwt.MapClaims{
		"user_id": "user-123",
		"iss":     "my-service",
		"exp":     time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	authMiddleware(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid issuer, got %d", rec.Code)
	}
}

func TestAuth_TokenWithoutExpClaim(t *testing.T) {
	cfg := AuthConfig{Enabled: true, Secret: testHMACSecret}
	// No exp claim — should be rejected (we require expiration)
	token := makeHMACToken(t, jwt.MapClaims{
		"user_id": "user-123",
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	authMiddleware(cfg).ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Errorf("expected rejection for token without exp, got 200")
	}
}

func TestAuth_Disabled(t *testing.T) {
	cfg := AuthConfig{Enabled: false}

	req := httptest.NewRequest("GET", "/", nil)
	// No token at all
	rec := httptest.NewRecorder()

	authMiddleware(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when auth disabled, got %d", rec.Code)
	}
}

func TestAuth_RSAToken(t *testing.T) {
	// Generate RSA key pair for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	cfg := AuthConfig{
		Enabled:   true,
		PublicKey: &privateKey.PublicKey,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"user_id": "rsa-user",
		"roles":   []any{"viewer"},
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign RSA token: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	authMiddleware(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for RSA token, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if body["user_id"] != "rsa-user" {
		t.Errorf("expected user_id=rsa-user, got %v", body["user_id"])
	}
}

func TestAuth_GetClaimsFromContext(t *testing.T) {
	cfg := AuthConfig{Enabled: true, Secret: testHMACSecret}
	token := makeHMACToken(t, jwt.MapClaims{
		"user_id": "ctx-user",
		"roles":   []any{"ops"},
		"exp":     time.Now().Add(time.Hour).Unix(),
	})

	var captured *Claims
	handler := Auth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetClaims(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured == nil {
		t.Fatal("expected claims in context, got nil")
	}
	if captured.UserID != "ctx-user" {
		t.Errorf("expected UserID=ctx-user, got %s", captured.UserID)
	}
	if len(captured.Roles) != 1 || captured.Roles[0] != "ops" {
		t.Errorf("expected Roles=[ops], got %v", captured.Roles)
	}
}
