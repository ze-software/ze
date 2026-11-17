// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact cleanup/cancel completion
package ppp

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// Actual owner Close is held after borrowed I/O wakes. Observers and a canceled
// context must not join the close gate, and a competing close returns busy.
func TestCleanupObserversStayPromptWhileOwnerCloseIsBlocked(t *testing.T) {
	driver, owner, control := admittedGenericCleanupOwner(t)
	owner.entered = make(chan struct{})
	resume := make(chan struct{})
	owner.resume = resume
	t.Cleanup(func() {
		if resume != nil {
			close(resume)
			resume = nil
		}
	})
	control.Cancel()
	select {
	case <-owner.entered:
	case <-time.After(time.Second):
		t.Fatal("worker did not enter actual owner Close")
	}
	statusDone, waitDone, closeDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var status SessionCleanupStatus
	var statusErr, waitErr, closeErr error
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	t.Cleanup(func() {
		if resume != nil {
			close(resume)
			resume = nil
		}
		<-statusDone
		<-waitDone
		<-closeDone
	})
	go func() { defer close(statusDone); status, statusErr = control.CleanupStatus() }()
	go func() { defer close(waitDone); _, waitErr = control.WaitRetired(canceled) }()
	go func() { defer close(closeDone); closeErr = control.CloseCanceledOwner(context.Background()) }()
	for _, done := range []<-chan struct{}{statusDone, waitDone, closeDone} {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("cleanup observer joined blocked physical Close")
		}
	}
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if status.Released {
		t.Fatal("blocked first Close was reported as released")
	}
	if !errors.Is(waitErr, context.Canceled) {
		t.Fatalf("blocked retirement wait=%v", waitErr)
	}
	if !errors.Is(closeErr, ErrCleanupInProgress) {
		t.Fatalf("competing close=%v, want in-progress", closeErr)
	}
	if status.Err != nil {
		t.Fatal("wait/contention error replaced the actual unfinished Close result")
	}
	close(resume)
	resume = nil
	bounded, finish := context.WithTimeout(context.Background(), time.Second)
	defer finish()
	actual, err := control.WaitRetired(bounded)
	if err != nil {
		t.Fatal(err)
	}
	if !actual.Retired || !actual.Released || actual.Err != nil {
		t.Fatalf("completed actual worker=%+v", actual)
	}
	if owner.closes.Load() != 1 {
		t.Fatal("confirmed first Close was repeated by retirement")
	}
	_ = driver
}

