// Design: docs/architecture/l2tp/subscriber-session-model.md -- original terminal counters on every final exit.
package ppp

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/netip"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// The wrapper observes actual owner calls from genuinely admitted Driver
// sessions. The held add is a native-operation seam, never an event ACK model.
type finalExitCounterOwner struct {
	counterReceiptOwner
	trace         sessionStopTrace
	baselinePanic bool
	connects      atomic.Int32
	closed        atomic.Bool
	installed     atomic.Bool
	afterClose    atomic.Bool
	addEntered    chan struct{}
	addResume     chan struct{}
	addOnce       sync.Once
	resumeOnce    sync.Once
}

func (owner *finalExitCounterOwner) Counters() (SessionCounters, error) {
	if owner.closed.Load() {
		owner.afterClose.Store(true)
		return SessionCounters{}, errors.New("original counter query ran after Close")
	}
	owner.trace.note("sample")
	if owner.baselinePanic {
		panic("original baseline provider panic")
	}
	return owner.counterReceiptOwner.Counters()
}
func (owner *finalExitCounterOwner) Connect() error {
	if err := owner.openOperation(); err != nil {
		return err
	}
	owner.trace.note("connect")
	owner.connects.Add(1)
	return owner.counterReceiptOwner.Connect()
}
func (owner *finalExitCounterOwner) openOperation() error {
	if owner.closed.Load() {
		owner.afterClose.Store(true)
		return errors.New("original activation ran after Close")
	}
	return nil
}
func (owner *finalExitCounterOwner) SetMTU(value int) error {
	if err := owner.openOperation(); err != nil {
		return err
	}
	return owner.counterReceiptOwner.SetMTU(value)
}
func (owner *finalExitCounterOwner) SetReceiveMRU(value uint16) error {
	if err := owner.openOperation(); err != nil {
		return err
	}
	return owner.counterReceiptOwner.SetReceiveMRU(value)
}
func (owner *finalExitCounterOwner) SetAdminUp() error {
	if err := owner.openOperation(); err != nil {
		return err
	}
	return owner.counterReceiptOwner.SetAdminUp()
}
func (owner *finalExitCounterOwner) Disconnect() error {
	if owner.closed.Load() {
		owner.afterClose.Store(true)
		return errors.New("original disconnect ran after Close")
	}
	owner.trace.note("disconnect")
	return owner.counterReceiptOwner.Disconnect()
}
func (owner *finalExitCounterOwner) AddAddressP2P(local, peer netip.Prefix) error {
	if err := owner.unlocked(); err != nil {
		return err
	}
	if owner.addEntered != nil {
		owner.addOnce.Do(func() { close(owner.addEntered) })
		<-owner.addResume
	}
	if owner.closed.Load() {
		owner.afterClose.Store(true)
		return errors.New("original address add ran after Close")
	}
	if err := owner.SessionHandle.AddAddressP2P(local, peer); err != nil {
		return err
	}
	owner.trace.note("install")
	owner.installed.Store(true)
	return nil
}
func (owner *finalExitCounterOwner) RemoveAddress(local netip.Prefix) error {
	if err := owner.unlocked(); err != nil {
		return err
	}
	if owner.closed.Load() {
		owner.afterClose.Store(true)
		return errors.New("original address withdrawal ran after Close")
	}
	if err := owner.SessionHandle.RemoveAddress(local); err != nil {
		return err
	}
	owner.trace.note("withdraw")
	owner.installed.Store(false)
	return nil
}
func (owner *finalExitCounterOwner) Close() error {
	owner.closed.Store(true)
	owner.trace.note("owner-close")
	return owner.counterReceiptOwner.Close()
}
func (owner *finalExitCounterOwner) resumeAdd() {
	if owner.addResume != nil {
		owner.resumeOnce.Do(func() { close(owner.addResume) })
	}
}

func newFinalCounterDriver(t *testing.T) *Driver {
	t.Helper()
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	return driver
}

