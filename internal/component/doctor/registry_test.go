// Design: docs/features/ai-first.md -- doctor check registration tests
//
// The registry itself is internal/core/diagnostic, and its own tests cover
// ordering, duplicate names, duplicate codes and every validation refusal
// (internal/core/diagnostic/doctor_registry_test.go). What is left here is the
// question only a binary carrying every registration can answer: do the codes
// the registered checks declare resolve for an operator.

package doctor

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestDoctorRegisteredCheckCodesHaveMetadata reads the registry `ze doctor`
// reads, so it answers for every check this binary carries rather than for the
// component's own.
//
// VALIDATES: every registered doctor-* code resolves through diagnostic.Lookup.
// PREVENTS: ze doctor emitting a code ze explain cannot describe.
func TestDoctorRegisteredCheckCodesHaveMetadata(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	phases := []diagnostic.DoctorCheckPhase{
		diagnostic.DoctorPhasePreConfig,
		diagnostic.DoctorPhaseMissingConfig,
		diagnostic.DoctorPhasePostConfig,
	}
	seen := 0
	for _, phase := range phases {
		for _, check := range diagnostic.DoctorChecksForPhase(phase) {
			seen++
			if len(check.Codes) == 0 {
				t.Errorf("check %q declares no diagnostic code", check.Name)
				continue
			}
			for _, code := range check.Codes {
				if !strings.HasPrefix(code, "doctor-") {
					t.Errorf("check %q declares non-doctor diagnostic code %q", check.Name, code)
					continue
				}
				if diagnostic.Lookup(code) == nil {
					t.Errorf("check %q declares %q without diagnostic metadata", check.Name, code)
				}
			}
		}
	}
	if seen == 0 {
		t.Fatal("no doctor check is registered: this test passed over an empty registry")
	}
}
