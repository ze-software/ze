// Design: docs/architecture/config/transaction-protocol.md -- shutdown while a transaction runs
// Related: main_reload.go -- reloadGate, fenceCommitReloads, stoodDownAnswer
// Related: config_upload.go -- configMutationOwner admits every commit through the gate

package hub

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/core/opererr"
)

// gateTestBound is how long a test waits for something the stop must do at
// once. The reloads these tests stand down wait on nothing else, so a wait
// that reaches this bound waited on a deadline the stop should have ended.
const gateTestBound = 5 * time.Second

// awaitGate waits for done, or fails the test with what.
func awaitGate(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(gateTestBound):
		t.Fatal(what)
	}
}

// awaitAnswer returns the answer a committing caller got, or fails the test.
func awaitAnswer(t *testing.T, answers <-chan error, what string) error {
	t.Helper()
	select {
	case err := <-answers:
		return err
	case <-time.After(gateTestBound):
		t.Fatal(what)
		return nil
	}
}

// fence runs the daemon's stop order for commit reloads with no grace, and
// returns a channel that closes when it has returned.
func fence(owner *configMutationOwner) <-chan struct{} {
	fenced := make(chan struct{})
	go func() {
		fenceCommitReloads(owner.runtime.Load().window, &owner.gate, 0)
		close(fenced)
	}()
	return fenced
}

// requireStopAnswer fails unless err is the operator's answer for a change the
// daemon's stop interrupted: the problem, and the fix alone on the last line.
func requireStopAnswer(t *testing.T, err error) {
	t.Helper()
	if _, ok := errors.AsType[*opererr.Error](err); !ok {
		t.Fatalf("answer = %v, want an operator error", err)
	}
	text := err.Error()
	if !strings.HasPrefix(text, stoodDownProblem+"\n") {
		t.Fatalf("answer does not open with the interruption:\n%s", text)
	}
	if !strings.HasSuffix(text, "\nfix: "+stoodDownFix) {
		t.Fatalf("answer does not end with its fix line:\n%s", text)
	}
}

// TestReloadGateStandsDownEveryAdmittedOperation proves the gate is the one
// owner of the cancel of every operation it admitted.
//
// VALIDATES: standDown ends the shutdown scope of each running operation with
// transaction.ErrShutdown and closes admission; an operation that left before
// it was never marked as stopped by a shutdown; close then joins.
// PREVENTS: a commit reload running at daemon stop with no cancel anyone holds.
func TestReloadGateStandsDownEveryAdmittedOperation(t *testing.T) {
	t.Parallel()
	gate := &reloadGate{}

	finished, leaveFinished, admitted := gate.admit(context.Background())
	if !admitted {
		t.Fatal("an open gate must admit an operation")
	}
	leaveFinished()

	first, leaveFirst, admitted := gate.admit(context.Background())
	if !admitted {
		t.Fatal("an open gate must admit a second operation")
	}
	second, leaveSecond, admitted := gate.admit(context.Background())
	if !admitted {
		t.Fatal("an open gate must admit a third operation")
	}
	if first.Err() != nil {
		t.Fatalf("an admitted operation's context ended before the stop: %v", context.Cause(first))
	}

	gate.standDown()

	for name, ctx := range map[string]context.Context{"first": first, "second": second} {
		if cause := context.Cause(ctx); !errors.Is(cause, transaction.ErrShutdown) {
			t.Fatalf("%s operation: cause = %v, want %v", name, cause, transaction.ErrShutdown)
		}
	}
	if cause := context.Cause(finished); errors.Is(cause, transaction.ErrShutdown) {
		t.Fatalf("an operation that left before the stop reads as stopped by shutdown: %v", cause)
	}
	if _, _, admitted := gate.admit(context.Background()); admitted {
		t.Fatal("a gate that stood its operations down must refuse a new one")
	}

	closed := make(chan struct{})
	go func() {
		gate.close()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("close returned while stood-down operations had not left")
	default:
	}
	leaveFirst()
	leaveSecond()
	awaitGate(t, closed, "close did not return after every operation left")
}

