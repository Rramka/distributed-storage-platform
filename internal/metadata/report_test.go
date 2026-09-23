package metadata

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/receipts"
	"github.com/Rramka/distributed-storage-platform/internal/store"
)

func TestReportDownloadMetersEgress(t *testing.T) {
	s := store.OpenForTest(t)
	ctx := context.Background()
	fleet, err := s.SeedCommittedFleet(ctx, nil, nil, 200)
	if err != nil {
		t.Fatal(err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	node := fleet.Nodes[0]
	pf := fleet.Planned.Pending[0]
	if err := s.Exec(ctx, `UPDATE nodes SET public_key = $2 WHERE id = $1`, node.ID, []byte(pub)); err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(time.Hour)
	if err := s.InsertDownloadTicket(ctx, nonce, fleet.File.ID, pf.Fragment.ID, node.ID, int64(pf.Fragment.SizeBytes), until); err != nil {
		t.Fatal(err)
	}
	rec, err := receipts.SignEgress(priv, node.ID, pf.Fragment.ID, pf.Fragment.SHA256, uint64(pf.Fragment.SizeBytes), time.Now(), nonce)
	if err != nil {
		t.Fatal(err)
	}
	svc := &StoreService{Store: s}
	if err := svc.ReportDownload(ctx, fleet.User.ID, fleet.File.ID, []string{rec.Raw}); err != nil {
		t.Fatal(err)
	}
	window := time.Now().UTC().Truncate(time.Hour)
	rows, err := s.ListNodeStats(ctx, node.ID, window, window.Add(time.Hour))
	if err != nil || len(rows) != 1 {
		t.Fatalf("stats %v %v", rows, err)
	}
	if rows[0].BytesServed != int64(pf.Fragment.SizeBytes) {
		t.Fatalf("bytes_served %d want %d", rows[0].BytesServed, pf.Fragment.SizeBytes)
	}
	if err := svc.ReportDownload(ctx, fleet.User.ID, fleet.File.ID, []string{rec.Raw}); err == nil {
		t.Fatal("duplicate receipt should fail")
	}
}

func TestReportDownloadRejectsUnmatched(t *testing.T) {
	s := store.OpenForTest(t)
	ctx := context.Background()
	fleet, err := s.SeedCommittedFleet(ctx, nil, nil, 200)
	if err != nil {
		t.Fatal(err)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	pf := fleet.Planned.Pending[0]
	rec, err := receipts.SignEgress(priv, fleet.Nodes[0].ID, pf.Fragment.ID, pf.Fragment.SHA256, uint64(pf.Fragment.SizeBytes), time.Now(), nonce)
	if err != nil {
		t.Fatal(err)
	}
	svc := &StoreService{Store: s}
	if err := svc.ReportDownload(ctx, fleet.User.ID, fleet.File.ID, []string{rec.Raw}); err == nil {
		t.Fatal("unmatched receipt should fail")
	}
}
