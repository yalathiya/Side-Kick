# 🚀 Sidekick — Zero-Config API Sidecar Proxy

Sidekick is a lightweight, plug-and-play API sidecar that adds **rate limiting, authentication, logging, observability, and resilience** to your backend — without modifying your application code.

It follows the **sidecar pattern**, running alongside your service and handling cross-cutting concerns like traffic control and monitoring.

---

## ✨ Core Capabilities

Sidekick is designed as a **complete API gateway/sidecar solution**, with the following capabilities:

### 🔁 Traffic Management

* Reverse proxy routing to upstream services
* Path-based request forwarding
* Header enrichment (`X-Forwarded-*`, `X-Sidekick`)

---

### 🚦 Rate Limiting

* Token Bucket algorithm
* Per-IP, per-user, and per-endpoint limits
* Configurable policies
* Distributed rate limiting (Redis — planned)

---

### 🔐 Authentication & Security

* JWT-based authentication
* API key support
* Role-based access control (RBAC)
* Secure header propagation

---

### 🧾 Logging & Tracing

* Structured request logging
* Request lifecycle tracking
* Unique `X-Request-ID` for tracing
* Correlation across services

---

### 📊 Observability

* Prometheus metrics (`/metrics`)
* Request count, latency, error rates
* Rate limit tracking
* Future: distributed tracing (OpenTelemetry)

---

### ❤️ Health & Reliability

* Health check endpoint (`/health`)
* Graceful error handling
* Upstream timeout management

---

### 🔁 Resilience (Planned)

* Circuit breaker
* Retry mechanism with backoff
* Failure tracking and recovery

---

### ⚙️ Configuration Management (Planned)

* SQLite-based configuration
* Hot reload (no restart required)
* CLI & dashboard for configuration

---

### ☸️ Deployment

* Runs as:

  * Local proxy
  * Docker container
  * Kubernetes sidecar
* Future: Helm chart support

---

## 🧱 Architecture

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
Auth Middleware (JWT/API Key)
      ↓
Circuit Breaker (planned)
      ↓
Retry Handler (planned)
      ↓
Reverse Proxy
      ↓
Upstream Service
```

---

## 📁 Project Structure

```
sidekick/
│
├── cmd/
│   └── sidekick/
│       └── main.go
│
├── internal/
│   ├── logging/
│   ├── metrics/
│   ├── ratelimit/
│   ├── auth/           (planned)
│   ├── config/         (planned)
│   ├── circuitbreaker/ (planned)
│   └── retry/          (planned)
│
└── go.mod
```

---

## ⚙️ Getting Started

### 1️⃣ Clone the repository

```bash
git clone https://github.com/your-username/sidekick.git
cd sidekick
```

---

### 2️⃣ Run Sidekick

```bash
go run ./cmd/sidekick
```

Server starts on:

```
http://localhost:8081
```

---

### 3️⃣ Test Endpoints

#### Health

```bash
curl http://localhost:8081/health
```

#### Proxy

```bash
curl http://localhost:8081/get
```

#### Metrics

```bash
curl http://localhost:8081/metrics
```

---

## 📊 Metrics Example

```
sidekick_requests_total{method="GET",route="/get",status="200"} 10
sidekick_ratelimit_hits_total 2
sidekick_request_duration_seconds_bucket{...}
```

---

## 🧾 Logging Example

```
[req-123] 200 GET /get  8.2ms  client=127.0.0.1
```

---

## 🎯 Design Goals

* ⚡ Minimal latency overhead
* 🪶 Lightweight & efficient
* 🔌 Zero-config developer experience
* 🔍 Built-in observability
* 🧩 Modular & extensible architecture

---

## 🚀 Roadmap

### Phase 1 (Current)

* Reverse proxy
* Rate limiting (in-memory)
* Logging + Request ID
* Prometheus metrics
* Health endpoint

---

### Phase 2

* JWT Authentication
* Config management (SQLite + hot reload)
* Redis-based distributed rate limiting

---

### Phase 3

* Circuit breaker
* Retry mechanism
* Advanced logging (Zap)
* Admin dashboard

---

### Phase 4

* Kubernetes sidecar integration
* Helm charts
* OpenTelemetry tracing

---

## 🛠️ Tech Stack

* **Go (Golang)**
* **Chi Router**
* **Prometheus Client**
* **net/http Reverse Proxy**

---

## 🤝 Contributing

Contributions are welcome!
Feel free to open issues or submit pull requests.

---

## 📄 License

MIT License

---

## 👨‍💻 Author

**Yash Lathiya**
Software Engineer | Backend & System Design Enthusiast

---

⭐ If you like this project, give it a star!
