# Sidekick - Project Context for LLM Assistance

> **Last Updated:** 2026-04-02 | **Current Phase:** Phase 2 (Complete) | **Branch:** `phase-2`

---

## 1. What Is Sidekick?

Sidekick is a **lightweight, production-ready API sidecar proxy** written in Go. It sits between clients and backend services to enforce cross-cutting concerns (rate limiting, logging, observability, authentication) **without any code changes** to the upstream application.

**Key value:** Zero-code integration. A developer sets 4 environment variables (or runs `sidekick setup`) and gets production-grade traffic management for any HTTP service, regardless of language.

**Design principle:** All advanced features are **opt-in**. Zero Phase 2 config = pure Phase 1 behavior. No breaking changes.

---

## 2. Architecture Overview

```
Client Request
    |
    v
Sidekick (Chi Router - Middleware Pipeline)
    |-- RequestID Middleware    (generate/preserve X-Request-ID)
    |-- RealIP Middleware       (extract client IP from X-Forwarded-For, X-Real-IP)
    |-- Logging Middleware      (structured log: [ReqID] Status Method Path DurationMs IP)
    |-- Metrics Middleware      (Prometheus: request count, duration histogram)
    |-- RateLimit Middleware    (token bucket per IP — in-memory or Redis)
    |-- Auth Middleware         (JWT validation, claims → X-User-ID, X-User-Roles headers)
    |-- Capture Middleware      (records request to ring buffer for live log)
    |-- Reverse Proxy           (forward to upstream, enrich headers)
    |
    v
Upstream Service (untouched)
    |
    v
Response (with X-Request-ID, X-Proxied-By headers)
```

**Special Routes (not proxied):**
- `GET /health` — Health check JSON
- `GET /metrics` — Prometheus metrics endpoint
- `GET /dashboard/` — Embedded SPA dashboard (3 tabs: Overview, Live Requests, Configuration)
- `GET /dashboard/api/stats` — JSON metrics snapshot
- `GET /dashboard/api/config` — Current env configuration JSON
- `GET /dashboard/api/requests` — Live request log with query params:
  - `?status=4xx,5xx` — filter by status group (2xx, 3xx, 4xx, 5xx)
  - `?method=GET,POST` — filter by HTTP method
  - `?route=/api/users` — filter by path prefix
  - `?sort=duration` — sort by slowest first (default: newest first), also `status`
  - `?limit=50` — max results (default 100)
- `GET /dashboard/api/request/{requestID}` — Full request detail
- `GET /dashboard/api/configdb` — SQLite dynamic config snapshot
- `POST /dashboard/api/configdb` — Set a dynamic config key-value
- `DELETE /dashboard/api/configdb/{key}` — Delete a dynamic config key

---

## 3. Project Structure

