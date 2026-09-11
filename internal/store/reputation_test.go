package store

import "testing"

func TestNextReputationClampAndDirection(t *testing.T) {
	t.Parallel()
	if got := NextReputation(0.5, SignalChallengePassed); got <= 0.5 {
		t.Fatalf("pass should raise: %v", got)
	}
	if got := NextReputation(0.5, SignalChallengeFailed); got >= 0.5 {
		t.Fatalf("fail should lower: %v", got)
	}
	if got := NextReputation(0.5, SignalUnplannedOffline); got >= 0.5 {
		t.Fatalf("offline should lower: %v", got)
	}
	if got := NextReputation(0, SignalChallengeFailed); got < 0 {
		t.Fatal("below zero")
	}
	v := float32(0.99)
	for i := 0; i < 40; i++ {
		v = NextReputation(v, SignalChallengePassed)
	}
	if v > 1 {
		t.Fatalf("above one %v", v)
	}
}
