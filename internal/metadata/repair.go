package metadata

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"net/http"

	"github.com/Rramka/distributed-storage-platform/internal/receipts"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/Rramka/distributed-storage-platform/internal/tickets"
	"github.com/google/uuid"
)

// RepairSource is a surviving stored placement the worker can GET.
type RepairSource struct {
	ShardIndex int16  `json:"shard_index"`
	FragmentID string `json:"fragment_id"`
	SHA256     string `json:"sha256"`
	SizeBytes  int    `json:"size_bytes"`
	NodeID     string `json:"node_id"`
	Endpoint   string `json:"endpoint"`
	Ticket     string `json:"ticket"`
}

// RepairTarget is a missing shard assigned to a fresh node.
type RepairTarget struct {
	ShardIndex int16  `json:"shard_index"`
	FragmentID string `json:"fragment_id"`
	SHA256     string `json:"sha256"`
	SizeBytes  int    `json:"size_bytes"`
	NodeID     string `json:"node_id"`
	Endpoint   string `json:"endpoint"`
	Ticket     string `json:"ticket"`
}

// RepairPlan is POST /internal/repair/chunks/{id}/plan.
type RepairPlan struct {
	ChunkID   string         `json:"chunk_id"`
	SizeBytes int            `json:"size_bytes"`
	SHA256    string         `json:"sha256"`
	Healthy   int            `json:"healthy"`
	Sources   []RepairSource `json:"sources"`
	Targets   []RepairTarget `json:"targets"`
}

// LostRow is GET /internal/repair/nodes/{id}/lost.
type LostRow struct {
	PlacementID            int64  `json:"placement_id"`
	FragmentID             string `json:"fragment_id"`
	ChunkID                string `json:"chunk_id"`
	ShardIndex             int16  `json:"shard_index"`
	SHA256                 string `json:"sha256"`
	SizeBytes              int    `json:"size_bytes"`
	Endpoint               string `json:"endpoint"`
	Ticket                 string `json:"ticket"`
	ReconstructedElsewhere bool   `json:"reconstructed_elsewhere"`
}

// PlanRepair issues GET tickets for survivors and PUT tickets for missing shards.
func (s *StoreService) PlanRepair(ctx context.Context, chunkID uuid.UUID) (RepairPlan, error) {
	if s.Place == nil || s.SignTicket == nil {
		return RepairPlan{}, ErrInvalid
	}
	ch, frags, err := s.Store.LoadRepairChunk(ctx, chunkID)
	if err != nil {
		return RepairPlan{}, err
	}
	plan := RepairPlan{
		ChunkID:   ch.ID.String(),
		SizeBytes: ch.SizeBytes,
		SHA256:    hex.EncodeToString(ch.SHA256),
	}
	var needs []PlaceNeed
	type missing struct {
		fr store.Fragment
	}
	var toPlace []missing
	for _, rf := range frags {
		if rf.Placement != nil && rf.Placement.Status == "stored" && rf.Node != nil {
			wire, _, err := s.SignTicket(tickets.OpGet, rf.Fragment.ID, rf.Node.ID, rf.Fragment.SHA256, uint64(rf.Fragment.SizeBytes))
			if err != nil {
				return RepairPlan{}, err
			}
			plan.Sources = append(plan.Sources, RepairSource{
				ShardIndex: rf.Fragment.ShardIndex,
				FragmentID: rf.Fragment.ID.String(),
				SHA256:     hex.EncodeToString(rf.Fragment.SHA256),
				SizeBytes:  rf.Fragment.SizeBytes,
				NodeID:     rf.Node.ID.String(),
				Endpoint:   rf.Node.Endpoint,
				Ticket:     wire,
			})
			plan.Healthy++
			continue
		}
		if rf.Placement != nil && rf.Placement.Status == "pending" && rf.Node != nil {
			wire, _, err := s.SignTicket(tickets.OpPut, rf.Fragment.ID, rf.Node.ID, rf.Fragment.SHA256, uint64(rf.Fragment.SizeBytes))
			if err != nil {
				return RepairPlan{}, err
			}
			plan.Targets = append(plan.Targets, RepairTarget{
				ShardIndex: rf.Fragment.ShardIndex,
				FragmentID: rf.Fragment.ID.String(),
				SHA256:     hex.EncodeToString(rf.Fragment.SHA256),
				SizeBytes:  rf.Fragment.SizeBytes,
				NodeID:     rf.Node.ID.String(),
				Endpoint:   rf.Node.Endpoint,
				Ticket:     wire,
			})
			continue
		}
		toPlace = append(toPlace, missing{fr: rf.Fragment})
		needs = append(needs, PlaceNeed{
			FragmentID: rf.Fragment.ID,
			ChunkID:    ch.ID,
			SizeBytes:  int64(rf.Fragment.SizeBytes),
		})
	}
	if len(needs) > 0 {
		assigns, err := s.Place(ctx, needs)
		if err != nil {
			return RepairPlan{}, ErrUnavailable
		}
		byFrag := map[uuid.UUID]store.Node{}
		var rows []store.Placement
		for _, a := range assigns {
			byFrag[a.FragmentID] = a.Node
			rows = append(rows, store.Placement{FragmentID: a.FragmentID, NodeID: a.Node.ID})
		}
		if err := s.Store.InsertPlacements(ctx, rows); err != nil {
			return RepairPlan{}, err
		}
		for _, m := range toPlace {
			n, ok := byFrag[m.fr.ID]
			if !ok {
				continue
			}
			wire, _, err := s.SignTicket(tickets.OpPut, m.fr.ID, n.ID, m.fr.SHA256, uint64(m.fr.SizeBytes))
			if err != nil {
				return RepairPlan{}, err
			}
			plan.Targets = append(plan.Targets, RepairTarget{
				ShardIndex: m.fr.ShardIndex,
				FragmentID: m.fr.ID.String(),
				SHA256:     hex.EncodeToString(m.fr.SHA256),
				SizeBytes:  m.fr.SizeBytes,
				NodeID:     n.ID.String(),
				Endpoint:   n.Endpoint,
				Ticket:     wire,
			})
		}
	}
	return plan, nil
}

