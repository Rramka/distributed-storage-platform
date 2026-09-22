package invariants

import (
	"context"
	"testing"

	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

func TestCheckAllEmptyFleet(t *testing.T) {
	s := store.OpenForTest(t)
	ctx := context.Background()
	if _, err := s.AnyCommittedFileID(ctx); err == nil {
		t.Skip("shared database already has committed files")
	}
	if err := CheckAll(ctx, s); err != nil {
		t.Fatal(err)
	}
}

func TestCheckDurabilityAndCaps(t *testing.T) {
	s := store.OpenForTest(t)
	ctx := context.Background()
	fleet, err := s.SeedCommittedFleet(ctx, nil, nil, 200)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("16/16 passes", func(t *testing.T) {
		if contains(under(t, s, ctx), fleet.ChunkID) {
			t.Fatal("seeded chunk below durability floor")
		}
		if contains(caps(t, s, ctx), fleet.ChunkID) {
			t.Fatal("seeded chunk violates caps")
		}
	})

	t.Run("11 healthy fails durability", func(t *testing.T) {
		var lostIDs []int64
		for i := 0; i < 5; i++ {
			lp, err := s.MarkPlacementLost(ctx, fleet.PlaceIDs[i], fleet.Nodes[i].ID)
			if err != nil {
				t.Fatal(err)
			}
			lostIDs = append(lostIDs, lp.PlacementID)
		}
		t.Cleanup(func() { _ = s.RestorePlacements(context.Background(), lostIDs) })
		if !contains(under(t, s, ctx), fleet.ChunkID) {
			t.Fatal("expected durability failure on seeded chunk")
		}
	})

	t.Run("duplicate node fails caps", func(t *testing.T) {
		fid := fleet.PlaceIDs[1]
		lp, err := s.MarkPlacementLost(ctx, fid, fleet.Nodes[1].ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.InsertStoredPlacement(ctx, fid, fleet.Nodes[0].ID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = s.Exec(context.Background(), `
				DELETE FROM fragment_placements
				WHERE fragment_id = $1 AND node_id = $2
			`, fid, fleet.Nodes[0].ID)
			_ = s.RestorePlacements(context.Background(), []int64{lp.PlacementID})
		})
		if !contains(caps(t, s, ctx), fleet.ChunkID) {
			t.Fatal("expected cap failure on duplicate node")
		}
	})

	t.Run("owner cap fails", func(t *testing.T) {
		owner := fleet.Nodes[0].OwnerID
		orig := make(map[uuid.UUID]uuid.UUID)
		for i := 0; i < 3; i++ {
			orig[fleet.Nodes[i].ID] = fleet.Nodes[i].OwnerID
			if err := s.Exec(ctx, `UPDATE nodes SET owner_id = $2 WHERE id = $1`, fleet.Nodes[i].ID, owner); err != nil {
				t.Fatal(err)
			}
		}
		t.Cleanup(func() {
			for id, o := range orig {
				_ = s.Exec(context.Background(), `UPDATE nodes SET owner_id = $2 WHERE id = $1`, id, o)
			}
		})
		if !contains(caps(t, s, ctx), fleet.ChunkID) {
			t.Fatal("expected owner-cap failure")
		}
	})
}

func under(t *testing.T, s *store.Store, ctx context.Context) []uuid.UUID {
	t.Helper()
	ids, err := s.UnderHealthyChunks(ctx, 12)
	if err != nil {
		t.Fatal(err)
	}
	return ids
}

func caps(t *testing.T, s *store.Store, ctx context.Context) []uuid.UUID {
	t.Helper()
	ids, err := s.ChunkCapViolations(ctx, 3, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	return ids
}

func contains(ids []uuid.UUID, want uuid.UUID) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}
