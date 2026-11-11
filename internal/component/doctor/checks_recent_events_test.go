// Design: docs/architecture/doctor-and-health-checks.md -- recent events on the report bus
// Related: checks_recent_events.go -- the runtime check under test

package doctor

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// recentEventsFixtureSource is the report-bus source the fixtures raise under.
const recentEventsFixtureSource = "fixture"

// resetReportBus empties the process-wide report bus now and after the test,
// so no event raised here reaches another test of this binary.
func resetReportBus(t *testing.T) {
	t.Helper()
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
}

// rowsWithCode answers the diagnostics that carry code.
func rowsWithCode(diags []diagnostic.Diagnostic, code string) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	for i := range diags {
		if diags[i].Code == code {
			rows = append(rows, diags[i])
		}
	}
	return rows
}

// TestDoctorRecentEventsRowsFromReportRing raises two error events and one
// warning under doctor codes on the report bus, then runs the runtime phase
// as show doctor runs it.
//
// VALIDATES: AC-11. The recent-events check answers one warning row per
// (source, code, subject) under the event's own code, naming the subject, the
// count and the last time, with the show command that lists the events as its
// fix; the answer still reads ready.
// PREVENTS: a transient problem raised on the report bus that show doctor
// never reports, or one that turns the box not ready although it recovered.
func TestDoctorRecentEventsRowsFromReportRing(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	resetReportBus(t)

	// Readiness before any event: the other runtime checks this binary
	// carries answer for the host, so the test compares, never assumes ready.
	readyBefore := readyOf(RunRuntimeChecks())

	errorCode := diagnostic.CodeDoctorDiskSpace
	warningCode := diagnostic.CodeDoctorClockSkew
	report.RaiseError(recentEventsFixtureSource, errorCode, "peer-a", "first failure")
	report.RaiseError(recentEventsFixtureSource, errorCode, "peer-a", "second failure")
	report.RaiseWarning(recentEventsFixtureSource, warningCode, "clock", "clock drifts")

	diags := RunRuntimeChecks()
	if readyOf(diags) != readyBefore {
		t.Fatalf("recent events changed readiness from %v: %+v", readyBefore, diags)
	}

	errorRows := rowsWithCode(diags, errorCode)
	if len(errorRows) != 1 {
		t.Fatalf("want one row under %s for two events on one subject, got %d: %+v", errorCode, len(errorRows), errorRows)
	}
	last := report.Errors(1)[0].Raised.UTC().Format(recentEventsTimeLayout)
	row := errorRows[0]
	if row.Severity != diagnostic.SeverityWarning {
		t.Errorf("an error event is a warning row in doctor, got severity %q", row.Severity)
	}
	for _, want := range []string{recentEventsFixtureSource, "peer-a", "2 times", last, "second failure"} {
		if !strings.Contains(row.Message, want) {
			t.Errorf("row message %q does not name %q", row.Message, want)
		}
	}
	if row.Fix != recentEventsFixErrors {
		t.Errorf("error row fix: want %q, got %q", recentEventsFixErrors, row.Fix)
	}

	warningRows := rowsWithCode(diags, warningCode)
	if len(warningRows) != 1 {
		t.Fatalf("want one row under %s for the active warning, got %d: %+v", warningCode, len(warningRows), warningRows)
	}
	if !strings.Contains(warningRows[0].Message, "clock drifts") {
		t.Errorf("warning row message %q does not carry the warning's message", warningRows[0].Message)
	}
	if !strings.Contains(warningRows[0].Message, "1 time,") {
		t.Errorf("warning row message %q does not name its count", warningRows[0].Message)
	}
	if warningRows[0].Fix != recentEventsFixWarnings {
		t.Errorf("warning row fix: want %q, got %q", recentEventsFixWarnings, warningRows[0].Fix)
	}
}

// TestDoctorRecentEventsSubjectsAreSeparateRows raises the same code on two
// subjects and checks the check's own answer.
//
// VALIDATES: two subjects under one code are two rows, each counting only its
// own events.
// PREVENTS: one peer's resets hidden inside another peer's count.
func TestDoctorRecentEventsSubjectsAreSeparateRows(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	resetReportBus(t)

	code := diagnostic.CodeDoctorDiskSpace
	report.RaiseError(recentEventsFixtureSource, code, "peer-a", "reset")
	report.RaiseError(recentEventsFixtureSource, code, "peer-b", "reset")
	report.RaiseError(recentEventsFixtureSource, code, "peer-b", "reset")

	rows := rowsWithCode(checkRecentEvents(diagnostic.DoctorCheckContext{}), code)
	if len(rows) != 2 {
		t.Fatalf("want one row per subject, got %d: %+v", len(rows), rows)
	}
	// Newest first: peer-b raised last.
	if !strings.Contains(rows[0].Message, "peer-b") || !strings.Contains(rows[0].Message, "2 times") {
		t.Errorf("first row should be peer-b, twice: %q", rows[0].Message)
	}
	if !strings.Contains(rows[1].Message, "peer-a") || !strings.Contains(rows[1].Message, "1 time,") {
		t.Errorf("second row should be peer-a, once: %q", rows[1].Message)
	}
}

