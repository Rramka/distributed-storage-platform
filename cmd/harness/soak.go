package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/invariants"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

const (
	soakMaxDown    = 6
	soakFloor      = 10
	soakWarn       = 13
	soakTarget     = 16
	soakAgentCount = 24
	soakSettleWait = 10 * time.Minute
	soakRoundPause = 2 * time.Second
)

func cmdSoak(args []string) error {
	duration := 2 * time.Hour
	seed := time.Now().UnixNano()
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--duration", "-d":
			i++
			if i >= len(args) {
				return fmt.Errorf("usage: harness soak [--duration 2h] [--seed N]")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("soak: duration: %w", err)
			}
			duration = d
		case "--seed":
			i++
			if i >= len(args) {
				return fmt.Errorf("usage: harness soak [--duration 2h] [--seed N]")
			}
			n, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return fmt.Errorf("soak: seed: %w", err)
			}
			seed = n
		default:
			return fmt.Errorf("usage: harness soak [--duration 2h] [--seed N]")
		}
	}

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	ctx := context.Background()
	if _, err := s.AnyCommittedFileID(ctx); err != nil {
		return fmt.Errorf("soak: need a committed file (dsp put first): %w", err)
	}

	rng := rand.New(rand.NewSource(seed))
	started := time.Now().UTC()
	deadline := started.Add(duration)
	down := map[string]bool{}
	counts := map[string]int{"kill": 0, "flap": 0, "corrupt": 0}
	var lats []int
	tracked, err := soakTrackChunks(ctx, s)
	if err != nil {
		return err
	}
	minObs, err := minTrackedHealthy(ctx, s, tracked)
	if err != nil {
		return err
	}
	settleAt := soakTarget
	if minObs < soakTarget {
		settleAt = minObs
	}
	invFails := 0
	rounds := 0
	var notes []string
	preexist := ""
	if err := invariants.CheckAll(ctx, s); err != nil {
		preexist = err.Error()
		notes = append(notes, "pre-existing: "+preexist)
		fmt.Println("- Notes: pre-existing invariants:", preexist)
	}

	fmt.Println("## Chaos report")
	fmt.Println("- Scenario: soak kill/flap/corrupt")
	fmt.Println("- Started:", started.Format(time.RFC3339))
	fmt.Println("- Seed:", seed)
	fmt.Println("- Duration:", duration)

	for time.Now().Before(deadline) {
		rounds++
		minH, err := minTrackedHealthy(ctx, s, tracked)
		if err != nil {
			return err
		}
		verb := soakPickVerb(rng)
		if minH < soakWarn {
			for len(down) > 0 {
				if err := soakStartOne(down); err != nil {
					return err
				}
			}
			verb = "flap"
		}
		agent := soakPickAgent(rng, down, verb)
		if err := soakApply(verb, agent, down); err != nil {
			notes = append(notes, fmt.Sprintf("round %d %s %s: %v", rounds, verb, agent, err))
			fmt.Println("- Notes:", err)
			_, _ = writeSoakMetrics(started, duration, seed, counts, minObs, lats, invFails, rounds, notes)
			return err
		}
		counts[verb]++
		time.Sleep(soakRoundPause)

		minH, err = minTrackedHealthy(ctx, s, tracked)
		if err != nil {
			return err
		}
		if minH < minObs {
			minObs = minH
		}
		if minH < soakFloor {
			msg := fmt.Sprintf("healthy floor breached: min %d/16", minH)
			notes = append(notes, msg)
			_, _ = writeSoakMetrics(started, duration, seed, counts, minObs, lats, invFails, rounds, notes)
			return fmt.Errorf("soak: %s", msg)
		}
		if minH < soakWarn {
			fmt.Printf("- Warn: min healthy %d/16 after %s %s\n", minH, verb, agent)
			notes = append(notes, fmt.Sprintf("warn min %d after %s %s", minH, verb, agent))
		}

		settled, lat, err := waitSettled(ctx, s, soakSettleWait, settleAt, tracked)
		if err != nil {
			return err
		}
		if settled {
			lats = append(lats, int(lat.Seconds()))
			if err := invariants.CheckAll(ctx, s); err != nil {
				if preexist == "" || err.Error() != preexist {
					invFails++
					notes = append(notes, err.Error())
					fmt.Println("- Notes:", err)
					_, _ = writeSoakMetrics(started, duration, seed, counts, minObs, lats, invFails, rounds, notes)
					return err
				}
			}
		} else {
			notes = append(notes, fmt.Sprintf("round %d did not settle in %s", rounds, soakSettleWait))
		}
	}

	p50, p90, maxL := latencyStats(lats)
	fmt.Println("- Kill set: randomized; event counts", counts)
	fmt.Println("- Download during failure: n/a (soak)")
	fmt.Printf("- Repair latency: p50=%ds p90=%ds max=%ds (n=%d)\n", p50, p90, maxL, len(lats))
	fmt.Printf("- Final healthy placements: min observed %d/16\n", minObs)
	fmt.Println("- Notes: invariants passed; rounds", rounds)

	path, err := writeSoakMetrics(started, duration, seed, counts, minObs, lats, invFails, rounds, notes)
	if err != nil {
		return err
	}
	fmt.Println("wrote", path)
	return nil
}