// TestStopStandsDownACommitReloadMidApply runs the daemon's stop order while a
// commit reload waits on a plugin that never answers its apply.
//
// Method: the reload seam stands for the plugin server's reload. It blocks
// until its context ends, as a transaction waiting on a silent plugin does,
// and answers with the context's cause, which is what the coordinator reads
// to tell a shutdown from an ordinary cancellation (the transaction package's
// TestCommitStoodDownDuringApplyEmitsNoRollback proves what it then does).
//
// VALIDATES: the stop returns once it stood the reload down, the reload's
// context carries transaction.ErrShutdown, and the committing caller is
// answered that the daemon's stop interrupted the change.
// PREVENTS: a stop that waits out a commit's apply deadline, and a caller that
// hangs or reads a bare "context canceled".
func TestStopStandsDownACommitReloadMidApply(t *testing.T) {
	t.Parallel()
	owner := &configMutationOwner{}
	applying := make(chan struct{})
	if err := owner.bind(nil, "", nil, func(ctx context.Context) error {
		close(applying)
		<-ctx.Done()
		return context.Cause(ctx)
	}); err != nil {
		t.Fatal(err)
	}

	answers := make(chan error, 1)
	go func() { answers <- owner.Reload(context.Background()) }()
	awaitGate(t, applying, "the commit reload did not start")

	awaitGate(t, fence(owner), "the stop did not return: it waited on a commit reload it should have stood down")

	err := awaitAnswer(t, answers, "the committing caller got no answer")
	requireStopAnswer(t, err)
	if !errors.Is(err, transaction.ErrShutdown) {
		t.Fatalf("answer = %v, want it to keep %v as its evidence", err, transaction.ErrShutdown)
	}
	if err := owner.Reload(context.Background()); !errors.Is(err, errDaemonStopping) {
		t.Fatalf("a commit after the stop = %v, want %v", err, errDaemonStopping)
	}
}

// TestStopEndsACommitRollbackTheCallerCancelStarted cancels the committing
// caller's context the ordinary way, which starts a rollback, and then runs
// the daemon's stop order.
//
// Method: the reload seam waits for its context to end, then waits under the
// rollback context the transaction package builds from it
// (transaction.RollbackContext, an hour's bound) and reports why that ended.
//
// VALIDATES: the caller going away does not end the rollback (its context
// ends only when the stop arrives, and the cause is then the shutdown, not
// the caller's cancellation), the stop returns, and the caller is answered
// with the interruption although its own context ended first.
// PREVENTS: the gate's scope making an ordinary cancellation abandon a
// rollback, and a stop waiting the rollback bound of a commit.
func TestStopEndsACommitRollbackTheCallerCancelStarted(t *testing.T) {
	t.Parallel()
	owner := &configMutationOwner{}
	applying := make(chan struct{})
	rollingBack := make(chan struct{})
	rollbackEnded := make(chan error, 1)
	if err := owner.bind(nil, "", nil, func(ctx context.Context) error {
		close(applying)
		<-ctx.Done()
		applyEnded := context.Cause(ctx)
		rollback, release := transaction.RollbackContext(ctx, time.Hour)
		defer release()
		close(rollingBack)
		<-rollback.Done()
		cause := context.Cause(rollback)
		rollbackEnded <- cause
		return errors.Join(applyEnded, cause)
	}); err != nil {
		t.Fatal(err)
	}

	caller, cancelCaller := context.WithCancel(context.Background())
	defer cancelCaller()
	answers := make(chan error, 1)
	go func() { answers <- owner.Reload(caller) }()
	awaitGate(t, applying, "the commit reload did not start")

	cancelCaller()
	awaitGate(t, rollingBack, "the caller's cancel did not end the apply")

	awaitGate(t, fence(owner), "the stop did not return: it waited on a rollback it should have ended")

	select {
	case cause := <-rollbackEnded:
		if !errors.Is(cause, transaction.ErrShutdown) {
			t.Fatalf("the rollback ended with %v, want %v: something other than the stop ended it", cause, transaction.ErrShutdown)
		}
	default:
		t.Fatal("the stop returned while the rollback was still running")
	}
	err := awaitAnswer(t, answers, "the committing caller got no answer")
	requireStopAnswer(t, err)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("answer = %v, want it to keep the caller's cancellation as evidence", err)
	}
}

