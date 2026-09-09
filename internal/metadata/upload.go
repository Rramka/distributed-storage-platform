package metadata

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/receipts"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/Rramka/distributed-storage-platform/internal/tickets"
	"github.com/google/uuid"
)

// PlaceNeed is a fragment the scheduler must assign.
type PlaceNeed struct {
	FragmentID uuid.UUID
	ChunkID    uuid.UUID
	SizeBytes  int64
}

// PlaceAssign is a scheduler assignment.
type PlaceAssign struct {
	FragmentID uuid.UUID
	Node       store.Node
}

// PlannedPlacement is returned to the client.
type PlannedPlacement struct {
	ChunkSeq   int    `json:"chunk_seq"`
	ShardIndex int16  `json:"shard_index"`
	FragmentID string `json:"fragment_id"`
	NodeID     string `json:"node_id"`
	Endpoint   string `json:"endpoint"`
	Ticket     string `json:"ticket"`
}

// PlanResult is the upload plan response.
type PlanResult struct {
	UploadID   uuid.UUID          `json:"upload_id"`
	FileID     uuid.UUID          `json:"file_id"`
	ExpiresAt  time.Time          `json:"expires_at"`
	Placements []PlannedPlacement `json:"placements"`
}

// DownloadFragment is one retrieval target.
type DownloadFragment struct {
	ShardIndex int16  `json:"shard_index"`
	FragmentID string `json:"fragment_id"`
	SHA256     string `json:"sha256"`
	SizeBytes  int    `json:"size_bytes"`
	NodeID     string `json:"node_id"`
	Endpoint   string `json:"endpoint"`
	Ticket     string `json:"ticket"`
}

// DownloadChunk is a chunk plus its fragments.
type DownloadChunk struct {
	Seq       int                `json:"seq"`
	SHA256    string             `json:"sha256"`
	SizeBytes int                `json:"size_bytes"`
	Fragments []DownloadFragment `json:"fragments"`
}

// DownloadResult is GET /download.
type DownloadResult struct {
	FileID         uuid.UUID       `json:"file_id"`
	VersionNo      int             `json:"version_no"`
	SizeBytes      int64           `json:"size_bytes"`
	ContentSHA256  string          `json:"content_sha256"`
	EncryptionMeta json.RawMessage `json:"encryption_meta"`
	Chunks         []DownloadChunk `json:"chunks"`
}

// PlanUpload creates/resumes a pending version and issues tickets.
func (s *StoreService) PlanUpload(ctx context.Context, userID uuid.UUID, m store.UploadManifest) (PlanResult, error) {
	if s.Place == nil || s.SignTicket == nil {
		return PlanResult{}, ErrInvalid
	}
	planned, err := s.Store.BeginUpload(ctx, userID, m)
	if err != nil {
		return PlanResult{}, err
	}
	var needs []PlaceNeed
	for _, pf := range planned.Pending {
		if pf.Placement != nil {
			continue
		}
		needs = append(needs, PlaceNeed{
			FragmentID: pf.Fragment.ID,
			ChunkID:    pf.Chunk.ID,
			SizeBytes:  int64(pf.Fragment.SizeBytes),
		})
	}
	var assigns []PlaceAssign
	if len(needs) > 0 {
		assigns, err = s.Place(ctx, needs)
		if err != nil {
			return PlanResult{}, ErrUnavailable
		}
		var rows []store.Placement
		for _, a := range assigns {
			rows = append(rows, store.Placement{FragmentID: a.FragmentID, NodeID: a.Node.ID})
		}
		if err := s.Store.InsertPlacements(ctx, rows); err != nil {
			return PlanResult{}, err
		}
	}
	byFrag := map[uuid.UUID]store.Node{}
	for _, a := range assigns {
		byFrag[a.FragmentID] = a.Node
	}
	for _, pf := range planned.Pending {
		if pf.Placement != nil {
			n, err := s.Store.NodeByID(ctx, pf.Placement.NodeID)
			if err != nil {
				return PlanResult{}, err
			}
			byFrag[pf.Fragment.ID] = n
		}
	}

	var out []PlannedPlacement
	var exp time.Time
	for _, pf := range planned.Pending {
		n := byFrag[pf.Fragment.ID]
		wire, until, err := s.SignTicket(tickets.OpPut, pf.Fragment.ID, n.ID, pf.Fragment.SHA256, uint64(pf.Fragment.SizeBytes))
		if err != nil {
			return PlanResult{}, err
		}
		if exp.IsZero() || until.Before(exp) {
			exp = until
		}
		out = append(out, PlannedPlacement{
			ChunkSeq:   pf.ChunkSeq,
			ShardIndex: pf.ShardIndex,
			FragmentID: pf.Fragment.ID.String(),
			NodeID:     n.ID.String(),
			Endpoint:   n.Endpoint,
			Ticket:     wire,
		})
	}
	return PlanResult{UploadID: planned.Version.ID, FileID: planned.File.ID, ExpiresAt: exp, Placements: out}, nil
}

