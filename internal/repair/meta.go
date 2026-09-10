package repair

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/metadata"
	"github.com/google/uuid"
)

// Meta talks to metadata's internal repair API. It never sees a signing seed.
type Meta struct {
	base   string
	client *http.Client
}

// NewMeta returns a metadata repair client.
func NewMeta(base string) *Meta {
	return &Meta{base: base, client: &http.Client{Timeout: 30 * time.Second}}
}

func (m *Meta) Plan(ctx context.Context, chunkID uuid.UUID) (metadata.RepairPlan, error) {
	var plan metadata.RepairPlan
	err := m.do(ctx, http.MethodPost, "/internal/repair/chunks/"+chunkID.String()+"/plan", nil, &plan)
	return plan, err
}

func (m *Meta) Commit(ctx context.Context, chunkID uuid.UUID, receipts []string) error {
	return m.do(ctx, http.MethodPost, "/internal/repair/chunks/"+chunkID.String()+"/commit", map[string]any{
		"receipts": receipts,
	}, nil)
}

func (m *Meta) Lost(ctx context.Context, nodeID uuid.UUID) ([]metadata.LostRow, error) {
	var out struct {
		Placements []metadata.LostRow `json:"placements"`
	}
	err := m.do(ctx, http.MethodGet, "/internal/repair/nodes/"+nodeID.String()+"/lost", nil, &out)
	return out.Placements, err
}

func (m *Meta) Flap(ctx context.Context, nodeID uuid.UUID, restore, expire []int64) error {
	return m.do(ctx, http.MethodPost, "/internal/repair/nodes/"+nodeID.String()+"/flap", map[string]any{
		"restore": restore,
		"expire":  expire,
	}, nil)
}

func (m *Meta) do(ctx context.Context, method, path string, body any, dst any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, m.base+path, rdr)
	if err != nil {
		return fmt.Errorf("repair.meta: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("repair.meta: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("repair.meta: http %d %s", resp.StatusCode, raw)
	}
	if dst == nil {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("repair.meta: %w", err)
	}
	return nil
}
