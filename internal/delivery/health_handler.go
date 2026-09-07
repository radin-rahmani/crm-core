package delivery

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
)

// HealthHandler exposes liveness and readiness probes for orchestrators
// like Kubernetes.
type HealthHandler struct {
	db  *sql.DB
	rdb *redis.Client
}

func NewHealthHandler(db *sql.DB, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, rdb: rdb}
}

// Liveness reports whether the process itself is up. It never checks
// dependencies — if this fails, the orchestrator should restart the pod.
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// Readiness reports whether the service can currently serve traffic.
// It checks downstream dependencies — if this fails, the orchestrator
// should stop routing traffic to this pod but should NOT restart it.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		http.Error(w, "database unavailable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err := h.rdb.Ping(ctx).Err(); err != nil {
		http.Error(w, "cache unavailable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
