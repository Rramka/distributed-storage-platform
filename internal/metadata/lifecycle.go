package metadata

import (
	"context"
	"net/http"
	"strconv"

	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/Rramka/distributed-storage-platform/internal/tickets"
)

// DeleteTicket is a DELETE capability for one expiring placement.
type DeleteTicket struct {
	PlacementID int64  `json:"placement_id"`
	FragmentID  string `json:"fragment_id"`
	NodeID      string `json:"node_id"`
	Endpoint    string `json:"endpoint"`
	Ticket      string `json:"ticket"`
}

// IssueDeleteTicket mints a DELETE ticket for an expiring placement.
func (s *StoreService) IssueDeleteTicket(ctx context.Context, placementID int64) (DeleteTicket, error) {
	if s.SignTicket == nil {
		return DeleteTicket{}, ErrInvalid
	}
	p, err := s.Store.ExpiringPlacementByID(ctx, placementID)
	if err != nil {
		return DeleteTicket{}, err
	}
	wire, _, err := s.SignTicket(tickets.OpDelete, p.FragmentID, p.NodeID, p.SHA256, uint64(p.SizeBytes))
	if err != nil {
		return DeleteTicket{}, err
	}
	return DeleteTicket{
		PlacementID: p.ID,
		FragmentID:  p.FragmentID.String(),
		NodeID:      p.NodeID.String(),
		Endpoint:    p.Endpoint,
		Ticket:      wire,
	}, nil
}

// MarkDeleted records agent-confirmed removal.
func (s *StoreService) MarkDeleted(ctx context.Context, placementID int64) error {
	return s.Store.MarkPlacementDeleted(ctx, placementID)
}

// ListExpiring is used by the reaper.
func (s *StoreService) ListExpiring(ctx context.Context, limit int) ([]store.ExpiringPlacement, error) {
	return s.Store.ListExpiringPlacements(ctx, limit)
}

// MountLifecycle registers internal deletion-reaper routes.
func MountLifecycle(mux *http.ServeMux, s *StoreService) {
	mux.HandleFunc("GET /internal/lifecycle/expiring", func(w http.ResponseWriter, r *http.Request) {
		rows, err := s.ListExpiring(r.Context(), 64)
		if writeErr(w, r, err) {
			return
		}
		if rows == nil {
			rows = []store.ExpiringPlacement{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"placements": rows})
	})
	mux.HandleFunc("POST /internal/lifecycle/placements/{id}/delete-ticket", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, r, ErrInvalid)
			return
		}
		tk, err := s.IssueDeleteTicket(r.Context(), id)
		if writeErr(w, r, err) {
			return
		}
		writeJSON(w, http.StatusOK, tk)
	})
	mux.HandleFunc("POST /internal/lifecycle/placements/{id}/deleted", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, r, ErrInvalid)
			return
		}
		if writeErr(w, r, s.MarkDeleted(r.Context(), id)) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
}
