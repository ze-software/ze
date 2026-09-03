// VALIDATES: the cmd=stop primitive (spec-fixit-runner-kill-background) --
//            stopNamedBackground terminates and reaps a named background process
//            mid-test (AC-1), fails closed on an unknown name (AC-2), and leaves a
//            stopped process safe for teardown to re-handle (AC-3).
// PREVENTS: (1) advancing to step N+1 before the OS has reaped the killed process
//            (a race that makes AC-1 non-deterministic); (2) a typo'd stop name
//            silently signaling nothing while the test still passes; (3) teardown
//            hanging or panicking on a process the stop step already reaped.

package runner

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// startSleeper starts a real long-lived child process for the stop tests. It is
// registered exactly as the runner registers a named cmd=background process.
func startSleeper(t *testing.T) *exec.Cmd {
	t.Helper()
	proc := exec.CommandContext(t.Context(), "sleep", "60")
	if err := proc.Start(); err != nil {
		t.Fatalf("start sleeper: %v", err)
	}
	return proc
}

// teardownTestDeadline bounds every stop/teardown call these tests make. The
// paths under test reap a process, so they return in milliseconds; the deadline
// only has to be short enough to report a hang rather than sit in it.
const teardownTestDeadline = 5 * time.Second

// withinDeadline runs fn and fails the test if it does not return within
// teardownTestDeadline, catching a stop/teardown path that hangs instead of
// reaping.
func withinDeadline(t *testing.T, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(teardownTestDeadline):
		t.Fatalf("%s did not complete within %s (hang)", what, teardownTestDeadline)
	}
}

// TestStopBackgroundKillsNamedProcess proves AC-1: a named background process is
// terminated AND reaped by the stop step, so it is already gone before the next
// step runs. The default signal (kill) leaves an ExitCode of -1 (signaled, not a
// clean exit), and the process is removed from both the tracking slice and the
// name map.
func TestStopBackgroundKillsNamedProcess(t *testing.T) {
	proc := startSleeper(t)
	bgProcs := []*exec.Cmd{proc}
	namedBg := map[string]*exec.Cmd{"responder": proc}

	cmd := RunCommand{Mode: modeStop, Seq: 2, Name: "responder", Signal: signalKill}
	var err error
	var stopped *exec.Cmd
	withinDeadline(t, "stopNamedBackground(kill)", func() {
		stopped, bgProcs, err = stopNamedBackground(cmd, bgProcs, namedBg)
	})
	if err != nil {
		t.Fatalf("stopNamedBackground returned error: %v", err)
	}
	if stopped != proc {
		t.Fatal("stopNamedBackground must return the stopped process for further pruning")
	}
	if proc.ProcessState == nil {
		t.Fatal("process was not reaped: ProcessState is nil after stop")
	}
	if proc.ProcessState.ExitCode() != -1 {
		t.Fatalf("expected signaled exit (ExitCode -1), got %d", proc.ProcessState.ExitCode())
	}
	if _, ok := namedBg["responder"]; ok {
		t.Fatal("stopped process still registered in namedBg")
	}
	for _, p := range bgProcs {
		if p == proc {
			t.Fatal("stopped process still present in bgProcs")
		}
	}
}

// TestStopBackgroundTermSignal confirms the signal=term path also terminates and
// reaps the target (a sleep exits on SIGTERM), so a test wanting a graceful stop
// gets one without the runner hanging.
func TestStopBackgroundTermSignal(t *testing.T) {
	proc := startSleeper(t)
	bgProcs := []*exec.Cmd{proc}
	namedBg := map[string]*exec.Cmd{"daemon": proc}

	cmd := RunCommand{Mode: modeStop, Seq: 3, Name: "daemon", Signal: signalTerm}
	var err error
	withinDeadline(t, "stopNamedBackground(term)", func() {
		_, bgProcs, err = stopNamedBackground(cmd, bgProcs, namedBg)
	})
	if err != nil {
		t.Fatalf("stopNamedBackground returned error: %v", err)
	}
	if proc.ProcessState == nil {
		t.Fatal("process was not reaped after SIGTERM")
	}
	if len(bgProcs) != 0 {
		t.Fatalf("expected stopped process removed from bgProcs, got %d entries", len(bgProcs))
	}
}