```
sidekick/
├── cmd/sidekick/main.go              # Entry point — wires middleware, handles "setup" subcommand
├── internal/
│   ├── config/
│   │   ├── config.go                 # Env config: Load(), AuthEnabled(), RedisEnabled(), ConfigDBEnabled()
│   │   └── config_test.go            # 7 tests
│   ├── ratelimit/
│   │   ├── limiter.go                # Limiter interface (Allow(key) bool)
│   │   ├── ratelimit.go              # In-memory TokenBucket implementation
│   │   ├── ratelimit_test.go         # 11 tests
│   │   ├── redis.go                  # Redis TokenBucket via Lua script, fallback to in-memory
│   │   └── redis_test.go             # 7 tests (+ integration tests when Redis available)
│   ├── proxy/
│   │   ├── proxy.go                  # httputil.ReverseProxy wrapper with header enrichment
│   │   └── proxy_test.go             # 12 tests
│   ├── logging/
│   │   └── logger.go                 # Simple structured logger (Info, Error, Request)
│   ├── metrics/
│   │   ├── metrics.go                # Prometheus metrics + Snapshot() for dashboard
│   │   └── metrics_test.go           # 8 tests
│   ├── middleware/
│   │   ├── requestid.go              # UUID generation/preservation via X-Request-ID
│   │   ├── realip.go                 # Client IP extraction + context storage
│   │   ├── logging.go                # Per-request structured logging
│   │   ├── metrics.go                # Prometheus counter/histogram recording
│   │   ├── ratelimit.go              # Rate limit enforcement (uses ratelimit.Limiter interface)
│   │   ├── health.go                 # /health endpoint handler
│   │   ├── auth.go                   # JWT auth: HMAC/RSA, claims → context + headers
│   │   ├── auth_test.go              # 13 tests (valid, expired, missing, bad sig, RSA, issuer)
│   │   ├── capture.go                # Request capture into ring buffer
│   │   └── *_test.go                 # 34+ tests across all middleware
│   ├── capture/
│   │   ├── capture.go                # RingBuffer: Add(), Entries(), Recent(), FindByRequestID()
│   │   └── capture_test.go           # 9 tests (wrap, concurrent, search)
│   ├── configdb/
│   │   ├── configdb.go               # SQLite config DB: Open(), SetConfig(), GetRateLimits(), hot reload
│   │   └── configdb_test.go          # 12 tests (CRUD, reload, persistence)
│   ├── dashboard/
│   │   ├── dashboard.go              # Chi router: Phase 1 + Phase 2 API handlers, Deps struct
│   │   ├── dashboard_test.go         # 13 tests
│   │   └── static/
│   │       └── index.html            # Embedded SPA (dark/light theme, 3 tabs, request drawer)
│   ├── setup/
│   │   ├── setup.go                  # Interactive CLI wizard (sidekick setup)
│   │   └── setup_test.go             # 4 tests
│   └── integration_test.go           # 8 integration tests (full middleware chain)
├── docs/
│   ├── technical-requirement-doc.md  # Phase 2-4 technical design
│   └── features-need-to-be-added.md  # Outstanding feature requests
├── user stories/                     # 7 user story docs with acceptance criteria
├── go.mod / go.sum                   # Go 1.25.4
├── .env.example                      # Configuration template (Phase 1 + Phase 2)
├── README.md                         # Full project documentation
└── context.md                        # THIS FILE
```

---

## 4. Tech Stack & Dependencies

| Dependency | Purpose |
|---|---|
| `go-chi/chi/v5` | Lightweight HTTP router with middleware support |
| `google/uuid` | UUID generation for request IDs |
| `joho/godotenv` | `.env` file loading |
| `prometheus/client_golang` | Prometheus metrics (counters, histograms) |
| `prometheus/client_model` | Prometheus metric serialization |
| `golang-jwt/jwt/v5` | JWT parsing and validation (HMAC + RSA) |
| `redis/go-redis/v9` | Redis client for distributed rate limiting |
| `jmoiron/sqlx` | SQL extensions for SQLite config DB |
| `modernc.org/sqlite` | Pure-Go SQLite driver (no CGO required) |
| Chart.js (CDN) | Dashboard charts |

---

## 5. Configuration

### Quick Start (Phase 1 — just these 4)
| Variable | Default | Description |
|---|---|---|
| `SIDEKICK_PORT` | `8081` | Port Sidekick listens on |
| `SIDEKICK_UPSTREAM_URL` | `http://localhost:8080` | Backend service URL |
| `SIDEKICK_RATE_LIMIT_RATE` | `10` | Tokens per second (rate limit) |
| `SIDEKICK_RATE_LIMIT_BURST` | `20` | Max burst size |

