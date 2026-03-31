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
