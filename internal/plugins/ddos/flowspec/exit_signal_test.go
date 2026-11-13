//go:build unix

// Design: docs/guide/ddos-mitigation.md -- what each end of the plugin does to a live FlowSpec rule
// Related: register.go (runEngine, the deferred exit), exit_test.go (the engine-pair harness)

package flowspec

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// TestRunEndingWithoutByeWithdrawsTheLiveAnnouncement proves that when the
// plugin's Run returns with no `bye` while the engine still answers, the
// deferred exit in runEngine withdraws the live rule and removes its record.
//
// VALIDATES: AC-13. Run returns on the signal context, which sdk.SignalContext
// cancels on SIGTERM. The sequence modeled is a FORKED ddos-flowspec sent
// SIGTERM while the daemon runs: its connection to the engine is still open,
// no `bye` arrives, and the deferred stopResponder is the only call that can
// withdraw. An internal plugin never takes this route: the daemon calls
// sdk.HostOwnsSignals (cmd/ze/hub/main.go), so its signal context ends only by
// its own cancel, and the daemon ends it by closing its connection
// (process.Process.Stop), after which no withdrawal can reach the engine.
// METHOD: runEngine over a real sdk.Plugin pair, one announcement through the
// bus, then SIGTERM to the test process. The test registers its own SIGTERM
// handler first, so the signal never takes its default action and end the test
// binary, whatever else is registered. The engine stub keeps serving, and its
// record and store are read once runEngine returned.
// PREVENTS: a deferred exit that never withdraws, which no other test reached
// with a live rule: TestStopResponderWithdrawsUnderItsOwnContext calls
// stopResponder directly, and in TestByeWithdrawsTheLiveAnnouncement the `bye`
// has already withdrawn.
//
// MUTATION: with stopResponder(resp) removed from the deferred exit in
// runEngine, the withdrawal count is 0.
func TestRunEndingWithoutByeWithdrawsTheLiveAnnouncement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	e, bus := startEngineStub(t, nil)
	announceVictim(t, e, bus)
	if !e.store.holdsAnnouncementRecord() {
		t.Fatalf("setup: the announcement was not recorded under %s", announcementRecordKey)
	}

	received := make(chan os.Signal, 1)
	signal.Notify(received, syscall.SIGTERM)
	defer signal.Stop(received)
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM to the test process: %v", err)
	}
	select {
	case <-received:
	case <-ctx.Done():
		t.Fatalf("SIGTERM was not delivered: %v", ctx.Err())
	}

	select {
	case <-e.done:
	case <-ctx.Done():
		t.Fatalf("runEngine did not return after its signal context ended: %v", ctx.Err())
	}
	if e.exitCode != 0 {
		t.Fatalf("runEngine returned %d after its signal context ended, want 0", e.exitCode)
	}

	if got := countCommands(e.routeCommands(), "/flow del "); got != 1 {
		t.Fatalf("withdrawals for %s on the way out without a bye = %d, want 1 (commands: %v)",
			exitTestVictim, got, e.routeCommands())
	}
	if e.store.holdsAnnouncementRecord() {
		t.Fatalf("the record %s is still stored after the way-out withdrawal reached the engine (state calls: %v)",
			announcementRecordKey, e.stateCalls())
	}
}
