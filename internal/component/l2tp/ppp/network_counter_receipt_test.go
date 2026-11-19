// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact terminal receipts before native release.
package ppp

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// This fixture uses actual Driver admission, original auth response and a real
// IPCP Configure exchange. Receipt authority comes only from the assignment's
// admitted Control, not a hand-built worker/epoch or a successor name lookup.
type counterReceiptOwner struct {
	SessionHandle
	worker         atomic.Pointer[pppSession]
	reads          atomic.Int32
	disconnects    atomic.Int32
	closes         atomic.Int32
	fault          atomic.Int32
	connectEntered chan struct{}
	connectResume  chan struct{}
	closeEntered   chan struct{}
	closeResume    chan struct{}
	readEntered    chan struct{}
	readResume     chan struct{}
	connectOnce    sync.Once
	closeOnce      sync.Once
	readOnce       sync.Once
	releaseConnect sync.Once
	releaseClose   sync.Once
	releaseRead    sync.Once
}

func (owner *counterReceiptOwner) unlocked() error {
	worker := owner.worker.Load()
	if worker == nil {
		return errors.New("provider called before original admission")
	}
	if !worker.mu.TryLock() {
		return errors.New("provider held worker metadata mutex")
	}
	worker.mu.Unlock()
	if !worker.driver.mu.TryLock() {
		return errors.New("provider held Driver metadata mutex")
	}
	worker.driver.mu.Unlock()
	return nil
}
func (owner *counterReceiptOwner) Connect() error {
	if err := owner.unlocked(); err != nil {
		return err
	}
	if owner.connectEntered != nil {
		owner.connectOnce.Do(func() { close(owner.connectEntered) })
		<-owner.connectResume
	}
	return owner.SessionHandle.Connect()
}
func (owner *counterReceiptOwner) SetMTU(value int) error {
	if err := owner.unlocked(); err != nil {
		return err
	}
	return owner.SessionHandle.SetMTU(value)
}
func (owner *counterReceiptOwner) SetReceiveMRU(value uint16) error {
	if err := owner.unlocked(); err != nil {
		return err
	}
	return owner.SessionHandle.SetReceiveMRU(value)
}
func (owner *counterReceiptOwner) SetAdminUp() error {
	if err := owner.unlocked(); err != nil {
		return err
	}
	return owner.SessionHandle.SetAdminUp()
}
func (owner *counterReceiptOwner) Disconnect() error {
	if err := owner.unlocked(); err != nil {
		return err
	}
	owner.disconnects.Add(1)
	if owner.fault.Load() == 3 {
		panic("original disconnect provider panic")
	}
	return owner.SessionHandle.Disconnect()
}
func (owner *counterReceiptOwner) Counters() (SessionCounters, error) {
	if err := owner.unlocked(); err != nil {
		return SessionCounters{}, err
	}
	reads := owner.reads.Add(1)
	if reads > 1 {
		if owner.readEntered != nil {
			owner.readOnce.Do(func() { close(owner.readEntered) })
			<-owner.readResume
		}
		switch owner.fault.Load() {
		case 1:
			return SessionCounters{}, errors.New("original counter provider failed")
		case 2:
			panic("original counter provider panic")
		}
	}
	return SessionCounters{RxBytes: uint64(100 * reads), TxBytes: uint64(300 * reads), RxPackets: uint64(7 * reads), TxPackets: uint64(11 * reads)}, nil
}
func (owner *counterReceiptOwner) Close() error {
	owner.closes.Add(1)
	if owner.closeEntered != nil {
		owner.closeOnce.Do(func() { close(owner.closeEntered) })
		<-owner.closeResume
	}
	return owner.SessionHandle.Close()
}
func (owner *counterReceiptOwner) unblock() {
	if owner.connectResume != nil {
		owner.releaseConnect.Do(func() { close(owner.connectResume) })
	}
	if owner.closeResume != nil {
		owner.releaseClose.Do(func() { close(owner.closeResume) })
	}
	if owner.readResume != nil {
		owner.releaseRead.Do(func() { close(owner.readResume) })
	}
}

type counterReceiptFixture struct {
	driver     *Driver
	owner      *counterReceiptOwner
	peer       net.Conn
	assignment EventSessionIPAssigned
	receipt    NetworkCounterReceipt
}

