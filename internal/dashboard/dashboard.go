package dashboard

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yash/sidekick/internal/config"
	"github.com/yash/sidekick/internal/metrics"
)

//go:embed static
var staticFiles embed.FS

// New returns a chi.Router that serves the dashboard UI and API.
func New(cfg *config.Config, m *metrics.Metrics, startTime time.Time) chi.Router {
	r := chi.NewRouter()

	// JSON API for the frontend
	r.Get("/api/stats", statsHandler(m, startTime))
	r.Get("/api/config", configHandler(cfg))

	// Serve embedded static files
	sub, _ := fs.Sub(staticFiles, "static")
	indexHTML, _ := fs.ReadFile(sub, "index.html")
	fileServer := http.FileServer(http.FS(sub))

	// Serve index.html for root path
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
	// Serve other static assets
	r.Handle("/*", fileServer)

	return r
}

func statsHandler(m *metrics.Metrics, startTime time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap := m.Snapshot()
		snap.UptimeSeconds = time.Since(startTime).Seconds()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snap)
	}
}

func configHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"port":             cfg.Port,
			"upstream_url":     cfg.UpstreamURL,
			"rate_limit_rate":  cfg.RateLimitRate,
			"rate_limit_burst": cfg.RateLimitBurst,
		})
	}
}
