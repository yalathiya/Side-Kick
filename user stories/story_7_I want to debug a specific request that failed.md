
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
