// Design: docs/research/l2tpv2-ze-integration.md -- retained native release ownership

package ppp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/report"
)

var errDriverNativePending = errors.New("fixture native release remains unconfirmed")

// This owner models the already-bound factory gate and retained-unit status.
// Real kernel duplication and birth-tag proof belong to the kernel package.
type driverNativeLease struct {
	SessionHandle
	mu          sync.Mutex
	closeMu     sync.Mutex
	closeCount  int
	onClose     func()
	counterRead func() (SessionCounters, error)
	closed      bool
	pending     bool
	bound       bool
	birthTag    string
	stickyErr   error
	sealEntered chan struct{}
	sealResume  <-chan struct{}
	sealOnce    sync.Once
}

func (owner *driverNativeLease) BindNativeReleaseSeal(seal NativeReleaseSeal) error {
	tag := seal.NativeBirthTag()
	owner.mu.Lock()
	owner.bound = true
	owner.birthTag = tag
	owner.mu.Unlock()
	return nil
}

func (owner *driverNativeLease) NativeReleaseBound() bool {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.bound && !owner.closed
}

func (owner *driverNativeLease) NativeReleaseBirthTag() string {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.birthTag
}

func (owner *driverNativeLease) NativeReleasePending() bool {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.closed && owner.pending
}

func (owner *driverNativeLease) Counters() (SessionCounters, error) {
	if owner.counterRead != nil {
		return owner.counterRead()
	}
	return owner.SessionHandle.Counters()
}

func (owner *driverNativeLease) Close() error {
	owner.closeMu.Lock()
	defer owner.closeMu.Unlock()
	cause := owner.SessionHandle.Close()
	owner.mu.Lock()
	owner.closed = true
	owner.closeCount++
	entered, resume := owner.sealEntered, owner.sealResume
	owner.mu.Unlock()
	if entered != nil {
		owner.sealOnce.Do(func() { close(entered) })
	}
	if resume != nil {
		<-resume
	}
	owner.mu.Lock()
	pending := owner.pending
	stickyErr := owner.stickyErr
	onClose := owner.onClose
	owner.mu.Unlock()
	if onClose != nil {
		onClose()
	}
	if pending {
		return errors.Join(cause, stickyErr, errDriverNativePending)
	}
	return errors.Join(cause, stickyErr)
}

