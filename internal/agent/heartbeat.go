package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"
)

type heartbeatBody struct {
	NodeID            string  `json:"node_id"`
	FreeBytes         int64   `json:"free_bytes"`
	UsedBytes         int64   `json:"used_bytes"`
	CPULoad           float64 `json:"cpu_load"`
	MemUsedRatio      float64 `json:"mem_used_ratio"`
	FragmentCount     uint32  `json:"fragment_count"`
	AgentVersion      string  `json:"agent_version"`
	DiskReadLatencyMS float64 `json:"disk_read_latency_ms"`
}

type heartbeatResponse struct {
	Messages []json.RawMessage `json:"messages"`
}

func (a *Agent) heartbeatLoop(ctx context.Context) {
	t := time.NewTicker(heartbeatEvery())
	defer t.Stop()
	_ = a.sendHeartbeat(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = a.sendHeartbeat(ctx)
		}
	}
}

func (a *Agent) sendHeartbeat(ctx context.Context) error {
	if a.healthmon == "" || a.mtls == nil {
		return nil
	}
	used, n := a.store.UsedBytes()
	capBytes := capacityBytes(a.cfg)
	free := capBytes - used
	if free < 0 {
		free = 0
	}
	body, _ := json.Marshal(heartbeatBody{
		NodeID:        a.id.ID.String(),
		FreeBytes:     free,
		UsedBytes:     used,
		CPULoad:       0,
		MemUsedRatio:  0,
		FragmentCount: uint32(n),
		AgentVersion:  a.cfg.AgentVersion,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.healthmon+"/internal/heartbeat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.mtls.Do(req)
	if err != nil {
		return fmt.Errorf("agent.heartbeat: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("agent.heartbeat: http %d", resp.StatusCode)
	}
	return nil
}

func memRatio() float64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return 0
}
