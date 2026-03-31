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
