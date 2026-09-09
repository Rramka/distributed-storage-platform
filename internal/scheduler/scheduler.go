// Package scheduler picks online nodes for fragment placements (naive M2).
// docs/06-scheduler-and-repair.md — hard constraints only; scoring is M4.
package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/apierr"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const liveKeyPrefix = "node:live:"
const reservePrefix = "reserve:"

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

// Place selects distinct online live nodes per chunk.
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

	usedInChunk := map[uuid.UUID]map[uuid.UUID]struct{}{}
	out := make([]Assignment, 0, len(needs))
	for _, need := range needs {
		if usedInChunk[need.ChunkID] == nil {
			usedInChunk[need.ChunkID] = map[uuid.UUID]struct{}{}
		}
		n, err := pick(live, usedInChunk[need.ChunkID])
		if err != nil {
			return nil, err
		}
		usedInChunk[need.ChunkID][n.ID] = struct{}{}
		key := reservePrefix + n.ID.String() + ":" + need.FragmentID.String()
		if err := s.Redis.Set(ctx, key, need.SizeBytes, s.TTL).Err(); err != nil {
			return nil, fmt.Errorf("scheduler.place: %w", err)
		}
		out = append(out, Assignment{FragmentID: need.FragmentID, Node: n})
	}
	return out, nil
}

func pick(nodes []store.Node, used map[uuid.UUID]struct{}) (store.Node, error) {
	var cand []store.Node
	for _, n := range nodes {
		if _, ok := used[n.ID]; ok {
			continue
		}
		cand = append(cand, n)
	}
	if len(cand) == 0 {
		// M2: reuse a node across chunks only; same chunk must be unique.
		// If a chunk needs more nodes than exist, fail.
		return store.Node{}, ErrUnavailable
	}
	i, err := rand.Int(rand.Reader, big.NewInt(int64(len(cand))))
	if err != nil {
		return store.Node{}, err
	}
	return cand[int(i.Int64())], nil
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
