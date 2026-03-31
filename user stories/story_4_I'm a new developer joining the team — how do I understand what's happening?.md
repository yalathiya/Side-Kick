
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

