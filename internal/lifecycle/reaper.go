// Package lifecycle reaps expiring placements after a customer delete.
// docs/03-data-model.md file deletion walkthrough. Runs inside cmd/repair.
package lifecycle

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/agent"
	"github.com/Rramka/distributed-storage-platform/internal/metadata"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

const tick = 15 * time.Second

// Reaper asks metadata for DELETE tickets and confirms agent removal.
type Reaper struct {
	Store *store.Store
	Meta  *Meta
	CA    *x509.Certificate
}

// Meta talks to metadata's internal lifecycle API.
type Meta struct {
	base   string
	client *http.Client
}

// NewMeta returns a lifecycle metadata client.
func NewMeta(base string) *Meta {
	return &Meta{base: base, client: &http.Client{Timeout: 30 * time.Second}}
}

// Run polls expiring placements until ctx is cancelled.
func (r *Reaper) Run(ctx context.Context) {
	t := time.NewTicker(tick)
	defer t.Stop()
	r.once(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.once(ctx)
		}
	}
}

func (r *Reaper) once(ctx context.Context) {
	if r.Store == nil {
		return
	}
	rows, err := r.Store.ListExpiringPlacements(ctx, 64)
	if err != nil {
		slog.Error("lifecycle list", "err", err)
		return
	}
	for _, p := range rows {
		if err := r.reap(ctx, p); err != nil {
			slog.Error("lifecycle reap", "err", err, "placement_id", p.ID)
		}
	}
}

func (r *Reaper) reap(ctx context.Context, p store.ExpiringPlacement) error {
	tk, err := r.Meta.DeleteTicket(ctx, p.ID)
	if err != nil {
		return err
	}
	if err := deleteFragment(ctx, r.CA, tk.Endpoint, tk.NodeID, tk.FragmentID, tk.Ticket); err != nil {
		return err
	}
	return r.Meta.ConfirmDeleted(ctx, p.ID)
}

func (m *Meta) DeleteTicket(ctx context.Context, id int64) (metadata.DeleteTicket, error) {
	var out metadata.DeleteTicket
	err := m.do(ctx, http.MethodPost, fmt.Sprintf("/internal/lifecycle/placements/%d/delete-ticket", id), &out)
	return out, err
}

func (m *Meta) ConfirmDeleted(ctx context.Context, id int64) error {
	return m.do(ctx, http.MethodPost, fmt.Sprintf("/internal/lifecycle/placements/%d/deleted", id), nil)
}

func (m *Meta) do(ctx context.Context, method, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, method, m.base+path, nil)
	if err != nil {
		return fmt.Errorf("lifecycle.meta: %w", err)
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("lifecycle.meta: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("lifecycle.meta: http %d %s", resp.StatusCode, raw)
	}
	if dst == nil {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("lifecycle.meta: %w", err)
	}
	return nil
}

func deleteFragment(ctx context.Context, caCert *x509.Certificate, endpoint, nodeID, fragID, ticket string) error {
	nid, err := uuid.Parse(nodeID)
	if err != nil {
		return err
	}
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		host = endpoint
		port = "7443"
	}
	serverName := host
	if override := os.Getenv("REPAIR_ENDPOINT_HOST"); override != "" {
		host = override
	}
	cl := &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: agent.ClientTLS(caCert, nid, serverName)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "https://"+net.JoinHostPort(host, port)+"/fragments/"+fragID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-DSP-Ticket", ticket)
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	raw, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("lifecycle.delete: http %d %s", resp.StatusCode, raw)
}
