// Design: docs/architecture/l2tp/subscriber-session-model.md -- captured original observation
//go:build linux

package l2tp

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/callsink"
)

// MUTATION: capture through subsystem.observer, rather than the original
// reactor binding, returns replacement history despite a still-current row.
func TestCapturedSessionHistoryKeepsOriginalObserverAndHiddenScope(t *testing.T) {
	generation := cqmAPIGeneration(t)
	original := cqmAPIObserver(t, generation, nil, 2, 2)
	root, selector, _, _ := sessionCaptureDispatchFixtureForObservation(t, original, generation)
	captured, err := root.CaptureSession(selector)
	require.NoError(t, err)
	require.NoError(t, captured.RecordDisconnect("alice", "original request", 6))
	replacement := cqmAPIObserver(t, cqmAPIGeneration(t), nil, 2, 2)
	root.observer = replacement
	replacement.recordDisconnect(replacement.generation, cqmAPIBirth(t), selector.TID, selector.SID, "mallory", "other root", 7)
	// Capture after the subsystem pointer changed must still borrow its row's
	// original reactor binding. Neither read may reselect the current observer.
	second, err := root.CaptureSession(selector)
	require.NoError(t, err)
	for _, reader := range []CapturedSession{captured, second} {
		events, readErr := reader.SessionEvents()
		require.NoError(t, readErr)
		require.Len(t, events, 1)
		require.Equal(t, "alice", events[0].Actor)
		require.Equal(t, selector.Birth, events[0].Birth)
		snapshot, snapshotErr := reader.Snapshot()
		require.NoError(t, snapshotErr)
		require.Equal(t, generation, snapshot.Generation)
		require.Equal(t, selector.Birth.String(), snapshot.SubscriberID)
		require.Equal(t, reader.Listener(), snapshot.Listener)
	}
	root.retiredReactors, root.reactors = root.reactors, nil
	original.RetainKnownSessions()
	retained, err := root.CaptureSession(selector)
	require.NoError(t, err, "a fresh capture discovers the retained original root")
	require.Equal(t, selector.Birth, retained.Birth())
	snapshot, err := retained.Snapshot()
	require.NoError(t, err)
	require.Equal(t, generation, snapshot.Generation)
	require.Equal(t, selector.Birth.String(), snapshot.SubscriberID)
	history, err := retained.SessionEvents()
	require.NoError(t, err)
	require.Equal(t, original.SessionEvents(selector.Birth), history, "fresh captured history reads the retained original binding")
}

// MUTATION: a captured history reader performs a fresh SID lookup and exposes
// the replacement row after the old actual admission has been removed.
func TestCapturedSessionHistoryRefusesActualSIDReuse(t *testing.T) {
	generation := cqmAPIGeneration(t)
	observer := cqmAPIObserver(t, generation, nil, 3, 2)
	root, selector, admit, _ := sessionCaptureDispatchFixtureForObservation(t, observer, generation)
	old, err := root.CaptureSession(selector)
	require.NoError(t, err)
	require.NoError(t, old.RecordDisconnect("alice", "first request", 6))
	require.NoError(t, old.Teardown(t.Context(), l2tpevents.TerminateCauseAdminReset, "first request"))
	replacement := admit()
	require.NotEqual(t, selector.Birth, replacement.Birth)
	current, err := root.CaptureSession(replacement)
	require.NoError(t, err)
	require.NoError(t, current.RecordDisconnect("bob", "replacement request", 6))
	events, err := old.SessionEvents()
	require.ErrorIs(t, err, ErrSessionNotFound)
	require.Nil(t, events)
	events, err = current.SessionEvents()
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, replacement.Birth, events[0].Birth)
	require.Equal(t, "bob", events[0].Actor)
}

