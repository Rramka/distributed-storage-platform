// Package repair reconstructs missing ciphertext shards. It never decrypts
// and never holds the ticket signing seed (docs/06-scheduler-and-repair.md).
package repair

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/events"
	"github.com/Rramka/distributed-storage-platform/internal/metadata"
	"github.com/Rramka/distributed-storage-platform/internal/pipeline"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

const repairThreshold = 13

// Worker consumes node.offline / node.online and repair jobs.
type Worker struct {
	Store         *store.Store
	Bus           *events.Bus
	Meta          *Meta
	CA            *x509.Certificate
	MaxConcurrent int
	BudgetBps     int64

	sem chan struct{}
	mu  sync.Mutex
	tok int64
	at  time.Time
}

// Run starts consumers until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	if w.MaxConcurrent <= 0 {
		w.MaxConcurrent = 4
	}
	w.sem = make(chan struct{}, w.MaxConcurrent)
	w.at = time.Now()
	w.enqueueUnhealthy(ctx)
	errCh := make(chan error, 3)
	go func() {
		errCh <- w.Bus.Consume(ctx, events.StreamNodeEvents, "repair-node-offline", events.SubjNodeOffline, w.onNodeOffline)
	}()
	go func() {
		errCh <- w.Bus.Consume(ctx, events.StreamNodeEvents, "repair-node-online", events.SubjNodeOnline, w.onNodeOnline)
	}()
	go func() { errCh <- w.drainJobs(ctx) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (w *Worker) onNodeOffline(ctx context.Context, _ string, data []byte) error {
	var ev events.NodeEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return err
	}
	lost, err := w.Store.MarkPlacementsLostByNode(ctx, ev.NodeID)
	if err != nil {
		return err
	}
	seen := map[uuid.UUID]struct{}{}
	for _, lp := range lost {
		if _, ok := seen[lp.ChunkID]; ok {
			continue
		}
		seen[lp.ChunkID] = struct{}{}
		healthy, err := w.Store.ChunkHealth(ctx, lp.ChunkID)
		if err != nil {
			return err
		}
		if events.PrioritySubject(healthy) == "" {
			continue
		}
		if err := w.Bus.PublishRepair(ctx, events.RepairJob{ChunkID: lp.ChunkID, Healthy: healthy}); err != nil {
			return err
		}
		slog.Info("repair queued", "chunk_id", lp.ChunkID, "healthy", healthy, "node_id", ev.NodeID)
	}
	return nil
}

func (w *Worker) enqueueUnhealthy(ctx context.Context) {
	ids, err := w.Store.UnderHealthyChunks(ctx, repairThreshold)
	if err != nil {
		slog.Error("repair scan", "err", err)
		return
	}
	for _, id := range ids {
		h, err := w.Store.ChunkHealth(ctx, id)
		if err != nil {
			slog.Error("repair scan health", "err", err, "chunk_id", id)
			continue
		}
		if err := w.Bus.PublishRepair(ctx, events.RepairJob{ChunkID: id, Healthy: h}); err != nil {
			slog.Error("repair scan publish", "err", err, "chunk_id", id)
			continue
		}
		slog.Info("repair queued from scan", "chunk_id", id, "healthy", h)
	}
}

func (w *Worker) onNodeOnline(ctx context.Context, _ string, data []byte) error {
	var ev events.NodeEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return err
	}
	// Interim flap re-validation: hash-verified GET stands in for challenges.
	rows, err := w.Meta.Lost(ctx, ev.NodeID)
	if err != nil {
		return err
	}
	var restore, expire []int64
	for _, row := range rows {
		body, err := getFragment(ctx, w.CA, row.Endpoint, ev.NodeID.String(), row.FragmentID, row.Ticket)
		if err != nil || !hashMatches(body, row.SHA256) {
			continue
		}
		w.account(len(body))
		if row.ReconstructedElsewhere {
			expire = append(expire, row.PlacementID)
			continue
		}
		restore = append(restore, row.PlacementID)
	}
	if len(restore) == 0 && len(expire) == 0 {
		return nil
	}
	return w.Meta.Flap(ctx, ev.NodeID, restore, expire)
}

