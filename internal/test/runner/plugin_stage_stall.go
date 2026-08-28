// Design: docs/architecture/testing/ci-format.md -- plugin startup stall watchdog budget
// Related: await_stderr.go -- the same derive-from-test-budget shape for the await fence
// Related: runner_exec_util.go -- withParallelHeadroom, applied on top of this value

package runner

import (
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// pluginStageStallFloor is the smallest stall window the harness hands a spawned
// daemon. It is the value both call sites hardcoded before this file existed, so
// deriving can only ever widen the window, never narrow it below what tests got.
//
// It sits above the product default (5s, plugin/server.defaultStageTimeout)
// because a test host runs many daemons at once.
const pluginStageStallFloor = 10 * time.Second

// pluginStageStallShare is the fraction of the test's own budget the stall
// watchdog may consume. Below 1 on purpose, and the reason is ORDERING, not
// machine speed.
//
// The watchdog's only job inside a test run is to convert a wedged plugin into a
// precise message ("no plugin progress for Xs while waiting for stage 3", with
// the stalled plugin named) instead of an opaque test-level timeout. That works
// only if it expires FIRST. The remaining share is the margin for that error to
// propagate and the daemon to exit before the outer budget kills it.
const pluginStageStallShare = 0.8

// pluginStageStall derives the plugin-startup stall window from the test budget
// the runner actually resolved, floored at pluginStageStallFloor and clamped
// back down to the budget.
//
// Both call sites used to pass a bare `ze_plugin_stage_timeout=10s` commented
// "Allow more time for plugin stage barriers under concurrent test load". That
// constant was sized against one machine, and it is the wrong axis: a bigger
// constant just moves the machine speed at which it lies.
//
// What the value bounds is NOT how long a stage may take. plugin.
// StartupCoordinator.WaitForStageProgress already waits on the CONDITION -- it
// blocks on progress events and restarts the window every time any plugin
// completes a stage (noteProgressLocked), so a 20-plugin tier that is merely
// slow never trips it. The value bounds only how long the whole tier may go with
// ZERO progress before the tier is declared wedged.
//
// So machine speed must enter through the test's budget, which is where an
// author already expresses "this environment is slow" -- the ospfv3 netns tests
// declare timeout=15s for work that costs 1.6s natively. Under QEMU emulation a
// single plugin's stage-3 work can exceed a fixed 10s of wall time between two
// progress events, and `./le qemu netns-test` runs `-p 1`, so
// withParallelHeadroom is a no-op there and could not compensate. Deriving keeps
// the intended ordering at any budget on any machine instead of at one specific
// pairing.
//
// The floor is clamped BACK DOWN to the budget for the same reason the await
// fence clamps its own: applying a 10s floor to a test that declared a 5s budget
// inverts the ordering this function exists to preserve, because the test-level
// timeout would then expire first and the watchdog's precise message would never
// be the one reported.
//
// Known and accepted: for a budget at or below the floor the clamp yields a
// window EQUAL to the budget, so the watchdog and the outer timeout race instead
// of ordering strictly. Dropping the floor (window = budget*share throughout)
// would order strictly at every budget, at the cost of narrowing the window
// below the old 10s for budgets in (floor, floor/share) = (10s, 12.5s). That is
// the same trade defaultAwaitStderrTimeout already made in this tree, and the
// affected band is one where a single zero-progress gap already consumes ~80% of
// the whole test budget -- such a test is failing either way. Kept identical to
// the await fence so the two derived gates stay one shape, not two.
func pluginStageStall(testBudget time.Duration) time.Duration {
	if testBudget <= 0 {
		return pluginStageStallFloor
	}
	derived := max(time.Duration(float64(testBudget)*pluginStageStallShare), pluginStageStallFloor)
	return min(derived, testBudget)
}

// pluginStageStallEnv renders the `ze_plugin_stage_timeout=<dur>` entry handed to
// a spawned daemon, deriving the window from the authored test budget and then
// applying the parallel headroom -- the same order await_stderr.go uses, so a
// concurrent run widens it exactly once rather than squaring it.
func (r *Runner) pluginStageStallEnv(testBudget time.Duration) string {
	var tb textbuf.Buffer
	return tb.Str("ze_plugin_stage_timeout=").
		Str(r.withParallelHeadroom(pluginStageStall(testBudget)).String()).
		String()
}
