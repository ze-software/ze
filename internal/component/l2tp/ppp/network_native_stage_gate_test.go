// Design: docs/research/l2tpv2-ze-integration.md -- exception-safe native stages before publication.
package ppp

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"testing/synctest"
)

// nativeStageOwner is a synchronous provider model at the actual native call
// boundaries. It starts no Driver, worker, carrier goroutine or OS resource.
// The roots below prove gate exception-safety, not worker panic containment.
type nativeStageOwner struct {
	SessionHandle
	connect                                func() error
	open                                   func(context.Context) (IPv6Carrier, error)
	connects, adds, removes, opens, closes int
}

func newNativeStageOwner() *nativeStageOwner {
	return &nativeStageOwner{SessionHandle: newTestSessionHandle(nil, "ppp-native-stage")}
}
func (owner *nativeStageOwner) Connect() error {
	owner.connects++
	if owner.connect != nil {
		return owner.connect()
	}
	return nil
}
func (owner *nativeStageOwner) AddAddressP2P(netip.Prefix, netip.Prefix) error {
	owner.adds++
	return nil
}
func (owner *nativeStageOwner) RemoveAddress(netip.Prefix) error {
	owner.removes++
	return nil
}
func (owner *nativeStageOwner) OpenIPv6Carrier(ctx context.Context) (IPv6Carrier, error) {
	owner.opens++
	if owner.open != nil {
		return owner.open(ctx)
	}
	return nil, errors.New("controlled unavailable IPv6 carrier")
}
func (owner *nativeStageOwner) Close() error {
	owner.closes++
	return owner.SessionHandle.Close()
}

type nativeStageCarrier struct {
	IPv6Carrier
	close func() error
}

func (carrier *nativeStageCarrier) Close() error { return carrier.close() }

func newNativeStageSession(owner *nativeStageOwner) (*pppSession, chan Event) {
	events := make(chan Event, 1)
	return &pppSession{
		dataplane: owner, logger: discardLogger(),
		stopCh: make(chan struct{}), sessStop: make(chan struct{}),
		eventsOut: events, channelConnected: true,
		maxMRU: 1500, localMRU: 1500, negotiatedMRU: 1500,
		localInterfaceID: [8]byte{7: 1}, peerInterfaceID: [8]byte{7: 2},
		peerInterfaceIDNegotiated: true,
	}, events
}

func requireNativeStageGateFree(t *testing.T, session *pppSession) {
	t.Helper()
	if !session.nativeReleaseMu.TryLock() {
		t.Fatal("native provider unwind retained the original owner gate")
	}
	session.nativeReleaseMu.Unlock()
}

func recoverNativeStage(call func()) (failure any) {
	defer func() { failure = recover() }()
	call()
	return nil
}

// A panic from the actual RA close adapter MUST propagate with the same owner
// gate released. A controlled same-goroutine recovery then enters closeFiles;
// it does not assert that the real Driver recovers a worker panic.
// MUTATION: omit prepareSessionDown's deferred nativeReleaseMu.Unlock.
func TestSessionDownProviderPanicReleasesNativeStageGate(t *testing.T) {
	owner := newNativeStageOwner()
	session, events := newNativeStageSession(owner)
	fault := errors.New("controlled original carrier close panic")
	closer := &raCarrierClose{carrier: &nativeStageCarrier{close: func() error { panic(fault) }}}
	session.ipv6Svc = &IPv6Service{stop: closer.Close}
	if got := recoverNativeStage(func() { session.sendEvent(EventSessionDown{}) }); got != fault {
		t.Fatalf("actual carrier panic = %v, want original %v", got, fault)
	}
	requireNativeStageGateFree(t, session)
	if len(events) != 0 {
		t.Fatal("the panicking native stage published Down")
	}
	session.closeFiles()
	if owner.closes != 1 {
		t.Fatalf("final cleanup entered actual Close %d times, want 1", owner.closes)
	}
	requireNativeStageGateFree(t, session)
}

