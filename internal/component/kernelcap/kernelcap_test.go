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

// VALIDATES: a degrading capability that cannot be determined warns with its
// unknown code and names what is lost without it, not a subsystem failure.
// PREVENTS: an unprivileged `ze doctor` telling an RSVP-TE operator the subsystem
// may not work, when only the transit MTU bound is in question.
func TestDegradingCapabilityUndeterminedNamesTheLoss(t *testing.T) {
	capability := capabilityFor("alpha", StateUnknown, errors.New("operation not permitted"))
	capability.Degrades = "alpha frames are not bounded"
	withEnrolment(t, capability)

	diags := Evaluate(config.NewTree())
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != "doctor-alpha-unknown" {
		t.Errorf("code is %q, want doctor-alpha-unknown", diags[0].Code)
	}
	if !strings.Contains(diags[0].Message, "without it alpha frames are not bounded") {
		t.Errorf("the warning does not name the loss: %s", diags[0].Message)
	}
	if strings.Contains(diags[0].Message, "may not work") {
		t.Errorf("the warning claims a degrading subsystem may not work: %s", diags[0].Message)
	}
	if err := Refuse(config.NewTree()); err != nil {
		t.Errorf("cannot-determine refused a start: %v", err)
	}
}

// VALIDATES: a capability whose absence only DEGRADES its subsystem warns with
// its absent code, names what is lost, and never refuses a start.
// PREVENTS: a stock kernel without Ze's CONFIG_MPLS_IP_MTU patch stopping an
// RSVP-TE router that forwards correctly and only cannot enforce a transit MTU
// (owner decision, 2026-10-08), and the loss going unreported.
func TestDegradingCapabilityAbsentWarnsAndStarts(t *testing.T) {
	capability := capabilityFor("alpha", StateAbsent, errors.New("unknown attribute"))
	capability.Degrades = "alpha frames are not bounded"
	withEnrolment(t, capability)

	diags := Evaluate(config.NewTree())
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diags), diags)
	}
	if diags[0].Severity != diagnostic.SeverityWarning {
		t.Errorf("severity is %q, want warning", diags[0].Severity)
	}
	if diags[0].Code != "doctor-alpha-unavailable" {
		t.Errorf("code is %q, want doctor-alpha-unavailable", diags[0].Code)
	}
	for _, want := range []string{"CONFIG_ALPHA", "alpha block", "unknown attribute", "alpha frames are not bounded"} {
		if !strings.Contains(diags[0].Message, want) {
			t.Errorf("the warning does not name %q: %s", want, diags[0].Message)
		}
	}
	if err := Refuse(config.NewTree()); err != nil {
		t.Errorf("a degrading capability refused a start: %v", err)
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

// VALIDATES: docker-hosts spec AC-1. ProbeAll probes EVERY enrolment, whatever
// the configuration, and answers one row per enrolled subsystem; a probe that
// gave no verdict reads unknown, never present.
// PREVENTS: a Docker host passing because no config asked for a feature, and a
// row set that drifts from the enrolment.
// Method: enroll capabilities no configuration uses and read the rows back.
func TestDoctorAllCapabilitiesIgnoresConfig(t *testing.T) {
	absentCause := errors.New("no such family")
	present := capabilityFor("alpha", StatePresent, nil)
	absent := capabilityFor("bravo", StateAbsent, absentCause)
	silent := capabilityFor("charlie", StateUnspecified, nil)
	for _, capability := range []*Capability{&present, &absent, &silent} {
		capability.InUse = neverInUse
	}
	withEnrolment(t, silent, absent, present)

	rows := ProbeAll()
	enrolled := Enrolled()
	if len(rows) != len(enrolled) {
		t.Fatalf("%d rows for %d enrolled capabilities", len(rows), len(enrolled))
	}
	want := []Row{
		{Subsystem: "alpha", Kernel: "CONFIG_ALPHA", State: "present"},
		{Subsystem: "bravo", Kernel: "CONFIG_BRAVO", State: "absent", Reason: "no such family"},
		{Subsystem: "charlie", Kernel: "CONFIG_CHARLIE", State: "unknown", Reason: errProbeNoVerdict.Error()},
	}
	for i := range want {
		if rows[i].Subsystem != enrolled[i] {
			t.Errorf("row %d is %s, enrolment order says %s", i, rows[i].Subsystem, enrolled[i])
		}
		if rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}
}

// withForcedAnswers forces probe answers for the duration of a test, through the
// Go seam rather than the variable: a unit test binary is not a zetest build, so
// the variable is not read here (probe_force_shipped.go).
func withForcedAnswers(t *testing.T, value string) {
	t.Helper()
	t.Cleanup(ForceAnswersForTest(value))
}

// refusingProbe fails the test if the real probe is consulted, which is what a
// forced answer exists to prevent.
func refusingProbe(t *testing.T, subsystem string) func() Result {
	return func() Result {
		t.Errorf("the real %s probe was consulted under a forced answer", subsystem)
		return Result{State: StatePresent}
	}
}

// VALIDATES: a forced answer gives ANY enrolled capability the answer it names,
// through both readers (Evaluate for doctor, start and validate; ProbeAll for
// the Docker-host check), and the row and the diagnostic say the answer was
// injected, for a forced present as much as for a forced fault. A capability the variable does not name keeps its real probe.
// PREVENTS: a branch that a functional test can reach only on a host whose
// kernel happens to lack the feature, the vacuity trap of
// ai/rules/interop-and-goal-validation.md, for every enrolment rather than XFRM
// alone.
func TestForcedAnswerReplacesTheNamedProbe(t *testing.T) {
	for state, want := range map[string]State{
		"present": StatePresent,
		"absent":  StateAbsent,
		"unknown": StateUnknown,
	} {
		t.Run(state, func(t *testing.T) {
			forced := capabilityFor("alpha", StatePresent, nil)
			forced.Probe = refusingProbe(t, "alpha")
			unforced := capabilityFor("bravo", StateAbsent, errors.New("real bravo answer"))
			withEnrolment(t, forced, unforced)
			withForcedAnswers(t, "alpha="+state)

			rows := ProbeAll()
			if rows[0].Subsystem != "alpha" || rows[0].State != want.String() {
				t.Errorf("forced row = %+v, want alpha %v", rows[0], want)
			}
			if !strings.Contains(rows[0].Reason, forceEnv) {
				t.Errorf("forced row reason %q does not name %s", rows[0].Reason, forceEnv)
			}
			if rows[1].State != "absent" || rows[1].Reason != "real bravo answer" {
				t.Errorf("unforced row = %+v, want bravo's real answer", rows[1])
			}

			diags := Evaluate(config.NewTree())
			alphaCodes := 0
			for i := range diags {
				if !strings.HasPrefix(diags[i].Code, "doctor-alpha-") &&
					diags[i].Code != diagnostic.CodeDoctorKernelCapabilityForced {
					continue
				}
				alphaCodes++
				if !strings.Contains(diags[i].Message, forceEnv) {
					t.Errorf("forced diagnostic does not name %s: %s", forceEnv, diags[i].Message)
				}
			}
			// A forced present is still a forced answer: it warns under its
			// own code rather than passing in silence, so a forced verdict is
			// never read as the host's.
			if alphaCodes != 1 {
				t.Errorf("alpha produced %d diagnostics, want 1: %+v", alphaCodes, diags)
			}
			if want == StatePresent {
				assertForcedPresentDiagnostic(t, diags)
			}
		})
	}
}

// VALIDATES: the variable names several subsystems at once, and a misspelt
// state or an entry with no state leaves the real probe in charge.
// PREVENTS: a typo in a test variable deciding whether a daemon starts.
func TestForcedAnswerListAndMisspellings(t *testing.T) {
	alpha := capabilityFor("alpha", StatePresent, nil)
	alpha.Probe = refusingProbe(t, "alpha")
	bravo := capabilityFor("bravo", StatePresent, nil)
	bravo.Probe = refusingProbe(t, "bravo")
	charlie := capabilityFor("charlie", StatePresent, nil)
	delta := capabilityFor("delta", StatePresent, nil)
	withEnrolment(t, alpha, bravo, charlie, delta)
	withForcedAnswers(t, "alpha=absent, bravo=unknown,charlie=abcent,delta")

	want := []string{"absent", "unknown", "present", "present"}
	rows := ProbeAll()
	for i := range want {
		if rows[i].State != want[i] {
			t.Errorf("%s = %s, want %s", rows[i].Subsystem, rows[i].State, want[i])
		}
	}
}

// assertForcedPresentDiagnostic checks the one diagnostic a forced present
// produces: the forced code, at warning severity, so it never refuses a start.
func assertForcedPresentDiagnostic(t *testing.T, diags []diagnostic.Diagnostic) {
	t.Helper()
	for i := range diags {
		if diags[i].Code != diagnostic.CodeDoctorKernelCapabilityForced {
			continue
		}
		if diags[i].Severity != diagnostic.SeverityWarning {
			t.Errorf("forced present severity = %q, want warning", diags[i].Severity)
		}
		return
	}
	t.Errorf("a forced present produced no %s diagnostic: %+v", diagnostic.CodeDoctorKernelCapabilityForced, diags)
}