func admitCounterReceipt(t *testing.T, driver *Driver, sid uint16, owner *counterReceiptOwner) counterReceiptFixture {
	t.Helper()
	if driver == nil {
		driver = NewProductionDriver(discardLogger())
		if err := driver.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(driver.Stop)
	}
	// Registered after Driver.Stop so a failed assertion releases every
	// artificial provider barrier before cleanup joins the original worker.
	t.Cleanup(owner.unblock)
	start, peer, original := frameLeaseStart(t, sid)
	owner.SessionHandle = original.SessionHandle
	start.Dataplane = owner
	attempt, err := NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	start.Attempt = attempt
	start.DisableIPCP = false
	frameLeaseProxy(&start)
	driver.SessionsIn() <- start
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	opened, valid := event.(EventLCPUp)
	if !valid {
		t.Fatalf("first real admission event=%T", event)
	}
	var authEvent AuthEvent
	select {
	case authEvent = <-driver.AuthEventsOut():
	case <-time.After(time.Second):
		t.Fatal("actual admitted auth request was not emitted")
	}
	request, valid := authEvent.(EventAuthRequest)
	if !valid {
		t.Fatalf("actual auth event=%T", authEvent)
	}
	if request.Control != opened.Control {
		t.Fatal("auth request changed admitted owner")
	}
	owner.worker.Store(opened.Control.worker)
	answerAuthRequestForTest(t, request, authResponseMsg{accept: true})
	// Drain this original's success before another helper borrows the same
	// Driver auth stream; it must not become that sibling's request.
	select {
	case authEvent = <-driver.AuthEventsOut():
	case <-time.After(time.Second):
		t.Fatal("actual admitted auth success was not emitted")
	}
	accepted, valid := authEvent.(eventAuthSuccess)
	if !valid {
		t.Fatalf("actual auth completion event=%T", authEvent)
	}
	if accepted.Control != request.Control {
		t.Fatal("auth success changed the original admitted control")
	}
	if accepted.TunnelID != request.TunnelID {
		t.Fatal("auth success changed the original tunnel label")
	}
	if accepted.SessionID != request.SessionID {
		t.Fatal("auth success changed the original session label")
	}
	ipcp, _ := pendingLifecycleNCP(t, peer, driver.IPEventsOut(), AddressFamilyIPv4)
	writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck, ipcp.Identifier, ipcp.Data)
	event = waitForEvent(t, driver.EventsOut(), time.Second)
	assigned, valid := event.(EventSessionIPAssigned)
	if !valid {
		t.Fatalf("genuine IPCP published %T", event)
	}
	if assigned.Control != request.Control {
		t.Fatal("IPCP changed original admitted control")
	}
	receipt, err := assigned.Control.CaptureNetworkCounters(assigned.NetworkLifetime)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := assigned.BaselineCounters.Counters()
	if err != nil {
		t.Fatal(err)
	}
	if baseline != (l2tpevents.TrafficTotals{RxBytes: 100, TxBytes: 300, RxPackets: 7, TxPackets: 11}) {
		t.Fatalf("baseline=%+v", baseline)
	}
	return counterReceiptFixture{driver: driver, owner: owner, peer: peer, assignment: assigned, receipt: receipt}
}

func requireReceiptTotals(t *testing.T, receipt NetworkCounterReceipt) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	sample, err := receipt.TerminalCounters(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got, err := sample.Counters()
	if err != nil {
		t.Fatal(err)
	}
	want := l2tpevents.TrafficTotals{RxBytes: 200, TxBytes: 600, RxPackets: 14, TxPackets: 22}
	if got != want {
		t.Fatalf("original duplex terminal totals=%+v, want %+v", got, want)
	}
}
func requireReceiptSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal(description)
	}
}

