// Design: docs/features/ai-first.md -- the doctor component's own check registrations

package doctor

import (
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestDoctorOwnedChecksReachTheRunner asks the registry the runner reads
// whether it holds every check this component declares.
//
// It is the wiring test for the move off hand-written calls: the table in
// doctor_checks.go is the declaration, the registry is what runChecks consults,
// and nothing else connects them but the init() in register.go. A check dropped
// from that init, or a registration the registry refused, leaves the check
// defined, tested by its own unit test, and never run.
//
// VALIDATES: every entry in doctorOwnedChecks is returned by the phase dispatch
// runChecks calls, at the phase and order it declares.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestDoctorOwnedChecksReachTheRunner(t *testing.T) {
	if len(doctorOwnedChecks) == 0 {
		t.Fatal("this component declares no check: the test below would pass over an empty table")
	}

	for i := range doctorOwnedChecks {
		want := doctorOwnedChecks[i]
		found := false
		for _, got := range diagnostic.DoctorChecksForPhase(want.Phase) {
			if got.Name != want.Name {
				continue
			}
			found = true
			if got.Order != want.Order {
				t.Errorf("check %q registered at order %d, declared %d", want.Name, got.Order, want.Order)
			}
			if got.Component != want.Component {
				t.Errorf("check %q registered for component %q, declared %q", want.Name, got.Component, want.Component)
			}
		}
		if !found {
			t.Errorf("check %q is declared for phase %q but the registry does not hold it", want.Name, want.Phase)
		}
	}
}

// TestRunChecksCallsNoDoctorOwnedCheckTwice drives the phase dispatch and
// counts the checks it answers for, so a check registered under two names in
// one phase reports twice to the operator.
//
// VALIDATES: each phase answers each declared check name once.
// PREVENTS: a duplicated diagnostic after a check is registered beside a call
// that was never removed.
func TestRunChecksCallsNoDoctorOwnedCheckTwice(t *testing.T) {
	for _, phase := range []diagnostic.DoctorCheckPhase{
		diagnostic.DoctorPhasePreConfig,
		diagnostic.DoctorPhaseMissingConfig,
		diagnostic.DoctorPhasePostConfig,
	} {
		seen := map[string]int{}
		for _, got := range diagnostic.DoctorChecksForPhase(phase) {
			seen[got.Name]++
		}
		for name, count := range seen {
			if count > 1 {
				t.Errorf("phase %q holds check %q %d times", phase, name, count)
			}
		}
	}
}