// TestDoctorRecentEventsCodeWithoutMetadata raises an event under a bus code
// no diagnostic metadata describes.
//
// VALIDATES: the event is still reported, under doctor-recent-events, and the
// message names the event's own code.
// PREVENTS: a doctor row under a code `ze show diagnostic` cannot describe,
// and an event dropped because its code is not a doctor code.
func TestDoctorRecentEventsCodeWithoutMetadata(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	resetReportBus(t)

	const busCode = "fixture-bus-only-code"
	if diagnostic.Lookup(busCode) != nil {
		t.Fatalf("fixture code %q is registered; pick a code with no metadata", busCode)
	}
	report.RaiseError(recentEventsFixtureSource, busCode, "worker", "crashed")

	diags := checkRecentEvents(diagnostic.DoctorCheckContext{})
	if got := rowsWithCode(diags, busCode); len(got) != 0 {
		t.Fatalf("a row carries code %q, which has no metadata: %+v", busCode, got)
	}
	rows := rowsWithCode(diags, diagnostic.CodeDoctorRecentEvents)
	if len(rows) != 1 {
		t.Fatalf("want one %s row, got %d: %+v", diagnostic.CodeDoctorRecentEvents, len(rows), diags)
	}
	if !strings.Contains(rows[0].Message, busCode) {
		t.Errorf("row message %q does not name the event's code %q", rows[0].Message, busCode)
	}
}

// TestDoctorRecentEventsEmptyBus runs the check over an empty bus.
//
// VALIDATES: no event, no row.
// PREVENTS: a row that reports a problem nobody raised.
func TestDoctorRecentEventsEmptyBus(t *testing.T) {
	resetReportBus(t)
	if diags := checkRecentEvents(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("empty bus answered rows: %+v", diags)
	}
}

// TestDoctorRecentEventsRegisteredRuntimePhase reads the registry show doctor
// reads.
//
// VALIDATES: the check is registered in the runtime phase alone and declares
// doctor-recent-events, followed by exactly the registered event codes.
// PREVENTS: the local ze doctor process reading its own empty bus as a quiet
// box, and the check missing from the daemon route.
func TestDoctorRecentEventsRegisteredRuntimePhase(t *testing.T) {
	found := 0
	want := append([]string{diagnostic.CodeDoctorRecentEvents}, diagnostic.EventCodes()...)
	for _, check := range diagnostic.DoctorChecks() {
		if check.Name != recentEventsCheckName {
			continue
		}
		found++
		if check.Phase != diagnostic.DoctorPhaseRuntime {
			t.Errorf("phase: want %q, got %q", diagnostic.DoctorPhaseRuntime, check.Phase)
		}
		if !slices.Equal(check.Codes, want) {
			t.Errorf("codes: want %v, got %v", want, check.Codes)
		}
	}
	if found != 1 {
		t.Fatalf("want the %s check registered once, found %d", recentEventsCheckName, found)
	}
}

// recentEventsTestEventCode is the fixture event code: described by metadata
// the test registers, then registered as an event code.
const recentEventsTestEventCode = "doctor-test-recent-event-registered"

// registerTestEventCode describes and registers the fixture event code, and
// removes the registration after the test. The metadata stays: the code
// registry has no single-entry removal, and no other test raises this code.
func registerTestEventCode(t *testing.T) {
	t.Helper()
	if diagnostic.Lookup(recentEventsTestEventCode) == nil {
		err := diagnostic.Register(diagnostic.CodeMeta{
			Code:        recentEventsTestEventCode,
			Title:       "Fixture event",
			Description: "A fixture event code for the recent-events tests.",
			Examples:    []string{"ze show errors"},
		})
		if err != nil {
			t.Fatalf("describe fixture code: %v", err)
		}
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: recentEventsTestEventCode}); err != nil {
		t.Fatalf("register fixture event code: %v", err)
	}
	t.Cleanup(func() { diagnostic.UnregisterEventCodeForTest(recentEventsTestEventCode) })
}

