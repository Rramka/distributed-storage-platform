package metadata

import (
	"net/http"

	"github.com/Rramka/distributed-storage-platform/internal/store"
)

// FleetSnapshot is the read-only demo view of the compose fleet.
type FleetSnapshot struct {
	Nodes  []store.Node      `json:"nodes"`
	Chunks []ChunkHealthView `json:"chunks"`
	Repair int               `json:"repair_in_progress"`
}

// ChunkHealthView is one committed chunk's healthy placement count.
type ChunkHealthView struct {
	ChunkID string `json:"chunk_id"`
	Healthy int    `json:"healthy"`
	Target  int    `json:"target"`
}

// MountDemo registers GET /internal/demo/fleet.
func MountDemo(mux *http.ServeMux, s *StoreService) {
	mux.HandleFunc("GET /internal/demo/fleet", func(w http.ResponseWriter, r *http.Request) {
		nodes, err := s.Store.ListAllNodes(r.Context())
		if writeErr(w, r, err) {
			return
		}
		if nodes == nil {
			nodes = []store.Node{}
		}
		health, err := s.Store.CommittedChunkHealth(r.Context())
		if writeErr(w, r, err) {
			return
		}
		chunks := make([]ChunkHealthView, 0, len(health))
		repairN := 0
		for id, n := range health {
			chunks = append(chunks, ChunkHealthView{ChunkID: id.String(), Healthy: n, Target: 16})
			if n < 16 {
				repairN++
			}
		}
		writeJSON(w, http.StatusOK, FleetSnapshot{Nodes: nodes, Chunks: chunks, Repair: repairN})
	})
}
