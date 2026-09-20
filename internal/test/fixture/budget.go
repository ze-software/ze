// Design: docs/architecture/testing/ci-format.md -- compiled functional-test observers
// Related: internal/test/runner/parallel.go -- ChildTestBudget, the value this file divides up

package fixture

import (
	"time"

	"github.com/ze-software/ze/internal/test/runner"
)

// WaitBudget answers how long ONE wait inside a fixture may run, as a
// percentage of the wall clock the functional runner gave the whole test.
//
// Every wall-clock deadline a fixture enforces comes from here, and the reason
// is that a fixture stating its own is stating a fact the .ci already declared.
// The two then disagree in both directions, and both were live in this tree:
// storageCommand held 25s inside cases declaring `option=timeout:value=90s`, so
// the authored budget never decided anything, while run_rs_observer held a
// constant LARGER than the .ci timeout, so its diagnosis was a branch that
// could not run.
//
// The value the runner publishes is already contention-derived: it passes
// through (*Runner).withParallelHeadroom, the single place a run's concurrency
// enters any deadline. So a wait sized from here scales with the load the run
// is under, and it cannot outlive the budget the runner will kill this test at.
// That bound is the point. A fixture that waits past the kill holds its daemon,
// its peers and its ports for no gain, which is the shape measured harmful on
// 2026-09-19 (plan/pre-release/spec-a-test-passes-at-any-concurrency.md).
//
// percent is what this one wait may take of the whole test, and it is under 100
// so the fixture's own message, which names what it was waiting for, is the one
// the operator reads rather than the runner's bare timeout kill.
//
// fallback is returned when no runner published a budget, which happens when a
// developer runs the fixture binary by hand. It is not a second declaration of
// the budget: nothing authored it, so there is nothing to derive from.
func WaitBudget(percent int, fallback time.Duration) time.Duration {
	if percent < 1 || percent > 100 {
		panic("BUG: WaitBudget percent must be 1..100")
	}
	budget := runner.ChildTestBudget()
	if budget <= 0 {
		return fallback
	}
	return budget * time.Duration(percent) / 100
}

// WaitAttempts answers what WaitBudget answers, in the unit Poll counts:
// attempts at one every delay.
//
// A fixed attempt count is a deadline wearing another name, so it derives from
// the same place. fallback is the count to use when no runner published a
// budget, and the result is at least one attempt because a poll that runs zero
// times reports "not ready" without ever having looked.
func WaitAttempts(percent int, delay time.Duration, fallback int) int {
	if delay <= 0 {
		panic("BUG: WaitAttempts delay must be positive")
	}
	budget := WaitBudget(percent, 0)
	if budget <= 0 {
		return max(1, fallback)
	}
	return max(1, int(budget/delay))
}
