package health

import (
	"encoding/json"
	"net/http"
	"time"
)

// Response is the JSON body for GET /healthz.
type Response struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Time    string `json:"time"`
}

// Handler returns a JSON 200 for process liveness. It does not check Postgres/Redis/NATS.
func Handler(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Response{
			Status:  "ok",
			Service: service,
			Time:    time.Now().UTC().Format(time.RFC3339),
		})
	}
}