// Real LCP and the requested authentication method complete before this
// helper returns. It deliberately leaves the real IPCP exchange to its caller.
func beginFinalCounterNetwork(t *testing.T, driver *Driver, sid uint16, owner *finalExitCounterOwner, dataplane SessionHandle, method AuthMethod, reauth time.Duration) (net.Conn, SessionControl) {
	t.Helper()
	start, peer, original := frameLeaseStart(t, sid)
	owner.SessionHandle = original.SessionHandle
	start.Dataplane = dataplane
	attempt, err := NewSessionAttempt(start.SubscriberID, dataplane)
	if err != nil {
		t.Fatal(err)
	}
	start.Attempt = attempt
	start.DisableIPCP = false
	start.AuthMethod = method
	start.ReauthInterval = reauth
	start.EchoInterval = time.Hour
	// Every artificial native barrier is released before Driver.Stop's cleanup.
	t.Cleanup(owner.resumeAdd)
	driver.SessionsIn() <- start
	openLifecycleLCP(t, peer)
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	opened, valid := event.(EventLCPUp)
	if !valid {
		t.Fatalf("real LCP admission event=%T", event)
	}
	owner.worker.Store(opened.Control.worker)
	if method == AuthMethodCHAPMD5 {
		challenge := readLifecyclePacket(t, peer, ProtoCHAP, CHAPCodeChallenge)
		answerLifecycleChallenge(t, peer, driver.AuthEventsOut(), method, challenge, true)
	} else {
		request := nextFinalCounterAuth(t, driver)
		if request.Control != opened.Control {
			t.Fatal("auth changed original admitted owner")
		}
		if request.Method != AuthMethodNone {
			t.Fatalf("no-auth request method=%v", request.Method)
		}
		answerAuthRequestForTest(t, request, authResponseMsg{accept: true})
	}
	select {
	case event := <-driver.AuthEventsOut():
		if _, valid := event.(eventAuthSuccess); !valid {
			t.Fatalf("actual authentication result=%T", event)
		}
	case <-time.After(time.Second):
		t.Fatal("actual authentication did not complete")
	}
	return peer, opened.Control
}

func nextFinalCounterAuth(t *testing.T, driver *Driver) EventAuthRequest {
	t.Helper()
	select {
	case event := <-driver.AuthEventsOut():
		request, valid := event.(EventAuthRequest)
		if !valid {
			t.Fatalf("actual authentication request=%T", event)
		}
		return request
	case <-time.After(time.Second):
		t.Fatal("actual authentication request was not emitted")
		return EventAuthRequest{}
	}
}

func finishFinalCounterIPCP(t *testing.T, peer net.Conn, driver *Driver) {
	t.Helper()
	request, _ := pendingLifecycleNCP(t, peer, driver.IPEventsOut(), AddressFamilyIPv4)
	writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck, request.Identifier, request.Data)
}

// A genuine admitted CHAP/IPCP network reaches the real periodic-auth wait.
// Global Stop takes its no-Down exit. The modeled final service writer and the
// real IPv4 withdrawal must still bracket this original's final measurement.
// MUTATION: restore R4's premature run-defer teardown and IPv6-only close stage.
func TestCounterReceiptGlobalStopDuringPeriodicAuthKeepsFinalOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		driver := newFinalCounterDriver(t)
		owner := &finalExitCounterOwner{}
		peer, control := beginFinalCounterNetwork(t, driver, 221, owner, owner, AuthMethodCHAPMD5, time.Second)
		finishFinalCounterIPCP(t, peer, driver)
		event := waitForEvent(t, driver.EventsOut(), time.Second)
		assigned, valid := event.(EventSessionIPAssigned)
		if !valid {
			t.Fatalf("real IPCP assignment=%T", event)
		}
		receipt, err := control.CaptureNetworkCounters(assigned.NetworkLifetime)
		if err != nil {
			t.Fatal(err)
		}
		assigned.Acknowledge()
		awaitLifecycleUp(t, driver.EventsOut())
		worker := control.worker
		worker.nativeReleaseMu.Lock()
		service := newIPv6Service(iPv6ServiceConfig{Dataplane: owner})
		service.stop = func() error { owner.trace.note("final-writer"); return nil }
		worker.mu.Lock()
		worker.ipv6Svc = service
		worker.mu.Unlock()
		worker.setOwnerServiceState(true, nil)
		worker.nativeReleaseMu.Unlock()
		challenge := readLifecyclePacket(t, peer, ProtoCHAP, CHAPCodeChallenge)
		writeCHAPResponseFrame(t, peer, challenge.Identifier, bytes.Repeat([]byte{0x33}, chapMD5DigestLen))
		pending := nextFinalCounterAuth(t, driver)
		if pending.Control != control {
			t.Fatal("periodic authentication changed owner")
		}
		if pending.Method != AuthMethodCHAPMD5 {
			t.Fatalf("periodic method=%v", pending.Method)
		}
		if pending.Identifier != challenge.Identifier {
			t.Fatal("periodic request did not consume the actual peer response")
		}
		// The provider answer remains held; this is the actual stopCh exit from
		// awaitAuthDecision, rather than a synthetic session-down publication.
		driver.Stop()
		want := []string{"install", "sample", "connect", "final-writer", "disconnect", "sample", "withdraw", "owner-close"}
		if got := owner.trace.snapshot(); !reflect.DeepEqual(got, want) {
			t.Fatalf("global-stop original cleanup order=%v, want %v", got, want)
		}
		if owner.afterClose.Load() {
			t.Fatal("native withdrawal ran after original Close")
		}
		if owner.installed.Load() {
			t.Fatal("global Stop left the original address installed")
		}
		if owner.closes.Load() != 1 {
			t.Fatalf("actual Close calls=%d", owner.closes.Load())
		}
		if owner.disconnects.Load() != 1 {
			t.Fatalf("actual Disconnect calls=%d", owner.disconnects.Load())
		}
		requireReceiptTotals(t, receipt)
	})
}