// MUTATION: applying lifecycle's known-ring seal to a validated capture loses
// optional original request history; reopening Stop admits it after closure.
func TestCapturedDisconnectHistoryUsesRetainedOriginalLaneUntilClosed(t *testing.T) {
	generation := cqmAPIGeneration(t)
	observer := cqmAPIObserver(t, generation, nil, 2, 1)
	root, selector, _, _ := sessionCaptureDispatchFixtureForObservation(t, observer, generation)
	captured, err := root.CaptureSession(selector)
	require.NoError(t, err)
	observer.RetainKnownSessions()
	require.Empty(t, observer.SessionSummaries())
	require.NoError(t, captured.RecordDisconnect("alice", "retained original", 6))
	history, err := captured.SessionEvents()
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, selector.Birth, history[0].Birth)
	require.Empty(t, observer.LoginSummaries(), "optional command observation never acquires quality custody")
	observer.Stop()
	require.NoError(t, captured.RecordDisconnect("alice", "closed observer", 6), "optional monitoring closure cannot reject a valid command")
	after, err := captured.SessionEvents()
	require.NoError(t, err)
	require.Equal(t, history, after)
}

// cqmAPINativeAdmission installs original observation before listener, ICRQ,
// ICCN and real Driver admission. It uses the existing pipe-backed native
// owner, not a kernel-I/O substitute claimed as real physical I/O. Cleanup MUST
// close the peer before Driver.Stop joins its workers.
func cqmAPINativeAdmission(t *testing.T, observer *Observer, bus *cqmAPIBus) *actorRouteAdmission {
	t.Helper()
	captureBirthSIDDraw(t)
	reactor := newReactorForSnapshot(t)
	reactor.params.Generation, reactor.observer = observer.generation, observer
	reactor.SetEventBus(bus)
	reactor.listener = newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), reactor.logger)
	require.NoError(t, reactor.listener.Start(t.Context()))
	t.Cleanup(func() { require.NoError(t, reactor.listener.Stop()) })
	insertEstablishedTunnel(t, reactor, 17, 27, netip.MustParseAddrPort("127.0.0.1:1701"))
	tunnel := reactor.tunnelsByLocalID[17]
	captureBirthAdmit(t, reactor, tunnel, 70)
	session := tunnel.sessions[7]
	birth, id := session.Birth(), session.SubscriberID()
	require.True(t, birth.IsValid())
	options := []byte{1, 4, 5, 220, 3, 4, 0xc0, 0x23}
	proxy := &iccnProxy{lcp: callsink.ProxyLCP{InitialReceived: options, LastSent: options, LastReceived: options}}
	var packet [BufSize]byte
	length := writeICCNBody(packet[:], 64000, 1, proxy)
	tunnel.handleICCN(session, packet[:length], reactor.params.Clock(), reactor.logger)
	require.Equal(t, L2TPSessionEstablished, session.state)
	session.pppInterface = "ppp7"
	lcp, err := ppp.NewLCPConfig(3*time.Second, 2, 10, 5)
	require.NoError(t, err)
	native, peer := net.Pipe()
	owner := &currentCoreHandle{Conn: native, identity: ppp.SessionIdentity{InterfaceName: "ppp7", NativeIndex: 7}}
	driver := ppp.NewProductionDriver(reactor.logger)
	if err := driver.Start(); err != nil {
		require.NoError(t, peer.Close())
		require.NoError(t, native.Close())
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := peer.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		driver.Stop()
		ClearSessionMetadata(id)
	})
	attempt, err := ppp.NewSessionAttempt(id, owner)
	require.NoError(t, err)
	session.pppAttempt = attempt
	wire := &currentCoreWire{driver: driver, peer: peer,
		configurations: make(map[ppp.AddressFamily]ppp.LCPPacket), answers: make(map[ppp.AddressFamily]ppp.IPResponseArgs)}
	wire.startSession = ppp.StartSession{
		LCP: lcp, TunnelID: 17, SessionID: 7, L2TPBirth: birth, SubscriberID: id,
		Attempt: attempt, Frames: owner, Dataplane: owner, LNSMode: true, DisableIPv6CP: true,
		AuthMethod: ppp.AuthMethodPAP, AuthFallbackOrder: []ppp.AuthMethod{ppp.AuthMethodPAP},
		AuthTimeout: time.Hour, IPTimeout: time.Hour, EchoInterval: time.Hour,
		ProxyLCPInitialRecv: options, ProxyLCPLastSent: options, ProxyLCPLastRecv: options,
	}
	select {
	case driver.SessionsIn() <- wire.startSession:
	case <-time.After(orderTestBound):
		t.Fatal("real Driver admission did not receive its original owner")
	}
	opened, ok := wire.event(t).(ppp.EventLCPUp)
	require.True(t, ok)
	require.Equal(t, attempt, opened.Control.Attempt())
	wire.opened = opened
	request := wire.authRequest(t, "alice")
	wire.acceptAuth(t, request)
	addressRequest := wire.poolRequest(t)
	answer := ppp.IPResponseArgs{Accept: true, Family: ppp.AddressFamilyIPv4,
		Local: netip.MustParseAddr("192.0.2.1"), Peer: netip.MustParseAddr("192.0.2.41")}
	require.NoError(t, driver.IPResponse(addressRequest, answer))
	wire.answers[ppp.AddressFamilyIPv4] = answer
	wire.configurations[ppp.AddressFamilyIPv4] = wire.read(t, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
	wire.assignment = wire.assign(t, ppp.AddressFamilyIPv4)
	reactor.handleSessionIPAssigned(wire.assignment)
	reactor.handleSessionUp(wire.nextUp(t))
	return &actorRouteAdmission{reactor: reactor, session: session, wire: wire}
}

