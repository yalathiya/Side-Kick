package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/yash/sidekick/internal/config"
	"github.com/yash/sidekick/internal/dashboard"
	"github.com/yash/sidekick/internal/logging"
	"github.com/yash/sidekick/internal/metrics"
	"github.com/yash/sidekick/internal/middleware"
	"github.com/yash/sidekick/internal/proxy"
	"github.com/yash/sidekick/internal/ratelimit"
)

func main() {
	startTime := time.Now()
	cfg := config.Load()
	logger := logging.New()
	m := metrics.New()
	limiter := ratelimit.New(cfg.RateLimitRate, cfg.RateLimitBurst)

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

	// Routes
	r.Get("/health", middleware.HealthCheck())
	r.Handle("/metrics", promhttp.Handler())
	r.Mount("/dashboard", dashboard.New(cfg, m, startTime))

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