// A baseline provider panic is a defect, not ordinary unavailable telemetry.
// The real original must fail before assignment/forwarding and physically close;
// another actual owner on the SAME Driver must still assign and answer Echo.
// MUTATION: suppress the private fatal baseline-preparation result.
func TestBaselineCounterProviderPanicRefusesOnlyOriginalNetwork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		driver := newFinalCounterDriver(t)
		owner := &finalExitCounterOwner{baselinePanic: true}
		peer, control := beginFinalCounterNetwork(t, driver, 222, owner, owner, AuthMethodNone, 0)
		finishFinalCounterIPCP(t, peer, driver)
		event := waitForEvent(t, driver.EventsOut(), time.Second)
		down, valid := event.(EventSessionDown)
		if !valid {
			t.Fatalf("baseline provider panic published %T instead of refusing the original", event)
		}
		if !strings.Contains(down.Reason, "original baseline provider panic") {
			t.Fatalf("original provider defect lost its cause: %q", down.Reason)
		}
		bounded, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		status, err := control.WaitRetired(bounded)
		if err != nil {
			t.Fatal(err)
		}
		if !status.Released {
			t.Fatal("baseline defect did not release the original owner")
		}
		if owner.connects.Load() != 0 {
			t.Fatal("baseline defect enabled original forwarding")
		}
		if owner.closes.Load() != 1 {
			t.Fatalf("baseline defect actual Close calls=%d", owner.closes.Load())
		}
		if owner.installed.Load() {
			t.Fatal("baseline defect leaked the original address")
		}
		for len(driver.EventsOut()) > 0 {
			switch extra := (<-driver.EventsOut()).(type) {
			case EventSessionIPAssigned:
				t.Fatalf("baseline defect published an assignment: %+v", extra)
			case EventSessionUp:
				t.Fatal("baseline defect published SessionUp")
			}
		}
		other := admitCounterReceipt(t, driver, 223, &counterReceiptOwner{})
		other.assignment.Acknowledge()
		awaitLifecycleUp(t, driver.EventsOut())
		frameLeaseEcho(t, other.peer)
		if other.owner.closes.Load() != 0 {
			t.Fatal("baseline defect closed the useful independent owner")
		}
	})
}

