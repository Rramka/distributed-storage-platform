package metadata

import (
	"encoding/hex"
	"net/http"

	"github.com/Rramka/distributed-storage-platform/internal/tickets"
	"github.com/google/uuid"
)

// ChallengeTicketReq is one (fragment, node, op) mint request.
type ChallengeTicketReq struct {
	FragmentID string `json:"fragment_id"`
	NodeID     string `json:"node_id"`
	Op         string `json:"op"`
}

// ChallengeTicket is a minted wire ticket.
type ChallengeTicket struct {
	FragmentID string `json:"fragment_id"`
	NodeID     string `json:"node_id"`
	Op         string `json:"op"`
	Ticket     string `json:"ticket"`
	SHA256     string `json:"sha256"`
	SizeBytes  int    `json:"size_bytes"`
	Endpoint   string `json:"endpoint"`
}

func (s *StoreService) issueChallengeTickets(r *http.Request, reqs []ChallengeTicketReq) ([]ChallengeTicket, error) {
	if s.SignTicket == nil {
		return nil, ErrInvalid
	}
	out := make([]ChallengeTicket, 0, len(reqs))
	for _, req := range reqs {
		fid, err := uuid.Parse(req.FragmentID)
		if err != nil {
			return nil, ErrInvalid
		}
		nid, err := uuid.Parse(req.NodeID)
		if err != nil {
			return nil, ErrInvalid
		}
		op := req.Op
		if op != tickets.OpGet && op != tickets.OpChallenge {
			return nil, ErrInvalid
		}
		fr, n, err := s.Store.LoadPlacementFragment(r.Context(), fid, nid)
		if err != nil {
			return nil, err
		}
		max := uint64(fr.SizeBytes)
		if op == tickets.OpChallenge {
			max = 64 << 10
		}
		wire, _, err := s.SignTicket(op, fr.ID, n.ID, fr.SHA256, max)
		if err != nil {
			return nil, err
		}
		out = append(out, ChallengeTicket{
			FragmentID: fr.ID.String(),
			NodeID:     n.ID.String(),
			Op:         op,
			Ticket:     wire,
			SHA256:     hex.EncodeToString(fr.SHA256),
			SizeBytes:  fr.SizeBytes,
			Endpoint:   n.Endpoint,
		})
	}
	return out, nil
}

// MountChallenges registers POST /internal/challenges/tickets.
func MountChallenges(mux *http.ServeMux, s *StoreService) {
	mux.HandleFunc("POST /internal/challenges/tickets", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Requests []ChallengeTicketReq `json:"requests"`
		}
		if !decode(w, r, &req) {
			return
		}
		ticketsOut, err := s.issueChallengeTickets(r, req.Requests)
		if writeErr(w, r, err) {
			return
		}
		if ticketsOut == nil {
			ticketsOut = []ChallengeTicket{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"tickets": ticketsOut})
	})
}
