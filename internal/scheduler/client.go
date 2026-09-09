package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/apierr"
)

// Client talks to the scheduler HTTP API.
type Client struct {
	base   string
	client *http.Client
}

// NewClient returns a scheduler client.
func NewClient(base string) *Client {
	return &Client{base: base, client: &http.Client{Timeout: 10 * time.Second}}
}

// Place requests node assignments.
func (c *Client) Place(ctx context.Context, needs []Need) ([]Assignment, error) {
	body, err := json.Marshal(map[string]any{"needs": needs})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/internal/placements", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scheduler.place: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var env apierr.Envelope
		_ = json.NewDecoder(resp.Body).Decode(&env)
		return nil, fmt.Errorf("scheduler.place: %s", env.Error.Code)
	}
	var out struct {
		Placements []Assignment `json:"placements"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Placements, nil
}

// PlaceFunc is an in-process placer used by tests.
type PlaceFunc func(ctx context.Context, needs []Need) ([]Assignment, error)

// Placer is implemented by Service and PlaceFunc.
type Placer interface {
	Place(ctx context.Context, needs []Need) ([]Assignment, error)
}

func (f PlaceFunc) Place(ctx context.Context, needs []Need) ([]Assignment, error) {
	return f(ctx, needs)
}