func (w *Worker) drainJobs(ctx context.Context) error {
	filters := []struct {
		durable string
		filter  string
	}{
		{"repair-jobs-critical", events.SubjRepairCritical},
		{"repair-jobs-high", events.SubjRepairHigh},
		{"repair-jobs-normal", events.SubjRepairNormal},
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		got := false
		for _, f := range filters {
			subj, data, ack, nak, err := w.Bus.FetchOne(ctx, events.StreamRepairJobs, f.durable, f.filter, 150*time.Millisecond)
			if err != nil {
				if events.IsTimeout(err) {
					continue
				}
				slog.Error("repair fetch", "err", err, "filter", f.filter)
				continue
			}
			_ = subj
			got = true
			w.acquire()
			err = w.onJob(ctx, data)
			w.release()
			if err != nil {
				slog.Error("repair job", "err", err)
				_ = nak()
				continue
			}
			_ = ack()
			break
		}
		if !got {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
		}
	}
}

func (w *Worker) onJob(ctx context.Context, data []byte) error {
	var job events.RepairJob
	if err := json.Unmarshal(data, &job); err != nil {
		return err
	}
	healthy, err := w.Store.ChunkHealth(ctx, job.ChunkID)
	if err != nil {
		return err
	}
	if healthy >= repairThreshold {
		slog.Info("repair skip recovered", "chunk_id", job.ChunkID, "healthy", healthy)
		return nil
	}
	plan, err := w.Meta.Plan(ctx, job.ChunkID)
	if err != nil {
		return err
	}
	if len(plan.Targets) == 0 {
		return nil
	}
	shards, err := w.fetchSources(ctx, plan.Sources)
	if err != nil {
		return err
	}
	if err := pipeline.ReconstructShards(shards); err != nil {
		return fmt.Errorf("repair.reconstruct: %w", err)
	}
	var receipts []string
	for _, tgt := range plan.Targets {
		idx := int(tgt.ShardIndex)
		if idx < 0 || idx >= pipeline.ECTotal || shards[idx] == nil {
			return fmt.Errorf("repair.job: missing rebuilt shard %d", tgt.ShardIndex)
		}
		if !hashMatches(shards[idx], tgt.SHA256) {
			return fmt.Errorf("repair.job: rebuilt hash mismatch shard %d", tgt.ShardIndex)
		}
		rec, err := putFragment(ctx, w.CA, tgt.Endpoint, tgt.NodeID, tgt.FragmentID, tgt.Ticket, shards[idx])
		if err != nil {
			return err
		}
		w.account(len(shards[idx]))
		receipts = append(receipts, rec)
	}
	return w.Meta.Commit(ctx, job.ChunkID, receipts)
}

func (w *Worker) fetchSources(ctx context.Context, sources []metadata.RepairSource) ([][]byte, error) {
	shards := make([][]byte, pipeline.ECTotal)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type hit struct {
		idx  int16
		data []byte
	}
	hits := make(chan hit, len(sources))
	var wg sync.WaitGroup
	for _, src := range sources {
		src := src
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := getFragment(ctx, w.CA, src.Endpoint, src.NodeID, src.FragmentID, src.Ticket)
			if err != nil || !hashMatches(data, src.SHA256) {
				if err != nil {
					slog.Debug("repair source", "err", err, "endpoint", src.Endpoint, "fragment", src.FragmentID)
				}
				return
			}
			w.account(len(data))
			select {
			case hits <- hit{idx: src.ShardIndex, data: data}:
			case <-ctx.Done():
			}
		}()
	}
	go func() {
		wg.Wait()
		close(hits)
	}()
	got := 0
	for h := range hits {
		if int(h.idx) < 0 || int(h.idx) >= pipeline.ECTotal || shards[h.idx] != nil {
			continue
		}
		shards[h.idx] = h.data
		got++
		if got >= pipeline.ECData {
			cancel()
			break
		}
	}
	if got < pipeline.ECData {
		return nil, fmt.Errorf("repair.fetch: only %d of %d shards", got, pipeline.ECData)
	}
	return shards, nil
}

func (w *Worker) acquire() {
	w.sem <- struct{}{}
}

func (w *Worker) release() {
	<-w.sem
}

func (w *Worker) account(n int) {
	if w.BudgetBps <= 0 || n <= 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(w.at).Seconds()
	w.tok += int64(elapsed * float64(w.BudgetBps))
	if w.tok > w.BudgetBps {
		w.tok = w.BudgetBps
	}
	w.at = now
	w.tok -= int64(n)
	if w.tok < 0 {
		wait := time.Duration(float64(-w.tok) / float64(w.BudgetBps) * float64(time.Second))
		w.mu.Unlock()
		time.Sleep(wait)
		w.mu.Lock()
		w.tok = 0
		w.at = time.Now()
	}
}