### Phase 2 — Opt-in (set to enable, omit to disable)
| Variable | Activates | Description |
|---|---|---|
| `SIDEKICK_JWT_SECRET` | JWT Auth | HMAC-SHA256 secret key |
| `SIDEKICK_JWT_PUBLIC_KEY` | JWT Auth (RSA) | Path to RSA public key PEM file |
| `SIDEKICK_JWT_ISSUER` | Issuer validation | Expected `iss` claim value |
| `SIDEKICK_REDIS_URL` | Distributed rate limiting | Redis connection URL |
| `SIDEKICK_REDIS_DB` | Redis DB selection | Database number (default 0) |
| `SIDEKICK_CONFIG_DB` | SQLite config DB | Path to SQLite file |
| `SIDEKICK_CONFIG_RELOAD_INTERVAL` | Hot reload interval | Seconds between polls (default 30) |
| `SIDEKICK_CAPTURE_BODIES` | Body capture | Enable request/response body logging |
| `SIDEKICK_LOG_BUFFER_SIZE` | Ring buffer size | Entries in live request log (default 100) |

### Interactive Setup
```bash
sidekick setup    # Walks through questions, writes .env file
```

---

## 6. Phase 1 - COMPLETED

- Reverse proxy with header enrichment
- In-memory token bucket rate limiting per IP (429 + Retry-After)
- Structured request logging with request IDs
- Prometheus metrics (3 metrics)
- Health check endpoint
- Real IP extraction
- Embedded SPA dashboard
- Graceful shutdown

---

## 7. Phase 2 - COMPLETED

### 7.1 JWT Authentication Middleware
**Files:** `internal/middleware/auth.go`, `auth_test.go`
- Extracts Bearer token from Authorization header
- Validates HMAC-SHA256 and RSA signatures
- Checks expiry (`exp` required) and issuer (`iss` optional)
- Attaches `Claims{UserID, Roles}` to context
- Forwards as `X-User-ID`, `X-User-Roles` headers to upstream
- Returns 401 (missing/expired) or 403 (bad signature/issuer)
- **Disabled by default** — enabled when `SIDEKICK_JWT_SECRET` is set

### 7.2 SQLite Configuration Management + Hot Reload
**Files:** `internal/configdb/configdb.go`, `configdb_test.go`
- SQLite database with `config` (key-value) and `rate_limits` (per-path rules) tables
- WAL mode for concurrent read performance
- In-memory cache via `atomic.Value` for lock-free reads
- Background polling (default 30s) for hot reload
- CRUD operations: SetConfig, DeleteConfig, AddRateLimit, RemoveRateLimit
- Dashboard API: GET/POST/DELETE endpoints for config management
- **Disabled by default** — enabled when `SIDEKICK_CONFIG_DB` is set

### 7.3 Distributed Rate Limiting with Redis
**Files:** `internal/ratelimit/redis.go`, `redis_test.go`, `limiter.go`
- `Limiter` interface: both `TokenBucket` and `RedisLimiter` implement `Allow(key) bool`
- Redis token bucket via Lua script (atomic operations)
- Key structure: `sidekick:ratelimit:<key>` with HMSET for tokens + last_check
- Auto-expiring keys based on capacity/rate
- **Graceful fallback:** if Redis goes down at runtime, falls back to in-memory limiter
- **Fail-open:** if no fallback provided, allows requests when Redis is down
- **Disabled by default** — enabled when `SIDEKICK_REDIS_URL` is set

### 7.4 Request Capture + Live Log
**Files:** `internal/capture/capture.go`, `capture_test.go`, `internal/middleware/capture.go`
- Fixed-size ring buffer (default 100 entries) — bounded memory regardless of traffic
- Captures: request ID, method, path, status, duration, client IP, user ID, headers
- Optional body capture (request + response, capped at 10KB each)
- Thread-safe for concurrent reads/writes
- **Always active** (lightweight) — body capture controlled by `SIDEKICK_CAPTURE_BODIES`

