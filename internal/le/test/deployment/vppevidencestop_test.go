package testdeployment

// The stop contract every VPP scenario daemon goes through.
//
// Goal: Pin down that a stop answers only once the process exited, and that a
// stop which did not end in a clean exit stops the run before the next daemon
// starts. A real-VPP run failed because ze, stopping, was still recording its
// boot configuration to a slow store when the next start opened the same store
// (`store is owned by another process: /run/ze/traffic.conf`).
// Method: A running whose signal function stands in for the container's pkill
// and decides on which signal the fake process exits. No process runs.

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

// fakeDaemon answers a running that exits only on the signal exitOn, and the
// signals it received, in order. exitOn 0 never exits. delay holds the exit
// back after the signal, the way a store sync holds ze's stop.
func fakeDaemon(t *testing.T, exitOn syscall.Signal, delay time.Duration) (*running, *[]syscall.Signal) {
	t.Helper()

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { write.Close() }) //nolint:errcheck // test pipe

	received := &[]syscall.Signal{}
	proc := &running{done: make(chan struct{}), read: read, command: "vpp: ze start traffic.conf"}
	proc.signal = func(signal syscall.Signal) error {
		*received = append(*received, signal)
		if signal == exitOn {
			time.AfterFunc(delay, func() { close(proc.done) })
		}
		return nil
	}
	return proc, received
}

// TestStopAwaitingExitWaitsForASlowCleanExit proves the stop waits for the
// exit rather than returning on the signal: the process exits well after
// SIGTERM, within the wait, and the stop answers nil only after done closed.
func TestStopAwaitingExitWaitsForASlowCleanExit(t *testing.T) {
	proc, received := fakeDaemon(t, syscall.SIGTERM, 300*time.Millisecond)

	if err := proc.stopAwaitingExit(5*time.Second, time.Second); err != nil {
		t.Fatalf("stop of a process that exited on SIGTERM: %v", err)
	}
	if !proc.exited() {
		t.Fatal("stop answered before the process exited")
	}
	if len(*received) != 1 || (*received)[0] != syscall.SIGTERM {
		t.Fatalf("signals sent: %v, want only SIGTERM", *received)
	}
}

// TestStopAwaitingExitNamesAKilledProcess proves a process that ignores
// SIGTERM is killed and the stop fails, naming it.
func TestStopAwaitingExitNamesAKilledProcess(t *testing.T) {
	proc, received := fakeDaemon(t, syscall.SIGKILL, 0)

	err := proc.stopAwaitingExit(100*time.Millisecond, time.Second)
	if !errors.Is(err, errProcessKilled) {
		t.Fatalf("stop error %v, want errProcessKilled", err)
	}
	if !strings.Contains(err.Error(), "ze start traffic.conf") {
		t.Fatalf("stop error %q does not name the process", err)
	}
	if len(*received) != 2 || (*received)[1] != syscall.SIGKILL {
		t.Fatalf("signals sent: %v, want SIGTERM then SIGKILL", *received)
	}
}

// TestStopAwaitingExitNamesASurvivor proves a process that outlives both
// signals fails the stop as still running, rather than returning as stopped.
func TestStopAwaitingExitNamesASurvivor(t *testing.T) {
	proc, _ := fakeDaemon(t, 0, 0)

	err := proc.stopAwaitingExit(50*time.Millisecond, 50*time.Millisecond)
	if !errors.Is(err, errProcessSurvived) {
		t.Fatalf("stop error %v, want errProcessSurvived", err)
	}
	if proc.exited() {
		t.Fatal("the fake process exited, so the test proved nothing")
	}
}

// TestStopAwaitingExitSendsNothingToAnExitedProcess proves a process that
// already exited answers nil with no signal.
func TestStopAwaitingExitSendsNothingToAnExitedProcess(t *testing.T) {
	proc, received := fakeDaemon(t, syscall.SIGTERM, 0)
	close(proc.done)

	if err := proc.stopAwaitingExit(time.Second, time.Second); err != nil {
		t.Fatalf("stop of an exited process: %v", err)
	}
	if len(*received) != 0 {
		t.Fatalf("signals sent to an exited process: %v", *received)
	}
}

// TestVPPStopFailureRefusesTheNextDaemon proves the scenario-level contract:
// a daemon whose stop failed is recorded once, and the next start refuses
// before it touches the container. Docker is nil, so a start that went past
// the refusal would panic.
func TestVPPStopFailureRefusesTheNextDaemon(t *testing.T) {
	v := &VPP{vppIface: &vppIface{}}

	clean, _ := fakeDaemon(t, syscall.SIGTERM, 0)
	v.stopVPPProcess(clean, newCollector())
	if err := v.checkDaemonsStopped(); err != nil {
		t.Fatalf("a clean stop recorded a failure: %v", err)
	}

	stuck, _ := fakeDaemon(t, 0, 0)
	first := stuck.stopAwaitingExit(10*time.Millisecond, 10*time.Millisecond)
	if !errors.Is(first, errProcessSurvived) {
		t.Fatalf("setup: stop error %v", first)
	}
	v.recordStopFailure(first)
	v.recordStopFailure(errProcessKilled)
	recorded := v.checkDaemonsStopped()
	if !errors.Is(recorded, errProcessSurvived) {
		t.Fatalf("recorded %v, want the first failure kept", recorded)
	}
	if errors.Is(recorded, errProcessKilled) {
		t.Fatalf("a later failure joined the first: %v", recorded)
	}
	if !strings.Contains(recorded.Error(), "ze start traffic.conf") {
		t.Fatalf("recorded %q does not name the daemon", recorded)
	}

	daemon, seen, startErr := v.startEvidenceDaemon("vpp", "traffic.conf", 0)
	if !errors.Is(startErr, errProcessSurvived) {
		t.Fatalf("start after a failed stop answered %v, want the stop failure", startErr)
	}
	if daemon != nil {
		t.Fatal("start after a failed stop answered a daemon")
	}
	if seen != nil {
		t.Fatal("start after a failed stop answered a collector")
	}
}
