// Design: docs/architecture/doctor-and-health-checks.md -- recent events on the report bus
// Related: checks_recent_events.go -- the runtime check under test

package doctor

import (
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
// doctor-recent-events.
// PREVENTS: the local ze doctor process reading its own empty bus as a quiet
// box, and the check missing from the daemon route.
func TestDoctorRecentEventsRegisteredRuntimePhase(t *testing.T) {
	found := 0
	for _, check := range diagnostic.DoctorChecks() {
		if check.Name != recentEventsCheckName {
			continue
		}
		found++
		if check.Phase != diagnostic.DoctorPhaseRuntime {
			t.Errorf("phase: want %q, got %q", diagnostic.DoctorPhaseRuntime, check.Phase)
		}
		if len(check.Codes) != 1 || check.Codes[0] != diagnostic.CodeDoctorRecentEvents {
			t.Errorf("codes: want [%s], got %v", diagnostic.CodeDoctorRecentEvents, check.Codes)
		}
	}
	if found != 1 {
		t.Fatalf("want the %s check registered once, found %d", recentEventsCheckName, found)
	}
}
