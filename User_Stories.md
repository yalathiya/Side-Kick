# User Stories — Sidekick Sidecar Proxy

## Why Sidekick Exists

Every backend service needs the same cross-cutting concerns: rate limiting, request logging, metrics, health checks, tracing. But implementing these inside every service leads to:

- **Duplicated code** across services
- **Inconsistent implementations** (each team does logging differently)
- **Security vulnerabilities** (hand-rolled rate limiters, missing input validation)
- **Wasted time** — developers spend 30-40% of effort on non-business-logic code

**Sidekick eliminates all of this.** You write your business logic. Sidekick handles everything else — as a standalone sidecar that sits in front of your service, requiring **zero code changes** to your application.

```
                    ┌──────────────────────────────────┐
  Client Request    │          SIDEKICK                │     Your Service
─────────────────►  │  Rate Limit → Log → Metrics →    │ ──►  (business
                    │  Request ID → Real IP → Proxy    │       logic only)
  ◄─────────────────│                                  │ ◄──
  Response + Headers│  + Dashboard + Prometheus + Health│
                    └──────────────────────────────────┘
```

---

## Story 1: "I just built an API — now I need production readiness"

### Persona
**Priya**, a backend developer who built a REST API in Python/Flask. She needs logging, rate limiting, and monitoring before going to production — but doesn't want to spend a week adding middleware to her Flask app.

### The Problem (Without Sidekick)
Priya would need to:
- Install and configure Flask-Limiter for rate limiting
- Add python-json-logger for structured logging
- Integrate prometheus_flask_instrumentator for metrics
- Write a /health endpoint
- Add request ID middleware
- Test all of these, handle edge cases, and maintain them

**Estimated effort: 3-5 days. Repeated for every service.**

### The Solution (With Sidekick)

Priya's Flask app stays untouched. She starts Sidekick in front of it:

**Step 1: Her Flask app runs on port 5000 (business logic only)**
```python
# app.py — no middleware, no logging config, no rate limiting
from flask import Flask, jsonify

app = Flask(__name__)

@app.route("/api/users")
def get_users():
    return jsonify([{"id": 1, "name": "Alice"}])

@app.route("/api/orders", methods=["POST"])
def create_order():
    return jsonify({"order_id": "abc-123"}), 201

if __name__ == "__main__":
    app.run(port=5000)
```

**Step 2: Configure Sidekick (one-time, 30 seconds)**
```bash
# .env
SIDEKICK_PORT=8081
SIDEKICK_UPSTREAM_URL=http://localhost:5000
SIDEKICK_RATE_LIMIT_RATE=10
SIDEKICK_RATE_LIMIT_BURST=20
```

**Step 3: Start Sidekick**
```bash
go run ./cmd/sidekick/
# [SIDEKICK] Sidekick started on :8081 → upstream http://localhost:5000
# [SIDEKICK] Dashboard: http://localhost:8081/dashboard/
```

**Step 4: All traffic goes through port 8081 instead of 5000**
```bash
curl http://localhost:8081/api/users
```

### What Priya Gets — Zero Code Changes

| Concern | What Happens | Proof |
|---|---|---|
| **Rate Limiting** | Each client IP gets 10 req/s with burst of 20. Excess returns `429 Too Many Requests` with `Retry-After` header | `curl` 25 times rapidly → first 20 succeed, rest get 429 |
| **Structured Logging** | Every request logged: `[req-id] 200 GET /api/users 12ms client=10.0.0.1` | Check terminal output |
| **Request Tracing** | Unique `X-Request-ID` header on every request/response. Pass it between services for distributed tracing | `curl -v` → see `X-Request-ID` in response |
| **Prometheus Metrics** | Request count, latency histogram, error rates, rate limit hits — all auto-collected | `curl http://localhost:8081/metrics` |
| **Health Check** | `/health` returns `{"status":"healthy","service":"sidekick"}` | Load balancers, K8s probes use this |
| **Dashboard** | Real-time charts: throughput, latency, errors, route breakdown | Open `http://localhost:8081/dashboard/` |

