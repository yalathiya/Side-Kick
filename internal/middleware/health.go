package middleware

import (
	"encoding/json"
	"net/http"
)

// HealthResponse is the JSON response for the health endpoint.
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// HealthCheck returns a handler for the /health endpoint.
func HealthCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(HealthResponse{
			Status:  "healthy",
			Service: "sidekick",
		})
	}
}
