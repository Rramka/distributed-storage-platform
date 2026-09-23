package main

import (
	"math/rand"
	"testing"
)

func TestLatencyStats(t *testing.T) {
	t.Parallel()
	p50, p90, max := latencyStats(nil)
	if p50 != 0 || p90 != 0 || max != 0 {
		t.Fatalf("empty %d %d %d", p50, p90, max)
	}
	got50, got90, gotMax := latencyStats([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	if got50 != 5 || got90 != 9 || gotMax != 10 {
		t.Fatalf("got p50=%d p90=%d max=%d", got50, got90, gotMax)
	}
}

func TestSoakPickVerbDistribution(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(1))
	seen := map[string]int{}
	for i := 0; i < 1000; i++ {
		seen[soakPickVerb(rng)]++
	}
	for _, v := range []string{"kill", "flap", "corrupt", "partition"} {
		if seen[v] == 0 {
			t.Fatalf("never picked %s", v)
		}
	}
}

func TestSoakPickAgentSkipsDown(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(2))
	down := map[string]bool{"agent1": true, "agent2": true}
	paused := map[string]bool{"agent3": true}
	for i := 0; i < 50; i++ {
		got := soakPickAgent(rng, down, paused)
		if down[got] || paused[got] {
			t.Fatalf("picked unavailable agent %s", got)
		}
	}
}
