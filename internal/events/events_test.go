package events

import (
	"testing"
)

func TestPrioritySubject(t *testing.T) {
	t.Parallel()
	cases := []struct {
		healthy int
		want    string
	}{
		{16, ""},
		{13, ""},
		{12, SubjRepairNormal},
		{11, SubjRepairHigh},
		{10, SubjRepairCritical},
		{9, SubjRepairCritical},
		{0, SubjRepairCritical},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(itoa(tc.healthy), func(t *testing.T) {
			t.Parallel()
			if got := PrioritySubject(tc.healthy); got != tc.want {
				t.Fatalf("healthy %d: got %q want %q", tc.healthy, got, tc.want)
			}
		})
	}
}

func TestNodeSubject(t *testing.T) {
	t.Parallel()
	if s, ok := nodeSubject("online"); !ok || s != SubjNodeOnline {
		t.Fatalf("online: %q %v", s, ok)
	}
	if s, ok := nodeSubject("suspect"); !ok || s != SubjNodeSuspect {
		t.Fatalf("suspect: %q %v", s, ok)
	}
	if s, ok := nodeSubject("offline"); !ok || s != SubjNodeOffline {
		t.Fatalf("offline: %q %v", s, ok)
	}
	if _, ok := nodeSubject("pending"); ok {
		t.Fatal("pending should not publish")
	}
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
