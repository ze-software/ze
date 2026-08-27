// Related: gaterun.go -- the run these tests drive from its entry point
//
// VALIDATES: spec-le-is-a-ze-binary AC-8 at its smallest -- the code a gate
// exits with is the code the caller sees, never a flattened 1.
// PREVENTS: a sweep that cannot tell "never ran" from "ran and failed", which
// is the distinction scripts/dev/commit_helper.py reads when it blocks on 3 and
// stays warn-only on 1.

package gaterun

import (
	"os"
	"testing"
)

// TestStreamAnswersTheChildsOwnCode is the exit-code rule at its smallest.
func TestStreamAnswersTheChildsOwnCode(t *testing.T) {
	for _, want := range []int{0, 1, 3, 7, 125} {
		var tb = "exit " + itoa(want) //nolint:gocritic // a fixture script, not a hot path
		if got := Stream([]string{"sh", "-c", tb}, t.TempDir(), os.Environ()); got != want {
			t.Errorf("Stream of `%s` answered %d, want %d", tb, got, want)
		}
	}
}

// TestStreamReportsACommandThatCouldNotStart keeps "never ran" apart from "ran
// and failed", which is what CannotStart exists for.
func TestStreamReportsACommandThatCouldNotStart(t *testing.T) {
	if got := Stream([]string{"ze-no-such-binary-anywhere"}, t.TempDir(), os.Environ()); got != CannotStart {
		t.Errorf("Stream of a missing binary answered %d, want %d", got, CannotStart)
	}
	if got := Stream(nil, t.TempDir(), os.Environ()); got != CannotStart {
		t.Errorf("Stream of an empty argv answered %d, want %d", got, CannotStart)
	}
}

// TestRunCarriesTheGateAndItsCommandIntoTheAnswer validates AC-7 for a forked
// gate. The output streams to the terminal. Therefore, the payload identifies
// the gate, its command, and its decision.
func TestRunCarriesTheGateAndItsCommandIntoTheAnswer(t *testing.T) {
	report, code := Run("ze-probe-check", []string{"sh", "-c", "exit 3"}, t.TempDir(), os.Environ())
	if code != 3 {
		t.Errorf("Run answered %d, want the command's own 3", code)
	}
	if report.Gate != "ze-probe-check" || report.Code != 3 {
		t.Errorf("the report is %+v, want the gate name and its code", report)
	}
	if len(report.Command) != 3 {
		t.Errorf("the report carries %v, want the whole command", report.Command)
	}
}

// itoa spells a small non-negative number, so the fixture script above needs no
// format verb (performance.md bans building strings with one).
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}
