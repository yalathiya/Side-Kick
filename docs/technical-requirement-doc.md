# 📄 Sidekick — Technical Design Document (Advanced)

---

# 1. 📌 Overview

Sidekick is a lightweight API sidecar that acts as a **policy enforcement layer** between clients and backend services.

It implements:

* Traffic control (rate limiting)
* Security (authentication)
* Observability (logging + metrics)
* Resilience (circuit breaker + retry)

---

# 2. 🧱 System Architecture

```text
Client
  ↓
Sidekick (Middleware Pipeline)
  ↓
Upstream Service
```

### Expanded Pipeline

```text
Request
 ↓
RequestID
 ↓
RealIP
 ↓
Logging
 ↓
Metrics
 ↓
Rate Limiter
 ↓
Auth Middleware (Phase 2)
 ↓
Circuit Breaker (Phase 3)
 ↓
Retry Handler (Phase 3)
 ↓
Reverse Proxy
 ↓
Upstream
```

---

# 3. ⚙️ Phase 2 — Advanced Features

---

## 3.1 🔐 JWT Authentication

### Flow

```text
Request → Extract Token → Validate → Attach Claims → Continue
```

### Implementation

```go
type Claims struct {
    UserID string
    Roles  []string
}
```

### Steps

1. Extract token from header:

```text
Authorization: Bearer <token>
```

2. Validate:

* Signature (HMAC / RSA)
* Expiry (`exp`)
* Issuer (`iss`)

3. Attach to context:

```go
ctx := context.WithValue(r.Context(), "user", claims)
```

4. Forward headers:

```
X-User-ID
X-User-Roles
```

### Edge Cases

* Missing token → 401
* Expired token → 401
* Invalid signature → 403

---

## 3.2 ⚙️ Configuration Management (SQLite + Hot Reload)

### Why SQLite?

* Lightweight
* Embedded (no external dependency)
* Fast reads

---

### Schema

```sql
CREATE TABLE config (
    key TEXT PRIMARY KEY,
    value TEXT
);

CREATE TABLE rate_limits (
    id INTEGER PRIMARY KEY,
    path TEXT,
    limit INTEGER,
    window_seconds INTEGER
);
```

---

### Config Loader

```go
type Config struct {
    RateLimits []RateLimit
}
```

---

### Hot Reload Strategy

#### Approach 1: File Watcher (MVP)

* Watch SQLite file changes
* Reload config into memory

#### Approach 2: Polling (safer)

* Check version/timestamp every N seconds

---

### In-Memory Cache

```go
var currentConfig atomic.Value
```

Why:

* Lock-free reads
* High performance

---

## 3.3 🌐 Distributed Rate Limiting (Redis)

### Problem

In-memory limiter is **per instance only**

---

### Solution: Redis Token Bucket (Lua Script)

```lua
-- KEYS[1] = key
-- ARGV[1] = capacity
-- ARGV[2] = refill_rate
-- ARGV[3] = current_time

local tokens = redis.call("GET", KEYS[1])
...
```

---

### Flow

```text
Request → Redis → Token Check → Allow/Deny
```

---

### Key Design

```
ratelimit:<ip>
ratelimit:<user>
ratelimit:<endpoint>
```

---

### Benefits

* Atomic operations
* Shared state across instances
* Scalable

---

### Trade-offs

| Pros            | Cons             |
| --------------- | ---------------- |
| Distributed     | Network latency  |
| Accurate limits | Redis dependency |

---

# 4. ⚙️ Phase 3 — Resilience Layer

---

## 4.1 🔁 Circuit Breaker

### States

```text
Closed → Open → Half-Open → Closed
```

---

### Data Structure

```go
type CircuitBreaker struct {
    failures int
    state    string
    lastFail time.Time
}
```

---

### Logic

* If failures > threshold → OPEN
* In OPEN → reject requests immediately
* After timeout → HALF-OPEN
* Success → CLOSE
* Failure → OPEN again

---

### Implementation Strategy

* Per upstream service
* Store in `sync.Map`

---

## 4.2 🔄 Retry Mechanism

### When to retry?

* Network error
* 5xx response

---

### Strategy

```text
Retry 3 times with exponential backoff
```

---

### Example

```text
Attempt 1 → immediate
Attempt 2 → +100ms
Attempt 3 → +300ms
```

---

### Code Concept

```go
for i := 0; i < retries; i++ {
    resp, err := call()
    if success {
        return resp
    }
    sleep(backoff)
}
```

---

### Idempotency Consideration

Only retry:

* GET
* Safe requests

---

# 5. ⚙️ Phase 4 — Platform Level Features

---

## 5.1 ☸️ Kubernetes Sidecar

### Deployment Model

```yaml
containers:
  - name: app
  - name: sidekick
```

---

### Communication

* Sidekick → localhost:app-port
* External → Sidekick

---

### Benefits

* No app changes required
* Language agnostic

---

## 5.2 📦 Helm Chart

### Features

* Configurable rate limits
* Redis integration
* Resource limits

---

## 5.3 🔍 OpenTelemetry (Tracing)

### Flow

```text
Request → TraceID → Span → Export
```

---

### Integration

* Inject trace context
* Export to Jaeger / Zipkin

---

### Benefit

* Full request visibility across services

---

# 6. 📊 Observability Enhancements

### New Metrics

* Upstream errors
* Circuit breaker state
* Retry count

---

# 7. 🚀 Performance Strategy

* Use `sync.Map` for concurrency
* Avoid blocking I/O
* Use connection pooling
* Minimize allocations

---

# 8. ⚠️ Trade-offs & Challenges

| Area            | Challenge             |
| --------------- | --------------------- |
| Redis           | Network latency       |
| Circuit breaker | Tuning thresholds     |
| Metrics         | Cardinality explosion |
| Config reload   | Consistency           |

---

# 9. 🧠 Key Design Decisions

| Decision            | Reason                    |
| ------------------- | ------------------------- |
| Sidecar pattern     | Decoupled architecture    |
| Go                  | Performance + concurrency |
| Middleware pipeline | Extensibility             |
| Redis Lua           | Atomic operations         |

---

# 10. 🏁 Conclusion

Sidekick evolves from a **lightweight proxy** to a **production-ready traffic control and observability layer**, capable of scaling across distributed environments.

---
