# Sidekick — Zero-Config API Sidecar Proxy

Sidekick is a lightweight, plug-and-play API sidecar that adds **rate limiting, logging, observability, and resilience** to your backend — without modifying your application code.

It follows the **sidecar pattern**, running alongside your service and handling cross-cutting concerns like traffic control and monitoring.

---

## Why Sidekick Exists

Every backend service needs the same cross-cutting concerns: rate limiting, request logging, metrics, health checks, tracing. But implementing these inside every service leads to:

- **Duplicated code** across services
- **Inconsistent implementations** (each team does logging differently)
- **Security vulnerabilities** (hand-rolled rate limiters, missing input validation)
- **Wasted time** — developers spend 30-40% of effort on non-business-logic code

**Sidekick eliminates all of this.** You write your business logic. Sidekick handles everything else — as a standalone sidecar that sits in front of your service, requiring **zero code changes** to your application.

```
                    ┌───────────────────────────────────┐
  Client Request    │          SIDEKICK                 │     Your Service
─────────────────►  │  Rate Limit → Log → Metrics →     │ ──►  (business
                    │  Request ID → Real IP → Proxy     │       logic only)
  ◄─────────────────│                                   │ ◄──
  Response + Headers│  + Dashboard + Prometheus + Health│
                    └───────────────────────────────────┘
```


## Core Capabilities

### Traffic Management

* Reverse proxy routing to upstream services
* Path-based request forwarding
* Header enrichment (`X-Forwarded-*`, `X-Sidekick`, `X-Request-ID`)

### Rate Limiting

* Token Bucket algorithm
* Per-IP rate limiting
* Configurable rate and burst size

### Logging & Tracing

* Structured request logging with method, path, status, duration, and client IP
* Unique `X-Request-ID` on every request
* Real IP extraction (`X-Forwarded-For`, `X-Real-IP`)

### Observability

* Prometheus metrics at `/metrics`
* `sidekick_requests_total` — request count by method/route/status
* `sidekick_request_duration_seconds` — latency histogram
* `sidekick_ratelimit_hits_total` — rate limit rejections

### Health Check

* `/health` endpoint returning JSON status

### Interactive Dashboard