// TestStopStandsDownAnUploadOnTheConfirmWindowWorker runs the daemon's stop
// order while an SSH upload's commit runs on the confirmed-commit window's
// worker and its reload waits on a silent plugin.
//
// VALIDATES: the stop is bounded although it stops the window before it
// closes the gate: the window's worker is running the commit, so the cancel
// must reach the reload while the stop still waits for that worker. The
// upload is answered with the interruption, with no cause quoted.
// PREVENTS: the stop parked on the window's worker for a commit's whole
// deadline, before it ever reaches the gate that holds the cancel.
func TestStopStandsDownAnUploadOnTheConfirmWindowWorker(t *testing.T) {
	previous := daemonConfirmWindow.Swap(nil)
	t.Cleanup(func() { daemonConfirmWindow.Store(previous) })
	owner := &configMutationOwner{}
	path := "router.conf"
	store := storage.BindConfigSource(storage.CreateMemory(), path, storage.ConfigSourceStored)
	if err := initializeConfigSource(store, path, []byte(uploadWindowBefore)); err != nil {
		t.Fatal(err)
	}
	window := startConfirmWindow(store, path, func() error { return owner.Reload(context.Background()) })
	t.Cleanup(func() {
		stopConfirmWindow(window)
		owner.close()
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	applying := make(chan struct{})
	if err := owner.bind(store, path, window, func(ctx context.Context) error {
		close(applying)
		<-ctx.Done()
		return context.Cause(ctx)
	}); err != nil {
		t.Fatal(err)
	}

	answers := make(chan error, 1)
	go func() { answers <- owner.Upload(context.Background(), []byte(uploadWindowAfter)) }()
	awaitGate(t, applying, "the upload's commit reload did not start")

	awaitGate(t, fence(owner), "the stop did not return: it waited on the window worker's commit")

	err := awaitAnswer(t, answers, "the uploading caller got no answer")
	requireStopAnswer(t, err)
	if want := opererr.New(stoodDownProblem, stoodDownWhy, stoodDownFix).Error(); err.Error() != want {
		t.Fatalf("upload answer:\n%s\nwant, with no cause quoted:\n%s", err, want)
	}
}

// TestCommitFailureBeforeTheStopKeepsItsOwnAnswer is the negative of the
// tests above.
//
// VALIDATES: a commit reload that fails for a reason of its own, with no stop,
// is answered with that reason unchanged.
// PREVENTS: every failed commit read as a daemon stop.
func TestCommitFailureBeforeTheStopKeepsItsOwnAnswer(t *testing.T) {
	t.Parallel()
	owner := &configMutationOwner{}
	refusal := errors.New("plugin bgp refused the config")
	if err := owner.bind(nil, "", nil, func(context.Context) error { return refusal }); err != nil {
		t.Fatal(err)
	}

	err := owner.Reload(context.Background())
	if !errors.Is(err, refusal) {
		t.Fatalf("answer = %v, want the reload's own refusal %v", err, refusal)
	}
	if err.Error() != refusal.Error() {
		t.Fatalf("answer text = %q, want the refusal's own text %q", err, refusal)
	}
	if _, ok := errors.AsType[*opererr.Error](err); ok {
		t.Fatalf("an ordinary failure was answered as an operator stop refusal: %v", err)
	}
}
