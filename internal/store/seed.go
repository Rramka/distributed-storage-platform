package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SeededFleet is a committed 10+6 file across distinct nodes (test fixture).
type SeededFleet struct {
	User     User
	Owners   []User
	Bucket   Bucket
	File     File
	Version  FileVersion
	Nodes    []Node
	Planned  PlannedUpload
	ChunkID  uuid.UUID
	PlaceIDs []uuid.UUID
}

var seedRegions = []string{
	"us-east", "us-east", "us-east",
	"us-west", "us-west", "us-west",
	"eu-west", "eu-west", "eu-west",
	"eu-central", "eu-central", "eu-central",
	"ap-south", "ap-south", "ap-south",
	"ap-northeast", "ap-northeast", "ap-northeast",
	"sa-east", "sa-east", "sa-east",
	"af-south", "af-south", "af-south",
}

// SeedCommittedFleet inserts 24 online nodes (12 owners × 2) and one committed
// file with 16 stored placements on distinct nodes. docs/10-mvp-roadmap.md W11.
func (s *Store) SeedCommittedFleet(ctx context.Context, encMeta json.RawMessage, contentSHA []byte, sizeBytes int64) (SeededFleet, error) {
	if len(contentSHA) != 32 {
		contentSHA = bytes32seed(1)
	}
	if sizeBytes <= 0 {
		sizeBytes = 100
	}
	if len(encMeta) == 0 {
		encMeta = json.RawMessage(`{"algo":"aes-256-gcm"}`)
	}

	var out SeededFleet
	u, err := s.CreateUser(ctx, "fleet-"+uuid.NewString()+"@example.com", "hash")
	if err != nil {
		return SeededFleet{}, err
	}
	out.User = u
	out.Owners = []User{u}
	for i := 0; i < 11; i++ {
		o, err := s.CreateUser(ctx, fmt.Sprintf("owner-%d-%s@example.com", i, uuid.NewString()), "hash")
		if err != nil {
			return SeededFleet{}, err
		}
		out.Owners = append(out.Owners, o)
	}

	b, err := s.CreateBucket(ctx, u.ID, "b-"+uuid.NewString()[:8])
	if err != nil {
		return SeededFleet{}, err
	}
	out.Bucket = b

	out.Nodes = make([]Node, 0, 24)
	for i := 0; i < 24; i++ {
		owner := out.Owners[i/2]
		asn := 64501 + (i % 8)
		n, err := s.CreateNode(ctx, CreateNodeParams{
			OwnerID:         owner.ID,
			CertFingerprint: []byte(uuid.NewString()),
			PublicKey:       bytes32seed(byte(i + 1)),
			CertPEM:         "pem",
			CertExpiresAt:   time.Now().Add(24 * time.Hour),
			OS:              "linux",
			AgentVersion:    "test",
			Country:         "US",
			Region:          seedRegions[i],
			ASN:             &asn,
			Endpoint:        fmt.Sprintf("127.0.0.1:%d", 19000+i),
			CapacityBytes:   1 << 30,
		})
		if err != nil {
			return SeededFleet{}, err
		}
		if err := s.TouchNode(ctx, n.ID, 0, 1<<30); err != nil {
			return SeededFleet{}, err
		}
		n, err = s.NodeByID(ctx, n.ID)
		if err != nil {
			return SeededFleet{}, err
		}
		out.Nodes = append(out.Nodes, n)
	}

	frags := make([]ManifestFragment, 16)
	for i := range frags {
		frags[i] = ManifestFragment{ShardIndex: int16(i), SizeBytes: 32, SHA256: bytes32seed(byte(i + 3))}
	}
	planned, err := s.BeginUpload(ctx, u.ID, UploadManifest{
		BucketID:       b.ID,
		Path:           "/f-" + uuid.NewString(),
		SizeBytes:      sizeBytes,
		ContentSHA256:  contentSHA,
		EncryptionMeta: encMeta,
		ChunkSize:      16 * 1024 * 1024,
		ECData:         10,
		ECParity:       6,
		Chunks:         []ManifestChunk{{Seq: 0, SizeBytes: int(sizeBytes), SHA256: contentSHA, Fragments: frags}},
	})
	if err != nil {
		return SeededFleet{}, err
	}
	out.Planned = planned
	out.File = planned.File
	out.Version = planned.Version

	var rows []Placement
	ids := make([]uuid.UUID, 0, 16)
	for i, pf := range planned.Pending {
		if i >= 16 {
			break
		}
		rows = append(rows, Placement{FragmentID: pf.Fragment.ID, NodeID: out.Nodes[i].ID})
		ids = append(ids, pf.Fragment.ID)
		out.ChunkID = pf.Chunk.ID
	}
	if err := s.InsertPlacements(ctx, rows); err != nil {
		return SeededFleet{}, err
	}
	f, v, err := s.CommitUpload(ctx, u.ID, planned.Version.ID, ids)
	if err != nil {
		return SeededFleet{}, err
	}
	out.File = f
	out.Version = v
	out.PlaceIDs = ids
	return out, nil
}

func bytes32seed(seed byte) []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = seed
	}
	return b
}
