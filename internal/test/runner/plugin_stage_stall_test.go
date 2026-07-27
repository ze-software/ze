package runner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPluginStageStallDerivation pins the stall window against the test budget,
// including every boundary of the floor/share/clamp interaction.
//
// VALIDATES: the window derives from the resolved test budget (share), never
// drops below the constant both call sites hardcoded before (floor), and never
// exceeds the budget (clamp) so the watchdog's precise message still wins the
// race against the outer test timeout.
// PREVENTS: regressing to a fixed constant, which is sized against one machine
// and lies at a different speed (ai/rules/fix-dont-record.md: "generous is a
// synonym for unknown").
func TestPluginStageStallDerivation(t *testing.T) {
	const floor = pluginStageStallFloor // 10s
	// Below this budget the floor binds; above it the share binds.
	const crossover = time.Duration(float64(floor) / pluginStageStallShare) // 12.5s

	tests := []struct {
		name   string
		budget time.Duration
		want   time.Duration
	}{
		// Unknown budget: the floor is all we have.
		{"zero budget falls back to the floor", 0, floor},
		{"negative budget falls back to the floor", -1 * time.Second, floor},

		// Budget under the floor: clamped DOWN to the budget. Applying the floor
		// here would invert the ordering the watchdog exists to preserve.
		{"budget below the floor clamps to the budget", 5 * time.Second, 5 * time.Second},
		{"budget one below the floor clamps to the budget", floor - time.Second, floor - time.Second},
		{"budget exactly the floor is the floor", floor, floor},

		// Between floor and crossover the share is smaller than the floor, so the
		// floor lifts it -- but never above the budget.
		{"just below crossover: floor lifts, budget still allows it", crossover - time.Second, floor},
		{"exactly at crossover: share equals floor", crossover, floor},
		{"just above crossover: share now binds", crossover + time.Second, time.Duration(float64(crossover+time.Second) * pluginStageStallShare)},

		// The real cases. ospfv3 netns tests declare timeout=15s.
		{"15s test budget derives 12s", 15 * time.Second, 12 * time.Second},
		{"60s test budget derives 48s", 60 * time.Second, 48 * time.Second},
		{"120s test budget derives 96s", 120 * time.Second, 96 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pluginStageStall(tt.budget)
			assert.Equal(t, tt.want, got, "budget %s", tt.budget)

			// Two invariants that must hold for EVERY budget, not just these rows.
			if tt.budget > 0 {
				assert.LessOrEqual(t, got, tt.budget,
					"the window must never exceed the test budget, or the outer timeout wins the race and the precise message is lost")
			}
			assert.Positive(t, got, "a non-positive window DISABLES the stall check in the product (WaitForStageProgress), which must never happen by accident")
		})
	}
}

// TestPluginStageStallNeverNarrowsBelowTheOldConstant proves the change can only
// widen the window relative to the hardcoded 10s both call sites used, for every
// budget that could already accommodate 10s.
//
// VALIDATES: no test that passed on the old constant loses window.
// PREVENTS: a share/floor edit silently tightening the watchdog.
func TestPluginStageStallNeverNarrowsBelowTheOldConstant(t *testing.T) {
	const oldConstant = 10 * time.Second

	for budget := time.Second; budget <= 120*time.Second; budget += time.Second {
		got := pluginStageStall(budget)
		if budget >= oldConstant {
			assert.GreaterOrEqual(t, got, oldConstant,
				"budget %s: derived %s is narrower than the old fixed 10s", budget, got)
		} else {
			// Budgets under 10s could never really have used a 10s window: the
			// test timeout expired first. Clamping to the budget is the fix.
			assert.Equal(t, budget, got, "budget %s must clamp to itself", budget)
		}
	}
}

// TestPluginStageStallEnvAppliesHeadroomOnce verifies the rendered env entry and
// that the parallel headroom is applied exactly once, on top of the derived
// window -- the same order await_stderr.go uses.
//
// VALIDATES: serial runs keep the authored window; concurrent runs widen it by
// ParallelTimeoutHeadroom, not by its square.
// PREVENTS: double-scaling (budget already widened, then widened again), which
// is the bug the runOrchestrated comment at the testBudget split warns about.
func TestPluginStageStallEnvAppliesHeadroomOnce(t *testing.T) {
	const budget = 15 * time.Second
	derived := pluginStageStall(budget) // 12s

	serial := (&Runner{concurrency: 1}).pluginStageStallEnv(budget)
	require.Equal(t, "ze_plugin_stage_timeout="+derived.String(), serial,
		"a serial run must hand over the derived window unscaled")

	parallel := (&Runner{concurrency: 4}).pluginStageStallEnv(budget)
	want := derived * time.Duration(ParallelTimeoutHeadroom)
	require.Equal(t, "ze_plugin_stage_timeout="+want.String(), parallel,
		"a concurrent run widens the derived window exactly once")

	// The env key must match what the product reads (ze.plugin.stage.timeout,
	// registered in internal/component/plugin/server/server.go). The runner spells
	// it with underscores, which internal/core/env resolves case- and
	// separator-insensitively.
	assert.Contains(t, serial, "ze_plugin_stage_timeout=")
}
