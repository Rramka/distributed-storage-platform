package healthmon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/metadata"
	"github.com/Rramka/distributed-storage-platform/internal/tickets"
	"github.com/google/uuid"
)

// ChallengeMeta mints get/challenge tickets from metadata. It never holds the signing seed.
type ChallengeMeta struct {
	base   string
	client *http.Client
}

// NewChallengeMeta talks to metadata at baseURL.
func NewChallengeMeta(baseURL string) *ChallengeMeta {
	return &ChallengeMeta{base: baseURL, client: &http.Client{Timeout: 15 * time.Second}}
}

// Tickets mints the requested ops for one placement.
func (m *ChallengeMeta) Tickets(ctx context.Context, fragmentID, nodeID uuid.UUID, ops ...string) ([]metadata.ChallengeTicket, error) {
	if m == nil || m.base == "" {
		return nil, fmt.Errorf("healthmon.tickets: metadata URL unset")
	}
	reqs := make([]metadata.ChallengeTicketReq, 0, len(ops))
	for _, op := range ops {
		if op == "" {
			op = tickets.OpChallenge
		}
		reqs = append(reqs, metadata.ChallengeTicketReq{
			FragmentID: fragmentID.String(),
			NodeID:     nodeID.String(),
			Op:         op,
		})
	}
	body := map[string]any{"requests": reqs}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.base+"/internal/challenges/tickets", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("healthmon.tickets: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("healthmon.tickets: http %d %s", resp.StatusCode, b)
	}
	var out struct {
		Tickets []metadata.ChallengeTicket `json:"tickets"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out.Tickets, nil
}