// CommitRepair verifies receipts and marks pending placements stored.
func (s *StoreService) CommitRepair(ctx context.Context, chunkID uuid.UUID, wires []string) error {
	targets, err := s.Store.PendingTargetsForChunk(ctx, chunkID)
	if err != nil {
		return err
	}
	var stored []uuid.UUID
	for _, w := range wires {
		matched := false
		for _, t := range targets {
			_, err := receipts.Verify(w, ed25519.PublicKey(t.Node.PublicKey), t.Node.ID, t.Fragment.ID, t.Fragment.SHA256, uint64(t.Fragment.SizeBytes))
			if err == nil {
				stored = append(stored, t.Fragment.ID)
				matched = true
				break
			}
		}
		if !matched {
			return ErrInvalid
		}
	}
	return s.Store.CommitRepairPlacements(ctx, stored)
}

// ListLostForNode returns lost placements plus GET tickets for flap re-validation.
func (s *StoreService) ListLostForNode(ctx context.Context, nodeID uuid.UUID) ([]LostRow, error) {
	if s.SignTicket == nil {
		return nil, ErrInvalid
	}
	lost, err := s.Store.ListLostOnNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	out := make([]LostRow, 0, len(lost))
	for _, lp := range lost {
		wire, _, err := s.SignTicket(tickets.OpGet, lp.FragmentID, lp.NodeID, lp.SHA256, uint64(lp.SizeBytes))
		if err != nil {
			return nil, err
		}
		out = append(out, LostRow{
			PlacementID:            lp.PlacementID,
			FragmentID:             lp.FragmentID.String(),
			ChunkID:                lp.ChunkID.String(),
			ShardIndex:             lp.ShardIndex,
			SHA256:                 hex.EncodeToString(lp.SHA256),
			SizeBytes:              lp.SizeBytes,
			Endpoint:               lp.Endpoint,
			Ticket:                 wire,
			ReconstructedElsewhere: lp.ReconstructedElsewhere,
		})
	}
	return out, nil
}

// ApplyFlap restores verified copies or expires surplus reconstructed elsewhere.
func (s *StoreService) ApplyFlap(ctx context.Context, restore, expire []int64) error {
	if err := s.Store.RestorePlacements(ctx, restore); err != nil {
		return err
	}
	return s.Store.ExpirePlacements(ctx, expire)
}

// MountRepair registers internal repair routes. Tickets stay in metadata.
func MountRepair(mux *http.ServeMux, s *StoreService) {
	mux.HandleFunc("POST /internal/repair/chunks/{chunkID}/plan", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("chunkID"))
		if err != nil {
			writeErr(w, r, ErrInvalid)
			return
		}
		plan, err := s.PlanRepair(r.Context(), id)
		if writeErr(w, r, err) {
			return
		}
		writeJSON(w, http.StatusOK, plan)
	})
	mux.HandleFunc("POST /internal/repair/chunks/{chunkID}/commit", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("chunkID"))
		if err != nil {
			writeErr(w, r, ErrInvalid)
			return
		}
		var req struct {
			Receipts []string `json:"receipts"`
		}
		if !decode(w, r, &req) {
			return
		}
		if writeErr(w, r, s.CommitRepair(r.Context(), id, req.Receipts)) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("GET /internal/repair/nodes/{nodeID}/lost", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("nodeID"))
		if err != nil {
			writeErr(w, r, ErrInvalid)
			return
		}
		rows, err := s.ListLostForNode(r.Context(), id)
		if writeErr(w, r, err) {
			return
		}
		if rows == nil {
			rows = []LostRow{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"placements": rows})
	})
	mux.HandleFunc("POST /internal/repair/nodes/{nodeID}/flap", func(w http.ResponseWriter, r *http.Request) {
		if _, err := uuid.Parse(r.PathValue("nodeID")); err != nil {
			writeErr(w, r, ErrInvalid)
			return
		}
		var req struct {
			Restore []int64 `json:"restore"`
			Expire  []int64 `json:"expire"`
		}
		if !decode(w, r, &req) {
			return
		}
		if writeErr(w, r, s.ApplyFlap(r.Context(), req.Restore, req.Expire)) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
}