**Total setup time: under 2 minutes. Zero lines changed in Flask app.**

---

## Story 2: "Our API is getting abused — we need rate limiting NOW"

### Persona
**Rahul**, a DevOps engineer. Their Node.js API is getting hammered by a single client making 500 req/s. The service is degrading for all users. They need rate limiting deployed in minutes, not days.

### The Problem (Without Sidekick)
- Need to write/integrate a rate limiter into the Express app
- Requires code review, testing, deployment cycle
- Can't deploy fast enough during the incident

### The Solution (With Sidekick)

**No code changes. No redeployment of the Node.js app. Deploy Sidekick in front.**

```bash
# Aggressive rate limiting: 5 req/s per IP, burst of 10
SIDEKICK_UPSTREAM_URL=http://localhost:3000 \
SIDEKICK_RATE_LIMIT_RATE=5 \
SIDEKICK_RATE_LIMIT_BURST=10 \
go run ./cmd/sidekick/
```

**Redirect traffic from port 3000 → port 8081** (via nginx/load balancer config or DNS).

### What Happens Under the Hood

```
Client (abusive IP: 203.0.113.50)
  │
  ├─► Request 1-10:  ALLOWED (burst budget)
  ├─► Request 11:    BLOCKED → 429 {"error": "rate limit exceeded"}
  │                   Response includes: Retry-After: 1
  │                   Metric incremented: sidekick_ratelimit_hits_total
  │
  ... 1 second passes, 5 new tokens added ...
  │
  ├─► Request 12-16: ALLOWED (refilled tokens)
  └─► Request 17:    BLOCKED → 429
```

### Verification
```bash
# Simulate abuse: 20 rapid requests
for i in $(seq 1 20); do
  echo "Request $i: $(curl -s -o /dev/null -w '%{http_code}' http://localhost:8081/api)"
done
# Output: first 10 return 200, rest return 429

# Check dashboard for rate limit visualization
open http://localhost:8081/dashboard/

# Check Prometheus metric
curl -s http://localhost:8081/metrics | grep ratelimit
# sidekick_ratelimit_hits_total 10
```

### Technical Detail: Token Bucket Algorithm
```
Each IP gets its own bucket:
  - Capacity: SIDEKICK_RATE_LIMIT_BURST (max tokens)
  - Refill rate: SIDEKICK_RATE_LIMIT_RATE (tokens per second)
  - Each request costs 1 token
  - If bucket empty → 429 rejected
  - Stale buckets auto-cleaned every 5 minutes (prevents memory leaks)
```

---

## Story 3: "I need to monitor 5 microservices — without modifying any of them"

### Persona
**Ananya**, a platform engineer. She manages 5 microservices (Go, Python, Node, Java, Rust). She needs unified metrics and dashboards across all of them, but each uses different frameworks and logging libraries.

### The Problem (Without Sidekick)
- Each service needs its own Prometheus client library
- Metric names and labels are inconsistent across languages
- Some services have no metrics at all
- Building a unified dashboard requires normalizing data from 5 different sources

### The Solution (With Sidekick)

Run one Sidekick instance per service. All produce **identical metrics** regardless of the upstream language.

```bash
# Service A (Go, port 3001)
SIDEKICK_PORT=8001 SIDEKICK_UPSTREAM_URL=http://localhost:3001 go run ./cmd/sidekick/ &

# Service B (Python, port 3002)
SIDEKICK_PORT=8002 SIDEKICK_UPSTREAM_URL=http://localhost:3002 go run ./cmd/sidekick/ &

# Service C (Node, port 3003)
SIDEKICK_PORT=8003 SIDEKICK_UPSTREAM_URL=http://localhost:3003 go run ./cmd/sidekick/ &
```

### What Ananya Gets

**Uniform metrics across ALL services** (same metric names, same labels):

