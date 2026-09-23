package metadata

import (
	"testing"

	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

func TestPickDownloadPlacementsPrefersOnline(t *testing.T) {
	t.Parallel()
	online := store.DownloadPlacement{Node: store.Node{ID: uuid.New(), Status: "online", Endpoint: "on"}}
	offline := store.DownloadPlacement{Node: store.Node{ID: uuid.New(), Status: "offline", Endpoint: "off"}}
	got := pickDownloadPlacements([]store.DownloadPlacement{offline, online, offline}, 1)
	if len(got) != 1 || got[0].Node.Status != "online" {
		t.Fatalf("got %+v", got)
	}
}

func TestPickDownloadPlacementsFallsBackToOffline(t *testing.T) {
	t.Parallel()
	var all []store.DownloadPlacement
	for i := 0; i < 3; i++ {
		all = append(all, store.DownloadPlacement{Node: store.Node{ID: uuid.New(), Status: "online"}})
	}
	for i := 0; i < 8; i++ {
		all = append(all, store.DownloadPlacement{Node: store.Node{ID: uuid.New(), Status: "offline"}})
	}
	got := pickDownloadPlacements(all, 10)
	if len(got) != 11 {
		t.Fatalf("got %d want all 11 when online < need", len(got))
	}
	online := 0
	for _, p := range got {
		if p.Node.Status == "online" {
			online++
		}
	}
	if online != 3 {
		t.Fatalf("online %d", online)
	}
}