// A real admitted worker exits after borrowed I/O closes, but its native owner
// remains unconfirmed. The Driver must keep that exact lease strongly owned,
// even after StopSession has removed its lookup key and joined the worker.
func TestDriverRetainsNativePendingOwnerAfterWorkerExit(t *testing.T) {
	driver := newSessionTestDriver()
	budget, err := NewNativeReleaseBudget(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.ConfigureNativeReleaseBudget(budget); err != nil {
		t.Fatal(err)
	}
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, _, original := frameLeaseStart(t, 51)
	owner := &driverNativeLease{SessionHandle: original.SessionHandle, bound: true, birthTag: "native/test/51"}
	start.Dataplane = owner
	start.SubscriberID = "opaque/native-pending-owner"
	start.Attempt, err = NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	frameLeaseProxy(&start)
	t.Cleanup(func() {
		owner.mu.Lock()
		owner.pending = false
		owner.mu.Unlock()
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	driver.SessionsIn() <- start
	events := drainTwoEvents(t, driver.EventsOut(), time.Second)
	opened, ok := events[0].(EventLCPUp)
	if !ok {
		t.Fatalf("first admission event = %T, want LCP-up", events[0])
	}
	control := opened.Control
	if err := control.ReserveNativeRelease(); err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	owner.pending = true
	owner.mu.Unlock()
	if err := driver.StopSession(control); err != nil {
		t.Fatal(err)
	}
	if !owner.NativeReleasePending() {
		t.Fatal("fixture did not retain the native unit")
	}
	driver.mu.Lock()
	_, retained := driver.owners[owner]
	driver.mu.Unlock()
	if !retained {
		t.Fatal("Driver forgot a native-pending owner after its worker exited")
	}
	if !driver.HasPendingNativeRelease() {
		t.Fatal("outer owner cannot observe retained cleanup responsibility")
	}
	if nativeBudgetUsed(budget) != 1 {
		t.Fatal("unconfirmed native release freed its quota")
	}
	owner.mu.Lock()
	owner.pending = false
	owner.mu.Unlock()
	if err := control.RetryNativeRelease(); err != nil {
		t.Fatal(err)
	}
	if driver.HasPendingNativeRelease() {
		t.Fatal("confirmed release retained the outer Driver")
	}
	if nativeBudgetUsed(budget) != 0 {
		t.Fatal("confirmed native release leaked its shared quota")
	}
}

// Isolate Stop's own Close path from the worker's competing close path. The
// blocked seal represents anchor-owned cleanup; a concurrent Driver admission
// check must acquire d.mu before the seal is allowed to return.
func TestDriverStopDoesNotHoldDriverLockAcrossNativeSeal(t *testing.T) {
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	_, _, original := frameLeaseStart(t, 52)
	resume := make(chan struct{})
	owner := &driverNativeLease{
		SessionHandle: original.SessionHandle, bound: true,
		sealEntered: make(chan struct{}), sealResume: resume,
	}
	worker := &pppSession{dataplane: owner, logger: discardLogger(), sessStop: make(chan struct{})}
	key := "opaque/isolated-stop-lock-probe"
	worker.subscriberID = key
	worker.control = SessionControl{driver: driver, worker: worker}
	driver.mu.Lock()
	driver.sessions[key] = worker
	driver.owners[owner] = worker
	driver.mu.Unlock()
	stopped := make(chan struct{})
	go func() { defer close(stopped); driver.Stop() }()
	probeDone := make(chan struct{})
	probeStarted := false
	t.Cleanup(func() {
		if resume != nil {
			close(resume)
		}
		<-stopped
		if probeStarted {
			<-probeDone
		}
	})
	select {
	case <-owner.sealEntered:
	case <-time.After(time.Second):
		t.Fatal("Stop did not reach the native seal")
	}
	probeStarted = true
	go func() {
		func() {
			driver.mu.Lock()
			defer driver.mu.Unlock()
		}()
		close(probeDone)
	}()
	select {
	case <-probeDone:
	case <-time.After(time.Second):
		t.Fatal("Stop held the Driver lock while the native seal was blocked")
	}
	close(resume)
	resume = nil
	<-stopped
}

func nativeBudgetUsed(budget *NativeReleaseBudget) int {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	return budget.used
}

// The two actual admitted Drivers share ONE count, including while one old
// worker has exited with an uncertain native owner. No per-Driver allowance
// can admit the second remote obligation until exact old cleanup succeeds.
func TestNativeReleaseBudgetSharedAcrossDriversAndPendingExit(t *testing.T) {
	budget, err := NewNativeReleaseBudget(1)
	if err != nil {
		t.Fatal(err)
	}
	firstDriver, firstOwner, first := admitNativeBudgetSession(t, budget, 61)
	secondDriver, secondOwner, second := admitNativeBudgetSession(t, budget, 62)
	if err := first.ReserveNativeRelease(); err != nil {
		t.Fatal(err)
	}
	if err := first.ReserveNativeRelease(); err != nil {
		t.Fatalf("same worker reservation is not idempotent: %v", err)
	}
	if err := second.ReserveNativeRelease(); !errors.Is(err, ErrNativeReleaseBudgetExhausted) {
		t.Fatalf("second Driver reservation = %v, want shared quota exhausted", err)
	}
	if nativeBudgetUsed(budget) != 1 {
		t.Fatal("shared count does not represent exactly one native obligation")
	}
	firstOwner.mu.Lock()
	firstOwner.pending = true
	firstOwner.mu.Unlock()
	if err := firstDriver.StopSession(first); err != nil {
		t.Fatal(err)
	}
	if !firstDriver.HasPendingNativeRelease() {
		t.Fatal("retired Driver lost its outer retention signal")
	}
	if err := second.ReserveNativeRelease(); !errors.Is(err, ErrNativeReleaseBudgetExhausted) {
		t.Fatalf("pending owner allowed replacement native operation: %v", err)
	}
	firstOwner.mu.Lock()
	firstOwner.pending = false
	firstOwner.mu.Unlock()
	if err := first.RetryNativeRelease(); err != nil {
		t.Fatal(err)
	}
	firstOwner.mu.Lock()
	closes := firstOwner.closeCount
	firstOwner.mu.Unlock()
	if err := first.RetryNativeRelease(); err != nil {
		t.Fatal(err)
	}
	firstOwner.mu.Lock()
	after := firstOwner.closeCount
	firstOwner.mu.Unlock()
	if after != closes {
		t.Fatal("completed old callback closed its lease again")
	}
	if nativeBudgetUsed(budget) != 0 {
		t.Fatal("completed old release did not return shared capacity exactly once")
	}
	if err := second.ReserveNativeRelease(); err != nil {
		t.Fatalf("fresh Driver could not use returned shared capacity: %v", err)
	}
	if err := secondDriver.StopSession(second); err != nil {
		t.Fatal(err)
	}
	if secondOwner.NativeReleasePending() {
		t.Fatal("confirmed second owner remained native pending")
	}
	if nativeBudgetUsed(budget) != 0 {
		t.Fatal("fresh owner's confirmed retirement leaked quota")
	}
}

// An ACK delivered while run still owns the transport MUST NOT close active
// I/O. Exit always reconciles again, so that early ACK cannot strand a quota.
func TestNativeReleaseEarlyAcknowledgmentReconcilesAtWorkerExit(t *testing.T) {
	budget, err := NewNativeReleaseBudget(1)
	if err != nil {
		t.Fatal(err)
	}
	driver, owner, control := admitNativeBudgetSession(t, budget, 63)
	if err := control.ReserveNativeRelease(); err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	before := owner.closeCount
	owner.mu.Unlock()
	if err := control.RetryNativeRelease(); err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	after := owner.closeCount
	owner.mu.Unlock()
	if after != before {
		t.Fatal("early cleanup acknowledgment closed an active worker")
	}
	if !control.Active() {
		t.Fatal("early cleanup acknowledgment stopped active protocol I/O")
	}
	if err := driver.StopSession(control); err != nil {
		t.Fatal(err)
	}
	if driver.HasPendingNativeRelease() {
		t.Fatal("worker exit missed an earlier successful cleanup acknowledgment")
	}
	if nativeBudgetUsed(budget) != 0 {
		t.Fatal("early acknowledgment stranded shared native quota")
	}
}

// Both first-operation callers race the SAME admitted worker at a one-entry
// quota. Its single reservation must remain idempotent under concurrency.
func TestNativeReleaseConcurrentFirstReservationCountsOnce(t *testing.T) {
	budget, err := NewNativeReleaseBudget(1)
	if err != nil {
		t.Fatal(err)
	}
	driver, _, control := admitNativeBudgetSession(t, budget, 67)
	start := make(chan struct{})
	result := make(chan error, 2)
	var callers sync.WaitGroup
	for range 2 {
		callers.Go(func() { <-start; result <- control.ReserveNativeRelease() })
	}
	close(start)
	callers.Wait()
	for range 2 {
		if err := <-result; err != nil {
			t.Fatalf("same-worker concurrent reservation = %v", err)
		}
	}
	if nativeBudgetUsed(budget) != 1 {
		t.Fatal("concurrent callers consumed more than their worker's one reservation")
	}
	if err := driver.StopSession(control); err != nil {
		t.Fatal(err)
	}
	if nativeBudgetUsed(budget) != 0 {
		t.Fatal("concurrent reservation leaked capacity on confirmed retirement")
	}
}

// A new actual Driver admission deliberately reuses the SAME opaque ID AND
// comparable owner pointer with a fresh physical frame stream. A completed
// old callback must not close that successor or release its new quota.
func TestNativeReleaseCompletedOldRetryCannotCloseSameLeaseSuccessor(t *testing.T) {
	budget, err := NewNativeReleaseBudget(1)
	if err != nil {
		t.Fatal(err)
	}
	driver, owner, old := admitNativeBudgetSession(t, budget, 68)
	if err := old.ReserveNativeRelease(); err != nil {
		t.Fatal(err)
	}
	if err := driver.StopSession(old); err != nil {
		t.Fatal(err)
	}
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	if _, ok := event.(EventSessionDown); !ok {
		t.Fatalf("old retirement event = %T, want session-down", event)
	}
	next, peer, original := frameLeaseStart(t, 68)
	owner.mu.Lock()
	owner.SessionHandle = original.SessionHandle
	owner.closed = false
	owner.birthTag = "native/test/successor68"
	before := owner.closeCount
	owner.mu.Unlock()
	next.Dataplane = owner
	next.SubscriberID = old.SubscriberID()
	next.Attempt, err = NewSessionAttempt(next.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	frameLeaseProxy(&next)
	driver.SessionsIn() <- next
	events := drainTwoEvents(t, driver.EventsOut(), time.Second)
	opened, ok := events[0].(EventLCPUp)
	if !ok {
		t.Fatalf("successor admission event = %T, want LCP-up", events[0])
	}
	current := opened.Control
	if current == old || current.Attempt() == old.Attempt() {
		t.Fatal("successor reused old worker admission")
	}
	if err := current.ReserveNativeRelease(); err != nil {
		t.Fatal(err)
	}
	if err := old.RetryNativeRelease(); err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	after := owner.closeCount
	owner.mu.Unlock()
	if after != before {
		t.Fatal("old native callback closed a same-lease successor")
	}
	if !current.Active() {
		t.Fatal("old native callback retired the successor")
	}
	if nativeBudgetUsed(budget) != 1 {
		t.Fatal("old callback returned the successor's native quota")
	}
	frameLeaseEcho(t, peer)
	if err := driver.StopSession(current); err != nil {
		t.Fatal(err)
	}
}

// Reload retries a bounded snapshot, honoring cancellation BEFORE EACH owner
// rather than spending one remote timeout for every pending session.
func TestNativeReleaseSweepChecksContextBeforeEachRetiredOwner(t *testing.T) {
	budget, err := NewNativeReleaseBudget(2)
	if err != nil {
		t.Fatal(err)
	}
	driver := newSessionTestDriver()
	if err := driver.ConfigureNativeReleaseBudget(budget); err != nil {
		t.Fatal(err)
	}
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	firstOwner, first := queueNativeBudgetSession(t, driver, 69)
	secondOwner, second := queueNativeBudgetSession(t, driver, 70)
	for _, fixture := range []struct {
		owner   *driverNativeLease
		control SessionControl
	}{
		{firstOwner, first}, {secondOwner, second},
	} {
		if err := fixture.control.ReserveNativeRelease(); err != nil {
			t.Fatal(err)
		}
		fixture.owner.mu.Lock()
		fixture.owner.pending = true
		fixture.owner.mu.Unlock()
		if err := driver.StopSession(fixture.control); err != nil {
			t.Fatal(err)
		}
		event := waitForEvent(t, driver.EventsOut(), time.Second)
		if _, ok := event.(EventSessionDown); !ok {
			t.Fatalf("retired event = %T, want session-down", event)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	before := 0
	for _, owner := range []*driverNativeLease{firstOwner, secondOwner} {
		owner.mu.Lock()
		owner.pending = false
		owner.onClose = cancel
		before += owner.closeCount
		owner.mu.Unlock()
	}
	if err := driver.RetryPendingNativeRelease(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled between-owner sweep = %v, want context-canceled", err)
	}
	after := 0
	for _, owner := range []*driverNativeLease{firstOwner, secondOwner} {
		owner.mu.Lock()
		after += owner.closeCount
		owner.mu.Unlock()
	}
	if after != before+1 {
		t.Fatal("sweep entered another native Close after its deadline was canceled")
	}
	if nativeBudgetUsed(budget) != 1 {
		t.Fatal("canceled sweep released capacity for an owner it did not retry")
	}
	if !driver.HasPendingNativeRelease() {
		t.Fatal("canceled sweep lost the unretried owner")
	}
	if err := driver.RetryPendingNativeRelease(context.Background()); err != nil {
		t.Fatal(err)
	}
	if nativeBudgetUsed(budget) != 0 || driver.HasPendingNativeRelease() {
		t.Fatal("fresh operator retry did not finish the remaining exact cleanup")
	}
}

// The callback can complete an operation that the first physical Close is
// currently sealing. It must return while the worker is still live in cleanup,
// BEFORE waiting for that Close's serialization gate.
func TestNativeReleaseEarlyCallbackDoesNotWaitForBlockedFirstClose(t *testing.T) {
	budget, err := NewNativeReleaseBudget(1)
	if err != nil {
		t.Fatal(err)
	}
	driver, owner, control := admitNativeBudgetSession(t, budget, 71)
	if err := control.ReserveNativeRelease(); err != nil {
		t.Fatal(err)
	}
	resume, entered := make(chan struct{}), make(chan struct{})
	owner.mu.Lock()
	owner.sealEntered = entered
	owner.sealResume = resume
	owner.mu.Unlock()
	stopped, retryDone := make(chan struct{}), make(chan struct{})
	stopResult, retryResult := make(chan error, 1), make(chan error, 1)
	retryStarted := false
	go func() { defer close(stopped); stopResult <- driver.StopSession(control) }()
	t.Cleanup(func() {
		if resume != nil {
			close(resume)
		}
		<-stopped
		if retryStarted {
			<-retryDone
		}
	})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first physical Close did not reach its native seal")
	}
	retryStarted = true
	go func() { defer close(retryDone); retryResult <- control.RetryNativeRelease() }()
	select {
	case err := <-retryResult:
		if err != nil {
			t.Fatalf("early callback = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("early callback waited for the Close whose operation it completes")
	}
	close(resume)
	resume = nil
	select {
	case err := <-stopResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("retirement did not finish after its native seal resumed")
	}
	if nativeBudgetUsed(budget) != 0 {
		t.Fatal("retirement lost the early acknowledgment and retained quota")
	}
}

// Cancellation while a sweep queues behind an earlier old-owner close must
// be rechecked AFTER that serialization gate and before another physical Close.
func TestNativeReleaseSweepCancellationWhileQueuedSkipsPhysicalRetry(t *testing.T) {
	budget, err := NewNativeReleaseBudget(1)
	if err != nil {
		t.Fatal(err)
	}
	driver, owner, control := admitNativeBudgetSession(t, budget, 72)
	if err := control.ReserveNativeRelease(); err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	owner.pending = true
	owner.mu.Unlock()
	if err := driver.StopSession(control); err != nil {
		t.Fatal(err)
	}
	resume, entered := make(chan struct{}), make(chan struct{})
	owner.mu.Lock()
	before := owner.closeCount
	owner.sealEntered = entered
	owner.sealResume = resume
	owner.mu.Unlock()
	firstDone, sweepDone := make(chan struct{}), make(chan struct{})
	firstResult, sweepResult := make(chan error, 1), make(chan error, 1)
	sweepStarted := false
	go func() { defer close(firstDone); firstResult <- control.RetryNativeRelease() }()
	t.Cleanup(func() {
		if resume != nil {
			close(resume)
		}
		<-firstDone
		if sweepStarted {
			<-sweepDone
		}
	})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first retired-owner retry did not reach the seal")
	}
	base, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	observed := &nativeCheckedContext{Context: base, checked: make(chan struct{})}
	sweepStarted = true
	go func() { defer close(sweepDone); sweepResult <- driver.RetryPendingNativeRelease(observed) }()
	select {
	case <-observed.checked:
	case <-time.After(time.Second):
		t.Fatal("sweep did not pass its initial live-context check")
	}
	cancel()
	close(resume)
	resume = nil
	select {
	case err := <-firstResult:
		if !errors.Is(err, ErrNativeReleasePending) {
			t.Fatalf("first held retry = %v, want native-pending", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first retry did not return after resuming its seal")
	}
	select {
	case err := <-sweepResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queued sweep = %v, want context-canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued canceled sweep did not return")
	}
	owner.mu.Lock()
	after := owner.closeCount
	owner.pending = false
	owner.mu.Unlock()
	if after != before+1 {
		t.Fatal("canceled queued sweep entered a second physical Close")
	}
	for _, issue := range report.Warnings() {
		if issue.Detail["native-birth"] == owner.NativeReleaseBirthTag() {
			if issue.Detail["error"] == context.Canceled.Error() {
				t.Fatal("paused sweep replaced the actual native failure with a cancellation")
			}
		}
	}
	if nativeBudgetUsed(budget) != 1 {
		t.Fatal("canceled queued sweep returned an unconfirmed owner's capacity")
	}
	if err := control.RetryNativeRelease(); err != nil {
		t.Fatal(err)
	}
	if nativeBudgetUsed(budget) != 0 {
		t.Fatal("fresh exact retry did not release retained capacity")
	}
}

type nativeCheckedContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (ctx *nativeCheckedContext) Err() error {
	// Capture the result BEFORE announcing this check. The test cancels only
	// after observing it, so the initial Driver sweep check certainly passes.
	err := ctx.Context.Err()
	ctx.once.Do(func() { close(ctx.checked) })
	return err
}

// Reused interface name/index labels cannot merge native birth warnings.
// Exact successful cleanup clears only its own native session issue; an
// independent close error and generic IPv6/network/session issues survive.
func TestNativeReleaseRecoveryClearsOnlyExactBirthWithNilClose(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	budget, err := NewNativeReleaseBudget(2)
	if err != nil {
		t.Fatal(err)
	}
	firstDriver, firstOwner, first := admitNativeBudgetSession(t, budget, 73)
	secondDriver, secondOwner, second := admitNativeBudgetSession(t, budget, 74)
	identity := firstOwner.Identity()
	if secondOwner.Identity() != identity {
		t.Fatal("fixture must reuse the exact physical interface labels")
	}
	sticky := errors.New("borrowed stream close remains failed")
	secondOwner.mu.Lock()
	secondOwner.stickyErr = sticky
	secondOwner.mu.Unlock()
	for _, fixture := range []struct {
		driver  *Driver
		owner   *driverNativeLease
		control SessionControl
	}{
		{firstDriver, firstOwner, first}, {secondDriver, secondOwner, second},
	} {
		if err := fixture.control.ReserveNativeRelease(); err != nil {
			t.Fatal(err)
		}
		fixture.owner.mu.Lock()
		fixture.owner.pending = true
		fixture.owner.mu.Unlock()
		if err := fixture.driver.StopSession(fixture.control); err != nil {
			t.Fatal(err)
		}
	}
	for _, operation := range []CleanupOperation{CleanupIPv6, CleanupNetwork, CleanupSession} {
		ReportCleanupFailure(identity, operation, errors.New("independent retained cleanup"))
	}
	if warnings := report.Warnings(); len(warnings) != 5 {
		t.Fatalf("two native births and three generic resources produced %d warnings, want five", len(warnings))
	}
	firstOwner.mu.Lock()
	firstOwner.pending = false
	firstOwner.mu.Unlock()
	if err := first.RetryNativeRelease(); err != nil {
		t.Fatal(err)
	}
	secondOwner.mu.Lock()
	secondOwner.pending = false
	secondOwner.mu.Unlock()
	if err := second.RetryNativeRelease(); !errors.Is(err, sticky) {
		t.Fatalf("confirmed native release lost its independent Close error: %v", err)
	}
	warnings := report.Warnings()
	if len(warnings) != 4 {
		t.Fatalf("exact recovery retained %d warnings, want four independent failures", len(warnings))
	}
	firstBirth, secondBirth := firstOwner.NativeReleaseBirthTag(), secondOwner.NativeReleaseBirthTag()
	foundSecond := false
	for _, issue := range warnings {
		if issue.Detail["native-birth"] == firstBirth {
			t.Fatal("successful exact native Close retained its warning")
		}
		if issue.Detail["native-birth"] == secondBirth {
			foundSecond = true
			if issue.Detail["error"] != sticky.Error() {
				t.Fatal("final exact Close failure lost its current retained cause")
			}
		}
	}
	if !foundSecond {
		t.Fatal("native confirmation with a non-nil Close error erased its warning")
	}
	if nativeBudgetUsed(budget) != 0 {
		t.Fatal("confirmed native release retained quota for independent descriptor errors")
	}
	// The fake preserves a sticky descriptor error like the kernel owner;
	// avoid reporting it as a new test cleanup error.
	secondOwner.mu.Lock()
	secondOwner.stickyErr = nil
	secondOwner.mu.Unlock()
}

// An outer caller may retain a failed retry result while a later exact retry
// succeeds. Its delayed report must not resurrect the already-cleared issue.
func TestNativeReleaseDelayedFailureReportCannotResurrectRecoveredBirth(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	budget, err := NewNativeReleaseBudget(1)
	if err != nil {
		t.Fatal(err)
	}
	driver, owner, control := admitNativeBudgetSession(t, budget, 75)
	if err := control.ReserveNativeRelease(); err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	owner.pending = true
	owner.mu.Unlock()
	if err := driver.StopSession(control); err != nil {
		t.Fatal(err)
	}
	late := control.RetryNativeRelease()
	if !errors.Is(late, ErrNativeReleasePending) {
		t.Fatalf("captured old retry = %v, want pending", late)
	}
	owner.mu.Lock()
	owner.pending = false
	owner.mu.Unlock()
	if err := control.RetryNativeRelease(); err != nil {
		t.Fatal(err)
	}
	driver.reportNativeReleaseFailure(control.worker, late)
	for _, issue := range report.Warnings() {
		if issue.Detail["native-birth"] == owner.NativeReleaseBirthTag() {
			t.Fatal("delayed old failure resurrected a confirmed native cleanup warning")
		}
	}
}

// Stop must overlap independent bounded native seals rather than spending a
// full remote timeout for every retained owner in sequence.
func TestDriverStopOverlapsIndependentRetiredNativeSeals(t *testing.T) {
	budget, err := NewNativeReleaseBudget(2)
	if err != nil {
		t.Fatal(err)
	}
	driver := newSessionTestDriver()
	if err := driver.ConfigureNativeReleaseBudget(budget); err != nil {
		t.Fatal(err)
	}
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	firstOwner, first := queueNativeBudgetSession(t, driver, 76)
	secondOwner, second := queueNativeBudgetSession(t, driver, 77)
	owners := []*driverNativeLease{firstOwner, secondOwner}
	for i, control := range []SessionControl{first, second} {
		if err := control.ReserveNativeRelease(); err != nil {
			t.Fatal(err)
		}
		owners[i].mu.Lock()
		owners[i].pending = true
		owners[i].mu.Unlock()
		if err := driver.StopSession(control); err != nil {
			t.Fatal(err)
		}
		event := waitForEvent(t, driver.EventsOut(), time.Second)
		if _, ok := event.(EventSessionDown); !ok {
			t.Fatalf("retired event = %T, want session-down", event)
		}
	}
	resume := make(chan struct{})
	entries := []chan struct{}{make(chan struct{}), make(chan struct{})}
	for i, owner := range owners {
		owner.mu.Lock()
		owner.sealEntered = entries[i]
		owner.sealResume = resume
		owner.mu.Unlock()
	}
	stopped := make(chan struct{})
	go func() { defer close(stopped); driver.Stop() }()
	t.Cleanup(func() {
		if resume != nil {
			close(resume)
		}
		<-stopped
	})
	for i, entered := range entries {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatalf("Stop serialized independent native seal %d behind another blocked seal", i)
		}
	}
	close(resume)
	resume = nil
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not join its finite native close fanout")
	}
	if nativeBudgetUsed(budget) != 2 || !driver.HasPendingNativeRelease() {
		t.Fatal("failed parallel native closes lost retained obligations")
	}
	for _, owner := range owners {
		owner.mu.Lock()
		owner.pending = false
		owner.mu.Unlock()
	}
	if err := driver.RetryPendingNativeRelease(context.Background()); err != nil {
		t.Fatal(err)
	}
	if nativeBudgetUsed(budget) != 0 {
		t.Fatal("stopped Driver could not complete retained exact cleanup")
	}
}

// Configuration and gate validation are required BEFORE a real operation can
// consume capacity. The pure quota rejects invalid construction and rebinding.
func TestNativeReleaseBudgetRequiresConfigurationAndBoundGate(t *testing.T) {
	for _, limit := range []int{0, -1} {
		if budget, err := NewNativeReleaseBudget(limit); err == nil || budget != nil {
			t.Fatalf("invalid budget %d was constructed", limit)
		}
	}
	budget, err := NewNativeReleaseBudget(1)
	if err != nil {
		t.Fatal(err)
	}
	driver := NewProductionDriver(discardLogger())
	if err := driver.ConfigureNativeReleaseBudget(nil); !errors.Is(err, ErrNativeReleaseBudgetMissing) {
		t.Fatal(err)
	}
	if err := driver.ConfigureNativeReleaseBudget(budget); err != nil {
		t.Fatal(err)
	}
	if err := driver.ConfigureNativeReleaseBudget(budget); err != nil {
		t.Fatal(err)
	}
	other, err := NewNativeReleaseBudget(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.ConfigureNativeReleaseBudget(other); err == nil {
		t.Fatal("Driver accepted a replacement quota")
	}
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	if err := driver.ConfigureNativeReleaseBudget(budget); err == nil {
		t.Fatal("Driver allowed quota configuration after Start")
	}
	_, owner, control := admitNativeBudgetSession(t, budget, 64)
	owner.mu.Lock()
	owner.bound = false
	owner.mu.Unlock()
	if err := control.ReserveNativeRelease(); !errors.Is(err, ErrNativeReleaseUnbound) {
		t.Fatalf("unbound reservation = %v", err)
	}
	if nativeBudgetUsed(budget) != 0 {
		t.Fatal("unbound operation consumed native capacity")
	}
	_, _, unconfigured := admitNativeBudgetSession(t, nil, 66)
	if err := unconfigured.ReserveNativeRelease(); !errors.Is(err, ErrNativeReleaseBudgetMissing) {
		t.Fatalf("unconfigured reservation = %v", err)
	}
	if (SessionControl{}).ReserveNativeRelease() == nil {
		t.Fatal("zero control obtained native capacity")
	}
}

// Native counter reads may block in their actual owner. They must not retain
// d.mu, and a worker canceled during that read must not publish its snapshot.
func TestSessionControlCountersUnlocksAndRejectsRetiredRead(t *testing.T) {
	budget, err := NewNativeReleaseBudget(1)
	if err != nil {
		t.Fatal(err)
	}
	driver, owner, control := admitNativeBudgetSession(t, budget, 65)
	const raw = uint64(987654)
	owner.counterRead = func() (SessionCounters, error) { return SessionCounters{RxBytes: raw}, nil }
	if counters, err := control.Counters(); err != nil || counters.RxBytes != raw {
		t.Fatalf("fresh exact transport read = %+v, %v", counters, err)
	}
	entered, resume := make(chan struct{}), make(chan struct{})
	readResume := resume
	owner.counterRead = func() (SessionCounters, error) {
		close(entered)
		<-readResume
		return SessionCounters{RxBytes: raw}, nil
	}
	result := make(chan error, 1)
	done := make(chan struct{})
	go func() { defer close(done); _, err := control.Counters(); result <- err }()
	var probed chan struct{}
	t.Cleanup(func() {
		if resume != nil {
			close(resume)
		}
		<-done
		if probed != nil {
			<-probed
		}
	})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("counter read did not reach its physical owner")
	}
	probed = make(chan struct{})
	go func() {
		func() {
			driver.mu.Lock()
			defer driver.mu.Unlock()
		}()
		close(probed)
	}()
	select {
	case <-probed:
	case <-time.After(time.Second):
		t.Fatal("counter read held the Driver lock")
	}
	if !control.Cancel() {
		t.Fatal("actual worker could not be canceled during counter read")
	}
	close(resume)
	resume = nil
	select {
	case err := <-result:
		if !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("retired in-flight read = %v, want session-not-found", err)
		}
	case <-time.After(time.Second):
		t.Fatal("counter read did not return after its physical owner completed")
	}
}

func admitNativeBudgetSession(t *testing.T, budget *NativeReleaseBudget, sid uint16) (*Driver, *driverNativeLease, SessionControl) {
	t.Helper()
	driver := newSessionTestDriver()
	if budget != nil {
		if err := driver.ConfigureNativeReleaseBudget(budget); err != nil {
			t.Fatal(err)
		}
	}
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	owner, control := queueNativeBudgetSession(t, driver, sid)
	return driver, owner, control
}

func queueNativeBudgetSession(t *testing.T, driver *Driver, sid uint16) (*driverNativeLease, SessionControl) {
	t.Helper()
	start, _, original := frameLeaseStart(t, sid)
	owner := &driverNativeLease{SessionHandle: original.SessionHandle, bound: true, birthTag: "native/test/" + fmt.Sprint(sid)}
	start.Dataplane = owner
	start.SubscriberID = "opaque/native-budget/" + fmt.Sprint(sid)
	attempt, err := NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	start.Attempt = attempt
	frameLeaseProxy(&start)
	t.Cleanup(func() {
		owner.mu.Lock()
		owner.pending = false
		owner.mu.Unlock()
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	driver.SessionsIn() <- start
	events := drainTwoEvents(t, driver.EventsOut(), time.Second)
	opened, ok := events[0].(EventLCPUp)
	if !ok {
		t.Fatalf("first admission event = %T, want LCP-up", events[0])
	}
	return owner, opened.Control
}
