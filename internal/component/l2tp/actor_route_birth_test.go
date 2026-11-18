// Design: docs/architecture/l2tp/subscriber-session-model.md -- original actor identity and captured route metadata
// Related: reactor_kernel.go -- actual PPP assignment and network withdrawal

package l2tp

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/callsink"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// TestActorRouteKeepsSameTupleOnTwoListenersApart drives two real PPP admissions
// with equal wire labels through the reactor. Neither assignment may remove the
// other's route, and both peers must continue answering useful LCP traffic.
// MUTATION: route records keyed by TID/SID replace the first listener's address.
func TestActorRouteKeepsSameTupleOnTwoListenersApart(t *testing.T) {
	captureBirthSIDDraw(t)
	bus := &recordingBus{}
	observer := newSubscriberRouteObserver(nil, bus)
	first := newActorRouteAdmission(t, observer, actorRouteInput{
		username: "first", address: netip.MustParseAddr("192.0.2.31"),
	})
	second := newActorRouteAdmission(t, observer, actorRouteInput{
		username: "second", address: netip.MustParseAddr("192.0.2.32"),
	})
	require.NotEqual(t, first.reactor.listener.Addr(), second.reactor.listener.Addr())
	require.NotEqual(t, first.session.Birth(), second.session.Birth())
	actorRouteEcho(t, first.wire)
	actorRouteEcho(t, second.wire)

	adds := actorRouteEntries(t, bus.events(), redistevents.ActionAdd, 0)
	removes := actorRouteEntries(t, bus.events(), redistevents.ActionRemove, 0)
	require.Len(t, adds, 2)
	require.ElementsMatch(t, []netip.Prefix{
		netip.MustParsePrefix("192.0.2.31/32"), netip.MustParsePrefix("192.0.2.32/32"),
	}, []netip.Prefix{adds[0].Prefix, adds[1].Prefix})
	require.Empty(t, removes, "another listener's admitted birth cannot replace this address")
	_, withdrawn, active := observer.Stats()
	require.Zero(t, withdrawn)
	require.Equal(t, 2, active)
}

// TestActorRouteCapturesOriginalFramedMetadata assigns different RADIUS profiles
// to equal labels on two listeners through actual auth requests. Withdrawing
// one network lifetime must leave the other address and framed route replayable.
// MUTATION: loading framed metadata through a reconstructed tuple loses both
// original profiles; keying their records by that tuple loses the survivor.
func TestActorRouteCapturesOriginalFramedMetadata(t *testing.T) {
	captureBirthSIDDraw(t)
	bus := &recordingBus{}
	observer := newSubscriberRouteObserver(nil, bus)
	first := newActorRouteAdmission(t, observer, actorRouteInput{
		username: "first", address: netip.MustParseAddr("192.0.2.41"),
		framedRoutes: []FramedRoute{{Prefix: netip.MustParsePrefix("198.51.100.0/25"), Metric: 11}},
	})
	second := newActorRouteAdmission(t, observer, actorRouteInput{
		username: "second", address: netip.MustParseAddr("192.0.2.42"),
		framedRoutes: []FramedRoute{{Prefix: netip.MustParsePrefix("198.51.100.128/25"), Metric: 22}},
	})
	first.reactor.handleLCPDown(first.wire.beginRestart(t))
	observer.reemitAll(81)
	actorRouteEcho(t, second.wire)

	adds := actorRouteEntries(t, bus.events(), redistevents.ActionAdd, 0)
	removes := actorRouteEntries(t, bus.events(), redistevents.ActionRemove, 0)
	replayed := actorRouteEntries(t, bus.events(), redistevents.ActionAdd, 81)
	require.Len(t, adds, 4, "each admitted actor must supply its original address and framed route")
	require.Contains(t, adds, redistevents.RouteChangeEntry{
		Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("198.51.100.0/25"),
		Metric: 11, SourceInstance: adds[0].SourceInstance,
	})
	require.Len(t, removes, 2, "only the first address and framed route may be withdrawn")
	require.ElementsMatch(t, []netip.Prefix{
		netip.MustParsePrefix("192.0.2.41/32"), netip.MustParsePrefix("198.51.100.0/25"),
	}, []netip.Prefix{removes[0].Prefix, removes[1].Prefix})
	require.Len(t, replayed, 2, "the second birth must retain both route contributions")
	require.ElementsMatch(t, []netip.Prefix{
		netip.MustParsePrefix("192.0.2.42/32"), netip.MustParsePrefix("198.51.100.128/25"),
	}, []netip.Prefix{replayed[0].Prefix, replayed[1].Prefix})
	for _, entry := range replayed {
		if entry.Prefix == netip.MustParsePrefix("198.51.100.128/25") {
			require.EqualValues(t, 22, entry.Metric)
		}
	}
}

