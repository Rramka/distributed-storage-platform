// Package scheduler picks online nodes for fragment placements.
// docs/06-scheduler-and-repair.md — hard constraints plus scored weighted sampling.
package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/apierr"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const liveKeyPrefix = "node:live:"
const reservePrefix = "reserve:"

// Hard placement caps (docs/06-scheduler-and-repair.md).
const (
	RegionCap = 3
	ASNCap    = 3
	OwnerCap  = 2
)

var (
	ErrUnavailable = errors.New("scheduler: no eligible nodes")
)

// Need is one fragment that requires a node.
type Need struct {
	FragmentID uuid.UUID `json:"fragment_id"`
	ChunkID    uuid.UUID `json:"chunk_id"`
	SizeBytes  int64     `json:"size_bytes"`
}

// Assignment is a chosen node for a fragment.
type Assignment struct {
	FragmentID uuid.UUID  `json:"fragment_id"`
	Node       store.Node `json:"node"`
}

// Service places fragments.
type Service struct {
	Store *store.Store
	Redis *redis.Client
	TTL   time.Duration
}

// Place selects distinct online live nodes per chunk under hard caps.
// Unplaceable fragments are omitted (partial fill); PlanUpload rejects
// a chunk that cannot reach the commit threshold.
func (s *Service) Place(ctx context.Context, needs []Need) ([]Assignment, error) {
	if s.TTL <= 0 {
		s.TTL = 15 * time.Minute
	}
	nodes, err := s.Store.ListOnlineNodes(ctx)
	if err != nil {
		return nil, err
	}
	var live []store.Node
	for _, n := range nodes {
		ok, err := s.Redis.Exists(ctx, liveKeyPrefix+n.ID.String()).Result()
		if err != nil {
			return nil, fmt.Errorf("scheduler.place: %w", err)
		}
		if ok == 1 && n.CapacityBytes-n.UsedBytes > 0 {
			live = append(live, n)
		}
	}
	if len(live) == 0 {
		return nil, ErrUnavailable
	}
	stats, err := s.Store.LatestNodeStats(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := s.Store.CountLivePlacementsAll(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(live))
	for _, n := range live {
		ids = append(ids, n.ID)
	}
	metrics := heartbeatMetrics(ctx, s.Redis, ids)
	for i := range live {
		if st, ok := stats[live[i].ID]; ok {
			live[i].UptimeRatio = st.UptimeRatio
			if live[i].LatencyMS == 0 {
				live[i].LatencyMS = st.AvgLatencyMS
			}
			if live[i].CPULoad == 0 {
				live[i].CPULoad = st.CPULoad
			}
			if live[i].MemUsedRatio == 0 {
				live[i].MemUsedRatio = st.MemUsedRatio
			}
		}
		if m, ok := metrics[live[i].ID]; ok {
			live[i].LatencyMS = m.LatencyMS
			live[i].CPULoad = m.CPULoad
			live[i].MemUsedRatio = m.MemUsedRatio
		}
		live[i].LivePlacements = counts[live[i].ID]
	}

	usedInChunk := map[uuid.UUID]*chunkUse{}
	if ids := chunkIDs(needs); s.Store != nil && len(ids) > 0 {
		occ, err := s.Store.ListOccupants(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, o := range occ {
			if usedInChunk[o.ChunkID] == nil {
				usedInChunk[o.ChunkID] = newChunkUse()
			}
			usedInChunk[o.ChunkID].add(o.Node)
		}
	}
	out := make([]Assignment, 0, len(needs))
	for _, need := range needs {
		if usedInChunk[need.ChunkID] == nil {
			usedInChunk[need.ChunkID] = newChunkUse()
		}
		n, err := pick(live, usedInChunk[need.ChunkID])
		if err != nil {
			continue
		}
		usedInChunk[need.ChunkID].add(n)
		for i := range live {
			if live[i].ID == n.ID {
				live[i].LivePlacements++
				break
			}
		}
		key := reservePrefix + n.ID.String() + ":" + need.FragmentID.String()
		if err := s.Redis.Set(ctx, key, need.SizeBytes, s.TTL).Err(); err != nil {
			return nil, fmt.Errorf("scheduler.place: %w", err)
		}
		out = append(out, Assignment{FragmentID: need.FragmentID, Node: n})
	}
	return out, nil
}

func chunkIDs(needs []Need) []uuid.UUID {
	seen := map[uuid.UUID]struct{}{}
	var ids []uuid.UUID
	for _, n := range needs {
		if _, ok := seen[n.ChunkID]; ok {
			continue
		}
		seen[n.ChunkID] = struct{}{}
		ids = append(ids, n.ChunkID)
	}
	return ids
}

type chunkUse struct {
	nodes   map[uuid.UUID]struct{}
	regions map[string]int
	asns    map[int]int
	owners  map[uuid.UUID]int
}

func newChunkUse() *chunkUse {
	return &chunkUse{
		nodes:   map[uuid.UUID]struct{}{},
		regions: map[string]int{},
		asns:    map[int]int{},
		owners:  map[uuid.UUID]int{},
	}
}

func (u *chunkUse) add(n store.Node) {
	u.nodes[n.ID] = struct{}{}
	if n.Region != "" {
		u.regions[n.Region]++
	}
	if n.ASN != nil {
		u.asns[*n.ASN]++
	}
	u.owners[n.OwnerID]++
}

func pick(nodes []store.Node, used *chunkUse) (store.Node, error) {
	var cand []store.Node
	var weights []float64
	var sum float64
	for _, n := range nodes {
		if !eligible(n, used) {
			continue
		}
		w := nodeScore(n)
		cand = append(cand, n)
		weights = append(weights, w)
		sum += w
	}
	if len(cand) == 0 {
		return store.Node{}, ErrUnavailable
	}
	if sum <= 0 {
		sum = float64(len(cand))
		for i := range weights {
			weights[i] = 1
		}
	}
	max := big.NewInt(1 << 53)
	r, err := rand.Int(rand.Reader, max)
	if err != nil {
		return store.Node{}, err
	}
	x := float64(r.Int64()) / float64(max.Int64()) * sum
	acc := 0.0
	for i, w := range weights {
		acc += w
		if x <= acc {
			return cand[i], nil
		}
	}
	return cand[len(cand)-1], nil
}

func eligible(n store.Node, used *chunkUse) bool {
	if _, ok := used.nodes[n.ID]; ok {
		return false
	}
	if n.Region != "" && used.regions[n.Region] >= RegionCap {
		return false
	}
	if n.ASN != nil && used.asns[*n.ASN] >= ASNCap {
		return false
	}
	if used.owners[n.OwnerID] >= OwnerCap {
		return false
	}
	if n.CapacityBytes-n.UsedBytes <= 0 {
		return false
	}
	if inProbation(n) && n.LivePlacements >= ProbationQuota {
		return false
	}
	return true
}

func inProbation(n store.Node) bool {
	if n.ProbationUntil != nil && time.Now().Before(*n.ProbationUntil) {
		return true
	}
	return false
}

func heartbeatMetrics(ctx context.Context, rdb *redis.Client, ids []uuid.UUID) map[uuid.UUID]store.Node {
	out := map[uuid.UUID]store.Node{}
	if rdb == nil {
		return out
	}
	for _, id := range ids {
		m, err := rdb.HGetAll(ctx, "node:metrics:"+id.String()).Result()
		if err != nil || len(m) == 0 {
			continue
		}
		var n store.Node
		n.ID = id
		if v, err := parseFloat(m["cpu_load"]); err == nil {
			n.CPULoad = float32(v)
		}
		if v, err := parseFloat(m["mem_used_ratio"]); err == nil {
			n.MemUsedRatio = float32(v)
		}
		if v, err := parseFloat(m["disk_read_latency_ms"]); err == nil {
			n.LatencyMS = float32(v)
		}
		out[id] = n
	}
	return out
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// Mount registers POST /internal/placements.
func Mount(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("POST /internal/placements", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Needs []Need `json:"needs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", apierr.NewRequestID())
			return
		}
		as, err := svc.Place(r.Context(), req.Needs)
		if errors.Is(err, ErrUnavailable) {
			apierr.Write(w, apierr.CodePlacementUnavailable, "no eligible nodes", apierr.NewRequestID())
			return
		}
		if err != nil {
			apierr.Write(w, apierr.CodeInternal, "internal error", apierr.NewRequestID())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"placements": as})
	})
}
