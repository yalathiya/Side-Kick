package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/yash/sidekick/internal/capture"
	"github.com/yash/sidekick/internal/config"
	"github.com/yash/sidekick/internal/configdb"
	"github.com/yash/sidekick/internal/dashboard"
	"github.com/yash/sidekick/internal/logging"
	"github.com/yash/sidekick/internal/metrics"
	"github.com/yash/sidekick/internal/middleware"
	"github.com/yash/sidekick/internal/proxy"
	"github.com/yash/sidekick/internal/ratelimit"
	"github.com/yash/sidekick/internal/setup"
)

func main() {
	// Handle subcommands
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		if err := setup.New().Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	startTime := time.Now()
	cfg := config.Load()
	logger := logging.New()
	m := metrics.New()

	// Rate limiter: Redis if configured, else in-memory
	var limiter ratelimit.Limiter
	inMemoryLimiter := ratelimit.New(cfg.RateLimitRate, cfg.RateLimitBurst)

	if cfg.RedisEnabled() {
		rl, err := ratelimit.NewRedis(cfg.RedisURL, cfg.RedisDB, cfg.RateLimitRate, cfg.RateLimitBurst, inMemoryLimiter)
		if err != nil {
			logger.Error("Redis unavailable, falling back to in-memory rate limiter: %v", err)
			limiter = inMemoryLimiter
		} else {
			logger.Info("Distributed rate limiting enabled (Redis)")
			limiter = rl
		}
	} else {
		limiter = inMemoryLimiter
	}

	// SQLite config DB: only when configured
	var cdb *configdb.DB
	if cfg.ConfigDBEnabled() {
		var err error
		cdb, err = configdb.Open(cfg.ConfigDBPath, cfg.ConfigReloadInterval)
		if err != nil {
			logger.Error("Failed to open config database: %v", err)
		} else {
			logger.Info("SQLite config database enabled (hot reload every %ds)", cfg.ConfigReloadInterval)
			defer cdb.Close()
		}
	}

	// Request capture ring buffer: always created (lightweight) for live log
	reqLog := capture.NewRingBuffer(cfg.LogBufferSize)

	// Create reverse proxy
	proxyHandler, err := proxy.New(cfg.UpstreamURL)
	if err != nil {
		logger.Error("Failed to create proxy: %v", err)
		os.Exit(1)
	}

	r := chi.NewRouter()

	// Middleware chain (order matters — matches the architecture diagram)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Metrics(m))
	r.Use(middleware.RateLimit(limiter, m))

	// Phase 2: JWT auth — only active when configured
	if cfg.AuthEnabled() {
		logger.Info("JWT authentication enabled")
		r.Use(middleware.Auth(middleware.AuthConfig{
			Enabled:   true,
			Secret:    []byte(cfg.JWTSecret),
			PublicKey: cfg.JWTPublicKey,
			Issuer:    cfg.JWTIssuer,
		}))
	}

	// Phase 2: Request capture (after auth so we can capture user ID from claims)
	r.Use(middleware.Capture(reqLog, middleware.CaptureConfig{
		CaptureBodies: cfg.CaptureBodies,
	}))

	// Routes
	r.Get("/health", middleware.HealthCheck())
	r.Handle("/metrics", promhttp.Handler())
	r.Mount("/dashboard", dashboard.New(dashboard.Deps{
		Cfg:       cfg,
		Metrics:   m,
		StartTime: startTime,
		ReqLog:    reqLog,
		ConfigDB:  cdb,
	}))

	// All other requests go to the reverse proxy
	r.Handle("/*", proxyHandler)

	srv := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		logger.Info("Sidekick started on %s → upstream %s", cfg.Addr(), cfg.UpstreamURL)
		logger.Info("Dashboard: http://localhost%s/dashboard/", cfg.Addr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error: %v", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Forced shutdown: %v", err)
	}
	logger.Info("Sidekick stopped")
}
