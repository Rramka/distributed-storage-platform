// Package healthmon accepts node heartbeats over mTLS and records Redis liveness.
// docs/05-node-agent.md. online → suspect → offline is published to NATS (M4).
package healthmon

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
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

// HeartbeatInterval is the agent heartbeat cadence (docs/05-node-agent.md).
const HeartbeatInterval = 10 * time.Second

// HeartbeatCountKey is the Redis counter for heartbeats in one UTC hour.
func HeartbeatCountKey(nodeID uuid.UUID, window time.Time) string {
	return fmt.Sprintf("node:hb:%s:%d", nodeID, window.UTC().Truncate(time.Hour).Unix())
}

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
	if err := store.AdmitNode(node, time.Now().UTC()); err != nil {
		apierr.Write(w, apierr.CodeForbidden, "node denied", apierr.NewRequestID())
		return
	}
	var req struct {
		NodeID            string  `json:"node_id"`
		FreeBytes         int64   `json:"free_bytes"`
		UsedBytes         int64   `json:"used_bytes"`
		CapacityHint      int64   `json:"capacity_bytes"`
		FragmentCount     uint32  `json:"fragment_count"`
		AgentVersion      string  `json:"agent_version"`
		CPULoad           float64 `json:"cpu_load"`
		MemUsedRatio      float64 `json:"mem_used_ratio"`
		DiskReadLatencyMS float64 `json:"disk_read_latency_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("healthmon heartbeat decode", "err", err, "node_id", node.ID)
	}
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
	window := time.Now().UTC().Truncate(time.Hour)
	hbKey := HeartbeatCountKey(node.ID, window)
	if err := s.Redis.Incr(r.Context(), hbKey).Err(); err != nil {
		slog.Error("healthmon heartbeat count", "err", err, "node_id", node.ID)
	} else if err := s.Redis.Expire(r.Context(), hbKey, 3*time.Hour).Err(); err != nil {
		slog.Error("healthmon heartbeat expire", "err", err, "node_id", node.ID)
	}
	if err := s.Redis.HSet(r.Context(), "node:metrics:"+node.ID.String(), map[string]any{
		"free_bytes":           req.FreeBytes,
		"used_bytes":           req.UsedBytes,
		"cpu_load":             req.CPULoad,
		"mem_used_ratio":       req.MemUsedRatio,
		"disk_read_latency_ms": req.DiskReadLatencyMS,
		"fragment_count":       req.FragmentCount,
	}).Err(); err != nil {
		slog.Error("healthmon metrics hset", "err", err, "node_id", node.ID)
	}
	if err := s.Redis.Expire(r.Context(), "node:metrics:"+node.ID.String(), 2*time.Minute).Err(); err != nil {
		slog.Error("healthmon metrics expire", "err", err, "node_id", node.ID)
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
	msgs := []any{}
	if node.CertExpiresAt != nil && time.Until(*node.CertExpiresAt) < 7*24*time.Hour {
		msgs = append(msgs, map[string]string{"type": "renew"})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"messages": msgs})
}

// MountScrub registers the agent self-report endpoint.
func MountScrub(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("POST /internal/scrub-report", s.handleScrubReport)
}

func (s *Server) handleScrubReport(w http.ResponseWriter, r *http.Request) {
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
	if err := store.AdmitNode(node, time.Now().UTC()); err != nil {
		apierr.Write(w, apierr.CodeForbidden, "node denied", apierr.NewRequestID())
		return
	}
	var req struct {
		FragmentID string `json:"fragment_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", apierr.NewRequestID())
		return
	}
	fid, err := uuid.Parse(req.FragmentID)
	if err != nil {
		apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", apierr.NewRequestID())
		return
	}
	lp, err := s.Store.MarkPlacementLost(r.Context(), fid, node.ID)
	if err != nil {
		apierr.Write(w, apierr.CodeInternal, "internal error", apierr.NewRequestID())
		return
	}
	_, _, _ = s.Store.ApplyReputationEvent(r.Context(), node.ID, store.SignalHonestSelfReport)
	_ = s.Store.InsertAudit(r.Context(), "node", &node.ID, "node.scrub", "fragment", &fid, nil)
	if s.Bus != nil && lp.ChunkID != uuid.Nil {
		healthy, err := s.Store.ChunkHealth(r.Context(), lp.ChunkID)
		if err == nil {
			_ = s.Bus.PublishRepair(r.Context(), events.RepairJob{ChunkID: lp.ChunkID, Healthy: healthy})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// TLSListenerConfig is unused placeholder for cmd wiring.
func TLSListenerConfig(cfg *tls.Config) *tls.Config { return cfg }