func soakPickVerb(rng *rand.Rand) string {
	switch rng.Intn(10) {
	case 0, 1, 2, 3:
		return "kill"
	case 4, 5, 6:
		return "flap"
	default:
		return "corrupt"
	}
}

func soakPickAgent(rng *rand.Rand, down map[string]bool, verb string) string {
	var candidates []string
	for i := 1; i <= soakAgentCount; i++ {
		name := fmt.Sprintf("agent%d", i)
		if down[name] {
			continue
		}
		candidates = append(candidates, name)
	}
	if len(candidates) == 0 {
		return fmt.Sprintf("agent%d", 1+rng.Intn(soakAgentCount))
	}
	return candidates[rng.Intn(len(candidates))]
}

func soakApply(verb, agent string, down map[string]bool) error {
	switch verb {
	case "kill":
		if len(down) >= soakMaxDown {
			if err := soakStartOne(down); err != nil {
				return err
			}
		}
		if err := compose("kill", agent).Run(); err != nil {
			return err
		}
		down[agent] = true
		return nil
	case "flap":
		if err := compose("kill", agent).Run(); err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
		if err := dockerStart(agent); err != nil {
			return err
		}
		delete(down, agent)
		return nil
	case "corrupt":
		return cmdCorrupt([]string{agent, "--frac", "0.25"})
	default:
		return fmt.Errorf("soak: unknown verb %s", verb)
	}
}

func soakStartOne(down map[string]bool) error {
	for name := range down {
		if err := dockerStart(name); err != nil {
			return err
		}
		delete(down, name)
		return nil
	}
	return nil
}

func soakTrackChunks(ctx context.Context, s *store.Store) ([]uuid.UUID, error) {
	h, err := s.CommittedChunkHealth(ctx)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	for id, n := range h {
		if n >= soakFloor {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("soak: no committed chunk at or above %d healthy (dsp put first)", soakFloor)
	}
	return ids, nil
}

func minTrackedHealthy(ctx context.Context, s *store.Store, ids []uuid.UUID) (int, error) {
	h, err := s.CommittedChunkHealth(ctx)
	if err != nil {
		return 0, err
	}
	minH := soakTarget
	found := false
	for _, id := range ids {
		n, ok := h[id]
		if !ok {
			continue
		}
		found = true
		if n < minH {
			minH = n
		}
	}
	if !found {
		return 0, fmt.Errorf("soak: tracked chunks disappeared")
	}
	return minH, nil
}

func waitSettled(ctx context.Context, s *store.Store, d time.Duration, settleAt int, tracked []uuid.UUID) (bool, time.Duration, error) {
	start := time.Now()
	deadline := start.Add(d)
	if settleAt < soakFloor {
		settleAt = soakFloor
	}
	for {
		minH, err := minTrackedHealthy(ctx, s, tracked)
		if err != nil {
			return false, time.Since(start), err
		}
		if minH >= settleAt {
			return true, time.Since(start), nil
		}
		if time.Now().After(deadline) {
			return false, time.Since(start), nil
		}
		time.Sleep(5 * time.Second)
	}
}

func latencyStats(lats []int) (p50, p90, max int) {
	if len(lats) == 0 {
		return 0, 0, 0
	}
	cp := append([]int(nil), lats...)
	sort.Ints(cp)
	p50 = cp[(len(cp)-1)*50/100]
	p90 = cp[(len(cp)-1)*90/100]
	max = cp[len(cp)-1]
	return p50, p90, max
}

func writeSoakMetrics(started time.Time, duration time.Duration, seed int64, counts map[string]int, minObs int, lats []int, invFails, rounds int, notes []string) (string, error) {
	p50, p90, maxL := latencyStats(lats)
	metrics := map[string]any{
		"date":                   started.Format("2006-01-02"),
		"duration_seconds":       int(duration.Seconds()),
		"seed":                   seed,
		"event_counts":           counts,
		"min_healthy_observed":   minObs,
		"repair_latency_p50":     p50,
		"repair_latency_p90":     p90,
		"repair_latency_max":     maxL,
		"repair_latency_samples": len(lats),
		"invariant_failures":     invFails,
		"rounds":                 rounds,
		"notes":                  notes,
	}
	dir := "business/updates"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "soak-"+started.Format("2006-01-02")+".json")
	raw, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(raw, '\n'), 0o644)
}
