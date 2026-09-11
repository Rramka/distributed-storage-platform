package healthmon

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/agent"
	"github.com/Rramka/distributed-storage-platform/internal/events"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/Rramka/distributed-storage-platform/internal/tickets"
	"github.com/google/uuid"
)

const (
	defaultChallengeTick     = 15 * time.Second
	defaultChallengeInterval = 24 * time.Hour
	defaultChallengeBatch    = 8
	defaultChallengeSetSize  = 8
	defaultChallengeMaxRange = 4096
	defaultChallengeDeadline = 8 * time.Second
	lowWater                 = 2
)

// Challenger audits stored placements with pre-computed challenge sets.
// docs/07-security.md — Health Monitor refresh path.
type Challenger struct {
	Store    *store.Store
	Bus      *events.Bus
	Meta     *ChallengeMeta
	CA       *x509.Certificate
	Tick     time.Duration
	Interval time.Duration
	Batch    int
	SetSize  int
	MaxRange int
	Deadline time.Duration

	GetFragment func(ctx context.Context, endpoint string, nodeID, fragID uuid.UUID, ticket string) ([]byte, error)
	DoChallenge func(ctx context.Context, endpoint string, nodeID, fragID uuid.UUID, ticket string, offset, length int, nonce []byte) ([]byte, error)
}

// ChallengeDurationsFromEnv reads CHALLENGE_INTERVAL / CHALLENGE_TICK.
func ChallengeDurationsFromEnv() (tick, interval time.Duration) {
	tick = defaultChallengeTick
	interval = defaultChallengeInterval
	if v := os.Getenv("CHALLENGE_TICK"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			tick = d
		}
	}
	if v := os.Getenv("CHALLENGE_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		}
	}
	return tick, interval
}

// Run ticks until ctx is cancelled.
func (c *Challenger) Run(ctx context.Context) {
	c.defaults()
	t := time.NewTicker(c.Tick)
	defer t.Stop()
	c.scan(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.scan(ctx)
		}
	}
}

func (c *Challenger) defaults() {
	if c.Tick <= 0 {
		c.Tick = defaultChallengeTick
	}
	if c.Interval <= 0 {
		c.Interval = defaultChallengeInterval
	}
	if c.Batch <= 0 {
		c.Batch = defaultChallengeBatch
	}
	if c.SetSize <= 0 {
		c.SetSize = defaultChallengeSetSize
	}
	if c.MaxRange <= 0 {
		c.MaxRange = defaultChallengeMaxRange
	}
	if c.Deadline <= 0 {
		c.Deadline = defaultChallengeDeadline
	}
	if c.GetFragment == nil {
		c.GetFragment = c.getFragment
	}
	if c.DoChallenge == nil {
		c.DoChallenge = c.doChallenge
	}
}

func (c *Challenger) scan(ctx context.Context) {
	targets, err := c.Store.ListDueAudits(ctx, c.Interval, c.Batch)
	if err != nil {
		slog.Error("challenge scan", "err", err)
		return
	}
	for _, t := range targets {
		if err := c.audit(ctx, t); err != nil {
			slog.Error("challenge audit", "err", err, "fragment_id", t.FragmentID, "node_id", t.NodeID)
		}
	}
}

func (c *Challenger) audit(ctx context.Context, t store.AuditTarget) error {
	if t.Remaining < lowWater {
		if err := c.refill(ctx, t); err != nil {
			return err
		}
	}
	ch, err := c.Store.TakeNextUnspent(ctx, t.FragmentID, t.NodeID)
	if err != nil {
		return err
	}
	issued, err := c.Meta.Tickets(ctx, t.FragmentID, t.NodeID, tickets.OpChallenge)
	if err != nil || len(issued) == 0 {
		return fmt.Errorf("healthmon.audit: mint: %w", err)
	}
	deadline, cancel := context.WithTimeout(ctx, c.Deadline)
	defer cancel()
	got, err := c.DoChallenge(deadline, t.Endpoint, t.NodeID, t.FragmentID, issued[0].Ticket, ch.Offset, ch.Length, ch.Nonce)
	_ = c.Store.MarkChallengeSpent(ctx, t.FragmentID, t.NodeID, ch.Seq)
	_ = c.Store.TouchLastAudit(ctx, t.FragmentID, t.NodeID)
	if err != nil || !bytesEqual(got, ch.Expected) {
		_, _, _ = c.Store.ApplyReputationEvent(ctx, t.NodeID, store.SignalChallengeFailed)
		_ = c.Store.BumpNodeAudit(ctx, t.NodeID, false)
		return c.failPlacement(ctx, t)
	}
	_, _, _ = c.Store.ApplyReputationEvent(ctx, t.NodeID, store.SignalChallengePassed)
	_ = c.Store.BumpNodeAudit(ctx, t.NodeID, true)
	return nil
}

