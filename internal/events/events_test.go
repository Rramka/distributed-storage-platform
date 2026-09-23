package events

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPrioritySubject(t *testing.T) {
	t.Parallel()
	cases := []struct {
		healthy int
		want    string
	}{
		{16, ""},
		{15, SubjRepairNormal},
		{13, SubjRepairNormal},
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

func TestUsageMsgID(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	window := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got := UsageMsgID(UsageStorage, id, window)
	want := fmt.Sprintf("usage:storage:%s:%d", id, time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC).Unix())
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if UsageMsgID(UsageStorage, id, window.Add(30*time.Minute)) != got {
		t.Fatal("same hour must share msg id")
	}
	a := UsageEvent{Kind: UsageEgress, SubjectID: id, Window: window, Dedup: "aa"}
	b := UsageEvent{Kind: UsageEgress, SubjectID: id, Window: window, Dedup: "bb"}
	if a.MsgID() == b.MsgID() {
		t.Fatal("egress receipts in the same hour must not share a msg id")
	}
	if a.MsgID() != UsageMsgIDWithDedup(UsageEgress, id, window, "aa") {
		t.Fatalf("got %q", a.MsgID())
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
