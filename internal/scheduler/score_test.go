package scheduler

import "testing"

func TestNodeScoreReputationMonotonic(t *testing.T) {
	t.Parallel()
	low := fakeNode(0, 0, "us-east", 64501)
	high := low
	low.Reputation = 0.2
	high.Reputation = 0.9
	if nodeScore(high) <= nodeScore(low) {
		t.Fatalf("high %v low %v", nodeScore(high), nodeScore(low))
	}
}

func TestNodeScoreNeutralBandwidth(t *testing.T) {
	t.Parallel()
	n := fakeNode(0, 0, "us-east", 64501)
	n.Reputation = 0.5
	s := nodeScore(n)
	if s <= 0 {
		t.Fatalf("score %v", s)
	}
}