func (c *Challenger) refill(ctx context.Context, t store.AuditTarget) error {
	issued, err := c.Meta.Tickets(ctx, t.FragmentID, t.NodeID, tickets.OpGet)
	if err != nil || len(issued) == 0 {
		return fmt.Errorf("healthmon.refill: mint: %w", err)
	}
	body, err := c.GetFragment(ctx, t.Endpoint, t.NodeID, t.FragmentID, issued[0].Ticket)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	if !bytesEqual(sum[:], t.SHA256) {
		_, _, _ = c.Store.ApplyReputationEvent(ctx, t.NodeID, store.SignalChallengeFailed)
		_ = c.Store.BumpNodeAudit(ctx, t.NodeID, false)
		return c.failPlacement(ctx, t)
	}
	set := ComputeChallengeSet(body, t.FragmentID, t.NodeID, c.SetSize, c.MaxRange)
	return c.Store.InsertChallengeSet(ctx, set)
}

func (c *Challenger) failPlacement(ctx context.Context, t store.AuditTarget) error {
	lp, err := c.Store.MarkPlacementLost(ctx, t.FragmentID, t.NodeID)
	if err != nil {
		return err
	}
	healthy, err := c.Store.ChunkHealth(ctx, lp.ChunkID)
	if err != nil {
		return err
	}
	if c.Bus == nil {
		return nil
	}
	return c.Bus.PublishRepair(ctx, events.RepairJob{ChunkID: lp.ChunkID, Healthy: healthy})
}

// ComputeChallengeSet builds N (offset, length, nonce, expected) tuples from ciphertext.
func ComputeChallengeSet(data []byte, fragmentID, nodeID uuid.UUID, n, maxRange int) []store.Challenge {
	if n <= 0 {
		n = defaultChallengeSetSize
	}
	if maxRange <= 0 {
		maxRange = defaultChallengeMaxRange
	}
	if maxRange > agent.MaxChallengeRange {
		maxRange = agent.MaxChallengeRange
	}
	size := len(data)
	if size == 0 {
		return nil
	}
	out := make([]store.Challenge, 0, n)
	for i := 0; i < n; i++ {
		length := maxRange
		if length > size {
			length = size
		}
		if length > 1 {
			length = 1 + int(randUint64()%uint64(length))
		}
		maxOff := size - length
		offset := 0
		if maxOff > 0 {
			offset = int(randUint64() % uint64(maxOff+1))
		}
		nonce := make([]byte, 16)
		_, _ = rand.Read(nonce)
		h := sha256.New()
		_, _ = h.Write(nonce)
		_, _ = h.Write(data[offset : offset+length])
		out = append(out, store.Challenge{
			FragmentID: fragmentID,
			NodeID:     nodeID,
			Seq:        i,
			Offset:     offset,
			Length:     length,
			Nonce:      nonce,
			Expected:   h.Sum(nil),
		})
	}
	return out
}

func randUint64() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 1
	}
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
}

func (c *Challenger) getFragment(ctx context.Context, endpoint string, nodeID, fragID uuid.UUID, ticket string) ([]byte, error) {
	rawURL, serverName, err := fragmentURL(endpoint, fragID.String())
	if err != nil {
		return nil, err
	}
	cl := c.httpClient(nodeID, serverName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-DSP-Ticket", ticket)
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("healthmon.get: http %d %s", resp.StatusCode, raw)
	}
	return raw, nil
}

func (c *Challenger) doChallenge(ctx context.Context, endpoint string, nodeID, fragID uuid.UUID, ticket string, offset, length int, nonce []byte) ([]byte, error) {
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		host = endpoint
		port = "7443"
	}
	serverName := host
	if override := os.Getenv("HEALTHMON_ENDPOINT_HOST"); override != "" {
		host = override
	}
	u := "https://" + net.JoinHostPort(host, port) + "/challenge?fragment_id=" + fragID.String() +
		"&offset=" + strconv.Itoa(offset) +
		"&length=" + strconv.Itoa(length) +
		"&nonce=" + hex.EncodeToString(nonce)
	cl := c.httpClient(nodeID, serverName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-DSP-Ticket", ticket)
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("healthmon.challenge: http %d %s", resp.StatusCode, raw)
	}
	var out struct {
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	sum, err := hex.DecodeString(out.SHA256)
	if err != nil {
		return nil, err
	}
	return sum, nil
}

func (c *Challenger) httpClient(nodeID uuid.UUID, serverName string) *http.Client {
	return &http.Client{
		Timeout:   c.Deadline,
		Transport: &http.Transport{TLSClientConfig: agent.ClientTLS(c.CA, nodeID, serverName)},
	}
}

func fragmentURL(endpoint, fragID string) (url, serverName string, err error) {
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		host = endpoint
		port = "7443"
	}
	serverName = host
	if override := os.Getenv("HEALTHMON_ENDPOINT_HOST"); override != "" {
		host = override
	}
	return "https://" + net.JoinHostPort(host, port) + "/fragments/" + fragID, serverName, nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
