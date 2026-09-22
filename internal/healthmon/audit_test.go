package healthmon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rramka/distributed-storage-platform/internal/events"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

func TestChallengerAuditMarksLostAndRepairs(t *testing.T) {
	s := store.OpenForTest(t)
	ctx := context.Background()
	fleet, err := s.SeedCommittedFleet(ctx, nil, nil, 128)
	if err != nil {
		t.Fatal(err)
	}
	targetNode := fleet.Nodes[0]
	fragID := fleet.PlaceIDs[0]
	before, err := s.NodeByID(ctx, targetNode.ID)
	if err != nil {
		t.Fatal(err)
	}

	set := make([]store.Challenge, 3)
	for i := range set {
		set[i] = store.Challenge{
			FragmentID: fragID,
			NodeID:     targetNode.ID,
			Seq:        i,
			Offset:     0,
			Length:     16,
			Nonce:      []byte("0123456789abcdef"),
			Expected:   bytes32(9),
		}
	}
	if err := s.InsertChallengeSet(ctx, set); err != nil {
		t.Fatal(err)
	}

	meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tickets": []map[string]string{{
				"fragment_id": fragID.String(),
				"node_id":     targetNode.ID.String(),
				"op":          "challenge",
				"ticket":      "dummy",
			}},
		})
	}))
	t.Cleanup(meta.Close)

	var published []events.RepairJob
	c := &Challenger{
		Store: s,
		Meta:  NewChallengeMeta(meta.URL),
		DoChallenge: func(ctx context.Context, endpoint string, nodeID, fragID uuid.UUID, ticket string, offset, length int, nonce []byte) ([]byte, error) {
			return bytes32(1), nil
		},
		OnRepair: func(ctx context.Context, job events.RepairJob) error {
			published = append(published, job)
			return nil
		},
	}
	c.defaults()
	fr, n, err := s.LoadPlacementFragment(ctx, fragID, targetNode.ID)
	if err != nil {
		t.Fatal(err)
	}
	t0 := store.AuditTarget{
		FragmentID: fragID,
		NodeID:     targetNode.ID,
		ChunkID:    fleet.ChunkID,
		SizeBytes:  fr.SizeBytes,
		SHA256:     fr.SHA256,
		Endpoint:   n.Endpoint,
		Remaining:  3,
	}
	if err := c.audit(ctx, t0); err != nil {
		t.Fatal(err)
	}

	after, err := s.NodeByID(ctx, targetNode.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Reputation >= before.Reputation {
		t.Fatalf("reputation did not drop: %f -> %f", before.Reputation, after.Reputation)
	}
	if len(published) != 1 || published[0].ChunkID != fleet.ChunkID {
		t.Fatalf("repair job %+v chunk %s", published, fleet.ChunkID)
	}

	lp, err := s.ListLostOnNode(ctx, targetNode.ID)
	if err != nil {
		t.Fatal(err)
	}
	lost := false
	ids := make([]int64, 0, len(lp))
	for _, p := range lp {
		ids = append(ids, p.PlacementID)
		if p.FragmentID == fragID {
			lost = true
		}
	}
	if !lost {
		t.Fatal("placement not marked lost")
	}
	t.Cleanup(func() { _ = s.RestorePlacements(context.Background(), ids) })
}

func bytes32(seed byte) []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = seed
	}
	return b
}
