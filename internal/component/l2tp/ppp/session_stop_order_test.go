// Design: docs/research/l2tpv2-ze-integration.md -- PPP service and frame-owner teardown

package ppp

import (
	"errors"
	"net/netip"
	"reflect"
	"sync"
	"testing"
	"time"
)

type sessionStopTrace struct {
	mu    sync.Mutex
	steps []string
}

func (s *sessionStopTrace) note(step string) {
	s.mu.Lock()
	s.steps = append(s.steps, step)
	s.mu.Unlock()
}

func (s *sessionStopTrace) snapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.steps...)
}

type sessionStopOwner struct {
	SessionHandle
	trace *sessionStopTrace
}

func (s *sessionStopOwner) Close() error {
	s.trace.note("owner-close")
	return s.SessionHandle.Close()
}

func sessionStopFixture() (*pppSession, *sessionStopTrace) {
	trace := &sessionStopTrace{}
	owner := &sessionStopOwner{
		SessionHandle: newTestSessionHandle(&fakeBackend{}, "fixture-session"),
		trace:         trace,
	}
	service := newIPv6Service(iPv6ServiceConfig{Dataplane: owner})
	service.stop = func() error {
		trace.note("final-ra")
		return nil
	}
	return &pppSession{
		tunnelID: 1, sessionID: 2,
		logger: discardLogger(), dataplane: owner, ipv6Svc: service,
		sessStop: make(chan struct{}), done: make(chan struct{}),
	}, trace
}

// The actual stop method is isolated from negotiation by a controlled session
// waiter. It must stop that session's IPv6 service before closing its frames.
func TestDriverStopSessionStopsIPv6BeforeFrameOwner(t *testing.T) {
	driver, trace, control := sessionStopDriver(t)
	if err := driver.StopSession(control); err != nil {
		t.Fatal(err)
	}
	if got := trace.snapshot(); !reflect.DeepEqual(got, []string{"final-ra", "owner-close"}) {
		t.Fatalf("session stop order %v", got)
	}
}

// Whole-driver cancellation owes the same service-before-owner order as the
// individual session path, even though it also stops the dispatch worker.
func TestDriverStopStopsIPv6BeforeFrameOwner(t *testing.T) {
	driver, trace, _ := sessionStopDriver(t)
	driver.Stop()
	if got := trace.snapshot(); !reflect.DeepEqual(got, []string{"final-ra", "owner-close"}) {
		t.Fatalf("driver stop order %v", got)
	}
}

func sessionStopDriver(t *testing.T) (*Driver, *sessionStopTrace, SessionControl) {
	t.Helper()
	driver := newSessionTestDriver()
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, _, _ := frameLeaseStart(t, 1)
	trace := &sessionStopTrace{}
	owner := &sessionStopOwner{SessionHandle: start.Dataplane, trace: trace}
	start.Dataplane = owner
	attempt, err := NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	start.Attempt = attempt
	frameLeaseProxy(&start)
	driver.SessionsIn() <- start
	up, opened := waitForEventOfType[EventSessionUp](t, driver.EventsOut(), time.Second)
	if !opened {
		t.Fatal("stop fixture did not admit its actual PPP worker")
	}
	driver.mu.Lock()
	session, found := driver.sessionForControlLocked(up.Control)
	driver.mu.Unlock()
	if !found {
		t.Fatal("observed stop fixture worker is absent")
	}
	service := newIPv6Service(iPv6ServiceConfig{Dataplane: owner})
	service.stop = func() error { trace.note("final-ra"); return nil }
	session.mu.Lock()
	session.ipv6Svc = service
	session.mu.Unlock()
	return driver, trace, up.Control
}

// Receipt of SessionDown permits the access transport to send PADT. The real
// event publisher must therefore finish the final RA before granting receipt.
func TestSessionDownStopsIPv6BeforePublication(t *testing.T) {
	session, trace := sessionStopFixture()
	events := make(chan Event, 1)
	session.eventsOut = events
	session.stopCh = make(chan struct{})
	session.sendEvent(EventSessionDown{TunnelID: 1, SessionID: 2, Reason: "finished"})
	// The test observes the publisher's completed handoff before emulating
	// the transport's close, so owner teardown cannot create the RA receipt.
	select {
	case event := <-events:
		if _, ending := event.(EventSessionDown); !ending {
			t.Fatalf("published %T, want SessionDown", event)
		}
	default:
		t.Fatal("the session did not publish its completed teardown")
	}
	trace.note("down-published")
	session.closeFiles()
	if got := trace.snapshot(); !reflect.DeepEqual(got, []string{"final-ra", "down-published", "owner-close"}) {
		t.Fatalf("publication order %v", got)
	}
}

// Holding the same session lock as activation makes its service installation
// visible to a simultaneous close; the closer cannot capture a stale nil.
func TestSessionCloseSeesInFlightIPv6Activation(t *testing.T) {
	session, trace := sessionStopFixture()
	service := session.ipv6Svc
	session.ipv6Svc = nil
	session.mu.Lock()
	closeStarted := make(chan struct{})
	closeDone := make(chan struct{})
	go func() {
		close(closeStarted)
		session.closeFiles()
		close(closeDone)
	}()
	<-closeStarted
	session.ipv6Svc = service
	session.mu.Unlock()
	<-closeDone
	if got := trace.snapshot(); !reflect.DeepEqual(got, []string{"final-ra", "owner-close"}) {
		t.Fatalf("activation/close order %v", got)
	}
}

// A failed old-lifetime withdrawal cannot be ignored and replaced by a fresh
// negotiation on the same data-plane handle. Keep its address and service state.
func TestNCPResetRefusesReuseAfterIPv6CleanupFailure(t *testing.T) {
	session, _, _ := newRFC1661Session(LCPStateOpened)
	owner := testHandleFor(session)
	want := errors.New("route withdrawal refused")
	backend, ok := owner.backend.(*fakeBackend)
	if !ok {
		t.Fatalf("session backend = %T, want *fakeBackend", owner.backend)
	}
	backend.removeRouteErr = want
	service := newIPv6Service(iPv6ServiceConfig{Dataplane: owner})
	service.routeInstalled = true
	service.delegatedPrefix = netip.MustParsePrefix("2001:db8:42::/64")
	session.ipv6Svc = service
	session.localIPv4 = netip.MustParseAddr("192.0.2.1")
	session.peerIPv4 = netip.MustParseAddr("192.0.2.2")
	session.ipcpState = LCPStateOpened
	if session.resetNCP() {
		t.Fatal("failed IPv6 withdrawal allowed reuse of the session")
	}
	if session.ipv6Svc != service {
		t.Fatal("failed service state was discarded")
	}
	if session.localIPv4 != netip.MustParseAddr("192.0.2.1") {
		t.Fatal("failed reset cleared the old address")
	}
	if session.ipcpState != LCPStateOpened {
		t.Fatal("failed reset initialized a replacement NCP lifetime")
	}
}