// MUTATION: publish readiness after actual owner.Close returns.
func TestCounterReceiptReadyBeforeBlockedNativeClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		owner := &counterReceiptOwner{closeEntered: make(chan struct{}), closeResume: make(chan struct{})}
		fixture := admitCounterReceipt(t, nil, 211, owner)
		fixture.assignment.Acknowledge()
		awaitLifecycleUp(t, fixture.driver.EventsOut())
		if !fixture.assignment.Control.Cancel() {
			t.Fatal("original cancellation failed")
		}
		requireReceiptSignal(t, owner.closeEntered, "original Close was not entered")
		sample, ready := fixture.receipt.TerminalSnapshot()
		if !ready {
			t.Fatal("original terminal receipt waited for blocked native Close")
		}
		if _, err := sample.Counters(); err != nil {
			t.Fatal(err)
		}
		requireReceiptTotals(t, fixture.receipt)
		if owner.disconnects.Load() != 1 {
			t.Fatalf("Disconnect calls=%d", owner.disconnects.Load())
		}
		owner.unblock()
		synctest.Wait()
		status, err := fixture.assignment.Control.CleanupStatus()
		if err != nil {
			t.Fatal(err)
		}
		if !status.Retired {
			t.Fatal("original worker did not retire after Close resumed")
		}
		if !status.Released {
			t.Fatal("original owner release was not confirmed")
		}
		// Retained receipt remains useful after the admitted worker and row retire.
		requireReceiptTotals(t, fixture.receipt)
	})
}

// MUTATION: transient release-gate contention completes the receipt Unknown;
// or hold worker/Driver metadata while original Connect is in flight.
func TestCounterReceiptWaitsForInFlightOriginalConnect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		owner := &counterReceiptOwner{connectEntered: make(chan struct{}), connectResume: make(chan struct{})}
		fixture := admitCounterReceipt(t, nil, 212, owner)
		fixture.assignment.Acknowledge()
		requireReceiptSignal(t, owner.connectEntered, "original Connect was not entered")
		if !fixture.assignment.Control.Cancel() {
			t.Fatal("original cancellation failed")
		}
		synctest.Wait()
		if _, ready := fixture.receipt.TerminalSnapshot(); ready {
			t.Fatal("in-flight original Connect became a completed receipt")
		}
		if owner.closes.Load() != 0 {
			t.Fatal("Close overtook original Connect")
		}
		// A second actual owner on the SAME Driver must negotiate, publish, and
		// answer a new LCP Echo while the first native operation stays blocked.
		other := admitCounterReceipt(t, fixture.driver, 213, &counterReceiptOwner{})
		other.assignment.Acknowledge()
		awaitLifecycleUp(t, fixture.driver.EventsOut())
		frameLeaseEcho(t, other.peer)
		if other.owner.closes.Load() != 0 {
			t.Fatal("independent useful owner was closed")
		}
		owner.unblock()
		synctest.Wait()
		requireReceiptTotals(t, fixture.receipt)
		if owner.disconnects.Load() != 1 {
			t.Fatalf("Disconnect calls=%d", owner.disconnects.Load())
		}
	})
}

