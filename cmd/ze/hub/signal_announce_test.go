// Design: docs/architecture/behavior/signals.md -- the stop signal is announced once forceCh is registered


package hub

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// TestWaitLoopLeavesStopSignalToCaller proves the stop starts even when the
// announce would never return. Goal: a `signal` subscriber that is wedged
// cannot delay SIGTERM, because waitLoop hands the stop signal back without
// announcing it, and run announces it only after forceCh is registered.
// Method: the announce blocks until the test ends; a SIGTERM on stopCh must
// come back from waitLoop at once, with the announce never called for it. A
// SIGHUP before it is still announced first, as the reload contract needs.
func TestWaitLoopLeavesStopSignalToCaller(t *testing.T) {
	t.Parallel()

	stopCh := make(chan os.Signal, 1)
	hupCh := make(chan os.Signal, 1)
	reloadCh := make(chan os.Signal, 1)
	announced := make(chan os.Signal, 2)
	wedged := make(chan struct{})
	t.Cleanup(func() { close(wedged) })
	announce := func(sig os.Signal) {
		announced <- sig
		if sig == syscall.SIGTERM {
			<-wedged
		}
	}

	hupCh <- syscall.SIGHUP
	returned := make(chan os.Signal, 1)
	go func() { returned <- waitLoop(stopCh, hupCh, reloadCh, nil, announce) }()

	select {
	case sig := <-announced:
		if sig != syscall.SIGHUP {
			t.Fatalf("first announce = %v, want SIGHUP", sig)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the SIGHUP was not announced")
	}

	stopCh <- syscall.SIGTERM
	select {
	case sig := <-returned:
		if sig != syscall.SIGTERM {
			t.Fatalf("waitLoop returned %v, want SIGTERM", sig)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("waitLoop did not return the stop signal: the announce held the stop")
	}
	select {
	case sig := <-announced:
		t.Fatalf("waitLoop announced the stop signal %v itself; run announces it after forceCh", sig)
	default:
	}
}
