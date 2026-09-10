// Package healthmon accepts node heartbeats over mTLS and records Redis liveness.
// docs/05-node-agent.md. online → suspect → offline is published to NATS (M4).
package healthmon

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/apierr"
	"github.com/Rramka/distributed-storage-platform/internal/events"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const liveTTL = 30 * time.Second

// Server handles POST /internal/heartbeat.
type Server struct {
	Store *store.Store
	Redis *redis.Client
	Bus   *events.Bus
}

// Mount registers heartbeat on mux (plain or TLS).
func Mount(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("POST /internal/heartbeat", s.handleHeartbeat)
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		apierr.Write(w, apierr.CodeUnauthenticated, "mtls required", apierr.NewRequestID())
		return
	}
	fp := sha256.Sum256(r.TLS.PeerCertificates[0].Raw)
	node, err := s.Store.NodeByFingerprint(r.Context(), fp[:])
	if err != nil {
		apierr.Write(w, apierr.CodeForbidden, "unknown node", apierr.NewRequestID())
		return
	}
	var req struct {
		NodeID        string `json:"node_id"`
		FreeBytes     int64  `json:"free_bytes"`
		UsedBytes     int64  `json:"used_bytes"`
		CapacityHint  int64  `json:"capacity_bytes"`
		FragmentCount uint32 `json:"fragment_count"`
		AgentVersion  string `json:"agent_version"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.NodeID != "" {
		id, err := uuid.Parse(req.NodeID)
		if err == nil && id != node.ID {
			apierr.Write(w, apierr.CodeForbidden, "node id mismatch", apierr.NewRequestID())
			return
		}
	}
	if err := s.Redis.Set(r.Context(), "node:live:"+node.ID.String(), "1", liveTTL).Err(); err != nil {
		slog.Error("healthmon redis", "err", err)
		apierr.Write(w, apierr.CodeInternal, "internal error", apierr.NewRequestID())
		return
	}
	from, to, err := s.Store.TouchNodeStatus(r.Context(), node.ID, req.UsedBytes, node.CapacityBytes)
	if err != nil {
		slog.Error("healthmon touch", "err", err)
		apierr.Write(w, apierr.CodeInternal, "internal error", apierr.NewRequestID())
		return
	}
	if from != to && s.Bus != nil {
		if err := s.Bus.PublishNode(r.Context(), events.NodeEvent{
			NodeID: node.ID,
			From:   from,
			To:     to,
			At:     time.Now().UTC(),
		}); err != nil {
			slog.Error("healthmon publish", "err", err, "node_id", node.ID)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"messages": []any{}})
}

// TLSListenerConfig is unused placeholder for cmd wiring.
func TLSListenerConfig(cfg *tls.Config) *tls.Config { return cfg }