// Holding the ACTUAL pending auth request at retirement isolates a canceled
// protocol worker from its physical owner. The close method must confirm that
// owner without joining the worker or consuming another request's authority.
func TestCanceledOwnerCloseConfirmsBeforeProtocolRetirement(t *testing.T) {
	driver, owner, request := admittedGenericCleanupRequest(t)
	control := request.Control
	if err := control.CloseCanceledOwner(context.Background()); !errors.Is(err, ErrSessionNotCanceled) {
		t.Fatalf("uncanceled close=%v", err)
	}
	if owner.closes.Load() != 0 {
		t.Fatal("uncanceled close reached native owner")
	}
	request.pending.mu.Lock()
	held := true
	t.Cleanup(func() {
		if held {
			request.pending.mu.Unlock()
		}
	})
	control.Cancel()
	if err := control.CloseCanceledOwner(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, err := control.CleanupStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Released || status.Retired || status.Err != nil {
		t.Fatalf("physical close versus protocol retirement=%+v", status)
	}
	driver.mu.Lock()
	stillOwned := driver.owners[owner] == control.worker
	driver.mu.Unlock()
	if !stillOwned {
		t.Fatal("active canceled worker released its owner index before retirement")
	}
	if owner.closes.Load() != 1 {
		t.Fatal("canceled-owner close did not perform exactly one physical Close")
	}
	expired, cancel := context.WithCancel(context.Background())
	cancel()
	if err := control.CloseCanceledOwner(expired); !errors.Is(err, context.Canceled) {
		t.Fatalf("expired close=%v", err)
	}
	status, err = control.CleanupStatus()
	if err != nil || status.Err != nil {
		t.Fatal("expired-before-dispatch overwrote successful actual Close")
	}
	request.pending.mu.Unlock()
	held = false
	bounded, finish := context.WithTimeout(context.Background(), time.Second)
	defer finish()
	status, err = control.WaitRetired(bounded)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Retired || !status.Released || status.Err != nil {
		t.Fatalf("retired exact worker=%+v", status)
	}
	if owner.closes.Load() != 1 {
		t.Fatal("retirement repeated the already confirmed Close")
	}
}

// A generic Close failure is visible after done and after the canonical key is
// gone. A later exact retry confirms recovery and updates the same snapshot.
func TestCleanupStatusRetainsGenericFailureUntilExactRetry(t *testing.T) {
	driver, owner, control := admittedGenericCleanupOwner(t)
	owner.fail.Store(true)
	if err := driver.StopSession(control); err != nil {
		t.Fatal(err)
	}
	status, err := control.WaitRetired(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Retired || status.Released || !errors.Is(status.Err, errGenericOwnerClose) {
		t.Fatalf("failed generic owner=%+v", status)
	}
	owner.fail.Store(false)
	if err := control.CloseCanceledOwner(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, err = control.CleanupStatus()
	if err != nil || !status.Released || status.Err != nil {
		t.Fatalf("exact generic recovery=%+v,%v", status, err)
	}
	status, err = control.WaitRetired(context.Background())
	if err != nil || !status.Retired || !status.Released || status.Err != nil {
		t.Fatalf("post-recovery original completion=%+v,%v", status, err)
	}
}

// Native unit release and an independent descriptor Close error are distinct.
// The quota/index may retire, but neither the observer nor close wrapper may
// turn that actual local error into a successful cleanup acknowledgment.
func TestCleanupStatusPreservesReleasedNativeLocalError(t *testing.T) {
	budget, err := NewNativeReleaseBudget(1)
	if err != nil {
		t.Fatal(err)
	}
	driver := NewProductionDriver(discardLogger())
	if err := driver.ConfigureNativeReleaseBudget(budget); err != nil {
		t.Fatal(err)
	}
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, _, original := frameLeaseStart(t, 91)
	owner := &driverNativeLease{SessionHandle: original.SessionHandle, bound: true, birthTag: "native/status/local-error"}
	start.Dataplane = owner
	start.SubscriberID = "opaque/native-local-close"
	start.Attempt, err = NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	frameLeaseProxy(&start)
	t.Cleanup(func() {
		owner.mu.Lock()
		owner.stickyErr = nil
		owner.pending = false
		owner.mu.Unlock()
		driver.Stop()
		if err := driver.RetryPendingNativeRelease(context.Background()); err != nil {
			t.Error(err)
		}
	})
	driver.SessionsIn() <- start
	opened, valid := waitForEvent(t, driver.EventsOut(), time.Second).(EventLCPUp)
	if !valid {
		t.Fatal("native status fixture omitted actual LCP-up")
	}
	control := opened.Control
	select {
	case event := <-driver.AuthEventsOut():
		request, ok := event.(EventAuthRequest)
		if !ok || request.Control != control {
			t.Fatal("native status fixture omitted its actual auth wait")
		}
	case <-time.After(time.Second):
		t.Fatal("native status fixture omitted auth wait")
	}
	if err := control.ReserveNativeRelease(); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("independent descriptor close failed")
	owner.mu.Lock()
	owner.stickyErr = cause
	owner.mu.Unlock()
	if err := driver.StopSession(control); err != nil {
		t.Fatal(err)
	}
	status, err := control.WaitRetired(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Retired || !status.Released || !errors.Is(status.Err, cause) {
		t.Fatalf("native release/local error=%+v", status)
	}
	if err := control.CloseCanceledOwner(context.Background()); !errors.Is(err, cause) {
		t.Fatalf("cached local close=%v", err)
	}
	if err := control.RetryNativeRelease(); !errors.Is(err, cause) {
		t.Fatalf("completed retry hid actual local cause=%v", err)
	}
	if nativeBudgetUsed(budget) != 0 {
		t.Fatal("confirmed native release retained rate quota for local descriptor error")
	}
	owner.mu.Lock()
	owner.stickyErr = nil
	owner.mu.Unlock()
}

type observedCleanupFrames struct {
	FrameChannel
	stream *observedCleanupStream
}
type observedCleanupStream struct {
	io.ReadWriter
	mu            sync.Mutex
	reads, writes int
}

func (s *observedCleanupStream) Read(buf []byte) (int, error) {
	s.mu.Lock()
	s.reads++
	s.mu.Unlock()
	return s.ReadWriter.Read(buf)
}
func (s *observedCleanupStream) Write(buf []byte) (int, error) {
	s.mu.Lock()
	s.writes++
	s.mu.Unlock()
	return s.ReadWriter.Write(buf)
}
func (f *observedCleanupFrames) Control() io.ReadWriter { return f.stream }

// The canceled-before-receive case NEVER mints a Control. Its actual rejected
// lease, failed Close result and later recovery remain on that SAME Attempt.
func TestCanceledQueuedAttemptRetainsRejectedCleanupWithoutControl(t *testing.T) {
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, _, original := frameLeaseStart(t, 92)
	owner := &genericCleanupLease{SessionHandle: original.SessionHandle}
	owner.fail.Store(true)
	stream := &observedCleanupStream{ReadWriter: start.Frames.Control()}
	start.Frames = &observedCleanupFrames{FrameChannel: start.Frames, stream: stream}
	start.Dataplane = owner
	start.SubscriberID = "opaque/queued-canceled/full-70001"
	attempt, err := NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	start.Attempt = attempt
	frameLeaseProxy(&start)
	t.Cleanup(func() {
		owner.fail.Store(false)
		if err := driver.RetryPendingNativeRelease(context.Background()); err != nil {
			t.Error(err)
		}
	})
	firstCancel := attempt.Cancel()
	secondCancel := attempt.Cancel()
	if !firstCancel || !secondCancel {
		t.Fatal("exact attempt cancellation is not idempotent")
	}
	if _, bound := attempt.Control(); bound {
		t.Fatal("unreceived canceled attempt fabricated Control")
	}
	driver.SessionsIn() <- start
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	refused, valid := event.(EventSessionRejected)
	if !valid {
		t.Fatalf("canceled attempt event=%T", event)
	}
	if refused.Attempt != attempt || refused.Reason != ErrSessionAttemptCanceled.Error() {
		t.Fatal("refusal lost original handoff or cancellation reason")
	}
	if _, bound := attempt.Control(); bound {
		t.Fatal("rejection fabricated admitted Control")
	}
	status, err := attempt.CleanupStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Admitted || !status.Finished || !status.LeaseAcquired || status.Released || !errors.Is(status.Err, errGenericOwnerClose) {
		t.Fatalf("actual rejected cleanup=%+v", status)
	}
	stream.mu.Lock()
	reads, writes := stream.reads, stream.writes
	stream.mu.Unlock()
	if reads != 0 || writes != 0 {
		t.Fatal("canceled pre-admission transfer performed protocol I/O")
	}
	if owner.closes.Load() != 1 {
		t.Fatal("refused first failure was silently retried or forgotten")
	}
	owner.fail.Store(false)
	if err := driver.RetryPendingNativeRelease(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, err = attempt.CleanupStatus()
	if err != nil || !status.Released || status.Err != nil || !status.Finished || status.Admitted {
		t.Fatalf("same rejected handoff recovery=%+v,%v", status, err)
	}
	if _, bound := attempt.Control(); bound {
		t.Fatal("successful rejection cleanup invented a Control")
	}
}

func TestAttemptDuplicateLeaseRefusalNeverClaimsRightfulRelease(t *testing.T) {
	driver, owner, request := admittedGenericCleanupRequest(t)
	control := request.Control
	incoming := StartSession{LCP: testLCPConfig(), TunnelID: 1, SessionID: 93,
		SubscriberID: "opaque/distinct-pending-admission", Frames: &testSessionFrames{control: control.worker.chanFile}, Dataplane: owner}
	attempt, err := NewSessionAttempt(incoming.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	incoming.Attempt = attempt
	driver.SessionsIn() <- incoming
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	refused, valid := event.(EventSessionRejected)
	if !valid || refused.Attempt != attempt {
		t.Fatalf("duplicate lease refusal=%T", event)
	}
	status, err := attempt.CleanupStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Admitted || !status.Finished || status.LeaseAcquired || status.Released || status.Err != nil {
		t.Fatalf("unacquired duplicate lease=%+v", status)
	}
	if _, bound := attempt.Control(); bound {
		t.Fatal("duplicate lease minted a second Control")
	}
	if !attempt.Cancel() {
		t.Fatal("valid refused Attempt lost cancellation identity")
	}
	if !control.Active() || owner.closes.Load() != 0 {
		t.Fatal("duplicate cancellation/refusal touched rightful worker")
	}
	original, bound := control.Attempt().Control()
	if !bound || original != control {
		t.Fatal("original admitted rendezvous changed")
	}
}

func TestAttemptRendezvousRejectsReplayAcrossDrivers(t *testing.T) {
	first, owner, request := admittedGenericCleanupRequest(t)
	control := request.Control
	other := NewProductionDriver(discardLogger())
	if err := other.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(other.Stop)
	replay := StartSession{LCP: testLCPConfig(), TunnelID: 1, SessionID: 94,
		SubscriberID: control.SubscriberID(), Attempt: control.Attempt(),
		Frames: &testSessionFrames{control: control.worker.chanFile}, Dataplane: owner}
	other.SessionsIn() <- replay
	event := waitForEvent(t, other.EventsOut(), time.Second)
	refused, valid := event.(EventSessionRejected)
	if !valid || !refused.ExistingSession {
		t.Fatalf("replayed admitted handoff=%T,%+v", event, event)
	}
	if owner.closes.Load() != 0 || !control.Active() {
		t.Fatal("another Driver's replay closed the originally bound worker")
	}
	malformed := replay
	malformed.SubscriberID = "opaque/malformed-raw-replay"
	other.SessionsIn() <- malformed
	event = waitForEvent(t, other.EventsOut(), time.Second)
	refused, valid = event.(EventSessionRejected)
	if !valid || !refused.ExistingSession {
		t.Fatalf("malformed raw replay lost original admission: %T,%+v", event, event)
	}
	if owner.closes.Load() != 0 || !control.Active() {
		t.Fatal("malformed raw replay closed originally bound lease")
	}
	observed, bound := control.Attempt().Control()
	if !bound || observed != control {
		t.Fatal("replay retargeted original admission rendezvous")
	}
	control.Attempt().Cancel()
	if _, err := control.WaitRetired(context.Background()); err != nil {
		t.Fatal(err)
	}
	observed, bound = control.Attempt().Control()
	if !bound || observed != control {
		t.Fatal("retirement erased originally bound worker rendezvous")
	}
	_ = first
}

func TestCleanupObserversRejectInvalidZeroCapabilities(t *testing.T) {
	if _, err := (SessionControl{}).CleanupStatus(); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal(err)
	}
	if _, err := (SessionControl{}).WaitRetired(context.Background()); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal(err)
	}
	if err := (SessionControl{}).CloseCanceledOwner(context.Background()); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal(err)
	}
	if _, err := (SessionAttempt{}).CleanupStatus(); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal(err)
	}
	if (SessionAttempt{}).Cancel() {
		t.Fatal("zero Attempt claimed cancellation authority")
	}
	if _, bound := (SessionAttempt{}).Control(); bound {
		t.Fatal("zero Attempt fabricated Control")
	}
}

// The echo reply proves the main Opened loop is running AFTER its real Up
// receipt was published. Cancel alone must retire it without caller FD Close;
// an auth wait or still-blocked Up producer cannot stand in for this proof.
func TestOpenedCancelRetiresAfterActualEchoWithoutEarlyClose(t *testing.T) {
	driver, owner, request := admittedGenericCleanupRequest(t)
	control := request.Control
	if err := driver.AuthResponse(request, true, "", nil); err != nil {
		t.Fatal(err)
	}
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	up, valid := event.(EventSessionUp)
	if !valid {
		t.Fatalf("opened fixture receipt=%T", event)
	}
	// Current Core Up is an ordinary notification; the real Echo below proves Opened loop entry.
	if up.Control != control {
		t.Fatal("actual Up changed the admitted worker")
	}
	frameLeaseEcho(t, owner.peer)
	if owner.closes.Load() != 0 {
		t.Fatal("caller/activation closed owner before cancellation")
	}
	control.Cancel()
	bounded, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	status, err := control.WaitRetired(bounded)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Retired || !status.Released || status.Err != nil {
		t.Fatalf("opened cancellation status=%+v", status)
	}
	downEvent := waitForEvent(t, driver.EventsOut(), time.Second)
	down, valid := downEvent.(EventSessionDown)
	if !valid || down.Cause != l2tpevents.TerminateCauseNASRequest || down.Reason != "session stopped" {
		t.Fatalf("opened exact cancellation outcome=%T,%+v", downEvent, downEvent)
	}
	if owner.closes.Load() != 1 {
		t.Fatal("normal retirement did not own exactly one physical Close")
	}
}

// The same opaque ID and physical lease object can be reused only after the
// first worker completed. Its old observer/close/cancel capabilities stay old.
func TestCleanupCapabilityCannotCloseReusedIDAndLeaseSuccessor(t *testing.T) {
	driver, owner, request := admittedGenericCleanupRequest(t)
	old := request.Control
	if err := driver.StopSession(old); err != nil {
		t.Fatal(err)
	}
	if _, err := old.WaitRetired(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, valid := waitForEvent(t, driver.EventsOut(), time.Second).(EventSessionDown); !valid {
		t.Fatal("first worker omitted actual terminal event")
	}
	start, peer, replacement := frameLeaseStart(t, 81)
	owner.SessionHandle = replacement.SessionHandle
	owner.peer = peer
	start.Dataplane = owner
	start.SubscriberID = old.SubscriberID()
	var err error
	start.Attempt, err = NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	frameLeaseProxy(&start)
	driver.SessionsIn() <- start
	opened, valid := waitForEvent(t, driver.EventsOut(), time.Second).(EventLCPUp)
	if !valid {
		t.Fatal("same-ID/same-lease replacement was not admitted")
	}
	var current EventAuthRequest
	select {
	case event := <-driver.AuthEventsOut():
		current, valid = event.(EventAuthRequest)
		if !valid {
			t.Fatalf("replacement request=%T", event)
		}
	case <-time.After(time.Second):
		t.Fatal("replacement omitted actual provider wait")
	}
	if current.Control != opened.Control || current.Control == old {
		t.Fatal("replacement did not mint distinct exact worker")
	}
	closes := owner.closes.Load()
	if err := old.CloseCanceledOwner(context.Background()); err != nil {
		t.Fatal(err)
	}
	old.Attempt().Cancel()
	oldStatus, err := old.CleanupStatus()
	if err != nil || !oldStatus.Retired || !oldStatus.Released || oldStatus.Err != nil {
		t.Fatalf("old observer retargeted successor: %+v,%v", oldStatus, err)
	}
	currentStatus, err := current.Control.CleanupStatus()
	if err != nil || currentStatus.Retired || currentStatus.Released || currentStatus.Err != nil {
		t.Fatalf("replacement inherited old release: %+v,%v", currentStatus, err)
	}
	if owner.closes.Load() != closes || !current.Control.Active() {
		t.Fatal("old cleanup/cancel closed replacement physical owner")
	}
	if err := driver.AuthResponse(current, true, "", nil); err != nil {
		t.Fatal(err)
	}
	up, valid := waitForEvent(t, driver.EventsOut(), time.Second).(EventSessionUp)
	if !valid {
		t.Fatal("fresh replacement reply did not complete its actual wait")
	}
	// Current Core Up is an ordinary notification; the real Echo below proves Opened loop entry.
	if up.Control != current.Control {
		t.Fatal("actual replacement Up changed the fresh worker")
	}
	frameLeaseEcho(t, peer)
	current.Control.Cancel()
	bounded, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := current.Control.WaitRetired(bounded); err != nil {
		t.Fatal(err)
	}
	if owner.closes.Load() != closes+1 {
		t.Fatal("replacement did not retain its own exactly-once close")
	}
}

// A receipt callback may run while its emitting worker holds worker.mu. The
// close API must use only the exact owner gate/result, never that mutex.
func TestCanceledOwnerCloseDoesNotAcquireEmittingWorkerMutex(t *testing.T) {
	_, owner, request := admittedGenericCleanupRequest(t)
	control := request.Control
	request.pending.mu.Lock()
	heldRequest := true
	control.worker.mu.Lock()
	heldWorker := true
	t.Cleanup(func() {
		if heldWorker {
			control.worker.mu.Unlock()
		}
		if heldRequest {
			request.pending.mu.Unlock()
		}
	})
	control.Cancel()
	done := make(chan error, 1)
	go func() { done <- control.CloseCanceledOwner(context.Background()) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		control.worker.mu.Unlock()
		heldWorker = false
		request.pending.mu.Unlock()
		heldRequest = false
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("close failed to return after releasing fixture gates")
		}
		t.Fatal("exact owner close acquired emitting worker mutex")
	}
	if owner.closes.Load() != 1 {
		t.Fatal("callback did not perform exact first Close")
	}
	status, err := control.CleanupStatus()
	if err != nil || !status.Released || status.Retired {
		t.Fatalf("held worker exact cleanup=%+v,%v", status, err)
	}
	control.worker.mu.Unlock()
	heldWorker = false
	request.pending.mu.Unlock()
	heldRequest = false
	bounded, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := control.WaitRetired(bounded); err != nil {
		t.Fatal(err)
	}
	if owner.closes.Load() != 1 {
		t.Fatal("ordinary worker cleanup repeated callback's confirmed Close")
	}
}

func TestMalformedFirstHandoffConsumesAttemptCleanupAndCannotReplayClosedLease(t *testing.T) {
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, _, original := frameLeaseStart(t, 95)
	owner := &genericCleanupLease{SessionHandle: original.SessionHandle}
	start.Dataplane = owner
	start.SubscriberID = "opaque/declared-attempt"
	attempt, err := NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	start.Attempt = attempt
	correct := start
	start.SubscriberID = "opaque/malformed-first-raw-id"
	driver.SessionsIn() <- start
	refused, valid := waitForEvent(t, driver.EventsOut(), time.Second).(EventSessionRejected)
	if !valid || refused.Attempt != attempt || refused.ExistingSession {
		t.Fatal("malformed first handoff did not refuse original attempt")
	}
	status, err := attempt.CleanupStatus()
	if err != nil || status.Admitted || !status.Finished || !status.LeaseAcquired || !status.Released || status.Err != nil {
		t.Fatalf("malformed original lease cleanup=%+v,%v", status, err)
	}
	if _, bound := attempt.Control(); bound {
		t.Fatal("refused original attempt fabricated admitted Control")
	}
	if owner.closes.Load() != 1 {
		t.Fatal("malformed first handoff did not close actual transferred lease")
	}
	driver.SessionsIn() <- correct
	replay, valid := waitForEvent(t, driver.EventsOut(), time.Second).(EventSessionRejected)
	if !valid || replay.Attempt != attempt || replay.Reason != ErrSessionAttemptUsed.Error() {
		t.Fatalf("closed-lease replay=%+v", replay)
	}
	if _, bound := attempt.Control(); bound {
		t.Fatal("corrected raw replay admitted already-closed original lease")
	}
	if owner.closes.Load() != 1 {
		t.Fatal("replay closed once-consumed lease again")
	}
}

// Service input follows the existing IPv6-service unit seam; admission and the
// cleanup Control are real. Native carrier I/O alone is substituted. A callback
// must refuse rather than join/delete a service that the worker still owns.
func TestCanceledOwnerClosePreservesLiveWorkerIPv6Cleanup(t *testing.T) {
	_, owner, request := admittedGenericCleanupRequest(t)
	control := request.Control
	handle, valid := owner.SessionHandle.(*testSessionHandle)
	if !valid {
		t.Fatalf("fixture physical handle=%T", owner.SessionHandle)
	}
	handle.openIPv6Carrier = func(context.Context) (IPv6Carrier, error) { return newServiceCarrier(), nil }
	request.pending.mu.Lock()
	held := true
	t.Cleanup(func() {
		if held {
			request.pending.mu.Unlock()
		}
	})
	control.worker.mu.Lock()
	control.worker.localInterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 1}
	control.worker.peerInterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 2}
	control.worker.peerInterfaceIDNegotiated = true
	control.worker.afterLCPOpenIPv6Service()
	service := control.worker.ipv6Svc
	control.worker.mu.Unlock()
	if service == nil {
		t.Fatal("production IPv6 service startup failed")
	}
	control.Cancel()
	if err := control.CloseCanceledOwner(context.Background()); !errors.Is(err, ErrCleanupInProgress) {
		t.Fatalf("live service callback=%v", err)
	}
	if owner.closes.Load() != 0 {
		t.Fatal("callback deleted owner before worker-owned service cleanup")
	}
	service.mu.Lock()
	installed := service.linkLocalInstalled
	service.mu.Unlock()
	if !installed {
		t.Fatal("live service prerequisite was not actually installed")
	}
	request.pending.mu.Unlock()
	held = false
	bounded, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	status, err := control.WaitRetired(bounded)
	if err != nil || !status.Retired || !status.Released || status.Err != nil {
		t.Fatalf("worker-owned service cleanup=%+v,%v", status, err)
	}
	service.mu.Lock()
	installed = service.linkLocalInstalled
	service.mu.Unlock()
	if installed {
		t.Fatal("worker did not complete actual IPv6 resource withdrawal")
	}
	if owner.closes.Load() != 1 {
		t.Fatal("worker's confirmed physical cleanup was repeated")
	}
}
