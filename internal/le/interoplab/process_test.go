package interoplab

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
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

// VALIDATES: G-2 for a terminal Ctrl-C: a docker child that shares no
// terminal with le runs in a process group of its own, so the terminal's
// SIGINT reaches le alone and le's signal root decides what stops; a child
// handed a terminal stays in le's group, where reading it raises no SIGTTIN.
// METHOD: a fixture child prints its own process group id (`ps -o pgid=`),
// once with every stream a pipe and once with a pseudo-terminal as its
// stdin, and the test compares each with this process's group. Linux and
// Darwin both provide pseudo-terminals, so failing to open one is a failure.
// PREVENTS: the teardown `docker rm -f` killed by the same Ctrl-C that stopped
// the run ("container ... was not removed: docker rm -f failed (exit 130)",
// real Ctrl-C proof row 3-prime of 2026-10-19), and a background-group child
// stopped on a terminal read.
func TestDockerChildRunsInItsOwnProcessGroup(t *testing.T) {
	own := syscall.Getpgrp()
	command := ProcessCommand{Arguments: []string{"sh", "-c", "ps -o pgid= -p $$"}, Timeout: 30 * time.Second}

	result, err := systemProcessRunner{}.Run(t.Context(), command)
	if err != nil {
		t.Fatalf("run the fixture child: %v", err)
	}
	group, err := strconv.Atoi(strings.TrimSpace(result.Stdout))
	if err != nil {
		t.Fatalf("read the child's process group from %q: %v", result.Stdout, err)
	}
	if group == own {
		t.Errorf("child runs in le's process group %d, want a group of its own", own)
	}

	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("open a pseudo-terminal: %v", err)
	}
	defer ptmx.Close() //nolint:errcheck // test cleanup
	defer tty.Close()  //nolint:errcheck // test cleanup
	command.Stdin = tty
	result, err = systemProcessRunner{}.Run(t.Context(), command)
	if err != nil {
		t.Fatalf("run the fixture child on a terminal: %v", err)
	}
	group, err = strconv.Atoi(strings.TrimSpace(result.Stdout))
	if err != nil {
		t.Fatalf("read the terminal child's process group from %q: %v", result.Stdout, err)
	}
	if group != own {
		t.Errorf("child handed a terminal runs in group %d, want le's group %d", group, own)
	}
}
