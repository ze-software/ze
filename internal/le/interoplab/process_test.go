package interoplab

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// VALIDATES: G-2: the system process runner tells a cancel from a deadline. A
// command whose context was canceled (a Ctrl-C reaching le's signal root)
// answers context.Canceled with no exit code, and its error text reads as an
// interrupt; only a command its bound ended answers the timeout exit code and
// reads as a hang.
// METHOD: run `sleep 10` under a context canceled after it starts, then under
// a bound shorter than the sleep, and read each result and the commandError
// text the Docker wrapper builds from it.
// PREVENTS: "docker exec ... failed (exit 124): context canceled", which names
// a timeout for an interrupt (the real Ctrl-C proofs of 2026-10-19).
func TestProcessRunnerNamesCancelAndDeadlineApart(t *testing.T) {
	const bound = 30 * time.Second
	canceled, cancel := context.WithCancel(t.Context())
	time.AfterFunc(100*time.Millisecond, cancel)
	result, err := systemProcessRunner{}.Run(canceled, ProcessCommand{Arguments: []string{"sleep", "10"}, Timeout: bound})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled run error = %v, want context.Canceled", err)
	}
	if result.ExitCode == timeoutExitCode {
		t.Errorf("canceled run answered the timeout exit code %d", timeoutExitCode)
	}
	text := (&commandError{Operation: "docker exec lab-ze sleep 10", ExitCode: result.ExitCode, Cause: err, Timeout: bound}).Error()
	if strings.Contains(text, "exit 124") {
		t.Errorf("canceled run reads as a timeout: %q", text)
	}
	if !strings.HasPrefix(text, "docker exec lab-ze sleep 10 was canceled before it finished") {
		t.Errorf("canceled run text = %q, want it to say the command was canceled", text)
	}

	result, err = systemProcessRunner{}.Run(t.Context(), ProcessCommand{Arguments: []string{"sleep", "10"}, Timeout: 100 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded run error = %v, want context.DeadlineExceeded", err)
	}
	if result.ExitCode != timeoutExitCode {
		t.Errorf("bounded run exit code = %d, want %d", result.ExitCode, timeoutExitCode)
	}
	text = (&commandError{Operation: "docker exec lab-ze sleep 10", ExitCode: result.ExitCode, Cause: err, Timeout: 100 * time.Millisecond}).Error()
	if !strings.HasPrefix(text, "docker exec lab-ze sleep 10 did not finish within 100ms") {
		t.Errorf("bounded run text = %q, want it to name the bound", text)
	}
}
