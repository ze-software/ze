// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
//
// The enrolment's own behavior, driven with capabilities this test registers, so
// it holds whatever the host's kernel is and whichever subsystems ship.

package kernelcap

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// withEnrolment replaces the enrolment for the duration of a test. Nothing else
// enrolls in this package's own test binary, because the owners that register the
// shipped capabilities are not imported here.
func withEnrolment(t *testing.T, capabilities ...Capability) {
	t.Helper()
	ResetForTest()
	t.Cleanup(ResetForTest)
	for _, capability := range capabilities {
		enrolForTest(capability)
	}
}

func probing(state State, reason error) func() Result {
	return func() Result { return Result{State: state, Reason: reason} }
}

func alwaysInUse(*config.Tree) bool { return true }

func neverInUse(*config.Tree) bool { return false }

func capabilityFor(subsystem string, state State, reason error) Capability {
	return Capability{
		Subsystem:   subsystem,
		Component:   subsystem,
		Kernel:      "CONFIG_" + strings.ToUpper(subsystem),
		ConfigLeaf:  subsystem + " block",
		CodeAbsent:  "doctor-" + subsystem + "-unavailable",
		CodeUnknown: "doctor-" + subsystem + "-unknown",
		InUse:       alwaysInUse,
		Probe:       probing(state, reason),
	}
}

// VALIDATES: AC-2 and AC-3. An absent capability is an ERROR naming the
// subsystem, the kernel feature and the configuration that asked, and Refuse
// turns it into the daemon's refusal.
// PREVENTS: a daemon that starts and does not work, and a refusal an operator
// cannot act on.
func TestAbsentCapabilityRefusesAndNamesTheCause(t *testing.T) {
	withEnrolment(t, capabilityFor("alpha", StateAbsent, errors.New("protocol not supported")))

	diags := Evaluate(config.NewTree())
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diags), diags)
	}
	if diags[0].Severity != diagnostic.SeverityError {
		t.Errorf("severity is %q, want error", diags[0].Severity)
	}
	if diags[0].Code != "doctor-alpha-unavailable" {
		t.Errorf("code is %q, want doctor-alpha-unavailable", diags[0].Code)
	}
	for _, want := range []string{"alpha", "CONFIG_ALPHA", "alpha block", "protocol not supported"} {
		if !strings.Contains(diags[0].Message, want) {
			t.Errorf("the message does not name %q: %s", want, diags[0].Message)
		}
	}

	err := Refuse(config.NewTree())
	if err == nil {
		t.Fatal("an absent capability did not refuse")
	}
	if !strings.Contains(err.Error(), "CONFIG_ALPHA") {
		t.Errorf("the refusal does not name the kernel feature: %v", err)
	}
}

// VALIDATES: the refusal names EVERY failing subsystem, not the first.
// PREVENTS: an operator who repairs one fault and restarts to meet the next
// paying a whole boot for each fault after the first. The sibling plugin setup
// gate names them all for the same reason (cmd/ze/hub/startup_gate.go).
func TestRefusalNamesEveryFailingSubsystem(t *testing.T) {
	withEnrolment(t,
		capabilityFor("alpha", StateAbsent, errors.New("protocol not supported")),
		capabilityFor("bravo", StateAbsent, errors.New("no such file")),
		capabilityFor("charlie", StatePresent, nil),
	)

	err := Refuse(config.NewTree())
	if err == nil {
		t.Fatal("two absent capabilities did not refuse")
	}
	for _, want := range []string{"alpha", "bravo", "CONFIG_ALPHA", "CONFIG_BRAVO"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "charlie") {
		t.Errorf("the refusal names a capability that is present: %v", err)
	}
}

// VALIDATES: AC-6. A capability that cannot be DETERMINED warns and never
// refuses, and the warning carries the reason the probe gave.
// PREVENTS: a working deployment turned into a dead one by an unreadable probe
// (R-1). The gate runs on every ze start on Linux, so this is the failure mode
// with the widest blast radius.
func TestUndeterminedCapabilityWarnsAndStarts(t *testing.T) {
	withEnrolment(t, capabilityFor("alpha", StateUnknown, errors.New("operation not permitted")))

	diags := Evaluate(config.NewTree())
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diags), diags)
	}
	if diags[0].Severity != diagnostic.SeverityWarning {
		t.Errorf("severity is %q, want warning", diags[0].Severity)
	}
	if diags[0].Code != "doctor-alpha-unknown" {
		t.Errorf("code is %q, want doctor-alpha-unknown", diags[0].Code)
	}
	if !strings.Contains(diags[0].Message, "operation not permitted") {
		t.Errorf("the warning does not name the reason: %s", diags[0].Message)
	}
	if err := Refuse(config.NewTree()); err != nil {
		t.Errorf("cannot-determine refused a start: %v", err)
	}
}

