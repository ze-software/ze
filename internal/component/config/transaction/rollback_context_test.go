// Design: docs/architecture/config/transaction-protocol.md -- shutdown during rollback, the commit bound, the shared rollback bound
// Related: rollback_context.go -- RollbackContext, WithShutdownScope
// Related: orchestrator.go -- rollBack, runOperationPath, handleRollbackAck

package transaction

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// restartRecorder is a RestartFunc that records the plugins it was asked to
// restart. Safe for concurrent use.
type restartRecorder struct {
	mu    sync.Mutex
	names []string
}

func (r *restartRecorder) restart(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = append(r.names, name)
	return nil
}

func (r *restartRecorder) restarted() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.names...)
}

// rollingBackCoordinator returns a coordinator for bgp and iface whose apply
// deadline is an hour, so its rollback bound is three hours: a test that
// returns in seconds returned because something other than the bound ended
// the rollback.
func rollingBackCoordinator(t *testing.T, gw *testGateway, restart RestartFunc) (*TxCoordinator, map[string][]DiffSection) {
	t.Helper()
	orch, err := NewTxCoordinator(gw, []Participant{
		{Name: "bgp", ConfigRoots: []string{"bgp"}},
		{Name: "iface", ConfigRoots: []string{"interface"}},
	}, restart)
	if err != nil {
		t.Fatalf("NewTxCoordinator: %v", err)
	}
	orch.SetApplyDeadlineOverride(time.Hour)
	return orch, map[string][]DiffSection{
		"bgp":       {{Root: "bgp"}},
		"interface": {{Root: "interface"}},
	}
}

// awaitResult returns Execute's result, or fails the test when it does not
// arrive: the rollback bound of these tests is three hours, so a result that
// is late was waiting on that bound.
func awaitResult(t *testing.T, resultCh <-chan *TxResult, what string) *TxResult {
	t.Helper()
	select {
	case result := <-resultCh:
		return result
	case <-time.After(5 * time.Second):
		t.Fatalf("Execute did not return: %s", what)
		return nil
	}
}

