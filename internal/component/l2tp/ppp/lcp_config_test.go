// Design: docs/research/l2tpv2-ze-integration.md -- validated session settings

package ppp

import (
	"testing"
	"time"
)

// TestLCPConfigRefusesInvalidSettings keeps invalid timers and counts from
// reaching a session, including direct callers that bypass the schema parser.
func TestLCPConfigRefusesInvalidSettings(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		timer                         time.Duration
		terminate, configure, failure int
	}{
		{"zero timer", 0, 2, 10, 5},
		{"below timer range", 99 * time.Millisecond, 2, 10, 5},
		{"above timer range", 60001 * time.Millisecond, 2, 10, 5},
		{"fractional milliseconds", 100*time.Millisecond + time.Nanosecond, 2, 10, 5},
		{"zero terminate", time.Second, 0, 10, 5},
		{"large terminate", time.Second, 256, 10, 5},
		{"zero configure", time.Second, 2, 0, 5},
		{"large configure", time.Second, 2, 256, 5},
		{"zero failure", time.Second, 2, 10, 0},
		{"large failure", time.Second, 2, 10, 256},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewLCPConfig(tc.timer, tc.terminate, tc.configure, tc.failure); err == nil {
				t.Fatal("invalid LCP settings accepted")
			}
		})
	}
}

// TestLCPConfigKeepsConfiguredBounds verifies that the accessors preserve both
// ends of each schema range; admission must not clamp or replace valid values.
func TestLCPConfigKeepsConfiguredBounds(t *testing.T) {
	for _, tc := range []struct {
		timer time.Duration
		limit int
	}{
		{100 * time.Millisecond, 1}, {60 * time.Second, 255},
	} {
		c, err := NewLCPConfig(tc.timer, tc.limit, tc.limit, tc.limit)
		if err != nil {
			t.Fatal(err)
		}
		if c.RestartTimer() != tc.timer {
			t.Fatalf("timer = %s, want %s", c.RestartTimer(), tc.timer)
		}
		if c.MaxTerminate() != tc.limit {
			t.Fatalf("MaxTerminate = %d, want %d", c.MaxTerminate(), tc.limit)
		}
		if c.MaxConfigure() != tc.limit {
			t.Fatalf("MaxConfigure = %d, want %d", c.MaxConfigure(), tc.limit)
		}
		if c.MaxFailure() != tc.limit {
			t.Fatalf("MaxFailure = %d, want %d", c.MaxFailure(), tc.limit)
		}
	}
}