// TestStopBackgroundUnknownNameFails proves AC-2 (fail-closed): a stop directive
// naming a process the runner never started is a hard error, and the tracking
// slice is returned unchanged (nothing was signaled).
func TestStopBackgroundUnknownNameFails(t *testing.T) {
	proc := startSleeper(t)
	defer func() { _ = proc.Process.Kill() }()
	bgProcs := []*exec.Cmd{proc}
	namedBg := map[string]*exec.Cmd{"responder": proc}

	cmd := RunCommand{Mode: modeStop, Seq: 2, Name: "ghost", Signal: signalKill}
	stopped, got, err := stopNamedBackground(cmd, bgProcs, namedBg)
	if err == nil {
		t.Fatal("expected error for unknown process name, got nil (fail-open)")
	}
	if stopped != nil {
		t.Fatal("unknown-name stop must return a nil stopped process")
	}
	if len(got) != 1 || got[0] != proc {
		t.Fatal("bgProcs must be unchanged when the stop name is unknown")
	}
	// The real, correctly-named process must be untouched (still running).
	if proc.ProcessState != nil {
		t.Fatal("an unknown-name stop must not reap any process")
	}
}

// TestTeardownToleratesStoppedProcess proves AC-3: after a stop step reaps a
// process, the two teardown paths the runner runs over every background process --
// the deferred Process.Kill() and terminateGracefully -- must not error out, panic,
// or hang on the already-dead process.
func TestTeardownToleratesStoppedProcess(t *testing.T) {
	proc := startSleeper(t)
	bgProcs := []*exec.Cmd{proc}
	namedBg := map[string]*exec.Cmd{"responder": proc}

	cmd := RunCommand{Mode: modeStop, Seq: 2, Name: "responder", Signal: signalKill}
	if _, _, err := stopNamedBackground(cmd, bgProcs, namedBg); err != nil {
		t.Fatalf("stopNamedBackground returned error: %v", err)
	}

	// Mirror the deferred teardown at runner_exec.go: Process.Kill() over every
	// tracked process. The stopped proc was pruned from bgProcs, but exercise the
	// idempotency directly to prove a double-kill is harmless.
	if proc.Process != nil {
		_ = proc.Process.Kill() // already reaped; returns an ignored error, must not panic
	}

	// terminateGracefully is what the end-of-test teardown calls on daemons; it
	// must return promptly on an already-reaped process (its Wait returns fast).
	withinDeadline(t, "terminateGracefully on stopped process", func() {
		terminateGracefully(proc)
	})
}

// TestBackgroundLifetimeStopsTheProcess drives the declared lifetime a
// cmd=background:timeout= asks for.
//
// VALIDATES: a background command's timeout= is its LIFETIME, so the process is
// stopped that long after it starts. cmd=background has documented timeout=DUR
// in its own format comment since it was written, and nothing read it, so an
// author who declared one got silence.
// PREVENTS: the accepted-and-discarded shape this file's own resolveOrchestratedTimeout
// comment names: an option that is worse than a rejected one, because the .ci
// reads as if the lifetime were set.
//
// The controls are what make it discriminate. A command with NO timeout must
// survive, or the test would pass against a runner that killed every background
// process; and an unparseable one must survive too, rather than being read as
// "stop immediately".
func TestBackgroundLifetimeStopsTheProcess(t *testing.T) {
	stopped := startSleeper(t)
	startBackgroundLifetime(t.Context(), RunCommand{Timeout: "150ms"}, stopped)

	survivor := startSleeper(t)
	startBackgroundLifetime(t.Context(), RunCommand{}, survivor)

	unparseable := startSleeper(t)
	startBackgroundLifetime(t.Context(), RunCommand{Timeout: "not-a-duration"}, unparseable)

	done := make(chan error, 1)
	go func() { done <- stopped.Wait() }()
	select {
	case <-done:
	case <-time.After(teardownTestDeadline):
		t.Fatal("a background command that declared timeout=150ms was never stopped")
	}

	// The two controls are still running, so their declared silence was honored.
	for name, proc := range map[string]*exec.Cmd{"no timeout": survivor, "unparseable timeout": unparseable} {
		if proc.ProcessState != nil {
			t.Errorf("the %q control was stopped, so the lifetime applies where none was declared", name)
		}
		_ = proc.Process.Kill()
		_ = proc.Wait()
	}
}

// TestBackgroundLifetimeEndsWithTheTest verifies the timer follows the test
// context, so a run that finishes first leaves no goroutine waiting to kill a
// process teardown has already reaped.
//
// VALIDATES: the lifetime timer is cancelled by the context.
// PREVENTS: a leaked goroutine per background command in every test that
// declares a lifetime longer than its own run.
func TestBackgroundLifetimeEndsWithTheTest(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	proc := startSleeper(t)
	startBackgroundLifetime(ctx, RunCommand{Timeout: "1h"}, proc)
	cancel()

	// The process must outlive the cancelled timer: cancellation stops the
	// timer, it does not fire it.
	time.Sleep(50 * time.Millisecond)
	if proc.ProcessState != nil {
		t.Error("a cancelled lifetime killed the process instead of standing down")
	}
	_ = proc.Process.Kill()
	_ = proc.Wait()
}
