// Related: gaterun.go -- the run these tests drive from its entry point
//
// VALIDATES: the code an action exits with is the code the caller sees, never
// a flattened 1.
// PREVENTS: a sweep that cannot tell "never ran" from "ran and failed", which
// is the distinction internal/le/commit/actions.go reads when it blocks on 3 and
// stays warn-only on 1.

package gaterun

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
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

// TestRunCarriesTheActionAndCommandIntoTheAnswer validates structured execution
// metadata while output streams directly to the terminal.
func TestRunCarriesTheActionAndCommandIntoTheAnswer(t *testing.T) {
	report, code := Run("probe", []string{"sh", "-c", "exit 3"}, t.TempDir(), os.Environ())
	if code != 3 {
		t.Errorf("Run answered %d, want the command's own 3", code)
	}
	if report.Action != "probe" || report.Code != 3 {
		t.Errorf("the report is %+v, want the action name and its code", report)
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

// VALIDATES: a caller can read a child's stdout without taking it away from the
// terminal.
// PREVENTS: a run reading a suite's own count by capturing its output, which
// would hold an hour of a suite's progress until the suite ended.
func TestStreamTeeGivesTheCallerACopyOfTheChildsOutput(t *testing.T) {
	var copied strings.Builder
	code := StreamTee([]string{"sh", "-c", "echo one; echo two >&2; echo three"},
		t.TempDir(), os.Environ(), &copied)
	if code != 0 {
		t.Fatalf("StreamTee answered %d, want 0", code)
	}
	if copied.String() != "one\nthree\n" {
		t.Errorf("the tee holds %q, want the child's stdout alone", copied.String())
	}
}

// A nil tee is Stream. The default path must not depend on a writer nobody
// passed.
func TestStreamTeeWithNoReaderIsStream(t *testing.T) {
	if code := StreamTee([]string{"sh", "-c", "exit 5"}, t.TempDir(), os.Environ(), nil); code != 5 {
		t.Errorf("StreamTee with a nil tee answered %d, want 5", code)
	}
}

// VALIDATES: a child that exited cleanly (a nil wait error) maps to status 0.
// PREVENTS: every successful child reading as exit 1 at callers that pass the
// wait error straight in, as the stress harness's DUT build and process waits do.
func TestExitCodeOfACleanExitIsZero(t *testing.T) {
	if got := ExitCode(nil); got != 0 {
		t.Fatalf("ExitCode(nil) = %d, want 0", got)
	}
}

// VALIDATES: StreamContext stops the child and its grandchildren when ctx is
// cancelled, and answers the shell's code for the kill rather than 0.
// METHOD: the child forks a sleeping grandchild that records its pid; the test
// cancels 300 ms in, then requires the run back well inside the sleep and the
// grandchild gone.
// PREVENTS: a cancel that kills only the direct child, leaving a test binary
// running and holding the terminal.
func TestStreamContextKillsTheGroupOnCancel(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "grandchild")
	ctx, cancel := context.WithCancel(t.Context())
	timer := time.AfterFunc(300*time.Millisecond, cancel)
	defer timer.Stop()

	began := time.Now()
	code := StreamContext(ctx, []string{"sh", "-c", "sleep 30 & echo $! > " + pidFile + "; sleep 30"}, dir, os.Environ())
	if elapsed := time.Since(began); elapsed > 10*time.Second {
		t.Fatalf("StreamContext came back after %v, want the cancel to stop it", elapsed)
	}
	if code != 128+int(syscall.SIGKILL) {
		t.Errorf("StreamContext answered %d, want %d for a killed child", code, 128+int(syscall.SIGKILL))
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("grandchild pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("grandchild pid %q: %v", raw, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatalf("grandchild %d still runs after the cancel", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