```bash
# Service A metrics
curl http://localhost:8001/metrics
# sidekick_requests_total{method="GET",route="/users",status="200"} 142
# sidekick_request_duration_seconds_bucket{...}

# Service B metrics — exact same format
curl http://localhost:8002/metrics
# sidekick_requests_total{method="POST",route="/orders",status="201"} 89
# sidekick_request_duration_seconds_bucket{...}
```

**Prometheus scrape config** (add all Sidekick instances):
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'sidekick'
    static_configs:
      - targets:
        - 'localhost:8001'  # Service A
        - 'localhost:8002'  # Service B
        - 'localhost:8003'  # Service C
```

**Per-service dashboard**: Each Sidekick serves its own dashboard:
- `http://localhost:8001/dashboard/` — Service A metrics
- `http://localhost:8002/dashboard/` — Service B metrics
- `http://localhost:8003/dashboard/` — Service C metrics

**Cross-service request tracing**: Sidekick generates `X-Request-ID` headers. If Service A calls Service B, pass the header along → trace the full request chain in logs.

---

## Story 4: "I'm a new developer joining the team — how do I understand what's happening?"

### Persona
**Dev**, a junior developer who just joined the team. They need to understand API traffic patterns, which endpoints are slow, and which ones error frequently.

### The Problem (Without Sidekick)
- Logs are scattered across multiple files
- No dashboard — must grep through raw logs
- No metrics — must count errors manually
- Takes days to build a mental model of the system

### The Solution (With Sidekick)

**Open the dashboard. Everything is visible in real-time.**

```bash
open http://localhost:8081/dashboard/
```

### What Dev Sees

**Stat Cards (top row):**
```
┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ Total Reqs  │ │ Success Rate│ │ Rate Limited│ │ Avg Latency │ │  Uptime     │
│   12,847    │ │   97.2%     │ │     23      │ │   14.3ms    │ │  2h 15m     │
│  48.2 req/s │ │ 12483/364   │ │ 429 errors  │ │ all routes  │ │             │
└─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘
```

**Route Breakdown Table:**
```
Method  Route           Status  Requests  Avg Latency  Total Time
GET     /api/users      200     5,231     8.2ms        42.9s
POST    /api/orders     201     3,102     45.1ms       139.9s
GET     /api/products   200     2,847     12.3ms       35.0s
GET     /api/users      500     312       2.1ms        0.7s      ← problem!
POST    /api/orders     429     23        0.1ms        0.002s
```

**Dev immediately sees:** `/api/users` has a 5.6% error rate (312 of 5543). The 500s are fast (2.1ms) → probably failing early (validation? database connection?). This would take hours to discover from raw logs.

### Terminal Logs (structured, traceable)
```
[SIDEKICK] [a1b2c3d4-...] 200 GET /api/users 8ms client=10.0.0.1
[SIDEKICK] [e5f6g7h8-...] 500 GET /api/users 2ms client=10.0.0.2
[SIDEKICK] [i9j0k1l2-...] 201 POST /api/orders 45ms client=10.0.0.1
[SIDEKICK] [m3n4o5p6-...] 429 POST /api/orders 0ms client=10.0.0.3
```

Every log line has: request ID, status, method, path, duration, client IP. Dev can trace any request end-to-end using the ID.

---

## Story 5: "I want to test my API's behavior under load"

### Persona
**Karan**, a developer preparing for a product launch. He needs to understand how his service behaves under heavy traffic and wants to see metrics in real-time during load testing.

### The Solution

**Step 1: Start the service with Sidekick**
```bash
SIDEKICK_UPSTREAM_URL=http://localhost:3000 go run ./cmd/sidekick/
```

**Step 2: Open the dashboard in one window**
```
http://localhost:8081/dashboard/
```

**Step 3: Run load test in another terminal**
```bash
# Using 'hey' load testing tool
hey -n 10000 -c 50 http://localhost:8081/api/products
```

**Step 4: Watch the dashboard live**
- **Requests Over Time** chart spikes to show throughput
- **Avg Latency** card shows if latency degrades under load
- **Status Code Distribution** reveals if errors increase
- **Rate Limit Hits** shows if burst is configured too low
- **Route Breakdown** shows per-endpoint performance

