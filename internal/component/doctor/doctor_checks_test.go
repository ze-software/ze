// Design: docs/features/ai-first.md -- the doctor component's own check registrations

package doctor

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
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

// doctorCodesFor runs `ze doctor --json` over one config and answers the codes
// it printed, whatever the exit status: the checks under test here are advisory
// beside checks that are not, and the exit status belongs to those.
func doctorCodesFor(t *testing.T, cfg string) []string {
	t.Helper()
	cfgPath := writeTestConfig(t, cfg)
	out := captureStdout(t, func() { Run([]string{"--json", cfgPath}) })

	var result diagnostic.DoctorResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("ze doctor --json printed something json cannot read: %v\n%s", err, out)
	}
	codes := make([]string, 0, len(result.Diagnostics))
	for i := range result.Diagnostics {
		codes = append(codes, result.Diagnostics[i].Code)
	}
	return codes
}

// TestDoctorPlatformCheckFunctional drives the platform judgement through the
// real entry point, on a platform detection forced to answer "unknown".
//
// VALIDATES: `ze doctor --json` prints doctor-platform-unknown, so the
// registered platform check is what the runner reaches, after the resolver
// gave it a platform to judge.
// PREVENTS: the split from the resolver leaving the judgement declared, unit
// tested, and never run.
func TestDoctorPlatformCheckFunctional(t *testing.T) {
	if err := env.Set(doctorPlatformEnv, "unknown"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = env.Set(doctorPlatformEnv, "") })

	codes := doctorCodesFor(t, minimalConfig)
	if !slices.Contains(codes, diagnostic.CodeDoctorPlatformUnknown) {
		t.Fatalf("codes = %v, want %q among them", codes, diagnostic.CodeDoctorPlatformUnknown)
	}
}

// TestDoctorListenersCheckFunctional drives the listener probe through the
// real entry point, with the probe forced to fail for the schema listener code.
//
// VALIDATES: `ze doctor --json` prints doctor-listen-unavailable for the SSH
// default endpoint, so the registered listeners check is what the runner
// reaches.
// PREVENTS: a listener check declared in the table and never run.
func TestDoctorListenersCheckFunctional(t *testing.T) {
	if err := env.Set(doctorListenerFailEnv, diagnostic.CodeDoctorListenUnavailable); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = env.Set(doctorListenerFailEnv, "") })

	codes := doctorCodesFor(t, "environment {\n\tssh {\n\t\tenabled true\n\t}\n}\n")
	if !slices.Contains(codes, diagnostic.CodeDoctorListenUnavailable) {
		t.Fatalf("codes = %v, want %q among them", codes, diagnostic.CodeDoctorListenUnavailable)
	}
}

// TestDoctorSemanticsCheckFunctional drives the config semantics check that
// internal/component/config registers through the real entry point.
//
// VALIDATES: `ze doctor --json` prints config-mcp-invalid for an MCP block
// that fails its own consistency check, so the check another component owns
// is what the runner reaches, under the code that component emits.
// PREVENTS: the semantic bridge going silent once its hand-written call left
// the runner.
func TestDoctorSemanticsCheckFunctional(t *testing.T) {
	const cfg = `
environment {
	mcp {
		enabled true
		auth-mode oauth
		server default {
			ip 127.0.0.1
			port 6274
		}
	}
}
`
	codes := doctorCodesFor(t, cfg)
	if !slices.Contains(codes, diagnostic.CodeConfigMCPInvalid) {
		t.Fatalf("codes = %v, want %q among them", codes, diagnostic.CodeConfigMCPInvalid)
	}
}
