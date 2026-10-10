//go:build zetest

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier

package kernelcap

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// VALIDATES: a zetest build, the functional-test DUT, reads
// ze.test.kernelcap.force and names it in the forced row.
// PREVENTS: the functional tests that force a verdict (test/ui/doctor-*-kernelcap.ci,
// test/plugin/kernel-capability-*.ci) silently testing the host's kernel.
func TestZetestBuildReadsTheForceVariable(t *testing.T) {
	t.Setenv("ze_test_kernelcap_force", "alpha=absent")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	withEnrolment(t, capabilityFor("alpha", StatePresent, nil))

	rows := ProbeAll()
	if len(rows) != 1 || rows[0].State != "absent" || !strings.Contains(rows[0].Reason, forceEnv) {
		t.Errorf("rows = %+v, want alpha forced absent naming %s", rows, forceEnv)
	}
}

// VALIDATES: the force variable's registered description names every state
// forcedState accepts, and forcedState accepts every state a probe can answer.
// PREVENTS: the grammar a test author reads in `ze env` and the grammar the
// parser takes drifting apart, as when denied was accepted and not described.
func TestForceGrammarNamesEveryForcibleState(t *testing.T) {
	var description string
	for _, entry := range env.AllEntries() {
		if entry.Key == forceEnv {
			description = entry.Description
		}
	}
	if description == "" {
		t.Fatalf("%s is not registered in a zetest build", forceEnv)
	}
	for _, state := range namedStates(t) {
		if !strings.Contains(description, state.String()) {
			t.Errorf("description %q does not name %s", description, state)
		}
		result, ok := forcedState(state.String())
		if !ok || result.State != state {
			t.Errorf("forcedState(%q) = %+v, %v; want %s", state.String(), result, ok, state)
		}
	}
}

// namedStates answers every State after StateUnspecified that String names,
// found by asking String rather than by naming the last state, so a state
// added later is checked with no edit here. String panics on an unnamed
// value, which is where the walk stops.
func namedStates(t *testing.T) []State {
	t.Helper()
	var states []State
	for state := StatePresent; ; state++ {
		if !stateIsNamed(state) {
			break
		}
		states = append(states, state)
	}
	if len(states) == 0 {
		t.Fatal("String names no state after StateUnspecified")
	}
	return states
}

// stateIsNamed reports whether String names state, recovering its panic for
// an unnamed value.
func stateIsNamed(state State) (named bool) {
	defer func() {
		if recover() != nil {
			named = false
		}
	}()
	return state.String() != ""
}