// Both original reconciliation branches invoke the real native stage: IPv4
// reaches Connect, while IPv6 installs its pair and reaches OpenIPv6Carrier.
// A deliberate provider panic propagates without stranding final cleanup.
// MUTATION: omit reconcileNCPFamily's deferred nativeReleaseMu.Unlock.
func TestReconcileNCPProviderPanicReleasesNativeStageGate(t *testing.T) {
	cases := []struct {
		name   string
		family AddressFamily
	}{
		{"ipv4-connect", AddressFamilyIPv4}, {"ipv6-carrier-start", AddressFamilyIPv6},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			owner := newNativeStageOwner()
			session, events := newNativeStageSession(owner)
			fault := errors.New("controlled original reconciliation panic")
			if test.family == AddressFamilyIPv4 {
				owner.connect = func() error { panic(fault) }
			}
			if test.family == AddressFamilyIPv6 {
				owner.open = func(context.Context) (IPv6Carrier, error) { panic(fault) }
			}
			if got := recoverNativeStage(func() { session.reconcileOpenedNCP(test.family) }); got != fault {
				t.Fatalf("native reconciliation panic = %v, want original %v", got, fault)
			}
			requireNativeStageGateFree(t, session)
			if owner.connects != 1 {
				t.Fatalf("actual Connect calls = %d, want 1", owner.connects)
			}
			if test.family == AddressFamilyIPv6 {
				if owner.adds != 1 {
					t.Fatalf("actual IPv6 pair adds = %d, want 1", owner.adds)
				}
				if owner.opens != 1 {
					t.Fatalf("actual carrier opens = %d, want 1", owner.opens)
				}
			}
			if len(events) != 0 {
				t.Fatal("provider panic was replaced by a published refusal")
			}
			session.closeFiles()
			if owner.closes != 1 {
				t.Fatalf("final cleanup Close calls = %d, want 1", owner.closes)
			}
			requireNativeStageGateFree(t, session)
		})
	}
}

// Useful success, stopped/no-connect and ordinary error returns preserve the
// original outcomes. In particular fail publishes only after its native helper
// returned, so its own Down stage can acquire the same owner gate.
func TestReconcileNCPReturnsReleaseNativeStageGate(t *testing.T) {
	cases := []struct {
		name                    string
		closed, connected, fail bool
		want                    bool
		connects                int
	}{
		{name: "stopped", closed: true, connected: true},
		{name: "not-connected", want: true},
		{name: "connect-success", connected: true, want: true, connects: 1},
		{name: "connect-error", connected: true, fail: true, connects: 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			owner := newNativeStageOwner()
			session, events := newNativeStageSession(owner)
			session.filesClosed, session.channelConnected = test.closed, test.connected
			if test.fail {
				owner.connect = func() error { return errors.New("controlled reconciliation refusal") }
			}
			if got := session.reconcileOpenedNCP(AddressFamilyIPv4); got != test.want {
				t.Fatalf("reconciliation = %v, want %v", got, test.want)
			}
			requireNativeStageGateFree(t, session)
			if owner.connects != test.connects {
				t.Fatalf("Connect calls = %d, want %d", owner.connects, test.connects)
			}
			if test.fail {
				select {
				case delivered := <-events:
					event, valid := delivered.(EventSessionDown)
					if !valid {
						t.Fatal("ordinary native error did not publish actual Down")
					}
					if event.Reason == "" {
						t.Fatal("ordinary native error lost its refusal reason")
					}
				default:
					t.Fatal("ordinary native error returned without publishing actual Down")
				}
			} else if len(events) != 0 {
				t.Fatal("successful/stopped reconciliation invented a Down")
			}
		})
	}
	// Carrier acquisition failure still withdraws its attempted pair and keeps
	// the independent family usable; it is not a whole-network refusal.
	owner := newNativeStageOwner()
	session, events := newNativeStageSession(owner)
	if !session.reconcileOpenedNCP(AddressFamilyIPv6) {
		t.Fatal("ordinary IPv6 carrier refusal ended the independent family")
	}
	requireNativeStageGateFree(t, session)
	if owner.adds != 1 {
		t.Fatalf("IPv6 adds = %d, want 1", owner.adds)
	}
	if owner.removes != 1 {
		t.Fatalf("IPv6 rollback withdrawals = %d, want 1", owner.removes)
	}
	if owner.opens != 1 {
		t.Fatalf("carrier opens = %d, want 1", owner.opens)
	}
	if len(events) != 0 {
		t.Fatal("ordinary IPv6 refusal invented a whole-network Down")
	}
}

// Hold the actual publisher on its unbuffered consumer, rather than measuring
// elapsed time. Both Down variants MUST already have released their native
// stage. The one test publisher is joined on success and on every failure.
// MUTATION: reacquire nativeReleaseMu around sendEvent's publication select.
func TestDownPublicationDoesNotHoldNativeStageGate(t *testing.T) {
	cases := []struct {
		name  string
		event Event
	}{
		{"session-down", EventSessionDown{}}, {"lcp-down", EventLCPDown{}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				owner := newNativeStageOwner()
				session, events := newNativeStageSession(owner)
				events = make(chan Event)
				session.eventsOut = events
				stop := make(chan struct{})
				session.stopCh = stop
				done := make(chan struct{})
				// One controlled publication lifecycle, with an explicit stop and join.
				go func() { defer close(done); session.sendEvent(test.event) }()
				defer func() { close(stop); <-done }()
				synctest.Wait()
				requireNativeStageGateFree(t, session)
				select {
				case delivered := <-events:
					if delivered == nil {
						t.Fatal("normal original Down was not delivered")
					}
				case <-done:
					t.Fatal("normal original publisher ended without delivering Down")
				}
				<-done
			})
		})
	}
}