// TestActorRouteContributionsKeepOriginalBirthTokens gives both listener births
// the same address prefix. Add, replay and removal must retain two
// independent producer tokens, with the survivor still answering PPP traffic.
// MUTATION: routeInstance(TID,SID) aliases both contributions and their removal.
func TestActorRouteContributionsKeepOriginalBirthTokens(t *testing.T) {
	captureBirthSIDDraw(t)
	bus := &recordingBus{}
	observer := newSubscriberRouteObserver(nil, bus)
	input := actorRouteInput{
		username: "same-login", address: netip.MustParseAddr("192.0.2.51"),
	}
	first := newActorRouteAdmission(t, observer, input)
	second := newActorRouteAdmission(t, observer, input)
	observer.reemitAll(91)
	first.reactor.handleLCPDown(first.wire.beginRestart(t))
	observer.reemitAll(92)
	actorRouteEcho(t, second.wire)

	adds := actorRouteEntries(t, bus.events(), redistevents.ActionAdd, 0)
	removes := actorRouteEntries(t, bus.events(), redistevents.ActionRemove, 0)
	before := actorRouteEntries(t, bus.events(), redistevents.ActionAdd, 91)
	after := actorRouteEntries(t, bus.events(), redistevents.ActionAdd, 92)
	require.Len(t, adds, 2)
	firstToken, secondToken := adds[0].SourceInstance, adds[1].SourceInstance
	require.NotZero(t, firstToken)
	require.NotZero(t, secondToken)
	require.NotEqual(t, firstToken, secondToken, "native contribution tokens must distinguish equal labels")
	require.Len(t, before, 2)
	require.Len(t, removes, 1)
	require.Len(t, after, 1)
	for _, entry := range removes {
		require.Equal(t, firstToken, entry.SourceInstance)
	}
	for _, entry := range after {
		require.Equal(t, secondToken, entry.SourceInstance)
	}
	for _, token := range []uint32{firstToken, secondToken} {
		count := 0
		for _, entry := range before {
			if entry.SourceInstance == token {
				count++
			}
		}
		require.Equal(t, 1, count, "each original address contribution must survive replay")
	}
}

type actorRouteInput struct {
	username     string
	address      netip.Addr
	framedRoutes []FramedRoute
}

type actorRouteAdmission struct {
	reactor *l2tpReactor
	session *L2TPSession
	wire    *currentCoreWire
}

