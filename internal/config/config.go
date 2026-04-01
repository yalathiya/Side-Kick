package config

import (
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
}

func Load() *Config {
	// Load .env file if it exists (does not override existing env vars)
	_ = godotenv.Load()

	cfg := &Config{
		Port:           getEnvInt("SIDEKICK_PORT", 8081),
		UpstreamURL:    getEnv("SIDEKICK_UPSTREAM_URL", "http://localhost:8080"),
		RateLimitRate:  getEnvFloat("SIDEKICK_RATE_LIMIT_RATE", 10),
		RateLimitBurst: getEnvInt("SIDEKICK_RATE_LIMIT_BURST", 20),
	}

	// Validate and sanitize values
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