// VALIDATES: a probe that returns no verdict is reported as cannot-determine and
// carries a reason saying so. The zero State is never read as a pass.
// PREVENTS: the sharpest failure this repository records: a zero value that
// behaves correctly. A Result nobody wrote would otherwise mean "present", so a
// probe that forgot a branch would silently authorize every start
// (ai/rules/principles.md).
func TestProbeWithNoVerdictIsNeverAPass(t *testing.T) {
	withEnrolment(t, capabilityFor("alpha", StateUnspecified, nil))

	diags := Evaluate(config.NewTree())
	if len(diags) != 1 {
		t.Fatalf("a verdictless probe produced %d diagnostics, want 1: %+v", len(diags), diags)
	}
	if diags[0].Severity != diagnostic.SeverityWarning {
		t.Errorf("severity is %q, want warning", diags[0].Severity)
	}
	if !strings.Contains(diags[0].Message, "no verdict") {
		t.Errorf("the warning does not say the probe gave no verdict: %s", diags[0].Message)
	}
}

// VALIDATES: AC-5. A configuration that does not use the subsystem is never
// probed and never reported, whatever the host lacks.
// PREVENTS: an absent feature nobody asked for reading as a fault, which under a
// refusal stops a router that was working (R-3).
func TestUnusedSubsystemIsNeverProbed(t *testing.T) {
	probed := false
	capability := capabilityFor("alpha", StateAbsent, errors.New("protocol not supported"))
	capability.InUse = neverInUse
	capability.Probe = func() Result {
		probed = true
		return Result{State: StateAbsent}
	}
	withEnrolment(t, capability)

	if diags := Evaluate(config.NewTree()); len(diags) != 0 {
		t.Errorf("an unused subsystem produced %d diagnostics: %+v", len(diags), diags)
	}
	if probed {
		t.Error("an unused subsystem was probed; a false predicate must end the evaluation")
	}
	if err := Refuse(config.NewTree()); err != nil {
		t.Errorf("an unused subsystem refused a start: %v", err)
	}
}

// VALIDATES: a nil tree gates nothing. `ze doctor` reaches the runner before a
// config is parsed, and a nil tree there must not be read as a config that uses
// every subsystem.
// PREVENTS: a refusal produced by the absence of a configuration.
func TestNilTreeGatesNothing(t *testing.T) {
	withEnrolment(t, capabilityFor("alpha", StateAbsent, errors.New("protocol not supported")))

	if diags := Evaluate(nil); len(diags) != 0 {
		t.Errorf("a nil tree produced %d diagnostics: %+v", len(diags), diags)
	}
	if err := Refuse(nil); err != nil {
		t.Errorf("a nil tree refused a start: %v", err)
	}
}

// VALIDATES: an incomplete enrolment is refused at registration. Every field the
// message needs is present before the capability can decide a start.
// PREVENTS: a refusal whose message names no subsystem, no kernel feature and no
// configuration, which an operator cannot act on.
func TestIncompleteEnrolmentPanics(t *testing.T) {
	for name, capability := range map[string]Capability{
		"no subsystem":   {Component: "a", Kernel: "K", ConfigLeaf: "l", CodeAbsent: "doctor-a", CodeUnknown: "doctor-b", InUse: alwaysInUse, Probe: probing(StatePresent, nil)},
		"no kernel":      {Subsystem: "a", Component: "a", ConfigLeaf: "l", CodeAbsent: "doctor-a", CodeUnknown: "doctor-b", InUse: alwaysInUse, Probe: probing(StatePresent, nil)},
		"no predicate":   {Subsystem: "a", Component: "a", Kernel: "K", ConfigLeaf: "l", CodeAbsent: "doctor-a", CodeUnknown: "doctor-b", Probe: probing(StatePresent, nil)},
		"no probe":       {Subsystem: "a", Component: "a", Kernel: "K", ConfigLeaf: "l", CodeAbsent: "doctor-a", CodeUnknown: "doctor-b", InUse: alwaysInUse},
		"no codes":       {Subsystem: "a", Component: "a", Kernel: "K", ConfigLeaf: "l", InUse: alwaysInUse, Probe: probing(StatePresent, nil)},
		"no config leaf": {Subsystem: "a", Component: "a", Kernel: "K", CodeAbsent: "doctor-a", CodeUnknown: "doctor-b", InUse: alwaysInUse, Probe: probing(StatePresent, nil)},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validate(capability); err == nil {
				t.Error("an incomplete enrolment was accepted")
			}
		})
	}
}
