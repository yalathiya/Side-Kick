package dashboard

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yash/sidekick/internal/capture"
	"github.com/yash/sidekick/internal/config"
	"github.com/yash/sidekick/internal/configdb"
	"github.com/yash/sidekick/internal/metrics"
)

//go:embed static
var staticFiles embed.FS

// Deps holds all dashboard dependencies. Nil fields are optional (Phase 2 disabled).
type Deps struct {
	Cfg       *config.Config
	Metrics   *metrics.Metrics
	StartTime time.Time
	ReqLog    *capture.RingBuffer // nil if capture disabled
	ConfigDB  *configdb.DB       // nil if SQLite config disabled
}

// New returns a chi.Router that serves the dashboard UI and API.
func New(deps Deps) chi.Router {
	r := chi.NewRouter()

	// Phase 1 APIs
	r.Get("/api/stats", statsHandler(deps.Metrics, deps.StartTime))
	r.Get("/api/config", configHandler(deps.Cfg))

	// Phase 2 APIs (gracefully return empty data when feature is disabled)
	r.Get("/api/requests", requestsHandler(deps.ReqLog))
	r.Get("/api/request/{requestID}", requestDetailHandler(deps.ReqLog))
	r.Get("/api/configdb", configDBHandler(deps.ConfigDB))
	r.Post("/api/configdb", configDBSetHandler(deps.ConfigDB))
	r.Delete("/api/configdb/{key}", configDBDeleteHandler(deps.ConfigDB))

	// Serve embedded static files
	sub, _ := fs.Sub(staticFiles, "static")
	indexHTML, _ := fs.ReadFile(sub, "index.html")
	fileServer := http.FileServer(http.FS(sub))

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
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
		resp := map[string]any{
			"port":               cfg.Port,
			"upstream_url":       cfg.UpstreamURL,
			"rate_limit_rate":    cfg.RateLimitRate,
			"rate_limit_burst":   cfg.RateLimitBurst,
			"auth_enabled":       cfg.AuthEnabled(),
			"jwt_issuer":         cfg.JWTIssuer,
			"has_jwt_secret":     cfg.JWTSecret != "",
			"has_jwt_pubkey":     cfg.JWTPublicKey != nil,
			"redis_enabled":      cfg.RedisEnabled(),
			"configdb_enabled":   cfg.ConfigDBEnabled(),
			"capture_bodies":     cfg.CaptureBodies,
		}
		json.NewEncoder(w).Encode(resp)
	}
}

func requestsHandler(reqLog *capture.RingBuffer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if reqLog == nil {
			json.NewEncoder(w).Encode([]any{})
			return
		}

		q := r.URL.Query()
		f := capture.Filter{
			Route: q.Get("route"),
			Sort:  q.Get("sort"),
		}

		if s := q.Get("status"); s != "" {
			f.StatusGroups = strings.Split(s, ",")
		}
		if m := q.Get("method"); m != "" {
			f.Methods = strings.Split(m, ",")
		}
		if l := q.Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				f.Limit = n
			}
		}

		json.NewEncoder(w).Encode(reqLog.Query(f))
	}
}

func requestDetailHandler(reqLog *capture.RingBuffer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		reqID := chi.URLParam(r, "requestID")
		if reqLog == nil {
			http.Error(w, `{"error":"request log not enabled"}`, http.StatusNotFound)
			return
		}
		entry := reqLog.FindByRequestID(reqID)
		if entry == nil {
			http.Error(w, `{"error":"request not found"}`, http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(entry)
	}
}

func configDBHandler(cdb *configdb.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if cdb == nil {
			json.NewEncoder(w).Encode(map[string]any{
				"enabled": false,
				"config":  map[string]string{},
			})
			return
		}
		snap := cdb.Current()
		json.NewEncoder(w).Encode(map[string]any{
			"enabled":     true,
			"config":      snap.Config,
			"rate_limits": snap.RateLimits,
			"loaded_at":   snap.LoadedAt,
		})
	}
}

func configDBSetHandler(cdb *configdb.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cdb == nil {
			http.Error(w, `{"error":"config database not enabled"}`, http.StatusBadRequest)
			return
		}
		var body struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Key == "" {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if err := cdb.SetConfig(body.Key, body.Value); err != nil {
			http.Error(w, `{"error":"failed to save config"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func configDBDeleteHandler(cdb *configdb.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cdb == nil {
			http.Error(w, `{"error":"config database not enabled"}`, http.StatusBadRequest)
			return
		}
		key := chi.URLParam(r, "key")
		if err := cdb.DeleteConfig(key); err != nil {
			http.Error(w, `{"error":"failed to delete config"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