### 7.5 Enhanced Dashboard
**File:** `internal/dashboard/static/index.html`, `dashboard.go`
- **3 tabs:** Overview (charts + stats), Live Requests (filterable log), Configuration
- **Dark/light theme toggle** (persisted to localStorage)
- **Feature badges** in header: shows which Phase 2 features are active (JWT, Redis, ConfigDB, Capture)
- **Live request log:** clickable rows → request detail drawer (headers, body, timing)
- **Preset filter buttons:** All Requests, Errors (4xx/5xx), Unauthorized (401/403), Server Errors (5xx), Slowest First, Success (2xx)
- **Filter controls:** route prefix input, method dropdown, sort dropdown (newest/slowest/status), result limit (25/50/100)
- **Server-side filtering:** all filters sent as query params to `/api/requests`, processed on backend via `capture.Query(Filter{...})`
- **Dynamic config viewer:** shows SQLite config entries + last reload time
- **New APIs:** `/api/requests`, `/api/request/{id}`, `/api/configdb` (GET/POST/DELETE)

### 7.6 Interactive CLI Setup Wizard
**Files:** `internal/setup/setup.go`, `setup_test.go`
- `sidekick setup` subcommand — walks through all config options interactively
- Sensible defaults for every question
- Phase 2 features presented as yes/no questions (default: no = skip)
- Writes formatted `.env` file with section headers

---

## 8. Key Code Patterns

### Middleware Signature
```go
func MiddlewareName(deps...) func(http.Handler) http.Handler
```

### Context Values
- `middleware.GetRequestID(ctx)` — request ID string
- `middleware.GetRealIP(ctx)` — client IP string
- `middleware.GetClaims(ctx)` — `*Claims{UserID, Roles}` or nil

### Rate Limiter Interface
```go
type Limiter interface { Allow(key string) bool }
// Implementations: *TokenBucket (in-memory), *RedisLimiter (Redis)
```

### Dashboard Dependencies
```go
dashboard.New(dashboard.Deps{Cfg, Metrics, StartTime, ReqLog, ConfigDB})
// Nil fields = feature disabled, APIs return empty data gracefully
```

### Config Feature Detection
```go
cfg.AuthEnabled()    // true when JWTSecret != "" || JWTPublicKey != nil
cfg.RedisEnabled()   // true when RedisURL != ""
cfg.ConfigDBEnabled() // true when ConfigDBPath != ""
```

---

## 9. Testing

- **166 total tests** across 10 packages
- Run: `go test ./...`
- Redis integration tests auto-skip when Redis unavailable
- Metrics isolation: each test creates `prometheus.NewRegistry()`
- SQLite tests use `t.TempDir()` for isolated databases
- JWT tests cover HMAC, RSA, expired, missing, bad signature, bad issuer, no-exp, disabled

---

## 10. How to Run

```bash
# Interactive setup (writes .env)
go run ./cmd/sidekick setup

# Start with defaults (Phase 1 only)
go run ./cmd/sidekick

# Start with Phase 2 features
SIDEKICK_JWT_SECRET=my-secret SIDEKICK_REDIS_URL=redis://localhost:6379 go run ./cmd/sidekick

# Build binary
go build -o sidekick ./cmd/sidekick

# Run tests
go test ./...
```

---

## 11. Phase 3 & 4 (Future - Not Started)

**Phase 3 - Resilience:**
- Circuit breaker (Closed -> Open -> Half-Open)
- Retry mechanism with exponential backoff
- Advanced logging and alerting

**Phase 4 - Platform:**
- Kubernetes sidecar deployment
- Helm chart
- OpenTelemetry tracing

---

## 12. Change Log

| Date | Change | Files |
|---|---|---|
| 2026-03-31 | Phase 1 complete: proxy, rate limiting, metrics, dashboard, 84 tests | All `internal/` |
| 2026-04-01 | Technical requirement doc, user stories | `docs/`, `user stories/` |
| 2026-04-02 | Enhanced config validation | `internal/config/` |
| 2026-04-02 | **Phase 2 complete:** JWT auth, SQLite config DB, Redis rate limiting, request capture, enhanced dashboard, setup wizard. 152 tests. | `internal/middleware/auth.go`, `internal/configdb/`, `internal/ratelimit/redis.go`, `internal/capture/`, `internal/setup/`, `internal/dashboard/`, `cmd/sidekick/main.go`, `internal/config/config.go` |