* Real-time metrics visualization at `/dashboard/`
* See [Dashboard Features](#dashboard-features) below for the full feature set

---

## Project Structure

```
sidekick/
├── cmd/sidekick/
│   └── main.go                  # Entry point
├── internal/
│   ├── config/config.go         # Env + .env configuration
│   ├── proxy/proxy.go           # Reverse proxy with header enrichment
│   ├── logging/logger.go        # Structured logger
│   ├── metrics/metrics.go       # Prometheus metric definitions
│   ├── ratelimit/ratelimit.go   # Token bucket rate limiter
│   ├── dashboard/
│   │   ├── dashboard.go         # Dashboard handler + API
│   │   └── static/index.html    # Embedded SPA dashboard
│   └── middleware/
│       ├── requestid.go         # X-Request-ID generation
│       ├── realip.go            # Client IP extraction
│       ├── logging.go           # Request logging
│       ├── metrics.go           # Metrics collection
│       ├── ratelimit.go         # Rate limit enforcement
│       └── health.go            # Health endpoint
├── .env.example                 # Configuration template
├── .gitignore
├── go.mod
└── go.sum
```

---

## Why Sidekick?

Every backend needs rate limiting, logging, metrics, and health checks. But implementing these inside every service means **duplicated code, inconsistent implementations, security risks, and wasted time.**

Sidekick sits in front of your service and handles all of it — **zero code changes** to your application.

| Concern | Without Sidekick | With Sidekick |
|---|---|---|
| Rate limiting | 200-500 lines + dependency + tests | **0 lines** — env var |
| Structured logging | 100-300 lines + library setup | **0 lines** — automatic |
| Request tracing (X-Request-ID) | 50-100 lines middleware | **0 lines** — automatic |
| Prometheus metrics | 100-200 lines + client library | **0 lines** — automatic |
| Health endpoint | 20-50 lines | **0 lines** — built-in |
| Monitoring dashboard | 500-2000 lines + frontend | **0 lines** — embedded |
| **Total per service** | **1000-3000 lines** | **4 env vars** |

> See [USER_STORIES.md](USER_STORIES.md) for detailed real-world scenarios with step-by-step walkthroughs.

---

## Getting Started

### 1. Clone the repository

```bash
git clone https://github.com/your-username/sidekick.git
cd sidekick
```

### 2. Configure

Copy the example env file and edit as needed:

```bash
cp .env.example .env
```

### 3. Run Tests

Always run the test suite before building, especially after making changes:

```bash
# Run all tests
go test ./... -v

# Run tests for a specific package
go test ./internal/ratelimit/ -v
go test ./internal/middleware/ -v
go test ./internal/proxy/ -v

# Run a specific test by name
go test ./internal/middleware/ -run TestRateLimit -v

# Run with race detector (recommended for concurrency testing)
go test ./... -race

# Run with coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

**Test coverage includes:**

| Package | Tests | What's covered |
|---|---|---|
| `config` | 7 | Defaults, env overrides, invalid input fallback, Addr() |
| `ratelimit` | 9 | Burst limit, per-key isolation, token refill, concurrency safety |
| `proxy` | 10 | Upstream forwarding, header enrichment, path/query/body preservation, upstream errors |
| `middleware` | 34 | RequestID (generation, preservation, uniqueness, UUID format), RealIP (XFF, X-Real-IP, IPv6, fallback), Logging (status capture, passthrough), Metrics (counters, labels, duration), RateLimit (429 response, Retry-After, metric increment, IP isolation), Health (JSON body, structure) |
| `dashboard` | 8 | Stats API, config API, HTML serving, element presence |
| `metrics` | 8 | Snapshot, counters, histograms, latency calculation, registry isolation |
| `integration` | 8 | Full middleware chain: health, proxy, rate limiting, IP isolation, request ID propagation, multi-method tracking |
| **Total** | **84** | |

### 4. Run Sidekick

```bash
go run ./cmd/sidekick/
```

Server starts on `http://localhost:8081` by default.

### 5. Test Endpoints

```bash
# Health check
curl http://localhost:8081/health

# Proxy a request to upstream
curl http://localhost:8081/get

# Prometheus metrics
curl http://localhost:8081/metrics

# Dashboard
open http://localhost:8081/dashboard/
```

---

## Configuration

Sidekick is configured via environment variables or a `.env` file. The `.env` file is auto-loaded on startup; env vars take precedence.

| Variable | Default | Description |
|---|---|---|
| `SIDEKICK_PORT` | `8081` | Port Sidekick listens on |
| `SIDEKICK_UPSTREAM_URL` | `http://localhost:8080` | Backend service to proxy to |
| `SIDEKICK_RATE_LIMIT_RATE` | `10` | Tokens per second (per IP) |
| `SIDEKICK_RATE_LIMIT_BURST` | `20` | Max burst size (per IP) |

---

## Architecture

```
Incoming Request
      ↓
RequestID Middleware
      ↓
RealIP Middleware
      ↓
Logging Middleware
      ↓
Metrics Middleware
      ↓
Rate Limiter
      ↓
Reverse Proxy
      ↓
Upstream Service
```

---

## Dashboard Features

The Sidekick dashboard is a fully interactive, real-time SPA embedded in the Go binary — zero external dependencies at runtime. It is designed to match the feature depth of industry tools like Kong Manager, Traefik Dashboard, Grafana, and Datadog APM.

### Phase 1 — Real-Time Monitoring (Current)

| Feature | UI Element | Description |
|---|---|---|
| Total Requests | Stat card | Cumulative request count |
| Requests/sec | Stat card + live line chart | Throughput computed from polling delta |
| Success Rate | Stat card (%) | 2xx+3xx vs 4xx+5xx ratio |
| Avg Latency | Stat card | Mean response time across all routes |
| Rate Limit Hits | Stat card | Total 429 rejections |
| Uptime | Stat card | Time since last server restart |
| Requests by Route | Bar chart | Traffic volume per endpoint |
| Status Code Distribution | Doughnut chart | 2xx / 3xx / 4xx / 5xx breakdown |
| Requests Over Time | Time-series line chart | Live req/s trend (60-point rolling window) |
| Route Breakdown Table | Sortable table | Method, route, status, count, avg/total latency |
| Configuration Viewer | Key-value cards | Current env var values |
| Health Status | Badge (header) | Live healthy/unhealthy indicator with pulse animation |
| Auto-refresh | 3s polling | All panels update automatically |

### Phase 2 — Traffic Analytics & Security

| Feature | UI Element | Description |
|---|---|---|
| Latency Percentiles | Multi-line chart | P50 / P95 / P99 latency over time |
| Latency Heatmap | Heatmap grid | Request duration distribution across time buckets |
| Top Clients | Ranked bar chart | Top-N client IPs by request count |
| Top Rate-Limited IPs | Ranked table | IPs hitting rate limits most frequently |
| Rate Limit Quota Bars | Progress bars (per-IP) | Visual bucket fill level for active clients |
| Request Size Distribution | Histogram | Request/response payload sizes |
| Error Rate Trend | Time-series overlay | 4xx and 5xx rates plotted over time |
| Error Breakdown | Stacked bar chart | Errors by route and status code |
| JWT Auth Status | Badge per request | Authenticated / Unauthenticated / Expired indicators |
| Blocked Requests | Counter + table | Requests denied by auth middleware |
| Live Request Log | Scrolling log stream | Real-time feed of recent requests (last 100) |
| Request Detail Drawer | Slide-out panel | Full request/response headers, timing breakdown, request ID |
| Log Search & Filter | Search bar + facets | Filter by method, route, status, IP, request ID |
| Config Hot Reload Status | Badge + timestamp | Shows last reload time and success/failure |
| Dark/Light Theme Toggle | Header toggle | User-selectable theme preference (persisted) |
| Time Range Selector | Dropdown / date picker | View metrics for last 5m / 15m / 1h / 6h / 24h |

### Phase 3 — Resilience & Advanced Observability

| Feature | UI Element | Description |
|---|---|---|
| Circuit Breaker Panel | State badges + chart | Open / Half-Open / Closed state per upstream, with state transition timeline |
| Circuit Breaker Config | Inline form | Threshold, timeout, and half-open settings display |
| Retry Metrics | Stat cards + chart | Retry attempts, success-after-retry rate, retry overhead |
| Upstream Health Table | Status table | Per-upstream health check results, response time, last checked |
| Upstream Latency Comparison | Grouped bar chart | Side-by-side latency comparison across upstreams |
| Connection Pool Stats | Gauge cards | Active / Idle / Total connections to upstream |
| Goroutine & Memory Monitor | Gauge + line chart | Live Go runtime stats (goroutines, heap, GC pauses) |
| CPU Usage | Line chart | Process CPU usage over time |
| Alerting Rules Panel | Rule list + toggles | Define error rate / latency thresholds with enable/disable |
| Alert History | Event timeline | Triggered alerts with severity, timestamp, and details |
| Notification Badges | Toast / bell icon | In-dashboard alert notifications |
| Request Waterfall | Waterfall chart | Full timing breakdown (DNS, connect, TLS, TTFB, transfer) |
| Distributed Trace View | Span timeline | Trace propagation across services via X-Request-ID |
| Export Metrics | Button | Download current metrics snapshot as JSON / CSV |

### Phase 4 — Operations & Multi-Instance

| Feature | UI Element | Description |
|---|---|---|
| Multi-Instance Overview | Instance grid | Health, throughput, error rate per Sidekick instance |
| Cluster Aggregate Metrics | Combined charts | Merged metrics across all instances |
| Instance Detail Drill-Down | Tabbed panel | Per-instance deep dive from cluster view |
| Deployment Timeline | Annotated timeline | Version deployments overlaid on metric charts |
| Config Diff Viewer | Side-by-side diff | Compare config between instances or versions |
| Route Management UI | CRUD table | Add / edit / remove proxy routes (with hot reload) |
| Rate Limit Rule Editor | Form + table | Create / modify rate limit rules per route or IP range |
| IP Allowlist / Blocklist | Editable list | Manage IP-based access control from the dashboard |
| Audit Log | Paginated table | All config changes with who, what, when |
| Webhook Manager | Form | Configure webhook notifications for alerts |
| Grafana / Datadog Links | External link buttons | Deep-link to external monitoring dashboards |
| API Documentation | Embedded Swagger UI | Auto-generated API docs for Sidekick's admin endpoints |
| Plugin Marketplace | Card grid | Enable/disable Sidekick plugins with toggle switches |

---

## Roadmap

- [x] **Phase 1** — Reverse proxy, rate limiting, logging, metrics, health endpoint, real-time dashboard
- [ ] **Phase 2** — JWT authentication, config management (SQLite + hot reload), Redis rate limiting, traffic analytics dashboard
- [ ] **Phase 3** — Circuit breaker, retry mechanism, advanced logging (Zap), alerting, runtime monitoring dashboard
- [ ] **Phase 4** — Kubernetes sidecar integration, Helm charts, OpenTelemetry tracing, multi-instance dashboard

---

## Tech Stack

* **Go (Golang)**
* **Chi Router**
* **Prometheus Client**
* **net/http Reverse Proxy**
* **godotenv**
* **Chart.js** (embedded dashboard)

---

## Contributing

Contributions are welcome! Feel free to open issues or submit pull requests.

---

## License

MIT License

---

## Author

**Yash Lathiya**
Software Engineer | Backend & System Design Enthusiast

---

If you like this project, give it a star!
