package scheduler

import (
	"testing"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

func TestPickExactlySatisfiableFleet(t *testing.T) {
	t.Parallel()
	nodes := satisfiableFleet()
	used := newChunkUse()
	seen := map[uuid.UUID]struct{}{}
	for i := 0; i < 16; i++ {
		n, err := pick(nodes, used)
		if err != nil {
			t.Fatalf("pick %d: %v", i, err)
		}
		if _, ok := seen[n.ID]; ok {
			t.Fatalf("duplicate node %s", n.ID)
		}
		seen[n.ID] = struct{}{}
		used.add(n)
	}
	assertCaps(t, used)
}

func TestPickRegionStarved(t *testing.T) {
	t.Parallel()
	nodes := make([]store.Node, 16)
	for i := range nodes {
		nodes[i] = fakeNode(i, i/2, "us-east", 64501+i%6)
	}
	used := newChunkUse()
	var got int
	for i := 0; i < 16; i++ {
		n, err := pick(nodes, used)
		if err != nil {
			break
		}
		used.add(n)
		got++
	}
	if got != RegionCap {
		t.Fatalf("placed %d, want region cap %d", got, RegionCap)
	}
}

func TestPickSingleOwner(t *testing.T) {
	t.Parallel()
	owner := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	nodes := make([]store.Node, 16)
	regions := []string{"us-east", "us-west", "eu-west", "eu-central", "ap-south", "ap-northeast"}
	for i := range nodes {
		n := fakeNode(i, 0, regions[i%len(regions)], 64501+i%6)
		n.OwnerID = owner
		nodes[i] = n
	}
	used := newChunkUse()
	var got int
	for i := 0; i < 16; i++ {
		n, err := pick(nodes, used)
		if err != nil {
			break
		}
		used.add(n)
		got++
	}
	if got != OwnerCap {
		t.Fatalf("placed %d, want owner cap %d", got, OwnerCap)
	}
}

func TestPickOneNodeDown(t *testing.T) {
	t.Parallel()
	nodes := satisfiableFleet()[:15]
	used := newChunkUse()
	var got int
	for i := 0; i < 16; i++ {
		n, err := pick(nodes, used)
		if err != nil {
			break
		}
		used.add(n)
		got++
	}
	if got != 15 {
		t.Fatalf("placed %d, want 15", got)
	}
	if got < store.CommitThreshold {
		t.Fatalf("15 live nodes should still reach commit threshold %d", store.CommitThreshold)
	}
}

func TestEligibleProbationQuota(t *testing.T) {
	t.Parallel()
	until := time.Now().Add(24 * time.Hour)
	n := fakeNode(0, 0, "us-east", 64501)
	n.ProbationUntil = &until
	n.LivePlacements = ProbationQuota
	if eligible(n, newChunkUse()) {
		t.Fatal("probation quota should exclude")
	}
	n.LivePlacements = ProbationQuota - 1
	if !eligible(n, newChunkUse()) {
		t.Fatal("under quota should be eligible")
	}
}

func TestWeightedSamplingPrefersHighReputation(t *testing.T) {
	t.Parallel()
	high := fakeNode(0, 0, "us-east", 64501)
	low := fakeNode(1, 1, "us-west", 64502)
	high.Reputation = 0.9
	low.Reputation = 0.2
	var nh, nl int
	const rounds = 4000
	for i := 0; i < rounds; i++ {
		n, err := pick([]store.Node{high, low}, newChunkUse())
		if err != nil {
			t.Fatal(err)
		}
		if n.ID == high.ID {
			nh++
		} else {
			nl++
		}
	}
	if nh <= nl {
		t.Fatalf("high reputation sampled %d vs low %d", nh, nl)
	}
}

func TestPickNeverBreaksCaps(t *testing.T) {
	t.Parallel()
	nodes := satisfiableFleet()
	for i := range nodes {
		nodes[i].Reputation = float32(0.2 + 0.05*float64(i%10))
	}
	used := newChunkUse()
	seen := map[uuid.UUID]struct{}{}
	for i := 0; i < 16; i++ {
		n, err := pick(nodes, used)
		if err != nil {
			t.Fatalf("pick %d: %v", i, err)
		}
		if _, ok := seen[n.ID]; ok {
			t.Fatalf("duplicate node %s", n.ID)
		}
		seen[n.ID] = struct{}{}
		used.add(n)
	}
	assertCaps(t, used)
}

func TestEligibleSkipsFullDisk(t *testing.T) {
	t.Parallel()
	n := fakeNode(0, 0, "us-east", 64501)
	n.UsedBytes = n.CapacityBytes
	if eligible(n, newChunkUse()) {
		t.Fatal("full disk should be ineligible")
	}
}

func satisfiableFleet() []store.Node {
	// 8 owners × 2 nodes; 6 regions at 3/3/3/3/2/2; 6 ASNs grouped independently.
	regions := []string{
		"us-east", "us-west", "eu-west", "eu-central", "ap-south", "ap-northeast",
		"us-east", "us-west", "eu-west", "eu-central", "ap-south", "ap-northeast",
		"us-east", "us-west", "eu-west", "eu-central",
	}
	asns := []int{
		64501, 64502, 64503, 64504, 64505, 64506,
		64504, 64505, 64506, 64501, 64502, 64503,
		64502, 64503, 64501, 64506,
	}
	out := make([]store.Node, 16)
	for i := 0; i < 16; i++ {
		out[i] = fakeNode(i, i/2, regions[i], asns[i])
	}
	return out
}

func fakeNode(i, owner int, region string, asn int) store.Node {
	a := asn
	return store.Node{
		ID:            uuid.MustParse("00000000-0000-0000-0000-0000000000" + twoDigit(i+1)),
		OwnerID:       uuid.MustParse("00000000-0000-0000-0000-0000000001" + twoDigit(owner+1)),
		Region:        region,
		ASN:           &a,
		CapacityBytes: 10 << 30,
		UsedBytes:     0,
		Status:        "online",
	}
}

func twoDigit(n int) string {
	return string([]byte{'0' + byte(n/10), '0' + byte(n%10)})
}

func assertCaps(t *testing.T, used *chunkUse) {
	t.Helper()
	for r, n := range used.regions {
		if n > RegionCap {
			t.Fatalf("region %s count %d", r, n)
		}
	}
	for a, n := range used.asns {
		if n > ASNCap {
			t.Fatalf("asn %d count %d", a, n)
		}
	}
	for o, n := range used.owners {
		if n > OwnerCap {
			t.Fatalf("owner %s count %d", o, n)
		}
	}
}