// An original canceled during a real native add keeps the same operation gate.
// The add must finish, its receipt must be withdrawn, then the owner may close.
// A second genuine owner stays useful while that native call remains held.
// MUTATION: bypass nativeReleaseMu in installNCPIPv4.
func TestCanceledOriginalAddressInstallPrecedesCleanupAndClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		driver := newFinalCounterDriver(t)
		owner := &finalExitCounterOwner{addEntered: make(chan struct{}), addResume: make(chan struct{})}
		peer, control := beginFinalCounterNetwork(t, driver, 224, owner, owner, AuthMethodNone, 0)
		finishFinalCounterIPCP(t, peer, driver)
		requireReceiptSignal(t, owner.addEntered, "the original native address add was not entered")
		if !control.Cancel() {
			t.Fatal("the exact original cancellation failed")
		}
		other := admitCounterReceipt(t, driver, 225, &counterReceiptOwner{})
		other.assignment.Acknowledge()
		awaitLifecycleUp(t, driver.EventsOut())
		frameLeaseEcho(t, other.peer)
		if err := control.CloseCanceledOwner(t.Context()); !errors.Is(err, ErrCleanupInProgress) {
			t.Fatalf("canceled original native add did not retain its close gate: %v", err)
		}
		if owner.closes.Load() != 0 {
			t.Fatal("Close overtook the held original native add")
		}
		owner.resumeAdd()
		bounded, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		status, err := control.WaitRetired(bounded)
		if err != nil {
			t.Fatal(err)
		}
		if !status.Released {
			t.Fatal("resolved original add did not finish physical cleanup")
		}
		if owner.afterClose.Load() {
			t.Fatal("late original native operation ran after Close")
		}
		if owner.installed.Load() {
			t.Fatal("canceled original native add leaked its receipt")
		}
		want := []string{"install", "withdraw", "owner-close"}
		if got := owner.trace.snapshot(); !reflect.DeepEqual(got, want) {
			t.Fatalf("canceled original add/cleanup order=%v, want %v", got, want)
		}
		if other.owner.closes.Load() != 0 {
			t.Fatal("original add cleanup closed the useful independent owner")
		}
	})
}

// A native owner can close its ordinary handle while retaining its original
// unit. Only the actual NativeReleasePending result releases the modeled
// installed address; an independent descriptor error remains observable.
type finalExitNativeOwner struct {
	driverNativeLease
	original *finalExitCounterOwner
}

func (owner *finalExitNativeOwner) Close() error {
	cause := owner.driverNativeLease.Close()
	if !owner.NativeReleasePending() {
		owner.original.installed.Store(false)
	}
	return cause
}

func beginDirectFinalCounterClose(t *testing.T, sid uint16, native *finalExitNativeOwner) (*Driver, SessionControl, NetworkCounterReceipt) {
	t.Helper()
	driver := NewProductionDriver(discardLogger())
	driver.eventsOut = make(chan Event)
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	peer, control := beginFinalCounterNetwork(t, driver, sid, native.original, native, AuthMethodNone, 0)
	finishFinalCounterIPCP(t, peer, driver)
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	assignment, valid := event.(EventSessionIPAssigned)
	if !valid {
		t.Fatalf("real IPCP assignment=%T", event)
	}
	receipt, err := control.CaptureNetworkCounters(assignment.NetworkLifetime)
	if err != nil {
		t.Fatal(err)
	}
	assignment.Acknowledge()
	// The actual worker reaches an unbuffered SessionUp publication. It has
	// released the native gate, but cannot run its final defer until drained.
	synctest.Wait()
	if !control.Cancel() {
		t.Fatal("exact original cancellation failed")
	}
	return driver, control, receipt
}

func finishDirectFinalCounterWorker(t *testing.T, driver *Driver, control SessionControl) SessionCleanupStatus {
	t.Helper()
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	if _, valid := event.(EventSessionUp); !valid {
		t.Fatalf("blocked original publisher=%T", event)
	}
	awaitLifecycleDown(t, driver.EventsOut())
	bounded, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	status, err := control.WaitRetired(bounded)
	if err != nil {
		t.Fatal(err)
	}
	return status
}

func copiedFinalIPv4Receipt(control SessionControl) netip.Prefix {
	worker := control.worker
	worker.nativeReleaseMu.Lock()
	defer worker.nativeReleaseMu.Unlock()
	return worker.installedIPv4
}