// MUTATION: any actual lifecycle publisher omitting or replacing its original
// reactor generation makes that event disappear from its scoped observer.
func TestObserverNativePublishersKeepOriginalScopeThroughBothDownPaths(t *testing.T) {
	observer := cqmAPIObserver(t, cqmAPIGeneration(t), nil, 4, 2)
	bus := newCQMNativeBus(t)
	(&Subsystem{observer: observer}).wireObserverSubscriptions(bus)
	admitted := cqmAPINativeAdmission(t, observer, bus)
	birth := admitted.session.Birth()
	state := observer.EchoState("alice")
	require.NotNil(t, state)
	require.Equal(t, "established", state.BucketState)
	history := observer.SessionEvents(birth)
	require.Len(t, history, 2)
	require.Equal(t, ObserverEventSessionIPAssigned, history[0].Type)
	require.Equal(t, ObserverEventSessionUp, history[1].Type)
	// This test judges the Echo publisher's stamp, using a real admitted PPP
	// control receipt. The RTT value itself is an explicit test input.
	admitted.reactor.handleEchoRTT(ppp.EventEchoRTT{
		TunnelID: 17, SessionID: 7, Birth: birth, SubscriberID: birth.String(),
		Control: admitted.wire.opened.Control, RTT: 13 * time.Millisecond,
	})
	require.Len(t, observer.SessionEvents(birth), 3)
	require.Equal(t, 13*time.Millisecond, observer.EchoState("alice").LastRTT)
	observer.RetainKnownSessions()
	admitted.reactor.handleLCPDown(admitted.wire.beginRestart(t))
	require.Empty(t, observer.SessionEvents(birth))
	require.Equal(t, "down", observer.EchoState("alice").BucketState)
	// Re-admit the optional capture lane after network withdrawal. The actual
	// terminal drain then has a known retained ring to release, making its
	// separately delayed Down publisher's generation independently observable.
	observer.recordDisconnect(observer.generation, birth, 17, 7, "alice", "terminal request", 6)
	require.Len(t, observer.SessionEvents(birth), 1)
	admitted.reactor.retireSessions()
	require.Empty(t, observer.SessionEvents(birth), "the delayed terminal Down carries the same original native scope")
	require.Equal(t, "down", observer.EchoState("alice").BucketState)
}
