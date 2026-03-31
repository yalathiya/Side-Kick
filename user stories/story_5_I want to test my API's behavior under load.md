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