// TestShutdownDuringRollbackEndsTheWait drives a transaction into its rollback
// with one participant that never acknowledges, then cancels with ErrShutdown.
//
// VALIDATES: a shutdown ends a rollback that waits on a silent participant.
// PREVENTS: the daemon's stop waiting the whole rollback bound (three times
// the apply budget) for a plugin it is about to stop.
func TestShutdownDuringRollbackEndsTheWait(t *testing.T) {
	gw := newTestGateway()
	bgp := testParticipant{name: "bgp", configRoots: []string{"bgp"}, applyErr: "refused"}
	iface := testParticipant{name: "iface", configRoots: []string{"interface"}}
	restarts := &restartRecorder{}
	orch, diffs := rollingBackCoordinator(t, gw, restarts.restart)

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
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
	cancel(ErrShutdown)

	result := awaitResult(t, resultCh, "a shutdown did not end the rollback ack wait")
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

// TestShutdownReachesARollbackAnOrdinaryCancelStarted cancels the caller's
// context the ordinary way during apply, which starts a rollback that
// outlives that cancellation, and then stands the transaction down through
// its shutdown scope. The context's own cause stays the ordinary one.
//
// VALIDATES: an ordinary cancel does not abandon the rollback (it is still
// waiting when the shutdown arrives), and the shutdown still ends it.
// PREVENTS: a reload whose own deadline expired holding the daemon's stop for
// the rollback bound because its context already carried another cause.
func TestShutdownReachesARollbackAnOrdinaryCancelStarted(t *testing.T) {
	gw := newTestGateway()
	bgp := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	iface := testParticipant{name: "iface", configRoots: []string{"interface"}}
	restarts := &restartRecorder{}
	orch, diffs := rollingBackCoordinator(t, gw, restarts.restart)

	caller, cancelCaller := context.WithCancel(context.Background())
	defer cancelCaller()
	ctx, standDown := WithShutdownScope(caller)
	defer standDown(nil)
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
		t.Fatalf("an ordinary cancel abandoned the rollback: state %s, err %v", result.State, result.Err)
	default:
	}
	if cause := context.Cause(ctx); errors.Is(cause, ErrShutdown) {
		t.Fatalf("cause = %v before the shutdown, want the ordinary cancellation", cause)
	}

	standDown(ErrShutdown)

	result := awaitResult(t, resultCh, "a shutdown did not reach a rollback started by an ordinary cancel")
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

// TestBrokenRollbackAckAfterShutdownRestartsNothing hands the coordinator a
// CodeBroken rollback ack under a rollback context a shutdown has ended, which
// is what the ack wait does when the ack and the shutdown are both ready.
//
// VALIDATES: a CodeBroken ack is recorded and restarts nothing once shutdown began.
// PREVENTS: the stopping daemon forking a plugin it is about to stop.
func TestBrokenRollbackAckAfterShutdownRestartsNothing(t *testing.T) {
	gw := newTestGateway()
	restarts := &restartRecorder{}
	orch, _ := rollingBackCoordinator(t, gw, restarts.restart)

	parent, cancel := context.WithCancelCause(context.Background())
	rollbackCtx, release := RollbackContext(parent, time.Hour)
	defer release()
	cancel(ErrShutdown)

	orch.handleRollbackAck(rollbackCtx, RollbackAck{TransactionID: orch.TransactionID(), Plugin: "iface", Code: CodeBroken})

	if got := restarts.restarted(); len(got) != 0 {
		t.Fatalf("restarted %v after shutdown, want none", got)
	}
	orch.mu.Lock()
	_, recorded := orch.rollbackAcks["iface"]
	orch.mu.Unlock()
	if !recorded {
		t.Fatal("the rollback ack was not recorded")
	}
	select {
	case <-rollbackCtx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the rollback context did not end after its parent was shut down")
	}
	if !errors.Is(context.Cause(rollbackCtx), ErrShutdown) {
		t.Fatalf("rollback context cause = %v, want ErrShutdown", context.Cause(rollbackCtx))
	}
}

// TestRollbackContextOutlivesAnOrdinaryCancel proves the other half of the
// contract: a plain cancel of the parent leaves the rollback context running.
//
// VALIDATES: an ordinary caller cancellation does not end a rollback context.
// PREVENTS: a caller deadline abandoning a rollback the running daemon needs.
func TestRollbackContextOutlivesAnOrdinaryCancel(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	rollbackCtx, release := RollbackContext(parent, time.Hour)
	defer release()
	cancel()
	if canceledByShutdown(rollbackCtx) {
		t.Fatal("canceledByShutdown answered true for an ordinary cancel")
	}

	// A context derived after the parent ended is the case a rollback is in:
	// the watcher of an already-ended parent runs at once, and it must leave
	// the rollback running. Its deadline is an hour away, so the only thing
	// that could end it inside this test is that watcher.
	live, releaseLive := RollbackContext(parent, time.Hour)
	defer releaseLive()
	watched := make(chan struct{})
	stop := context.AfterFunc(parent, func() { close(watched) })
	defer stop()
	<-watched
	if err := live.Err(); err != nil {
		t.Fatalf("a rollback context derived from a canceled parent ended: %v (cause %v)", err, context.Cause(live))
	}
	if err := rollbackCtx.Err(); err != nil {
		t.Fatalf("an ordinary cancel of the parent ended the rollback context: %v", err)
	}
}

// operationPathFixture wires one decomposed operation owned by iface, with
// verify acknowledged at once. The caller wires apply, commit and rollback.
func operationPathFixture(t *testing.T, gw *testGateway, participants []testParticipant) *TxCoordinator {
	t.Helper()
	orch := newTestOrchestrator(t, gw, participants)
	orch.SetOperationPlanner(func(context.Context, OperationPlanRequest) ([]ConfigOperation, error) {
		return []ConfigOperation{{
			ID:     "op-add-eth0",
			Root:   "interface",
			Owner:  "iface",
			Type:   testOpAddInterface,
			Verb:   VerbCreate,
			Target: ResourceRef{Kind: ResourceInterface, Name: "eth0"},
		}}, nil
	})
	for i := range participants {
		p := participants[i]
		gw.SubscribeConfigEvent(EventVerifyFor(p.name), func([]byte) {
			p.respondVerify(gw, orch.TransactionID())
		})
	}
	gw.SubscribeConfigEvent(EventOperationVerifyFor("iface"), func(payload []byte) {
		var ev ConfigOperationVerifyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		raw, err := json.Marshal(ConfigOperationVerifyAck{
			TransactionID: ev.TransactionID, Plugin: "iface", OperationID: ev.Operation.ID, Status: CodeOK,
		})
		if err != nil {
			return
		}
		gw.mustEmit(EventOperationVerifyOK, raw)
	})
	return orch
}

// TestApplyThatUsesItsBudgetStillCommits acknowledges the apply halfway
// through the apply deadline and the commit only after the apply deadline has
// passed, which is inside the commit's own interval.
//
// VALIDATES: the commit phase is bounded by an interval of its own.
// PREVENTS: a fully applied transaction rolled back because its commit acks
// were given only what the apply left of the apply deadline.
func TestApplyThatUsesItsBudgetStillCommits(t *testing.T) {
	const applyDeadline = 2 * time.Second

	gw := newTestGateway()
	iface := testParticipant{name: "iface", configRoots: []string{"interface"}}
	orch := operationPathFixture(t, gw, []testParticipant{iface})
	orch.SetApplyDeadlineOverride(applyDeadline)

	var mu sync.Mutex
	var applyDeadlineMS, commitDeadlineMS int64
	gw.SubscribeConfigEvent(EventOperationApplyFor("iface"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		mu.Lock()
		applyDeadlineMS = ev.DeadlineMS
		mu.Unlock()
		raw, err := json.Marshal(ConfigOperationApplyAck{
			TransactionID: ev.TransactionID, Plugin: "iface", OperationID: ev.Operation.ID, Status: CodeOK,
		})
		if err != nil {
			return
		}
		// The apply takes half its budget.
		acked := time.UnixMilli(ev.DeadlineMS).Add(-applyDeadline / 2)
		time.AfterFunc(time.Until(acked), func() { gw.mustEmit(EventOperationApplyOK, raw) })
	})
	gw.SubscribeConfigEvent(EventOperationCommitFor("iface"), func(payload []byte) {
		var ev ConfigOperationCommitEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		mu.Lock()
		commitDeadlineMS = ev.DeadlineMS
		applyEnds := time.UnixMilli(applyDeadlineMS)
		mu.Unlock()
		raw, err := json.Marshal(ConfigOperationCommitAck{TransactionID: ev.TransactionID, Plugin: "iface", Status: CodeOK})
		if err != nil {
			return
		}
		// The commit answers once the APPLY deadline is behind it.
		acked := applyEnds.Add(300 * time.Millisecond)
		time.AfterFunc(time.Until(acked), func() { gw.mustEmit(EventOperationCommitOK, raw) })
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result := orch.Execute(ctx, map[string][]DiffSection{"interface": {{Root: "interface"}}})

	if result.State != StateCommitted {
		t.Fatalf("state = %s (err=%v), want %s", result.State, result.Err, StateCommitted)
	}
	if len(gw.findEmitted(EventRollback)) != 0 {
		t.Fatal("a rollback was emitted for a transaction whose commit answered inside its own interval")
	}
	mu.Lock()
	defer mu.Unlock()
	// The commit event advertises the commit's own deadline, which is what the
	// bridge turns into the commit RPC's context deadline.
	if commitDeadlineMS <= applyDeadlineMS {
		t.Fatalf("commit deadline-ms = %d, want it after the apply deadline-ms %d", commitDeadlineMS, applyDeadlineMS)
	}
}

// TestSectionRestoreRunsUnderTheBudgetItIsHanded gives the section restore a
// rollback context that ends in 50 ms while the coordinator's own rollback
// deadline is three hours, with a participant that never acknowledges.
//
// VALIDATES: the section restore after an operation undo adds no bound of its own.
// PREVENTS: the operation undo and the section restore each taking a full
// rollback deadline, twice the documented bound.
func TestSectionRestoreRunsUnderTheBudgetItIsHanded(t *testing.T) {
	gw := newTestGateway()
	orch := newTestOrchestrator(t, gw, []testParticipant{{name: "dns", configRoots: []string{"dns"}}})
	orch.mu.Lock()
	orch.applyDeadline = time.Hour
	orch.mu.Unlock()

	ctx, release := RollbackContext(context.Background(), 50*time.Millisecond)
	defer release()
	handed, _ := ctx.Deadline()

	errCh := make(chan error, 1)
	go func() { errCh <- orch.restoreSectionsAfterOperationUndo(ctx, errors.New("apply refused")) }()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("restore returned nil, want the cause and the rollback outcome")
		}
		if !strings.Contains(err.Error(), "dns did not answer the rollback in time") {
			t.Fatalf("err = %q, want it to name dns as unanswered", err.Error())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the section restore outlived the rollback budget it was handed")
	}

	emitted := gw.findEmitted(EventRollback)
	if len(emitted) != 1 {
		t.Fatalf("rollback emitted %d time(s), want 1", len(emitted))
	}
	var ev RollbackEvent
	if err := json.Unmarshal(emitted[0].Payload, &ev); err != nil {
		t.Fatalf("unmarshal rollback event: %v", err)
	}
	if ev.DeadlineMS != handed.UnixMilli() {
		t.Fatalf("rollback deadline-ms = %d, want the handed deadline %d", ev.DeadlineMS, handed.UnixMilli())
	}
}

// TestOperationUndoAndSectionRestoreShareOneBound runs the whole operation
// path into a rollback: iface's operation applies, dns's section apply is
// refused, the operation is undone and the sections are restored.
//
// VALIDATES: the operation undo and the section restore advertise one deadline.
// PREVENTS: the section restore starting a fresh rollback deadline after the
// operation undo spent its own.
func TestOperationUndoAndSectionRestoreShareOneBound(t *testing.T) {
	gw := newTestGateway()
	iface := testParticipant{name: "iface", configRoots: []string{"interface"}}
	dns := testParticipant{name: "dns", configRoots: []string{"dns"}, applyErr: "port in use"}
	orch := operationPathFixture(t, gw, []testParticipant{iface, dns})
	orch.SetApplyDeadlineOverride(10 * time.Second)

	gw.SubscribeConfigEvent(EventOperationApplyFor("iface"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		raw, err := json.Marshal(ConfigOperationApplyAck{
			TransactionID: ev.TransactionID, Plugin: "iface", OperationID: ev.Operation.ID, Status: CodeOK,
		})
		if err != nil {
			return
		}
		gw.mustEmit(EventOperationApplyOK, raw)
	})
	gw.SubscribeConfigEvent(EventApplyFor("dns"), func([]byte) {
		dns.respondApply(gw, orch.TransactionID())
	})
	var undoDeadlineMS, restoreDeadlineMS int64
	gw.SubscribeConfigEvent(EventOperationRollbackFor("iface"), func(payload []byte) {
		var ev ConfigOperationRollbackEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		undoDeadlineMS = ev.DeadlineMS
		for i := range ev.Operations {
			raw, err := json.Marshal(ConfigOperationRollbackAck{
				TransactionID: ev.TransactionID, Plugin: "iface", OperationID: ev.Operations[i].ID, Status: CodeOK,
			})
			if err != nil {
				return
			}
			gw.mustEmit(EventOperationRollbackOK, raw)
		}
	})
	gw.SubscribeConfigEvent(EventRollback, func(payload []byte) {
		var ev RollbackEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		restoreDeadlineMS = ev.DeadlineMS
		iface.respondRollback(gw, orch.TransactionID())
		dns.respondRollback(gw, orch.TransactionID())
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result := orch.Execute(ctx, map[string][]DiffSection{
		"interface": {{Root: "interface"}},
		"dns":       {{Root: "dns"}},
	})

	if result.State != StateRolledBack {
		t.Fatalf("state = %s (err=%v), want %s", result.State, result.Err, StateRolledBack)
	}
	if undoDeadlineMS == 0 {
		t.Fatal("the applied operation was not undone, so the test proved nothing about the shared bound")
	}
	if restoreDeadlineMS != undoDeadlineMS {
		t.Fatalf("section restore deadline-ms = %d, operation undo deadline-ms = %d: want one shared bound",
			restoreDeadlineMS, undoDeadlineMS)
	}
}
