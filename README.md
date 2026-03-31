# 🚀 Sidekick — Zero-Config API Sidecar Proxy

Sidekick is a lightweight, plug-and-play API sidecar that adds **rate limiting, logging, metrics, and request tracing** to your application — without modifying your code.

It runs alongside your service and enhances it with production-grade backend capabilities.

---

## ✨ Features

* 🔁 **Reverse Proxy** — Forward requests to upstream services
* 🚦 **Rate Limiting** — Token bucket (per IP)
* 🧾 **Request Logging** — Method, path, status, latency, client IP
* 🆔 **Request Tracing** — Unique `X-Request-ID` for every request
* 📊 **Prometheus Metrics** — `/metrics` endpoint
* ❤️ **Health Check** — `/health` endpoint

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
│   │   └── logging.go
│   ├── metrics/
│   │   └── metrics.go
│   ├── ratelimit/
│   │   ├── limiter.go
│   │   └── middleware.go
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

### 2️⃣ Run the server

```bash
go run ./cmd/sidekick
```

Server will start on:

```
http://localhost:8081
```

---

### 3️⃣ Test endpoints

#### Health Check

```bash
curl http://localhost:8081/health
```

#### Proxy Request

```bash
curl http://localhost:8081/get
```

#### Metrics

```bash
curl http://localhost:8081/metrics
```

---

## 📊 Example Metrics

```
sidekick_requests_total{method="GET",route="/get",status="200"} 10
sidekick_ratelimit_hits_total 2
sidekick_request_duration_seconds_bucket{...}
```

---

## 🚦 Rate Limiting

* Algorithm: **Token Bucket**
* Default: **5 requests / 10 seconds per IP**
* Response when exceeded:

```
HTTP 429 Too Many Requests
Retry-After: 1
```

---

## 🧾 Logging Example

```
[abc123] 200 GET /get  8.2ms  client=127.0.0.1
```

---

## 🎯 Design Goals

* ⚡ Low latency overhead
* 🧩 Zero configuration
* 🪶 Lightweight (single binary)
* 🔍 Built-in observability
* 🔌 Easy integration with any backend

---

## 🚀 Roadmap

* 🔐 JWT Authentication
* 🧠 Config + Hot Reload (SQLite)
* 🌐 Distributed Rate Limiting (Redis)
* 🔁 Circuit Breaker & Retry
* 📦 Kubernetes Sidecar Deployment
* 📑 Structured Logging (`zap`)

---

## 🛠️ Tech Stack

* Go (Golang)
* Chi Router
* Prometheus Client
* net/http Reverse Proxy

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
