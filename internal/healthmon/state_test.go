package healthmon

import (
	"testing"
	"time"
)

func TestNextStatus(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	suspect := 30 * time.Second
	offline := 5 * time.Minute
	cases := []struct {
		name    string
		current string
		age     time.Duration
		want    string
	}{
		{"online_fresh", "online", 10 * time.Second, "online"},
		{"online_suspect", "online", 30 * time.Second, "suspect"},
		{"online_just_under_offline", "online", 5*time.Minute - time.Second, "suspect"},
		{"online_offline", "online", 5 * time.Minute, "offline"},
		{"suspect_hold", "suspect", 2 * time.Minute, "suspect"},
		{"suspect_offline", "suspect", 5 * time.Minute, "offline"},
		{"pending_untouched", "pending", 10 * time.Minute, "pending"},
		{"draining_untouched", "draining", 10 * time.Minute, "draining"},
		{"zero_seen", "online", 0, "online"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var seen time.Time
			if tc.age > 0 {
				seen = now.Add(-tc.age)
			}
			got := NextStatus(tc.current, seen, now, suspect, offline)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