**Step 5: Check Prometheus for detailed histograms**
```bash
# P99 latency from histogram
curl -s http://localhost:8081/metrics | grep 'duration_seconds_bucket'
```

---

## Story 6: "I need health checks for my Kubernetes deployment"

### Persona
**Sneha**, a DevOps engineer deploying services on Kubernetes. She needs liveness and readiness probes, but the upstream service doesn't have a `/health` endpoint.

### The Solution

Sidekick provides `/health` out of the box. Configure K8s probes to hit Sidekick:

```yaml
# kubernetes deployment.yaml
spec:
  containers:
    # Your application
    - name: api
      image: my-api:latest
      ports:
        - containerPort: 3000

    # Sidekick sidecar
    - name: sidekick
      image: sidekick:latest
      ports:
        - containerPort: 8081
      env:
        - name: SIDEKICK_UPSTREAM_URL
          value: "http://localhost:3000"
        - name: SIDEKICK_PORT
          value: "8081"
      livenessProbe:
        httpGet:
          path: /health
          port: 8081
        initialDelaySeconds: 5
        periodSeconds: 10
      readinessProbe:
        httpGet:
          path: /health
          port: 8081
        initialDelaySeconds: 3
        periodSeconds: 5
```

**Health response:**
```json
{"status": "healthy", "service": "sidekick"}
```

**K8s gets:** standardized health checks without touching application code.

---

## Story 7: "I want to debug a specific request that failed"

### Persona
**Amit**, a developer investigating a customer-reported error. The customer says "my order failed" but Amit doesn't know which request it was.

### The Solution

Every Sidekick request gets a unique `X-Request-ID`. If the frontend captures and displays this ID to the user, debugging becomes trivial:

**Frontend shows:** "Something went wrong. Reference: `a1b2c3d4-e5f6-7890-abcd-ef1234567890`"

**Amit searches logs:**
```bash
grep "a1b2c3d4-e5f6-7890-abcd-ef1234567890" /var/log/sidekick.log
# [SIDEKICK] [a1b2c3d4-e5f6-7890-abcd-ef1234567890] 500 POST /api/orders 234ms client=203.0.113.50
```

**Found it.** POST to `/api/orders`, took 234ms, returned 500, from IP `203.0.113.50`.

If the client sends their own `X-Request-ID`, Sidekick preserves it:
```bash
curl -H "X-Request-ID: customer-trace-999" http://localhost:8081/api/orders
# Sidekick uses "customer-trace-999" instead of generating a new one
```

---

## Quick Reference: What You Get vs What You Write

| Concern | Without Sidekick (you write) | With Sidekick (you write) |
|---|---|---|
| Rate limiting | 200-500 lines + dependency + tests | **0 lines** — configured via env |
| Structured logging | 100-300 lines + library setup | **0 lines** — automatic |
| Request tracing | 50-100 lines of middleware | **0 lines** — automatic |
| Prometheus metrics | 100-200 lines + client library | **0 lines** — automatic |
| Health endpoint | 20-50 lines | **0 lines** — built-in |
| Real IP extraction | 30-80 lines of middleware | **0 lines** — automatic |
| Monitoring dashboard | 500-2000 lines + frontend framework | **0 lines** — embedded |
| **Total** | **1000-3000 lines across every service** | **4 env vars in a .env file** |

---

## Setup Cheatsheet

```bash
# 1. Clone
git clone https://github.com/your-username/sidekick.git && cd sidekick

# 2. Configure (edit .env with your upstream URL)
cp .env.example .env

# 3. Test
go test ./... -v

# 4. Run
go run ./cmd/sidekick/

# 5. Verify
curl http://localhost:8081/health          # Health check
curl http://localhost:8081/metrics         # Prometheus metrics
open http://localhost:8081/dashboard/      # Live dashboard
```

**That's it. Your service now has production-grade observability, rate limiting, and monitoring — with zero code changes.**
