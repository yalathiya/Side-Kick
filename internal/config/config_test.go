package config

import (
	"os"
	"testing"
)

func clearEnv() {
	os.Unsetenv("SIDEKICK_PORT")
	os.Unsetenv("SIDEKICK_UPSTREAM_URL")
	os.Unsetenv("SIDEKICK_RATE_LIMIT_RATE")
	os.Unsetenv("SIDEKICK_RATE_LIMIT_BURST")
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv()

	cfg := Load()

	if cfg.Port != 8081 {
		t.Errorf("expected default Port=8081, got %d", cfg.Port)
	}
	if cfg.UpstreamURL != "http://localhost:8080" {
		t.Errorf("expected default UpstreamURL=http://localhost:8080, got %s", cfg.UpstreamURL)
	}
	if cfg.RateLimitRate != 10 {
		t.Errorf("expected default RateLimitRate=10, got %f", cfg.RateLimitRate)
	}
	if cfg.RateLimitBurst != 20 {
		t.Errorf("expected default RateLimitBurst=20, got %d", cfg.RateLimitBurst)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	clearEnv()
	t.Setenv("SIDEKICK_PORT", "9090")
	t.Setenv("SIDEKICK_UPSTREAM_URL", "http://my-api:3000")
	t.Setenv("SIDEKICK_RATE_LIMIT_RATE", "50.5")
	t.Setenv("SIDEKICK_RATE_LIMIT_BURST", "100")

	cfg := Load()

	if cfg.Port != 9090 {
		t.Errorf("expected Port=9090, got %d", cfg.Port)
	}
	if cfg.UpstreamURL != "http://my-api:3000" {
		t.Errorf("expected UpstreamURL=http://my-api:3000, got %s", cfg.UpstreamURL)
	}
	if cfg.RateLimitRate != 50.5 {
		t.Errorf("expected RateLimitRate=50.5, got %f", cfg.RateLimitRate)
	}
	if cfg.RateLimitBurst != 100 {
		t.Errorf("expected RateLimitBurst=100, got %d", cfg.RateLimitBurst)
	}
}

func TestLoad_InvalidEnvFallsBackToDefaults(t *testing.T) {
	clearEnv()
	t.Setenv("SIDEKICK_PORT", "not-a-number")
	t.Setenv("SIDEKICK_RATE_LIMIT_RATE", "not-a-float")
	t.Setenv("SIDEKICK_RATE_LIMIT_BURST", "xyz")

	cfg := Load()

	if cfg.Port != 8081 {
		t.Errorf("expected fallback Port=8081, got %d", cfg.Port)
	}
	if cfg.RateLimitRate != 10 {
		t.Errorf("expected fallback RateLimitRate=10, got %f", cfg.RateLimitRate)
	}
	if cfg.RateLimitBurst != 20 {
		t.Errorf("expected fallback RateLimitBurst=20, got %d", cfg.RateLimitBurst)
	}
}

func TestConfig_Addr(t *testing.T) {
	tests := []struct {
		port int
		want string
	}{
		{8081, ":8081"},
		{3000, ":3000"},
		{443, ":443"},
		{0, ":0"},
	}

	for _, tt := range tests {
		cfg := &Config{Port: tt.port}
		if got := cfg.Addr(); got != tt.want {
			t.Errorf("Config{Port: %d}.Addr() = %q, want %q", tt.port, got, tt.want)
		}
	}
}

func TestGetEnv(t *testing.T) {
	t.Setenv("TEST_KEY_EXISTS", "value123")

	if got := getEnv("TEST_KEY_EXISTS", "fallback"); got != "value123" {
		t.Errorf("expected value123, got %s", got)
	}
	if got := getEnv("TEST_KEY_MISSING", "fallback"); got != "fallback" {
		t.Errorf("expected fallback, got %s", got)
	}
}

func TestGetEnvInt(t *testing.T) {
	t.Setenv("TEST_INT_VALID", "42")
	t.Setenv("TEST_INT_INVALID", "abc")

	if got := getEnvInt("TEST_INT_VALID", 0); got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
	if got := getEnvInt("TEST_INT_INVALID", 99); got != 99 {
		t.Errorf("expected fallback 99, got %d", got)
	}
	if got := getEnvInt("TEST_INT_MISSING", 77); got != 77 {
		t.Errorf("expected fallback 77, got %d", got)
	}
}

func TestGetEnvFloat(t *testing.T) {
	t.Setenv("TEST_FLOAT_VALID", "3.14")
	t.Setenv("TEST_FLOAT_INVALID", "abc")

	if got := getEnvFloat("TEST_FLOAT_VALID", 0); got != 3.14 {
		t.Errorf("expected 3.14, got %f", got)
	}
	if got := getEnvFloat("TEST_FLOAT_INVALID", 2.72); got != 2.72 {
		t.Errorf("expected fallback 2.72, got %f", got)
	}
	if got := getEnvFloat("TEST_FLOAT_MISSING", 1.0); got != 1.0 {
		t.Errorf("expected fallback 1.0, got %f", got)
	}
}
