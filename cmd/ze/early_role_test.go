// Design: docs/architecture/system-architecture.md -- a role selected by an environment key before command dispatch

package main

import (
	"testing"

	"github.com/ze-software/ze/internal/core/earlyrole"
)

// earlyRoleTestKey is set by no program, so the developer's environment
// cannot select the role this file registers.
const earlyRoleTestKey = "ZE_TEST_EARLY_ROLE_DISPATCH_ORDER"

// TestEarlyRoleRunsBeforeCommandDispatch proves the wiring of the early-role
// leaf into the binary's entry point: with a registered key in the
// environment, the role runs, its result is the exit code, and neither the
// personality setup nor the command dispatch runs.
//
// Method: register a counting role, replace the personality's setup and
// dispatch hooks with counters, set the key, and call dispatchMain with a
// command line that would otherwise dispatch.
//
// MUTATION: remove the earlyrole.Dispatch call from dispatchMain. The role
// then never runs and both hooks do, so every assertion below goes red.
func TestEarlyRoleRunsBeforeCommandDispatch(t *testing.T) {
	t.Cleanup(earlyrole.Snapshot())
	savedSetup, savedDispatch := binarySetup, binaryDispatch
	t.Cleanup(func() { binarySetup, binaryDispatch = savedSetup, savedDispatch })

	setups, dispatches := 0, 0
	binarySetup = func(args []string) ([]string, int) {
		setups++
		return args, 0
	}
	binaryDispatch = func([]string) int {
		dispatches++
		return 0
	}

	const roleExitCode = 42
	roleRuns := 0
	earlyrole.Register(earlyRoleTestKey, func() int {
		roleRuns++
		return roleExitCode
	})
	t.Setenv(earlyRoleTestKey, "3")

	code := dispatchMain([]string{"show", "version"})

	if roleRuns != 1 {
		t.Fatalf("the early role ran %d times, want 1", roleRuns)
	}
	if code != roleExitCode {
		t.Errorf("dispatchMain answered exit code %d, want the role's %d", code, roleExitCode)
	}
	if setups != 0 {
		t.Errorf("the personality setup ran %d times under an early role, want 0", setups)
	}
	if dispatches != 0 {
		t.Errorf("the command dispatch ran %d times under an early role, want 0", dispatches)
	}
}
