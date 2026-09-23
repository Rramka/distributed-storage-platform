package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

const scrubBatch = 8

func (a *Agent) scrubLoop(ctx context.Context) {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = a.scrubOnce(ctx)
		}
	}
}

func (a *Agent) scrubOnce(ctx context.Context) error {
	var lost []uuid.UUID
	n := 0
	err := a.store.Each(func(id uuid.UUID, meta FragMeta) error {
		if n >= scrubBatch {
			return nil
		}
		n++
		if a.fragmentHashOK(id, meta) {
			return nil
		}
		lost = append(lost, id)
		return nil
	})
	if err != nil {
		return err
	}
	for _, id := range lost {
		_ = a.store.Delete(id)
		if err := a.reportScrub(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) fragmentHashOK(id uuid.UUID, meta FragMeta) bool {
	f, err := os.Open(a.store.pathFor(id))
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}
	return equalSHA(h.Sum(nil), meta.SHA256)
}

func (a *Agent) reportScrub(ctx context.Context, id uuid.UUID) error {
	if a.healthmon == "" || a.mtls == nil {
		return nil
	}
	body, _ := json.Marshal(map[string]string{"fragment_id": id.String()})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.healthmon+"/internal/scrub-report", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.mtls.Do(req)
	if err != nil {
		return fmt.Errorf("agent.scrub: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("agent.scrub: http %d", resp.StatusCode)
	}
	return nil
}
