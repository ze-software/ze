package sdk

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// sendSelfTERM delivers SIGTERM to this test process and waits until the
// guard channel has it, so the signal is known to have been dispatched to
// every registered handler before the caller looks at a context.
func sendSelfTERM(t *testing.T) {
	t.Helper()
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, syscall.SIGTERM)
	defer signal.Stop(guard)
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}
	select {
	case <-guard:
	case <-time.After(5 * time.Second):
		t.Fatal("SIGTERM was never delivered")
	}
}

// TestSignalContextCancelsOnSIGTERM is the fork-mode contract: a plugin that
// is its own process cancels on SIGTERM.
//
// VALIDATES: SignalContext without HostOwnsSignals cancels on SIGTERM.
// PREVENTS: a subprocess plugin killed without running its deferreds.
func TestSignalContextCancelsOnSIGTERM(t *testing.T) {
	hostOwnsSignals.Store(false)
	ctx, cancel := SignalContext()
	defer cancel()

	sendSelfTERM(t)

	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("a fork-mode plugin context did not cancel on SIGTERM")
	}
}

// TestSignalContextIgnoresSIGTERMWhenHostOwnsSignals is the in-process
// contract: the daemon's shutdown, not the signal, ends the plugin.
//
// VALIDATES: after HostOwnsSignals, SIGTERM leaves the plugin context alive,
// and the CancelFunc still ends it.
// PREVENTS: every in-process plugin exiting the instant the daemon receives
// SIGTERM, while a config reload is still inside its shutdown grace (reload
// verify then waits on plugins that are gone, and the bgp plugin's exit tears
// the sessions down with Administrative Shutdown ahead of the ordered stop).
func TestSignalContextIgnoresSIGTERMWhenHostOwnsSignals(t *testing.T) {
	HostOwnsSignals()
	t.Cleanup(func() { hostOwnsSignals.Store(false) })
	ctx, cancel := SignalContext()

	sendSelfTERM(t)

	select {
	case <-ctx.Done():
		t.Fatal("an in-process plugin context canceled on the daemon's SIGTERM")
	case <-time.After(200 * time.Millisecond):
	}

	cancel()
	if ctx.Err() == nil {
		t.Fatal("the CancelFunc did not end the plugin context")
	}
}
