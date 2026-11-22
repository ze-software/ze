// Design: docs/architecture/config/transaction-protocol.md -- shutdown while a transaction runs
// Related: rollback_context.go -- WithShutdownScope, a scope opened under another scope
// Related: rollback_context_test.go -- the same proofs for a transaction under one scope

package transaction

import (
	"context"
	"errors"
	"testing"
	"time"
)

// commitContext builds the context a commit-driven reload hands the
// coordinator, layer by layer as the daemon does:
//
//   - the caller's context (a CLI, web or API request);
//   - the scope the hub's reload gate opens for the admitted commit, whose
//     cancel the daemon's stop holds (cmd/ze/hub, reloadGate.admit);
//   - the reload's own 30-second deadline, here an hour so no test ends on it;
//   - the scope the plugin server opens for the transaction.
//
// standDown is the hub's cancel. cancelCaller is the caller going away.
func commitContext(t *testing.T) (ctx context.Context, standDown context.CancelCauseFunc, cancelCaller context.CancelFunc) {
	t.Helper()
	caller, cancelCaller := context.WithCancel(context.Background())
	t.Cleanup(cancelCaller)
	gated, standDown := WithShutdownScope(caller)
	t.Cleanup(func() { standDown(nil) })
	bounded, cancelBound := context.WithTimeout(gated, time.Hour)
	t.Cleanup(cancelBound)
	ctx, endTransaction := WithShutdownScope(bounded)
	t.Cleanup(func() { endTransaction(nil) })
	return ctx, standDown, cancelCaller
}

// requireShutdownAbort fails unless result is a shutdown abort that restarted
// no plugin.
func requireShutdownAbort(t *testing.T, result *TxResult, restarts *restartRecorder) {
	t.Helper()
	if result.State != StateAborted {
		t.Fatalf("state = %s (err=%v), want %s", result.State, result.Err, StateAborted)
	}
	if !errors.Is(result.Err, ErrShutdown) {
		t.Fatalf("err = %v, want it to wrap ErrShutdown", result.Err)
	}
	if got := restarts.restarted(); len(got) != 0 {
		t.Fatalf("restarted %v during shutdown, want none", got)
	}
}

// TestCommitStoodDownDuringApplyEmitsNoRollback stops the daemon while a
// commit-driven transaction waits on a plugin that never answers its apply.
//
// VALIDATES: the hub's cancel, which ends the OUTER scope of a commit, stands
// the transaction down mid-apply: it returns at once, as a shutdown abort,
// with no rollback event and no plugin restarted.
// PREVENTS: a daemon stop waiting the whole apply deadline of a commit, then
// its rollback deadline, on one silent plugin.
func TestCommitStoodDownDuringApplyEmitsNoRollback(t *testing.T) {
	gw := newTestGateway()
	bgp := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	iface := testParticipant{name: "iface", configRoots: []string{"interface"}}
	restarts := &restartRecorder{}
	orch, diffs := rollingBackCoordinator(t, gw, restarts.restart)

	ctx, standDown, _ := commitContext(t)
	resultCh := make(chan *TxResult, 1)
	go func() { resultCh <- orch.Execute(ctx, diffs) }()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	bgp.respondVerify(gw, orch.TransactionID())
	iface.respondVerify(gw, orch.TransactionID())
	// The apply is out and no plugin answers it.
	waitForEmit(t, gw, EventApplyFor("bgp"))

	standDown(ErrShutdown)

	result := awaitResult(t, resultCh, "the hub's cancel did not end the apply wait of a commit")
	requireShutdownAbort(t, result, restarts)
	if rollbacks := gw.findEmitted(EventRollback); len(rollbacks) != 0 {
		t.Fatalf("a commit stood down mid-apply emitted %d rollback events, want none", len(rollbacks))
	}
}

// TestCommitStoodDownDuringRollbackEndsTheWait stops the daemon while a
// commit-driven transaction rolls back and one plugin never acknowledges.
//
// VALIDATES: the hub's cancel reaches a rollback through two nested scopes:
// the wait ends at once, as a shutdown abort, and no plugin is restarted.
// PREVENTS: a daemon stop waiting the rollback bound (three times the apply
// budget) of a commit for a plugin it is about to stop.
func TestCommitStoodDownDuringRollbackEndsTheWait(t *testing.T) {
	gw := newTestGateway()
	bgp := testParticipant{name: "bgp", configRoots: []string{"bgp"}, applyErr: "refused"}
	iface := testParticipant{name: "iface", configRoots: []string{"interface"}}
	restarts := &restartRecorder{}
	orch, diffs := rollingBackCoordinator(t, gw, restarts.restart)

	ctx, standDown, _ := commitContext(t)
	resultCh := make(chan *TxResult, 1)
	go func() { resultCh <- orch.Execute(ctx, diffs) }()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	bgp.respondVerify(gw, orch.TransactionID())
	iface.respondVerify(gw, orch.TransactionID())
	waitForEmit(t, gw, EventApplyFor("bgp"))
	bgp.respondApply(gw, orch.TransactionID())

	// The rollback is out and iface never answers it.
	waitForEmit(t, gw, EventRollback)
	bgp.respondRollback(gw, orch.TransactionID())

	standDown(ErrShutdown)

	result := awaitResult(t, resultCh, "the hub's cancel did not end the rollback ack wait of a commit")
	requireShutdownAbort(t, result, restarts)
}

// TestCommitCallerGoingAwayDoesNotAbandonTheRollback cancels the committing
// caller's context the ordinary way during apply, and only later stops the
// daemon.
//
// VALIDATES: a caller that goes away starts a rollback and does not end it:
// the coordinator is still waiting for the silent plugin, and the context's
// cause is not a shutdown, until the hub's cancel arrives. Then it ends.
// PREVENTS: the hub's scope turning every end of a commit's caller into a
// shutdown, which would leave plugins half-applied in a daemon that keeps
// running.
func TestCommitCallerGoingAwayDoesNotAbandonTheRollback(t *testing.T) {
	gw := newTestGateway()
	bgp := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	iface := testParticipant{name: "iface", configRoots: []string{"interface"}}
	restarts := &restartRecorder{}
	orch, diffs := rollingBackCoordinator(t, gw, restarts.restart)

	ctx, standDown, cancelCaller := commitContext(t)
	resultCh := make(chan *TxResult, 1)
	go func() { resultCh <- orch.Execute(ctx, diffs) }()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	bgp.respondVerify(gw, orch.TransactionID())
	iface.respondVerify(gw, orch.TransactionID())
	waitForEmit(t, gw, EventApplyFor("bgp"))
	cancelCaller()

	// The ordinary cancel started the rollback. bgp answers it and iface
	// does not, so the coordinator is waiting under its three-hour bound.
	waitForEmit(t, gw, EventRollback)
	bgp.respondRollback(gw, orch.TransactionID())
	select {
	case result := <-resultCh:
		t.Fatalf("the caller going away abandoned the rollback: state %s, err %v", result.State, result.Err)
	default:
	}
	if cause := context.Cause(ctx); errors.Is(cause, ErrShutdown) {
		t.Fatalf("cause = %v before the daemon stopped, want the ordinary cancellation", cause)
	}

	standDown(ErrShutdown)

	result := awaitResult(t, resultCh, "the hub's cancel did not reach a rollback the caller's cancel started")
	requireShutdownAbort(t, result, restarts)
}
