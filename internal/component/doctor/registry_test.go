// Design: docs/features/ai-first.md -- doctor check registration tests
//
// The registry itself is internal/core/diagnostic, and its own tests cover
// ordering, duplicate names, duplicate codes and every validation refusal
// (internal/core/diagnostic/doctor_registry_test.go). What is left here is the
// question only a binary carrying every registration can answer: do the codes
// the registered checks declare resolve for an operator.

package doctor

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestDoctorRegisteredCheckCodesHaveMetadata reads the registry `ze doctor`
// reads, so it answers for every check this binary carries rather than for the
// component's own.
//
// A code is checked whatever namespace it belongs to. The registry accepts the
// codes a check really emits (internal/core/diagnostic/doctor_registry.go,
// DoctorCheck), so the semantic-validation bridge declares config-* codes, and
// those owe `ze show diagnostic` an answer exactly as a doctor-* code does.
//
// VALIDATES: every code a registered check declares resolves through
// diagnostic.Lookup.
// PREVENTS: ze doctor emitting a code ze show diagnostic cannot describe.
func TestDoctorRegisteredCheckCodesHaveMetadata(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	// DoctorChecks lists every phase, the runtime phase show doctor alone runs
	// included, so a phase added later is read here with no edit.
	seen := 0
	for _, check := range diagnostic.DoctorChecks() {
		seen++
		if len(check.Codes) == 0 {
			t.Errorf("check %q declares no diagnostic code", check.Name)
			continue
		}
		for _, code := range check.Codes {
			if diagnostic.Lookup(code) == nil {
				t.Errorf("check %q declares %q without diagnostic metadata", check.Name, code)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no doctor check is registered: this test passed over an empty registry")
	}
}

// TestDoctorRuntimePhaseRunsOnlyThroughTheRuntimeRunner registers a runtime
// check and a post-config check, then runs each phase's runner.
//
// VALIDATES: RunRuntimeChecks runs the runtime check with its fix carried,
// and the local route's runner (runChecks) does not run it.
// PREVENTS: the local ze doctor process running a runtime check over empty
// state and reading it as healthy.
func TestDoctorRuntimePhaseRunsOnlyThroughTheRuntimeRunner(t *testing.T) {
	const name = "test-runtime-only"
	ran := 0
	check := diagnostic.DoctorCheck{
		Name: name, Phase: diagnostic.DoctorPhaseRuntime, Component: "test",
		Dependencies: []string{"x"}, Platforms: []string{diagnostic.DoctorPlatformAny},
		Codes: []string{"doctor-test-runtime-only"},
		Check: func(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
			ran++
			return []diagnostic.Diagnostic{{
				Code: "doctor-test-runtime-only", Severity: diagnostic.SeverityWarning,
				Message: "runtime state is degraded", Fix: "show test runtime",
			}}
		},
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { diagnostic.UnregisterDoctorCheckForTest(name) })

	// The local route's own runner, over a config file that does not exist,
	// as ze doctor runs it with no daemon.
	for _, d := range runChecks(filepath.Join(t.TempDir(), "absent.conf")) {
		if d.Code == "doctor-test-runtime-only" {
			t.Errorf("the local route answered the runtime check: %+v", d)
		}
	}
	if ran != 0 {
		t.Fatalf("the local route ran the runtime check %d times", ran)
	}

	var got *diagnostic.Diagnostic
	diags := RunRuntimeChecks()
	for i := range diags {
		if diags[i].Code == "doctor-test-runtime-only" {
			got = &diags[i]
		}
	}
	if got == nil {
		t.Fatalf("RunRuntimeChecks did not run the runtime check: %+v", diags)
	}
	if got.Fix != "show test runtime" {
		t.Errorf("fix = %q, want %q", got.Fix, "show test runtime")
	}
}
