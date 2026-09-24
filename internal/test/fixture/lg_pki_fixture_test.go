package fixture

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/runner"
)

// VALIDATES: the looking-glass PKI fences wait a share of the runner budget.
// PREVENTS: a fixed 30-second banner wait failing a start whose live-store
// fsync the shared disk held for longer (reload/lg-pki-reference-reload-broken,
// 2026-09-24), inside a test the .ci gave 120 seconds.
// TestLGPKIFencesDeriveFromTheRunnerBudget proves the goal: the start fence and
// each reload fence grow with the budget the runner published, the three fences
// the longest driver runs stay under that budget, and a hand run keeps the
// fallback of 30 seconds.
func TestLGPKIFencesDeriveFromTheRunnerBudget(t *testing.T) {
	cases := []struct {
		name       string
		published  string
		wantStart  time.Duration
		wantReload time.Duration
	}{
		{"authored 120s budget", "120s", 48 * time.Second, 30 * time.Second},
		{"same budget widened by contention", "360s", 144 * time.Second, 90 * time.Second},
		{"no runner published one", "", 30 * time.Second, 30 * time.Second},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(runner.TestBudgetEnv, c.published)
			env.ResetCache()
			start := time.Duration(lgPKIStartAttempts()) * lgPKIPollDelay
			reload := time.Duration(lgPKIReloadAttempts()) * lgPKIPollDelay
			if start != c.wantStart {
				t.Fatalf("start fence with %q published = %v, want %v", c.published, start, c.wantStart)
			}
			if reload != c.wantReload {
				t.Fatalf("reload fence with %q published = %v, want %v", c.published, reload, c.wantReload)
			}
			budget := runner.ChildTestBudget()
			if budget > 0 && start+2*reload >= budget {
				t.Fatalf("start %v plus two reloads %v reach the budget %v", start, reload, budget)
			}
		})
	}
}
