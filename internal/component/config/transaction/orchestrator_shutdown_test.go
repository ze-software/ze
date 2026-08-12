// Related: orchestrator.go -- ErrShutdown and canceledByShutdown
// Related: ../../plugin/server/reload.go -- Server.stopTransaction sets the cause

package transaction

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestShutdownDuringApplySkipsRollbackAndRestart proves a transaction canceled
// with ErrShutdown does not fan a rollback out to plugins and does not restart
// any of them.
//
// VALIDATES: shutdown cancels the transaction instead of letting it infer a crash.
// PREVENTS: the daemon restarting plugins it is about to kill, and telling
// operators a plugin crashed while the process is exiting.
func TestShutdownDuringApplySkipsRollbackAndRestart(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}, rollbackCode: CodeBroken}

	var restarted []string
	restartFn := func(name string) error {
		restarted = append(restarted, name)
		return nil
	}
	orch, err := NewTxCoordinator(gw, []Participant{
		{Name: "bgp", ConfigRoots: []string{"bgp"}},
		{Name: "iface", ConfigRoots: []string{"interface"}},
	}, restartFn)
	if err != nil {
		t.Fatalf("NewTxCoordinator: %v", err)
	}

	diffs := map[string][]DiffSection{
		"bgp":       {{Root: "bgp"}},
		"interface": {{Root: "interface"}},
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	p1.respondVerify(gw, orch.TransactionID())
	p2.respondVerify(gw, orch.TransactionID())

	// The apply is in flight and no plugin has acked it. This is the state
	// Server.Stop finds when a reload is running: stand the transaction down
	// rather than closing its plugin connections underneath it.
	waitForEmit(t, gw, EventApplyFor("bgp"))
	cancel(ErrShutdown)

	select {
	case result := <-resultCh:
		if result.State != StateAborted {
			t.Fatalf("state = %s, want %s", result.State, StateAborted)
		}
		if !errors.Is(result.Err, ErrShutdown) {
			t.Fatalf("err = %v, want it to wrap ErrShutdown", result.Err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Execute did not return after shutdown cancellation")
	}

	if got := gw.findEmitted(EventRollback); len(got) != 0 {
		t.Fatalf("rollback emitted %d time(s) during shutdown, want 0", len(got))
	}
	if len(restarted) != 0 {
		t.Fatalf("restarted %v during shutdown, want none", restarted)
	}
}

// TestShutdownDuringVerifySkipsAbortPublish proves the verify phase takes the
// same route: a canceled transaction publishes nothing.
//
// VALIDATES: shutdown cancellation is not reported as a config commit failure.
// PREVENTS: a spurious commit-aborted entry on the report bus at shutdown.
func TestShutdownDuringVerifySkipsAbortPublish(t *testing.T) {
	gw := newTestGateway()
	orch := newTestOrchestrator(t, gw, []testParticipant{
		{name: "bgp", configRoots: []string{"bgp"}},
	})

	diffs := map[string][]DiffSection{"bgp": {{Root: "bgp"}}}

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	cancel(ErrShutdown)

	select {
	case result := <-resultCh:
		if result.State != StateAborted {
			t.Fatalf("state = %s, want %s", result.State, StateAborted)
		}
		if !errors.Is(result.Err, ErrShutdown) {
			t.Fatalf("err = %v, want it to wrap ErrShutdown", result.Err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Execute did not return after shutdown cancellation")
	}

	if got := gw.findEmitted(EventVerifyAbort); len(got) != 0 {
		t.Fatalf("verify-abort emitted %d time(s) during shutdown, want 0", len(got))
	}
	if issue := findReportError(reportCodeCommitAborted, orch.TransactionID()); issue != nil {
		t.Fatalf("report bus carries %s for a shutdown cancellation", reportCodeCommitAborted)
	}
}

// TestOrdinaryCancelDuringApplyStillRollsBack is the discriminating half of
// the pair above. Only ErrShutdown skips the rollback. A reload whose own
// 30-second deadline expires mid-apply leaves participants half-applied in a
// daemon that keeps running. That daemon must still be told to undo it.
//
// VALIDATES: the shutdown branch keys on the cause, not on cancellation.
// PREVENTS: widening the skip to every canceled transaction, which would
// silently drop the rollback a timed-out reload owes.
func TestOrdinaryCancelDuringApplyStillRollsBack(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1})

	diffs := map[string][]DiffSection{"bgp": {{Root: "bgp"}}}

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	p1.respondVerify(gw, orch.TransactionID())

	waitForEmit(t, gw, EventApplyFor("bgp"))
	cancel(nil) // an ordinary cancellation: cause is context.Canceled

	select {
	case result := <-resultCh:
		if result.State != StateRolledBack {
			t.Fatalf("state = %s, want %s", result.State, StateRolledBack)
		}
		if errors.Is(result.Err, ErrShutdown) {
			t.Fatalf("err = %v, must not wrap ErrShutdown", result.Err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Execute did not return after cancellation")
	}

	if got := gw.findEmitted(EventRollback); len(got) == 0 {
		t.Fatal("rollback not emitted for an ordinary cancellation")
	}
}