// The actual guarded callback closes an IPv4-only owner while SessionUp is
// blocked. A confirmed destroyed unit retires its copied receipt even when
// Close reports a separate error, and later worker exit must perform no I/O
// through that already-closed owner.
// MUTATION: clear the copied receipt only when Close returns nil rather than
// when exact physical release is confirmed despite an independent error.
func TestDirectOriginalCloseRetiresIPv4ReceiptAfterPhysicalConfirmation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		descriptorFailure := errors.New("original descriptor close failed after unit destruction")
		original := &finalExitCounterOwner{}
		native := &finalExitNativeOwner{original: original}
		native.driverNativeLease = driverNativeLease{
			SessionHandle: original, bound: true, birthTag: "original-confirmed-counter-unit", stickyErr: descriptorFailure,
		}
		driver, control, receipt := beginDirectFinalCounterClose(t, 226, native)
		if err := control.CloseCanceledOwner(t.Context()); !errors.Is(err, descriptorFailure) {
			t.Fatalf("confirmed native release lost independent descriptor error: %v", err)
		}
		if copiedFinalIPv4Receipt(control).IsValid() {
			t.Fatal("confirmed destroyed unit retained a copied IPv4 receipt")
		}
		requireReceiptTotals(t, receipt)
		status := finishDirectFinalCounterWorker(t, driver, control)
		if !status.Released {
			t.Fatal("confirmed destroyed original was reported pending")
		}
		if !errors.Is(status.Err, descriptorFailure) {
			t.Fatalf("worker exit lost independent Close error: %v", status.Err)
		}
		if original.afterClose.Load() {
			t.Fatal("worker exit used a native operation after direct original Close")
		}
		if original.installed.Load() {
			t.Fatal("confirmed original native release left the modeled address")
		}
		if original.closes.Load() != 1 {
			t.Fatalf("confirmed original Close was repeated: %d", original.closes.Load())
		}
		if original.reads.Load() != 2 {
			t.Fatalf("confirmed original sample was requeried: %d", original.reads.Load())
		}
	})
}

// An uncertain direct close retains its exact IPv4 receipt and strong original
// owner after worker exit. Only recovery of that SAME native release retires
// the receipt; neither retirement nor an earlier final sample proves release.
// MUTATION: clear installedIPv4 after any Close attempt, including pending;
// or retry address withdrawal through the already-closed handle on worker exit.
func TestPendingDirectOriginalCloseRetainsIPv4ReceiptUntilExactRecovery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		original := &finalExitCounterOwner{}
		native := &finalExitNativeOwner{original: original}
		native.driverNativeLease = driverNativeLease{
			SessionHandle: original, pending: true, bound: true, birthTag: "original-pending-counter-unit",
		}
		driver, control, receipt := beginDirectFinalCounterClose(t, 227, native)
		if err := control.CloseCanceledOwner(t.Context()); !errors.Is(err, ErrNativeReleasePending) {
			t.Fatalf("pending original close was certified complete: %v", err)
		}
		if !copiedFinalIPv4Receipt(control).IsValid() {
			t.Fatal("pending original Close dropped its copied IPv4 receipt")
		}
		requireReceiptTotals(t, receipt)
		status := finishDirectFinalCounterWorker(t, driver, control)
		if status.Released {
			t.Fatal("worker retirement falsely confirmed native unit release")
		}
		if !errors.Is(status.Err, ErrNativeReleasePending) {
			t.Fatalf("pending native cause=%v", status.Err)
		}
		if !copiedFinalIPv4Receipt(control).IsValid() {
			t.Fatal("worker exit dropped the pending original receipt")
		}
		if original.afterClose.Load() {
			t.Fatal("pending worker exit mutated the closed original handle")
		}
		if !original.installed.Load() {
			t.Fatal("pending native unit lost its modeled original address")
		}
		driver.mu.Lock()
		retained := driver.owners[native] == control.worker
		driver.mu.Unlock()
		if !retained {
			t.Fatal("pending original owner was not strongly retained")
		}
		native.mu.Lock()
		native.pending = false
		native.mu.Unlock()
		if err := control.RetryNativeRelease(); err != nil {
			t.Fatalf("exact native recovery failed: %v", err)
		}
		recovered, err := control.CleanupStatus()
		if err != nil {
			t.Fatal(err)
		}
		if !recovered.Released {
			t.Fatal("confirmed exact recovery remained pending")
		}
		if recovered.Err != nil {
			t.Fatalf("confirmed exact recovery retained stale error: %v", recovered.Err)
		}
		if copiedFinalIPv4Receipt(control).IsValid() {
			t.Fatal("confirmed exact recovery did not retire copied receipt")
		}
		if original.installed.Load() {
			t.Fatal("confirmed exact recovery retained the modeled address")
		}
		if original.afterClose.Load() {
			t.Fatal("exact recovery queried/mutated a closed original")
		}
		if original.reads.Load() != 2 {
			t.Fatalf("exact recovery requeried frozen counters: %d", original.reads.Load())
		}
		if original.disconnects.Load() != 1 {
			t.Fatalf("exact recovery repeated Disconnect: %d", original.disconnects.Load())
		}
		requireReceiptTotals(t, receipt)
	})
}
