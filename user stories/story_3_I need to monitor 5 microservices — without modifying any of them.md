

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