// newActorRouteAdmission uses actual ICRQ/ICCN allocation, Driver admission,
// PAP and IPCP. Only the native data plane is the existing pipe-backed owner;
// this helper claims no kernel-worker or real native-I/O proof. Cleanup MUST
// close the peer before Driver.Stop joins its owned worker.
func newActorRouteAdmission(t *testing.T, observer *subscriberRouteObserver, input actorRouteInput) *actorRouteAdmission {
	t.Helper()
	reactor := newReactorForSnapshot(t)
	reactor.listener = newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), reactor.logger)
	require.NoError(t, reactor.listener.Start(t.Context()))
	t.Cleanup(func() { require.NoError(t, reactor.listener.Stop()) })
	reactor.routeObserver = observer
	insertEstablishedTunnel(t, reactor, 17, 27, netip.MustParseAddrPort("127.0.0.1:1701"))
	tunnel := reactor.tunnelsByLocalID[17]
	captureBirthAdmit(t, reactor, tunnel, 70)
	session := tunnel.sessions[7]
	birth := session.Birth()
	require.True(t, birth.IsValid())
	id := birth.String()
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
		if closeErr := peer.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		if closeErr := native.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	wire := &currentCoreWire{
		driver: driver, peer: peer,
		configurations: make(map[ppp.AddressFamily]ppp.LCPPacket),
		answers:        make(map[ppp.AddressFamily]ppp.IPResponseArgs),
	}
	// The Driver owns the transferred owner. The test MUST NOT Close it again.
	t.Cleanup(func() {
		if err := peer.Close(); err != nil {
			t.Error(err)
		}
		driver.Stop()
		ClearSessionMetadata(id)
	})
	attempt, err := ppp.NewSessionAttempt(id, owner)
	require.NoError(t, err)
	reactor.tunnelsMu.Lock()
	session.pppAttempt = attempt
	reactor.tunnelsMu.Unlock()
	wire.startSession = ppp.StartSession{
		LCP: lcp, TunnelID: 17, SessionID: 7,
		L2TPBirth: birth, SubscriberID: id, Attempt: attempt, Frames: owner, Dataplane: owner,
		LNSMode: true, DisableIPv6CP: true, AuthMethod: ppp.AuthMethodPAP,
		AuthFallbackOrder: []ppp.AuthMethod{ppp.AuthMethodPAP}, AuthTimeout: time.Hour,
		IPTimeout: time.Hour, EchoInterval: time.Hour, ProxyLCPInitialRecv: options,
		ProxyLCPLastSent: options, ProxyLCPLastRecv: options,
	}
	select {
	case driver.SessionsIn() <- wire.startSession:
	case <-time.After(orderTestBound):
		t.Fatal("actual Driver did not receive the explicit original admission")
	}
	opened, valid := wire.event(t).(ppp.EventLCPUp)
	require.True(t, valid)
	require.Equal(t, attempt, opened.Control.Attempt())
	require.Equal(t, birth, opened.Birth)
	require.Equal(t, id, opened.SubscriberID)
	wire.opened = opened
	request := wire.authRequest(t, input.username)
	require.Equal(t, id, request.SubscriberID)
	require.Equal(t, opened.Control, request.Control)
	StoreSessionMetadata(request.SubscriberID, &AuthMetadata{FramedRoutes: input.framedRoutes})
	wire.acceptAuth(t, request)
	addressRequest := wire.poolRequest(t)
	require.Equal(t, id, addressRequest.SubscriberID)
	require.Equal(t, opened.Control, addressRequest.Control)
	answer := ppp.IPResponseArgs{
		Accept: true, Family: ppp.AddressFamilyIPv4,
		Local: netip.MustParseAddr("192.0.2.1"), Peer: input.address,
	}
	require.NoError(t, driver.IPResponse(addressRequest, answer))
	wire.answers[ppp.AddressFamilyIPv4] = answer
	wire.configurations[ppp.AddressFamilyIPv4] = wire.read(t, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
	wire.assignment = wire.assign(t, ppp.AddressFamilyIPv4)
	require.Equal(t, opened.Control, wire.assignment.Control)
	require.Equal(t, birth, wire.assignment.Birth)
	require.Equal(t, id, wire.assignment.SubscriberID)
	reactor.handleSessionIPAssigned(wire.assignment)
	up := wire.nextUp(t)
	require.Equal(t, opened.Control, up.Control)
	require.Equal(t, birth, up.Birth)
	require.Equal(t, id, up.SubscriberID)
	reactor.handleSessionUp(up)
	return &actorRouteAdmission{reactor: reactor, session: session, wire: wire}
}

func actorRouteEcho(t *testing.T, wire *currentCoreWire) {
	t.Helper()
	wire.write(t, ppp.ProtoLCP, ppp.LCPEchoRequest, 0x73, []byte{0, 0, 0, 0, 0x5a})
	reply := wire.read(t, ppp.ProtoLCP, ppp.LCPEchoReply)
	require.EqualValues(t, 0x73, reply.Identifier)
	require.Len(t, reply.Data, 5)
	require.EqualValues(t, 0x5a, reply.Data[4])
	require.True(t, wire.opened.Control.Active(), "the useful peer must retain its original live control")
}

func actorRouteEntries(t *testing.T, emitted []emittedEvent, action redistevents.RouteAction, replay uint64) []redistevents.RouteChangeEntry {
	t.Helper()
	var entries []redistevents.RouteChangeEntry
	for _, event := range emitted {
		batch, valid := event.payload.(*redistevents.RouteChangeBatch)
		require.True(t, valid)
		require.NotNil(t, batch)
		if batch.ReplayID != replay {
			continue
		}
		for _, entry := range batch.Entries {
			if entry.Action == action {
				entries = append(entries, entry)
			}
		}
	}
	return entries
}
