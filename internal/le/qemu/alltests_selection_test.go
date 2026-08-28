package qemu

import (
	"strings"
	"testing"
)

// VALIDATES: the needs-linux selection reaches every child of the in-VM run,
// leaves the suite list whole, and is recorded in what the run answers.
// PREVENTS: the tight loop being lost with the Make retirement. The retired
// `ze-qemu-needs-linux-test` target exported ZE_QEMU_LINUX_ONLY=1 around the
// same driver, so a developer who changed one Linux-only path paid for one VM
// boot rather than the whole matrix. It also prevents the filtered run reading
// as a whole one: every suite still starts and still answers 0.

// The filter lives in the .ci runner (parseAndAdd,
// internal/test/runner/record_parse.go), so the run's whole job is to tell each
// child which population it wants. Every child is told, because the unit phase
// compiles the runner's own tests and the functional suites read the variable
// at parse time.
func TestTheNeedsLinuxSelectionReachesEveryChild(t *testing.T) {
	t.Setenv(linuxOnlyKey, "")
	run := vmFixture(t)
	run.LinuxOnly = true
	rec := &recorder{}
	run.Run = rec.run

	report, code := run.Execute()
	if code != 0 {
		t.Fatalf("a run whose every child answered 0 exited %d: %v", code, report.Failed)
	}
	for i, environ := range rec.envs {
		if !hasEnv(environ, linuxOnlyKey, linuxOnlyValue) {
			t.Fatalf("command %d was not told to filter: %v", i, rec.calls[i])
		}
	}

	// The SUITE LIST is untouched. Narrowing it here would make the selection a
	// second hand-written population beside vmSuites, which is the hole the
	// coverage guard exists to refuse.
	unfiltered := vmFixture(t)
	plain := &recorder{}
	unfiltered.Run = plain.run
	unfiltered.Execute()
	if len(rec.calls) != len(plain.calls) {
		t.Errorf("the selection ran %d commands and an unfiltered run ran %d;"+
			" the filter belongs to the .ci runner, not to the suite list",
			len(rec.calls), len(plain.calls))
	}
}

// A filtered run answers the same shape as a whole one: the same suites start
// and each answers 0. The report therefore names the population, so nobody
// reads ALL PHASES PASSED as a verdict over tests that never ran.
func TestAFilteredRunSaysWhichPopulationItCovered(t *testing.T) {
	t.Setenv(linuxOnlyKey, "")
	run := vmFixture(t)
	run.LinuxOnly = true
	run.Run = (&recorder{}).run

	report, _ := run.Execute()
	if report.Selection != linuxOnlySelection {
		t.Errorf("the report records the selection %q, want %q", report.Selection, linuxOnlySelection)
	}
	text := report.Text()
	if !strings.Contains(text, "option="+linuxOnlySelection) {
		t.Errorf("the summary does not name the population it covered: %q", text)
	}
	if !strings.Contains(text, "ALL PHASES PASSED") {
		t.Errorf("a filtered run with no failure does not say so: %q", text)
	}
}

// The whole run is the default, and it tells no child to filter. A leaked
// variable would silently shrink the nightly population to the marked tests.
func TestAnUnfilteredRunTellsNoChildToFilter(t *testing.T) {
	t.Setenv(linuxOnlyKey, "")
	run := vmFixture(t)
	rec := &recorder{}
	run.Run = rec.run

	report, _ := run.Execute()
	for i, environ := range rec.envs {
		if hasEnv(environ, linuxOnlyKey, linuxOnlyValue) {
			t.Fatalf("command %d filters without being asked: %v", i, rec.calls[i])
		}
	}
	if report.Selection != "" {
		t.Errorf("a whole run records the selection %q", report.Selection)
	}
	if strings.Contains(report.Text(), "selection:") {
		t.Errorf("a whole run claims a selection: %q", report.Text())
	}
}

// The nightly workflow exports the variable around the guest command
// (.github/workflows/qemu-nightly.yml, job needs-linux). Reading it here is
// what lets the run RECORD that it was filtered, rather than passing the
// variable through unread and reporting a whole run.
func TestTheSelectionIsReadFromTheEnvironment(t *testing.T) {
	t.Setenv(linuxOnlyKey, linuxOnlyValue)
	if !newAllTests().LinuxOnly {
		t.Errorf("%s=%s did not select the needs-linux population", linuxOnlyKey, linuxOnlyValue)
	}

	for _, value := range []string{"", "0", "true", "yes"} {
		t.Setenv(linuxOnlyKey, value)
		if newAllTests().LinuxOnly {
			t.Errorf("%s=%q selected the needs-linux population; only %q does",
				linuxOnlyKey, value, linuxOnlyValue)
		}
	}
}

// A population this action cannot honor is refused by name, before the run
// starts. Accepting it would run the WHOLE suite while the caller believed the
// command had narrowed it, which is the worst of both answers.
func TestAnUnknownPopulationIsRefusedBeforeTheRunStarts(t *testing.T) {
	for _, args := range [][]string{
		{"all-tests", "only", "everything"},
		{"all-tests", "only", ""},
		{"all-tests", "only"},
		{"all-tests", "needs-linux"},
	} {
		payload, code := Answer(args)
		if code != 2 {
			t.Errorf("%v answered code %d, want 2", args, code)
		}
		if payload != nil {
			t.Errorf("%v answered the payload %v", args, payload)
		}
	}
}
