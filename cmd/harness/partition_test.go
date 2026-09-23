package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestParsePartitionArgs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		args    []string
		wantErr bool
		region  string
		agents  int
		hold    time.Duration
		heal    bool
	}{
		{"agents", []string{"agent1", "agent2"}, false, "", 2, partitionDefaultHold, false},
		{"region", []string{"--region", "eu-west", "--for", "30s", "--heal"}, false, "eu-west", 0, 30 * time.Second, true},
		{"both", []string{"agent1", "--region", "eu-west"}, true, "", 0, 0, false},
		{"empty", nil, true, "", 0, 0, false},
		{"bad flag", []string{"--bogus"}, true, "", 0, 0, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePartitionArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.region != tc.region || len(got.agents) != tc.agents || got.hold != tc.hold || got.heal != tc.heal {
				t.Fatalf("%+v", got)
			}
		})
	}
}

func TestPartitionFloor(t *testing.T) {
	t.Parallel()
	if got := partitionFloor(3, 16); got != 13 {
		t.Fatalf("got %d", got)
	}
	if partitionHealthyFloor != 13 {
		t.Fatalf("floor const %d", partitionHealthyFloor)
	}
}

func TestBelowFloor(t *testing.T) {
	t.Parallel()
	a := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	b := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	h := map[uuid.UUID]int{a: 16, b: 12}
	bad := belowFloor(h, 13)
	if len(bad) != 1 || bad[0] != b {
		t.Fatalf("%v", bad)
	}
	if minHealth(h) != 12 {
		t.Fatalf("min %d", minHealth(h))
	}
	if minHealth(nil) != 0 {
		t.Fatal("empty")
	}
}
