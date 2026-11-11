// Design: docs/architecture/doctor-and-health-checks.md -- event codes the recent-events check declares
// Related: event_codes.go -- the registration under test

package diagnostic

import (
	"slices"
	"testing"
)

// TestRegisterEventCodeRefusesUndescribed registers one code whose metadata
// sits in builtinCodes (not yet registered, as in init()), one with no
// metadata anywhere, and one that is not a kebab-case code.
//
// VALIDATES: AC-2's survivor. A code with no `ze show diagnostic` entry is
// refused at registration, and a code builtinCodes describes is accepted
// before RegisterBuiltinCodes has run, because owners register from init().
// PREVENTS: an event code declared to doctor that the operator cannot look
// up, and a registration that fails only because init() ran first.
func TestRegisterEventCodeRefusesUndescribed(t *testing.T) {
	const undescribed = "doctor-test-event-undescribed"
	t.Cleanup(func() { UnregisterEventCodeForTest(CodeDoctorPluginCrashed) })

	if err := RegisterEventCode(EventCode{Code: undescribed}); err == nil {
		t.Errorf("RegisterEventCode accepted %s, which no metadata describes", undescribed)
	}
	if slices.Contains(EventCodes(), undescribed) {
		t.Errorf("refused code %s is listed by EventCodes", undescribed)
	}
	if err := RegisterEventCode(EventCode{Code: "Not A Code"}); err == nil {
		t.Error("RegisterEventCode accepted a code that is not lower kebab case")
	}

	if err := RegisterEventCode(EventCode{Code: CodeDoctorPluginCrashed}); err != nil {
		t.Fatalf("RegisterEventCode refused %s, which builtinCodes describes: %v", CodeDoctorPluginCrashed, err)
	}
	// A cross-cutting code has several owners, so a second registration of
	// one code is accepted and lists it once.
	if err := RegisterEventCode(EventCode{Code: CodeDoctorPluginCrashed}); err != nil {
		t.Fatalf("a second owner's registration of %s was refused: %v", CodeDoctorPluginCrashed, err)
	}
	listed := 0
	for _, code := range EventCodes() {
		if code == CodeDoctorPluginCrashed {
			listed++
		}
	}
	if listed != 1 {
		t.Errorf("EventCodes lists %s %d times, want once", CodeDoctorPluginCrashed, listed)
	}
}

// TestDoctorChecksAddEventCodes registers a check that declares the event
// codes and one that does not, then lists the registry.
//
// VALIDATES: AC-2. DoctorChecks and DoctorChecksForPhase answer every
// registered event code in the Codes of a check whose DeclaresEventCodes is
// set, after its own codes, and leave every other check's Codes as written.
// PREVENTS: an event code that the gate's linked side and ze doctor never see
// as declared.
func TestDoctorChecksAddEventCodes(t *testing.T) {
	const declaring, plain = "test-event-declarer", "test-event-plain"
	noop := func(DoctorCheckContext) []Diagnostic { return nil }
	for _, check := range []DoctorCheck{
		{Name: declaring, Phase: DoctorPhaseRuntime, Component: "test", Dependencies: []string{"report-bus"}, Platforms: []string{DoctorPlatformAny}, Codes: []string{CodeDoctorRecentEvents}, DeclaresEventCodes: true, Check: noop},
		{Name: plain, Phase: DoctorPhaseRuntime, Component: "test", Dependencies: []string{"report-bus"}, Platforms: []string{DoctorPlatformAny}, Codes: []string{CodeDoctorDiskSpace}, Check: noop},
	} {
		if err := RegisterDoctorCheck(check); err != nil {
			t.Fatalf("register %s: %v", check.Name, err)
		}
		t.Cleanup(func() { UnregisterDoctorCheckForTest(check.Name) })
	}
	if err := RegisterEventCode(EventCode{Code: CodeDoctorPluginStopped}); err != nil {
		t.Fatalf("register event code: %v", err)
	}
	t.Cleanup(func() { UnregisterEventCodeForTest(CodeDoctorPluginStopped) })

	judge := func(route string, checks []DoctorCheck) {
		t.Helper()
		found := 0
		for _, check := range checks {
			switch check.Name {
			case declaring:
				found++
				if len(check.Codes) == 0 || check.Codes[0] != CodeDoctorRecentEvents {
					t.Errorf("%s: declaring check lost its own code first: %v", route, check.Codes)
				}
				if !slices.Contains(check.Codes, CodeDoctorPluginStopped) {
					t.Errorf("%s: declaring check codes %v miss event code %s", route, check.Codes, CodeDoctorPluginStopped)
				}
			case plain:
				found++
				if !slices.Equal(check.Codes, []string{CodeDoctorDiskSpace}) {
					t.Errorf("%s: plain check codes %v, want only its own", route, check.Codes)
				}
			}
		}
		if found != 2 {
			t.Errorf("%s: listed %d of the two fixture checks", route, found)
		}
	}
	judge("DoctorChecks", DoctorChecks())
	judge("DoctorChecksForPhase", DoctorChecksForPhase(DoctorPhaseRuntime))
}