// TestEventCodeRegistrationDeclared registers an event code as an owner does
// from init(), raises it, and raises a code nobody registered.
//
// VALIDATES: AC-2. The recent-events check as the registry lists it declares
// the registered code, and its row carries that code; a raised code with no
// registration and no metadata is reported under doctor-recent-events; a code
// with no metadata is refused at registration.
// PREVENTS: a transient code whose line or raise the gate cannot count
// covered, and an event code declared that `ze show diagnostic` cannot
// describe.
func TestEventCodeRegistrationDeclared(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	resetReportBus(t)
	registerTestEventCode(t)

	declared := false
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == recentEventsCheckName && slices.Contains(check.Codes, recentEventsTestEventCode) {
			declared = true
		}
	}
	if !declared {
		t.Fatalf("the %s check does not declare the registered event code %s", recentEventsCheckName, recentEventsTestEventCode)
	}

	const unregistered = "fixture-unregistered-event"
	report.RaiseError(recentEventsFixtureSource, recentEventsTestEventCode, "worker", "lost its queue")
	report.RaiseError(recentEventsFixtureSource, unregistered, "worker", "crashed")

	diags := checkRecentEvents(diagnostic.DoctorCheckContext{})
	if rows := rowsWithCode(diags, recentEventsTestEventCode); len(rows) != 1 {
		t.Errorf("want one row under the registered event code %s, got %+v", recentEventsTestEventCode, diags)
	}
	fallback := rowsWithCode(diags, diagnostic.CodeDoctorRecentEvents)
	if len(fallback) != 1 || !strings.Contains(fallback[0].Message, unregistered) {
		t.Errorf("want the unregistered code %s under %s, got %+v", unregistered, diagnostic.CodeDoctorRecentEvents, diags)
	}

	const undescribed = "doctor-test-recent-event-undescribed"
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: undescribed}); err == nil {
		t.Errorf("an event code with no metadata, %s, was accepted", undescribed)
	}
}

// TestRecentEventsRendersRaiseFix raises an error event and a warning that
// each carry their own fix, and one warning that carries none.
//
// VALIDATES: AC-3. A row's Fix is the fix its raise carried; a raise with no
// fix keeps the show command that lists the event.
// PREVENTS: a producer that knows the command fixing its subject having that
// command replaced by a generic listing.
func TestRecentEventsRendersRaiseFix(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	resetReportBus(t)

	const errorFix = "ze config edit plugin"
	const warningFix = "ze show plugin list"
	report.RaiseErrorFix(recentEventsFixtureSource, diagnostic.CodeDoctorDiskSpace, "disk", "full", errorFix)
	report.RaiseWarningFix(recentEventsFixtureSource, diagnostic.CodeDoctorClockSkew, "clock", "drifts", warningFix)
	report.RaiseWarning(recentEventsFixtureSource, diagnostic.CodeDoctorRandomSeed, "seed", "low entropy")

	diags := checkRecentEvents(diagnostic.DoctorCheckContext{})
	for _, want := range []struct{ code, fix string }{
		{diagnostic.CodeDoctorDiskSpace, errorFix},
		{diagnostic.CodeDoctorClockSkew, warningFix},
		{diagnostic.CodeDoctorRandomSeed, recentEventsFixWarnings},
	} {
		rows := rowsWithCode(diags, want.code)
		if len(rows) != 1 {
			t.Errorf("want one row under %s, got %+v", want.code, diags)
			continue
		}
		if rows[0].Fix != want.fix {
			t.Errorf("%s row fix: want %q, got %q", want.code, want.fix, rows[0].Fix)
		}
	}
}

// TestRecentEventsSkipsWarningAStateCheckOwns proves one fault gives one row:
// an active warning whose code a runtime check declares (doctor-plugin-stopped,
// declared by plugin-stopped) is not repeated as a recent-events row, while a
// warning no check declares and every error event still are.
func TestRecentEventsSkipsWarningAStateCheckOwns(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	const owner = "test-state-owner"
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name: owner, Phase: diagnostic.DoctorPhaseRuntime, Order: 1, Component: "test",
		Dependencies: []string{"plugin-state"},
		Platforms:    []string{diagnostic.DoctorPlatformAny}, Codes: []string{diagnostic.CodeDoctorPluginStopped},
		Check: func(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic { return nil },
	}); err != nil {
		t.Fatalf("register owner check: %v", err)
	}
	t.Cleanup(func() { diagnostic.UnregisterDoctorCheckForTest(owner) })

	report.RaiseWarning("plugin", diagnostic.CodeDoctorPluginStopped, "p1", "configured plugin p1 is not running")
	report.RaiseWarning("other", diagnostic.CodeDoctorInternalDefect, "s1", "a warning no check owns")
	report.RaiseError("plugin", diagnostic.CodeDoctorPluginCrashed, "p1", "plugin process exited unexpectedly: p1")

	rows := checkRecentEvents(diagnostic.DoctorCheckContext{})
	var stopped, defect, crashed int
	for _, row := range rows {
		switch {
		case strings.Contains(row.Message, "is not running"):
			stopped++
		case strings.Contains(row.Message, "no check owns"):
			defect++
		case strings.Contains(row.Message, "exited unexpectedly"):
			crashed++
		}
	}
	if stopped != 0 {
		t.Errorf("doctor-plugin-stopped rendered %d times by recent-events, want 0: its state check reports it", stopped)
	}
	if defect != 1 || crashed != 1 {
		t.Errorf("unowned warning rows = %d, crash event rows = %d, want 1 and 1", defect, crashed)
	}
}
