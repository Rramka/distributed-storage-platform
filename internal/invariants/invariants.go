// Package invariants asserts the platform durability and placement-cap invariants.
// docs/10-mvp-roadmap.md continuously measured invariants. Ledger is skipped until M5.
package invariants

import (
	"context"
	"fmt"

	"github.com/Rramka/distributed-storage-platform/internal/scheduler"
	"github.com/Rramka/distributed-storage-platform/internal/store"
)

const minHealthy = 12

// CheckDurability fails if any committed chunk has fewer than 12 healthy placements.
func CheckDurability(ctx context.Context, s *store.Store) error {
	ids, err := s.UnderHealthyChunks(ctx, minHealthy)
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		return fmt.Errorf("invariants.durability: %d chunks below %d healthy (first %s)", len(ids), minHealthy, ids[0])
	}
	return nil
}

// CheckCaps fails if stored placements of any committed chunk break hard caps.
func CheckCaps(ctx context.Context, s *store.Store) error {
	ids, err := s.ChunkCapViolations(ctx, scheduler.RegionCap, scheduler.ASNCap, scheduler.OwnerCap)
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		return fmt.Errorf("invariants.caps: %d chunks violate placement caps (first %s)", len(ids), ids[0])
	}
	return nil
}

// CheckAll runs durability and cap checks. Ledger is skipped on the solo track.
func CheckAll(ctx context.Context, s *store.Store) error {
	if err := CheckDurability(ctx, s); err != nil {
		return err
	}
	return CheckCaps(ctx, s)
}
