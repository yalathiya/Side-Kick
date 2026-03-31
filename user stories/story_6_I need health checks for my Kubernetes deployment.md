
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

