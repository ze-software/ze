package qemu

import (
	"io"
	"strings"
	"testing"
)

// VALIDATES: a phase's verdict carries how many tests it executed, and a
// functional suite that executed none is a failure however it exited.
// PREVENTS: the exit code being the only evidence. The .ci runner answers 0
// when its selection is empty (Runner.Run, internal/test/runner/runner.go), so
// a suite whose directory moved or whose verb changed reports success and the
// summary renders ALL PHASES PASSED over a population that never started.

func TestTheCountIsReadFromTheRunnersSummaryLine(t *testing.T) {
	cases := []struct {
		name  string
		out   string
		seen  bool
		tests int
	}{
		{name: "a passing suite", out: "pass  59/59  100.0%  22.7s\n", seen: true, tests: 59},
		{
			name:  "a failing suite counts every test it executed",
			out:   "fail  40/42  95.2%  3.2s  failed 2 [a, b]\n",
			seen:  true,
			tests: 42,
		},
		{
			name:  "a suite whose whole population skipped",
			out:   "pass  0/0  100.0%  0.1s  skip 12 [a]\n",
			seen:  true,
			tests: 0,
		},
		{
			name: "a suite that printed a usage message and ran nothing",
			out:  "timeout: unrecognized option: kill-after=15s\nBusyBox v1.37.0\n",
			seen: false,
		},
		{
			name: "a test that printed the word pass",
			out:  "  observer: pass 1 of 2 checks\n",
			seen: false,
		},
		{
			name:  "the same line under a terminal's color escapes",
			out:   "\x1b[92mpass\x1b[0m  7/7  100.0%  1.0s\n",
			seen:  true,
			tests: 7,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var tally suiteTally
			if _, err := io.WriteString(&tally, c.out); err != nil {
				t.Fatalf("write: %v", err)
			}
			if tally.Seen != c.seen {
				t.Fatalf("Seen is %v, want %v, for output %q", tally.Seen, c.seen, c.out)
			}
			if tally.Tests != c.tests {
				t.Errorf("Tests is %d, want %d", tally.Tests, c.tests)
			}
		})
	}
}

// A line arrives in whatever pieces the pipe delivers, so the count must not
// depend on where a write ends.
func TestTheCountSurvivesALineSplitAcrossWrites(t *testing.T) {
	var tally suiteTally
	for _, piece := range []string{"pas", "s  12", "/12  100", ".0%  1s\n"} {
		if _, err := io.WriteString(&tally, piece); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if !tally.Seen || tally.Tests != 12 {
		t.Errorf("the tally read Seen=%v Tests=%d, want true and 12", tally.Seen, tally.Tests)
	}
}

// One test printing a megabyte on one line must not grow the tally with it.
func TestALineWithoutAnEndIsBounded(t *testing.T) {
	var tally suiteTally
	if _, err := io.WriteString(&tally, strings.Repeat("x", tallyLineMax*4)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if len(tally.line) > tallyLineMax {
		t.Errorf("the tally holds %d bytes of one line, want at most %d", len(tally.line), tallyLineMax)
	}

	// The bound must not cost the next line. A suite prints its summary after
	// whatever a test printed before it.
	if _, err := io.WriteString(&tally, "\npass  3/3  100.0%  1s\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !tally.Seen || tally.Tests != 3 {
		t.Errorf("the tally read Seen=%v Tests=%d after a discarded line, want true and 3",
			tally.Seen, tally.Tests)
	}
}

// The defect in one run: a suite exits 0 having executed nothing, and the run
// counts it as a pass.
func TestASuiteThatExecutedNoTestIsAFailureEvenWhenItExitsZero(t *testing.T) {
	run := vmFixture(t)
	silent := &recorder{}
	run.Run = func(argv, environ []string, _ io.Writer) int {
		// Every child answers 0 and prints no summary line, which is what
		// `ze-test <suite> --all` does when its selection is empty.
		return silent.run(argv, environ, nil)
	}

	report, code := run.Execute()
	if code == 0 {
		t.Fatal("a run whose every suite executed no test exited 0")
	}
	if len(report.Failed) == 0 {
		t.Fatal("no phase was named as failed")
	}
	if strings.Contains(report.Text(), "ALL PHASES PASSED") {
		t.Errorf("the summary reads as a pass:\n%s", report.Text())
	}

	for _, phase := range report.Phases {
		if !strings.HasPrefix(phase.Name, "functional/") || phase.Skipped {
			continue
		}
		if phase.Counted {
			t.Errorf("phase %q claims a count it never read", phase.Name)
		}
		if !strings.Contains(phase.Reason, "summary line") {
			t.Errorf("phase %q gives the reason %q, want it to name the missing summary line",
				phase.Name, phase.Reason)
		}
	}
}

// The count reaches the report, so a reader of `| json` can tell a suite that
// ran two thousand tests from one that ran two.
func TestTheReportCarriesWhatEachSuiteExecuted(t *testing.T) {
	run := vmFixture(t)
	rec := &recorder{}
	run.Run = rec.run

	report, code := run.Execute()
	if code != 0 {
		t.Fatalf("a run whose every child answered 0 exited %d: %v", code, report.Failed)
	}
	for _, phase := range report.Phases {
		if !strings.HasPrefix(phase.Name, "functional/") || phase.Skipped {
			continue
		}
		if !phase.Counted || phase.Tests != 7 {
			t.Errorf("phase %q reports Counted=%v Tests=%d, want true and 7",
				phase.Name, phase.Counted, phase.Tests)
		}
	}
	if !strings.Contains(report.Text(), "executed across the functional suites") {
		t.Errorf("the summary does not say how many tests ran:\n%s", report.Text())
	}
}
