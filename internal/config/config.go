package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           int
	UpstreamURL    string
	RateLimitRate  float64 // tokens per second
	RateLimitBurst int     // max burst size

	// Phase 2 — all optional. Zero-value = disabled = pure Phase 1 behavior.
	JWTSecret    string         // set → JWT auth enabled
	JWTPublicKey *rsa.PublicKey // optional RSA key, loaded from PEM file
	JWTIssuer    string         // optional issuer validation

	RedisURL string // set → distributed rate limiting enabled
	RedisDB  int

	ConfigDBPath         string // set → SQLite config management enabled
	ConfigReloadInterval int    // polling interval in seconds (default 30)

	CaptureBodies bool // opt-in request/response body capture
	LogBufferSize int  // ring buffer for live request log (default 100)
}

func Load() *Config {
	// Load .env file if it exists (does not override existing env vars)
	_ = godotenv.Load()

	cfg := &Config{
		Port:           getEnvInt("SIDEKICK_PORT", 8081),
		UpstreamURL:    getEnv("SIDEKICK_UPSTREAM_URL", "http://localhost:8080"),
		RateLimitRate:  getEnvFloat("SIDEKICK_RATE_LIMIT_RATE", 10),
		RateLimitBurst: getEnvInt("SIDEKICK_RATE_LIMIT_BURST", 20),

		// Phase 2 — only active when explicitly set
		JWTSecret: getEnv("SIDEKICK_JWT_SECRET", ""),
		JWTIssuer: getEnv("SIDEKICK_JWT_ISSUER", ""),

		RedisURL: getEnv("SIDEKICK_REDIS_URL", ""),
		RedisDB:  getEnvInt("SIDEKICK_REDIS_DB", 0),

		ConfigDBPath:         getEnv("SIDEKICK_CONFIG_DB", ""),
		ConfigReloadInterval: getEnvInt("SIDEKICK_CONFIG_RELOAD_INTERVAL", 30),

		CaptureBodies: getEnvBool("SIDEKICK_CAPTURE_BODIES", false),
		LogBufferSize: getEnvInt("SIDEKICK_LOG_BUFFER_SIZE", 100),
	}

	// Validate and sanitize Phase 1 values
	if cfg.Port <= 0 {
		cfg.Port = 8081
	}
	if cfg.RateLimitRate <= 0 {
		cfg.RateLimitRate = 10
	}
	if cfg.RateLimitBurst <= 0 {
		cfg.RateLimitBurst = 20
	}
	if cfg.UpstreamURL == "" || !isValidURL(cfg.UpstreamURL) {
		cfg.UpstreamURL = "http://localhost:8080"
	}

	// Phase 2 validation
	if cfg.ConfigReloadInterval <= 0 {
		cfg.ConfigReloadInterval = 30
	}
	if cfg.LogBufferSize <= 0 {
		cfg.LogBufferSize = 100
	}

	// Load RSA public key from file if path is set
	if path := getEnv("SIDEKICK_JWT_PUBLIC_KEY", ""); path != "" {
		if key, err := loadRSAPublicKey(path); err == nil {
			cfg.JWTPublicKey = key
		}
	}

	return cfg
}

func (c *Config) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}

func isValidURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

// AuthEnabled returns true when JWT authentication is configured.
func (c *Config) AuthEnabled() bool {
	return c.JWTSecret != "" || c.JWTPublicKey != nil
}

// RedisEnabled returns true when Redis URL is configured.
func (c *Config) RedisEnabled() bool {
	return c.RedisURL != ""
}

// ConfigDBEnabled returns true when SQLite config DB path is configured.
func (c *Config) ConfigDBEnabled() bool {
	return c.ConfigDBPath != ""
}

func loadRSAPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}
	return rsaKey, nil
}
