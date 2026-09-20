package fixture

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/runner"
)

// VALIDATES: a fixture wall-clock deadline is a share of the runner budget.
// PREVENTS: a fixture stating a naked deadline that ignores option=timeout:value=.
// TestWaitBudgetDerivesFromTheRunnerBudget proves the goal of this helper: the
// wall clock a fixture waits is a share of the budget the runner published, so
// it moves with the run's contention (the runner widens that value through
// withParallelHeadroom) instead of being a constant written in the fixture.
func TestWaitBudgetDerivesFromTheRunnerBudget(t *testing.T) {
	cases := []struct {
		name      string
		published string
		percent   int
		fallback  time.Duration
		want      time.Duration
	}{
		{"authored budget", "90s", 75, 25 * time.Second, 67500 * time.Millisecond},
		{"same budget tripled by contention", "270s", 75, 25 * time.Second, 202500 * time.Millisecond},
		{"no runner published one", "", 75, 25 * time.Second, 25 * time.Second},
		{"unparseable value falls back", "soon", 75, 25 * time.Second, 25 * time.Second},
		{"whole budget", "60s", 100, time.Second, 60 * time.Second},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(runner.TestBudgetEnv, c.published)
			env.ResetCache()
			if got := WaitBudget(c.percent, c.fallback); got != c.want {
				t.Fatalf("WaitBudget(%d, %v) with %q published = %v, want %v", c.percent, c.fallback, c.published, got, c.want)
			}
		})
	}
}

// TestWaitAttemptsConvertsTheSameBudget proves an attempt count is the same
// deadline in another unit: 450 attempts at 100ms was 45s hand-matched to a 60s
// budget, and the count now falls out of the budget instead.
func TestWaitAttemptsConvertsTheSameBudget(t *testing.T) {
	cases := []struct {
		name      string
		published string
		want      int
	}{
		{"60s budget at 75 percent", "60s", 450},
		{"contended 180s budget", "180s", 1350},
		{"no runner published one", "", 450},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(runner.TestBudgetEnv, c.published)
			env.ResetCache()
			if got := WaitAttempts(75, 100*time.Millisecond, 450); got != c.want {
				t.Fatalf("WaitAttempts with %q published = %d, want %d", c.published, got, c.want)
			}
		})
	}
}

// TestWaitBudgetRefusesAnImpossibleShare proves the guard is a guard: a share
// outside 1..100 is a programmer error, and a silent clamp would hand back a
// plausible duration for a percent nobody meant to write.
func TestWaitBudgetRefusesAnImpossibleShare(t *testing.T) {
	for _, percent := range []int{0, -1, 101} {
		t.Run(time.Duration(percent).String(), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("WaitBudget(%d, ...) returned instead of refusing", percent)
				}
			}()
			_ = WaitBudget(percent, time.Second)
		})
	}
}
