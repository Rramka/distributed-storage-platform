package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/invariants"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

const (
	partitionTotal        = 16
	partitionRegionCap    = 3
	partitionHealthyFloor = partitionTotal - partitionRegionCap // 13
	partitionDefaultHold  = 8 * time.Minute
	partitionHealWait     = 10 * time.Minute
	partitionPollInterval = 5 * time.Second
)

type partitionOpts struct {
	agents []string
	region string
	hold   time.Duration
	heal   bool
}

func cmdPartition(args []string) error {
	opts, err := parsePartitionArgs(args)
	if err != nil {
		return err
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	ctx := context.Background()
	names, err := resolvePartitionAgents(ctx, s, opts)
	if err != nil {
		return err
	}

	health, err := s.AnyChunkHealth(ctx)
	if err != nil {
		return err
	}
	if len(health) == 0 {
		return fmt.Errorf("partition: no committed chunks (dsp put first)")
	}

	var once sync.Once
	unpause := func() {
		once.Do(func() {
			fmt.Println("unpause", joinNames(names))
			_ = compose(append([]string{"unpause"}, names...)...).Run()
		})
	}
	defer unpause()

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("## Chaos report")
	fmt.Println("- Scenario: partition (cgroup freeze)")
	fmt.Println("- Started:", time.Now().UTC().Format(time.RFC3339))
	if opts.region != "" {
		fmt.Println("- Region:", opts.region)
	}
	fmt.Println("- Pause set:", joinNames(names))
	fmt.Println("pause", joinNames(names))
	if err := compose(append([]string{"pause"}, names...)...).Run(); err != nil {
		return err
	}

	deadline := time.Now().Add(opts.hold)
	for time.Now().Before(deadline) {
		select {
		case <-sigCtx.Done():
			return fmt.Errorf("partition: interrupted")
		case <-time.After(partitionPollInterval):
		}
		h, err := s.AnyChunkHealth(ctx)
		if err != nil {
			return err
		}
		if bad := belowFloor(h, partitionHealthyFloor); len(bad) > 0 {
			return fmt.Errorf("partition: chunk %s at %d/16 below floor %d", bad[0], h[bad[0]], partitionHealthyFloor)
		}
		fmt.Printf("- Healthy min %d/16 while partitioned\n", minHealth(h))
	}

	unpause()
	if !opts.heal {
		return nil
	}
	fmt.Println("- Heal: waiting for flap re-validation / repair")
	settleDeadline := time.Now().Add(partitionHealWait)
	var final map[uuid.UUID]int
	for time.Now().Before(settleDeadline) {
		select {
		case <-sigCtx.Done():
			return fmt.Errorf("partition: interrupted during heal")
		default:
		}
		h, err := s.AnyChunkHealth(ctx)
		if err != nil {
			return err
		}
		final = h
		if minHealth(h) >= partitionTotal {
			break
		}
		time.Sleep(partitionPollInterval)
	}
	minH := minHealth(final)
	fmt.Printf("- Final healthy placements: min %d/16 across %d chunks\n", minH, len(final))
	if err := invariants.CheckAll(ctx, s); err != nil {
		fmt.Println("- Notes:", err)
		return fmt.Errorf("partition heal: %w", err)
	}
	fmt.Println("- Notes: invariants passed")
	if minH < partitionTotal {
		return fmt.Errorf("partition heal: did not reach 16/16 (min %d)", minH)
	}
	return nil
}

func parsePartitionArgs(args []string) (partitionOpts, error) {
	o := partitionOpts{hold: partitionDefaultHold}
	usage := fmt.Errorf("usage: harness partition <agent...> | --region <r> [--for 8m] [--heal]")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--region", "-region":
			i++
			if i >= len(args) {
				return o, usage
			}
			o.region = args[i]
		case "--for", "-for":
			i++
			if i >= len(args) {
				return o, usage
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return o, fmt.Errorf("partition: duration: %w", err)
			}
			o.hold = d
		case "--heal", "-heal":
			o.heal = true
		default:
			if len(args[i]) > 0 && args[i][0] == '-' {
				return o, usage
			}
			o.agents = append(o.agents, args[i])
		}
	}
	if o.region == "" && len(o.agents) == 0 {
		return o, usage
	}
	if o.region != "" && len(o.agents) > 0 {
		return o, fmt.Errorf("partition: specify agents or --region, not both")
	}
	return o, nil
}

func resolvePartitionAgents(ctx context.Context, s *store.Store, opts partitionOpts) ([]string, error) {
	if len(opts.agents) > 0 {
		return opts.agents, nil
	}
	nodes, err := s.NodesByRegion(ctx, opts.region)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("partition: no nodes in region %q", opts.region)
	}
	var names []string
	seen := map[string]bool{}
	for _, n := range nodes {
		name, err := agentService(n.Endpoint)
		if err != nil {
			fmt.Fprintf(os.Stderr, "partition: skip %s: %v\n", n.Endpoint, err)
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("partition: no compose agents in region %q", opts.region)
	}
	return names, nil
}

func minHealth(h map[uuid.UUID]int) int {
	if len(h) == 0 {
		return 0
	}
	minH := partitionTotal
	for _, n := range h {
		if n < minH {
			minH = n
		}
	}
	return minH
}

func belowFloor(h map[uuid.UUID]int, floor int) []uuid.UUID {
	var bad []uuid.UUID
	for id, n := range h {
		if n < floor {
			bad = append(bad, id)
		}
	}
	return bad
}

func partitionFloor(regionCap, total int) int {
	return total - regionCap
}

func joinNames(names []string) string {
	return strings.Join(names, " ")
}