// CommitUpload verifies receipts and commits the version.
func (s *StoreService) CommitUpload(ctx context.Context, userID, uploadID uuid.UUID, wires []string) (store.File, store.FileVersion, error) {
	targets, err := s.Store.PendingTargetsForVersion(ctx, uploadID)
	if err != nil {
		return store.File{}, store.FileVersion{}, err
	}
	byFrag := map[uuid.UUID]store.PlacementTarget{}
	for _, t := range targets {
		byFrag[t.Fragment.ID] = t
	}
	var stored []uuid.UUID
	for _, t := range targets {
		if t.Placement.Status == "stored" {
			stored = append(stored, t.Fragment.ID)
		}
	}
	for _, w := range wires {
		matched := false
		for _, t := range targets {
			if t.Placement.Status == "stored" {
				continue
			}
			_, err := receipts.Verify(w, ed25519.PublicKey(t.Node.PublicKey), t.Node.ID, t.Fragment.ID, t.Fragment.SHA256, uint64(t.Fragment.SizeBytes))
			if err == nil {
				stored = append(stored, t.Fragment.ID)
				matched = true
				break
			}
		}
		if !matched {
			return store.File{}, store.FileVersion{}, ErrInvalid
		}
	}
	f, v, err := s.Store.CommitUpload(ctx, userID, uploadID, stored)
	if err != nil {
		return store.File{}, store.FileVersion{}, err
	}
	return f, v, nil
}

// PlanDownload issues retrieval tickets for stored placements.
func (s *StoreService) PlanDownload(ctx context.Context, userID, fileID uuid.UUID) (DownloadResult, error) {
	if s.SignTicket == nil {
		return DownloadResult{}, ErrInvalid
	}
	f, v, chunks, err := s.Store.LoadDownload(ctx, userID, fileID)
	if err != nil {
		return DownloadResult{}, err
	}
	out := DownloadResult{
		FileID:         f.ID,
		VersionNo:      v.VersionNo,
		SizeBytes:      v.SizeBytes,
		ContentSHA256:  hex.EncodeToString(v.ContentSHA256),
		EncryptionMeta: json.RawMessage(v.EncryptionMeta),
	}
	for _, ch := range chunks {
		dc := DownloadChunk{Seq: ch.Chunk.Seq, SHA256: hex.EncodeToString(ch.Chunk.SHA256), SizeBytes: ch.Chunk.SizeBytes}
		for _, p := range ch.Placements {
			wire, _, err := s.SignTicket(tickets.OpGet, p.Fragment.ID, p.Node.ID, p.Fragment.SHA256, uint64(p.Fragment.SizeBytes))
			if err != nil {
				return DownloadResult{}, err
			}
			dc.Fragments = append(dc.Fragments, DownloadFragment{
				ShardIndex: p.Fragment.ShardIndex,
				FragmentID: p.Fragment.ID.String(),
				SHA256:     hex.EncodeToString(p.Fragment.SHA256),
				SizeBytes:  p.Fragment.SizeBytes,
				NodeID:     p.Node.ID.String(),
				Endpoint:   p.Node.Endpoint,
				Ticket:     wire,
			})
		}
		out.Chunks = append(out.Chunks, dc)
	}
	return out, nil
}

// SignTicketFromSigner adapts tickets.Signer.
func SignTicketFromSigner(s *tickets.Signer) func(op string, fragmentID, nodeID uuid.UUID, sha []byte, maxBytes uint64) (string, time.Time, error) {
	return func(op string, fragmentID, nodeID uuid.UUID, sha []byte, maxBytes uint64) (string, time.Time, error) {
		t, err := s.Sign(tickets.Ticket{
			Op:         op,
			FragmentID: fragmentID,
			NodeID:     nodeID,
			SHA256:     sha,
			MaxBytes:   maxBytes,
		})
		if err != nil {
			return "", time.Time{}, err
		}
		return t.Raw, t.ExpiresAt, nil
	}
}