// Provider failures are completed unavailable measurements; caller cancellation
// while the original read is in flight is pending work that can later succeed.
func TestCounterReceiptProviderFailureAndCancellation(t *testing.T) {
	for _, mode := range []struct {
		name  string
		fault int32
		cause string
	}{
		{"error", 1, "original counter provider failed"},
		{"counter-panic", 2, "original counter provider panic"},
		{"disconnect-panic", 3, "original disconnect provider panic"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				owner := &counterReceiptOwner{}
				fixture := admitCounterReceipt(t, nil, 214, owner)
				fixture.assignment.Acknowledge()
				awaitLifecycleUp(t, fixture.driver.EventsOut())
				owner.fault.Store(mode.fault)
				if !fixture.assignment.Control.Cancel() {
					t.Fatal("original cancellation failed")
				}
				synctest.Wait()
				sample, ready := fixture.receipt.TerminalSnapshot()
				if !ready {
					t.Fatal("provider failure left original receipt pending")
				}
				if _, err := sample.Counters(); err == nil {
					t.Fatal("provider failure invented measured zero")
				} else if !strings.Contains(err.Error(), mode.cause) {
					t.Fatalf("provider cause=%v", err)
				}
				if owner.closes.Load() != 1 {
					t.Fatalf("provider failure skipped actual Close: calls=%d", owner.closes.Load())
				}
				if owner.disconnects.Load() != 1 {
					t.Fatalf("provider failure repeated Disconnect: calls=%d", owner.disconnects.Load())
				}
			})
		})
	}
	t.Run("canceled-wait-can-later-complete", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			owner := &counterReceiptOwner{readEntered: make(chan struct{}), readResume: make(chan struct{})}
			fixture := admitCounterReceipt(t, nil, 215, owner)
			fixture.assignment.Acknowledge()
			awaitLifecycleUp(t, fixture.driver.EventsOut())
			if !fixture.assignment.Control.Cancel() {
				t.Fatal("original cancellation failed")
			}
			requireReceiptSignal(t, owner.readEntered, "terminal read was not entered")
			ctx, cancel := context.WithCancel(t.Context())
			waitResult := make(chan error, 1)
			waitDone := make(chan struct{})
			go func() {
				defer close(waitDone)
				_, err := fixture.receipt.TerminalCounters(ctx)
				waitResult <- err
			}()
			t.Cleanup(func() { owner.unblock(); cancel(); <-waitDone })
			synctest.Wait()
			cancel()
			select {
			case err := <-waitResult:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("canceled in-flight waiter=%v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("in-flight terminal waiter ignored cancellation")
			}
			<-waitDone
			if _, ready := fixture.receipt.TerminalSnapshot(); ready {
				t.Fatal("wait cancellation completed original measurement")
			}
			owner.unblock()
			synctest.Wait()
			requireReceiptTotals(t, fixture.receipt)
			if owner.closes.Load() != 1 {
				t.Fatal("resolved original read did not continue physical cleanup")
			}
		})
	})
}

// MUTATION: sample terminal counters before resetNCP joins the final IPv6 writer.
func TestCounterReceiptResetIncludesFinalIPv6Writer(t *testing.T) {
	session, _, _ := newRFC1661Session(LCPStateOpened)
	owner := testHandleFor(session)
	var finalAdvertisement atomic.Bool
	owner.readCounters = func() (SessionCounters, error) {
		if finalAdvertisement.Load() {
			return SessionCounters{RxBytes: 200, TxBytes: 600, RxPackets: 14, TxPackets: 22}, nil
		}
		return SessionCounters{RxBytes: 100, TxBytes: 300, RxPackets: 7, TxPackets: 11}, nil
	}
	if _, _, err := session.prepareNetworkCounters(); err != nil {
		t.Fatal(err)
	}
	session.networkCounterMu.Lock()
	receipt := NetworkCounterReceipt{network: session.networkCurrent}
	session.networkCounterMu.Unlock()
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	session.channelConnected = true
	service := newIPv6Service(iPv6ServiceConfig{Dataplane: owner})
	service.stop = func() error { finalAdvertisement.Store(true); return nil }
	session.ipv6Svc = service
	if !session.resetNCP() {
		t.Fatal("actual reset refused a clean original network")
	}
	if !finalAdvertisement.Load() {
		t.Fatal("reset did not complete the final IPv6 writer")
	}
	requireReceiptTotals(t, receipt)
	session.closeFiles()
	requireReceiptTotals(t, receipt)
}

// MUTATION: keep nativeReleaseMu through blocked EventSessionUp publication.
func TestCounterReceiptCloseDoesNotWaitForSessionUpConsumer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		driver := NewProductionDriver(discardLogger())
		driver.eventsOut = make(chan Event)
		if err := driver.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(driver.Stop)
		fixture := admitCounterReceipt(t, driver, 216, &counterReceiptOwner{})
		fixture.assignment.Acknowledge()
		synctest.Wait()
		if !fixture.assignment.Control.Cancel() {
			t.Fatal("original cancellation failed")
		}
		if err := fixture.assignment.Control.CloseCanceledOwner(t.Context()); err != nil {
			t.Fatalf("physical cleanup waited for the blocked SessionUp consumer: %v", err)
		}
		requireReceiptTotals(t, fixture.receipt)
		if fixture.owner.closes.Load() != 1 {
			t.Fatal("original Close was not performed")
		}
		event := waitForEvent(t, driver.EventsOut(), time.Second)
		if _, valid := event.(EventSessionUp); !valid {
			t.Fatalf("blocked publisher=%T", event)
		}
	})
}
