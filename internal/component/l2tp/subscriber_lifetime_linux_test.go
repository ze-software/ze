//go:build linux

// Design: docs/architecture/l2tp/subscriber-session-model.md
// RFC 1661 Sections 3.4 and 3.6: a new LCP exchange ends the network
// lifetime, not the underlying transport. NCPs complete independently.
package l2tp

import (
	"context"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
)

var l2tpLifetimeMetrics = metrics.NewPrometheusRegistry()

// BindMetrics has process-lifetime ownership and no reset API. Run only the
// metric-reading scenario in a child so shuffled package tests cannot claim
// its collector first, and this test cannot claim another test's collector.
func isolatedL2TPLifetime(t *testing.T) bool {
	t.Helper()
	const key = "ZE_L2TP_SUBSCRIBER_LIFETIME_TEST"
	if os.Getenv(key) == t.Name() {
		return true
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1", "-test.timeout=30s")
	cmd.Env = append(os.Environ(), key+"="+t.Name())
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "isolated subscriber lifecycle:\n%s", output)
	return false
}

func l2tpActiveSubscribers(t *testing.T) float64 {
	t.Helper()
	subscriber.BindMetrics(l2tpLifetimeMetrics)
	gauge := l2tpLifetimeMetrics.GaugeVec("ze_subscriber_sessions", "Number of active subscriber sessions.", []string{"access_type"}).With(string(subscriber.AccessL2TP))
	var sample dto.Metric
	writer, ok := gauge.(interface{ Write(*dto.Metric) error })
	require.True(t, ok, "gauge %T cannot write a metric sample", gauge)
	require.NoError(t, writer.Write(&sample))
	return sample.GetGauge().GetValue()
}

// assertNoL2TPLifetimePacket checks the already-completed synchronous send
// boundary, rather than sleeping for an assumed quiet interval.
func assertNoL2TPLifetimePacket(t *testing.T, peer *net.UDPConn) {
	t.Helper()
	raw, err := peer.SyscallConn()
	require.NoError(t, err)
	var recvErr error
	require.NoError(t, raw.Control(func(fd uintptr) {
		var buf [4096]byte
		_, _, recvErr = unix.Recvfrom(int(fd), buf[:], unix.MSG_DONTWAIT)
	}))
	require.ErrorIs(t, recvErr, unix.EAGAIN, "network withdrawal must not send CDN")
}

// PREVENTS: losing the first NCP's values at SessionUp, ignoring LCPDown,
// keeping routes/authorization/timers across reauthentication, and counting
// an incomplete network lifetime as an active subscriber.
func TestL2TPSubscriberNetworkLifetime(t *testing.T) {
	if !isolatedL2TPLifetime(t) {
		return
	}
	for _, activateFirst := range []bool{true, false} {
		name := "active"
		if !activateFirst {
			name = "configuring"
		}
		t.Run(name, func(t *testing.T) {
			const tid, sid = uint16(410), uint16(411)
			_, r, stop := newUnstartedReactor(t)
			t.Cleanup(stop)
			peer, peerAddr := openPeerSocket(t)
			tun := mkTunnel(r, tid, 510, peerAddr)
			transport := addEstablishedSession(t, tun, sid, 511, true)
			id := transport.SubscriberID()
			transport.pppInterface = "ppp41"
			t.Cleanup(func() { cancelSessionTimeouts(transport) })
			t.Cleanup(func() { ClearSessionMetadata(id) })

			bus := newTestBus()
			reg := subscriber.NewRegistry()
			bridge := newSubscriberBridge(reg, bus, r.logger)
			t.Cleanup(bridge.stop)
			r.eventBus = bus
			reader := StartSubscriberRouteObserver(r.logger, bus, subscriber.AccessL2TP, l2tpevents.ProtocolID, l2tpevents.RouteChange)
			t.Cleanup(reader.Stop)
			liveRoutes := make(map[netip.Prefix]bool)
			var removed []netip.Prefix
			bus.Subscribe(l2tpevents.Namespace, redistevents.EventType, func(payload any) {
				batch, ok := payload.(*redistevents.RouteChangeBatch)
				if !ok {
					t.Errorf("route event payload is %T, want *redistevents.RouteChangeBatch", payload)
					return
				}
				for _, entry := range batch.Entries {
					switch entry.Action {
					case redistevents.ActionAdd:
						liveRoutes[entry.Prefix] = true
					case redistevents.ActionRemove:
						delete(liveRoutes, entry.Prefix)
						removed = append(removed, entry.Prefix)
					case redistevents.ActionUnspecified:
						t.Errorf("unexpected route action %v for %s", entry.Action, entry.Prefix)
					default:
						// Route actions are an open event-bus input; report unknown actions.
						t.Errorf("unexpected route action %v for %s", entry.Action, entry.Prefix)
					}
				}
			})
			var up []subscriber.Session
			var down []subscriber.Session
			subevents.SessionUp.Subscribe(bus, func(p *subevents.SessionUpPayload) {
				registered, ok := reg.Get(id)
				require.True(t, ok, "registry insertion must precede session-up publication")
				require.Equal(t, p.Session, registered)
				up = append(up, p.Session)
			})
			subevents.SessionDown.Subscribe(bus, func(p *subevents.SessionDownPayload) {
				_, ok := reg.Get(id)
				require.False(t, ok, "registry removal must precede withdrawal publication")
				down = append(down, p.Session)
			})
			baseline := l2tpActiveSubscribers(t)
			oldRoute := netip.MustParsePrefix("198.51.100.0/24")
			oldV6Route := netip.MustParsePrefix("2001:db8:41::/64")
			StoreSessionMetadata(id, &AuthMetadata{
				FramedPool: "old-pool", FilterID: "rate:20mbit/5mbit",
				SessionTimeout: 3600, IdleTimeout: 3600,
				FramedRoutes: []FramedRoute{{Prefix: oldRoute}, {Prefix: oldV6Route}},
			})
			wire := newCurrentCoreAdmission(t, r, &currentCoreAdmissionConfig{ids: currentCoreSessionIDs{tunnelID: tid, sessionID: sid}, ipv6: true, subscriberID: ""})
			wire.prepareNetwork(t, "old-user", ppp.IPResponseArgs{Accept: true, Family: ppp.AddressFamilyIPv4,
				Local: netip.MustParseAddr("192.0.2.1"), Peer: netip.MustParseAddr("192.0.2.41"),
				DNSPrimary: netip.MustParseAddr("192.0.2.53"), DNSSecondary: netip.MustParseAddr("192.0.2.54")},
				[8]byte{2, 0, 0, 0, 0, 0, 0, 41})
			first := wire.assign(t, ppp.AddressFamilyIPv4)
			firstV6 := ppp.EventSessionIPAssigned{TunnelID: tid, SessionID: sid, Family: ppp.AddressFamilyIPv6, Username: "old-user", InterfaceID: [8]byte{2, 0, 0, 0, 0, 0, 0, 41}}
			r.handlePPPEvent(first)
			configured, ok := reg.Get(id)
			require.True(t, ok, "IPCP assignment before SessionUp must be retained")
			require.Equal(t, subscriber.StateConfiguring, configured.State)
			require.Equal(t, "old-user", configured.Username)
			require.Equal(t, first.Peer, configured.IPv4Addr)
			require.Equal(t, first.DNSPrimary, configured.DNSPrimary)
			require.Equal(t, first.DNSSecondary, configured.DNSSecondary)
			require.Equal(t, baseline, l2tpActiveSubscribers(t))
			// The IPv4 install announces the host and the IPv4 framed route; the
			// IPv6 framed route waits for the IPv6 install (AC-4).
			oldPrefixes := []netip.Prefix{netip.PrefixFrom(first.Peer, 32), oldRoute}
			require.Equal(t, map[netip.Prefix]bool{oldPrefixes[0]: true, oldRoute: true}, liveRoutes,
				"the IPv4 assignment must announce exactly its host and IPv4 framed route")

			var timeoutCtx, idleCtx context.Context
			if activateFirst {
				firstV6 = wire.assign(t, ppp.AddressFamilyIPv6)
				r.handlePPPEvent(firstV6)
				oldPrefixes = append(oldPrefixes, oldV6Route)
				require.Len(t, liveRoutes, len(oldPrefixes))
				for _, prefix := range oldPrefixes {
					require.True(t, liveRoutes[prefix], "missing published route %s", prefix)
				}
				r.handlePPPEvent(wire.nextUp(t))
				require.Len(t, up, 1)
				assertL2TPLifetimeAddresses(t, &up[0], first, firstV6)
				require.Equal(t, "old-pool", up[0].PoolName)
				require.Equal(t, baseline+1, l2tpActiveSubscribers(t))
				// Wrap the real timeout owners so cancellation is observable without
				// waiting an hour for a RADIUS timer to expire.
				require.NotNil(t, transport.sessionTimeoutCancel)
				require.NotNil(t, transport.idleTimeoutCancel)
				var cancelTimeout, cancelIdle context.CancelFunc
				timeoutCtx, cancelTimeout = context.WithCancel(context.Background())
				idleCtx, cancelIdle = context.WithCancel(context.Background())
				oldTimeout, oldIdle := transport.sessionTimeoutCancel, transport.idleTimeoutCancel
				transport.sessionTimeoutCancel = func() { oldTimeout(); cancelTimeout() }
				transport.idleTimeoutCancel = func() { oldIdle(); cancelIdle() }
			}
			withdrawal := wire.beginRestart(t)
			require.True(t, withdrawal.NetworkPhase)
			r.handlePPPEvent(withdrawal)
			_, ok = reg.Get(id)
			require.False(t, ok, "LCPDown must withdraw the old subscriber")
			require.Empty(t, liveRoutes, "LCPDown must publish route withdrawals")
			require.ElementsMatch(t, oldPrefixes, removed)
			require.Nil(t, LoadSessionMetadata(id), "old authorization must not survive replacement authentication")
			require.Len(t, down, 1)
			tidDown, sidDown := down[0].PPPKey()
			require.Equal(t, tid, tidDown)
			require.Equal(t, sid, sidDown)
			require.Equal(t, baseline, l2tpActiveSubscribers(t), "configuring withdrawal must not decrement active subscribers")
			if activateFirst {
				require.ErrorIs(t, timeoutCtx.Err(), context.Canceled)
				require.ErrorIs(t, idleCtx.Err(), context.Canceled)
			}
			require.Same(t, transport, tun.lookupSession(sid), "LCP restart must retain the transport identifiers")
			assertNoL2TPLifetimePacket(t, peer)

			// A different authorization and both NCPs reuse the exact transport;
			// reverse the completion order to catch family overwrite in either direction.
			newRoute := netip.MustParsePrefix("203.0.113.0/24")
			StoreSessionMetadata(id, &AuthMetadata{FramedPool: "new-pool", FramedRoutes: []FramedRoute{{Prefix: newRoute}}})
			second := first
			second.Username = "new-user"
			second.Peer = netip.MustParseAddr("192.0.2.42")
			second.DNSPrimary = netip.MustParseAddr("198.51.100.53")
			second.DNSSecondary = netip.MustParseAddr("198.51.100.54")
			secondV6 := firstV6
			secondV6.Username = "new-user"
			secondV6.InterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 42}
			wire.finishRestart(t)
			wire.prepareNetwork(t, "new-user", ppp.IPResponseArgs{Accept: true, Family: ppp.AddressFamilyIPv4,
				Local: netip.MustParseAddr("192.0.2.1"), Peer: second.Peer,
				DNSPrimary: second.DNSPrimary, DNSSecondary: second.DNSSecondary}, secondV6.InterfaceID)
			secondV6 = wire.assign(t, ppp.AddressFamilyIPv6)
			r.handlePPPEvent(secondV6)
			configured, ok = reg.Get(id)
			require.True(t, ok)
			require.Equal(t, subscriber.StateConfiguring, configured.State)
			require.Equal(t, secondV6.InterfaceID, configured.IPv6InterfaceID)
			second = wire.assign(t, ppp.AddressFamilyIPv4)
			r.handlePPPEvent(second)
			r.handlePPPEvent(wire.nextUp(t))
			latest := up[len(up)-1]
			assertL2TPLifetimeAddresses(t, &latest, second, secondV6)
			require.Equal(t, "new-user", latest.Username)
			require.Equal(t, "new-pool", latest.PoolName)
			require.Equal(t, baseline+1, l2tpActiveSubscribers(t))
			require.Equal(t, map[netip.Prefix]bool{netip.PrefixFrom(second.Peer, 32): true, newRoute: true}, liveRoutes)

			r.handlePPPEvent(wire.finishPeerDown(t))
			require.Nil(t, tun.lookupSession(sid))
			_, ok = reg.Get(id)
			require.False(t, ok)
			require.Empty(t, liveRoutes)
			require.Nil(t, LoadSessionMetadata(id))
			require.Len(t, down, 2)
			require.Equal(t, second.Peer, down[1].IPv4Addr)
			require.Equal(t, baseline, l2tpActiveSubscribers(t))
			require.NoError(t, peer.SetReadDeadline(time.Now().Add(2*time.Second)))
			var packet [4096]byte
			n, _, err := peer.ReadFromUDP(packet[:])
			require.NoError(t, err)
			header, err := ParseMessageHeader(packet[:n])
			require.NoError(t, err)
			it := NewAVPIterator(packet[header.PayloadOff:int(header.Length)])
			_, attr, _, value, found := it.Next()
			require.True(t, found)
			require.Equal(t, AVPMessageType, attr)
			message, err := readAVPUint16(value)
			require.NoError(t, err)
			require.Equal(t, MsgCDN, MessageType(message), "only final SessionDown terminates the transport")
		})
	}
}

// Proxy identity remains available for the initial admission, but an accepted
// PPP principal takes precedence and neither can survive a new LCP lifetime.
func TestL2TPAssignedPrincipalAndProxyLifetime(t *testing.T) {
	for _, tc := range []struct {
		name      string
		proxy     string
		principal string
		want      string
	}{
		{name: "PPP authentication", principal: "alice", want: "alice"},
		{name: "PPP replaces proxy", proxy: "proxy-user", principal: "alice", want: "alice"},
		{name: "proxy admission", proxy: "proxy-user", want: "proxy-user"},
		{name: "explicit no-auth"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const tid, sid = uint16(420), uint16(421)
			_, r, stop := newUnstartedReactor(t)
			t.Cleanup(stop)
			tun := mkTunnel(r, tid, 520, netip.MustParseAddrPort("192.0.2.1:1701"))
			transport := addEstablishedSession(t, tun, sid, 521, true)
			transport.username = tc.proxy
			transport.pppInterface = "ppp42"
			bus := newTestBus()
			r.eventBus = bus
			reg := subscriber.NewRegistry()
			bridge := newSubscriberBridge(reg, bus, r.logger)
			t.Cleanup(bridge.stop)
			var accountingPrincipals []string
			l2tpevents.SessionIPAssigned.Subscribe(bus, func(p *l2tpevents.SessionIPAssignedPayload) {
				accountingPrincipals = append(accountingPrincipals, p.Username)
			})
			wire := newCurrentCoreAdmission(t, r, &currentCoreAdmissionConfig{ids: currentCoreSessionIDs{tunnelID: tid, sessionID: sid}, ipv6: false, subscriberID: ""})
			answer := ppp.IPResponseArgs{Accept: true, Family: ppp.AddressFamilyIPv4,
				Local: netip.MustParseAddr("192.0.2.1"), Peer: netip.MustParseAddr("192.0.2.42")}
			wire.prepareNetwork(t, tc.principal, answer, [8]byte{})
			assigned := wire.assign(t, ppp.AddressFamilyIPv4)
			r.handlePPPEvent(assigned)
			sess, ok := reg.Get(transport.SubscriberID())
			require.True(t, ok)
			require.Equal(t, tc.want, sess.Username)

			withdrawal := wire.beginRestart(t)
			require.True(t, withdrawal.NetworkPhase)
			r.handlePPPEvent(withdrawal)
			wire.finishRestart(t)
			wire.prepareNetwork(t, "", answer, [8]byte{})
			assigned = wire.assign(t, ppp.AddressFamilyIPv4)
			r.handlePPPEvent(assigned)
			sess, ok = reg.Get(transport.SubscriberID())
			require.True(t, ok)
			require.Empty(t, sess.Username, "replacement no-auth admission must not inherit an identity")
			require.Equal(t, []string{tc.want, ""}, accountingPrincipals)
		})
	}
}

func assertL2TPLifetimeAddresses(t *testing.T, sess *subscriber.Session, v4, v6 ppp.EventSessionIPAssigned) {
	t.Helper()
	require.Equal(t, subscriber.StateActive, sess.State)
	require.Equal(t, v4.Peer, sess.IPv4Addr)
	require.Equal(t, v4.DNSPrimary, sess.DNSPrimary)
	require.Equal(t, v4.DNSSecondary, sess.DNSSecondary)
	require.Equal(t, v6.InterfaceID, sess.IPv6InterfaceID)
}

// PREVENTS: invalid or unowned assignments manufacturing subscriber state.
func TestL2TPSubscriberRejectsUnusableAssignments(t *testing.T) {
	_, r, stop := newUnstartedReactor(t)
	t.Cleanup(stop)
	bus := newTestBus()
	reg := subscriber.NewRegistry()
	bridge := newSubscriberBridge(reg, bus, r.logger)
	t.Cleanup(bridge.stop)
	r.eventBus = bus
	tun := mkTunnel(r, 420, 520, netip.MustParseAddrPort("127.0.0.1:1701"))
	addEstablishedSession(t, tun, 421, 521, true)
	publications := 0
	subevents.SessionIPAssigned.Subscribe(bus, func(*subevents.SessionIPAssignedPayload) { publications++ })
	wire := newCurrentCoreWire(t, r, &currentCoreSessionIDs{tunnelID: 420, sessionID: 421})
	for index, metadata := range []ppp.EventSessionIPAssigned{
		{TunnelID: 420, SessionID: 421, Family: ppp.AddressFamilyIPv4},
		{TunnelID: 420, SessionID: 421, Family: ppp.AddressFamilyIPv6},
		{TunnelID: 420, SessionID: 421, Family: ppp.AddressFamilyIPv4, Peer: netip.MustParseAddr("2001:db8::99")},
		{TunnelID: 420, SessionID: 999, Family: ppp.AddressFamilyIPv4, Peer: netip.MustParseAddr("192.0.2.99")},
		{TunnelID: 999, SessionID: 421, Family: ppp.AddressFamilyIPv4, Peer: netip.MustParseAddr("192.0.2.99")},
	} {
		event := wire.assignment
		if index > 0 {
			event = wire.replacementAssignment(t)
		}
		// Deliberately malformed consumer metadata retains the original
		// private receipt; PPP did not produce these invalid values.
		event.TunnelID, event.SessionID, event.Family = metadata.TunnelID, metadata.SessionID, metadata.Family
		event.Peer, event.Username, event.InterfaceID = metadata.Peer, metadata.Username, metadata.InterfaceID
		r.handlePPPEvent(event)
		_, ok := reg.Get(wire.startSession.SubscriberID)
		require.False(t, ok)
	}
	// The bridge is also a bus consumer: malformed strings must not turn
	// into a registry entry carrying the zero address as a real assignment.
	_, err := l2tpevents.SessionIPAssigned.Emit(bus, &l2tpevents.SessionIPAssignedPayload{Birth: wire.startSession.L2TPBirth, SubscriberID: wire.startSession.SubscriberID, TunnelID: 420, SessionID: 421, PeerAddr: "not-an-address"})
	require.NoError(t, err)
	_, ok := reg.Get(wire.startSession.SubscriberID)
	require.False(t, ok)
	require.Equal(t, 0, publications)
}

// Seed an already-published network lifetime so the LCPDown regression has
// an independent RED even when an old consumer also loses pre-Up assignments.
func TestL2TPSubscriberLCPDownWithdrawsPublishedLifetime(t *testing.T) {
	const tid, sid = uint16(450), uint16(451)
	_, r, stop := newUnstartedReactor(t)
	t.Cleanup(stop)
	peer, peerAddr := openPeerSocket(t)
	tun := mkTunnel(r, tid, 550, peerAddr)
	transport := addEstablishedSession(t, tun, sid, 551, true)
	id := transport.SubscriberID()
	bus := newTestBus()
	reg := subscriber.NewRegistry()
	bridge := newSubscriberBridge(reg, bus, r.logger)
	t.Cleanup(bridge.stop)
	r.eventBus = bus
	reader := StartSubscriberRouteObserver(r.logger, bus, subscriber.AccessL2TP, l2tpevents.ProtocolID, l2tpevents.RouteChange)
	t.Cleanup(reader.Stop)
	addr := netip.MustParseAddr("192.0.2.45")
	reg.Add(&subscriber.Session{
		ID: id, L2TPBirth: transport.Birth(), AccessType: subscriber.AccessL2TP,
		TunnelID: tid, SessionID: sid, State: subscriber.StateConfiguring, IPv4Addr: addr,
	})
	StoreSessionMetadata(id, &AuthMetadata{FramedPool: "old-pool"})
	t.Cleanup(func() { ClearSessionMetadata(id) })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	transport.sessionTimeoutCancel = cancel
	var removed []netip.Prefix
	bus.Subscribe(l2tpevents.Namespace, redistevents.EventType, func(payload any) {
		batch, ok := payload.(*redistevents.RouteChangeBatch)
		if !ok {
			t.Errorf("route event payload is %T, want *redistevents.RouteChangeBatch", payload)
			return
		}
		for _, entry := range batch.Entries {
			if entry.Action == redistevents.ActionRemove {
				removed = append(removed, entry.Prefix)
			}
		}
	})
	var down []subscriber.Session
	subevents.SessionDown.Subscribe(bus, func(p *subevents.SessionDownPayload) { down = append(down, p.Session) })

	wire := newCurrentCoreWire(t, r, &currentCoreSessionIDs{tunnelID: tid, sessionID: sid})
	withdrawal := wire.withdraw(t)
	require.True(t, withdrawal.NetworkPhase)
	seedL2TPRouteNotice(t, bus, transport.Birth(), tid, sid, addr, withdrawal.NetworkLifetime)
	r.handlePPPEvent(withdrawal)
	require.Equal(t, []netip.Prefix{netip.PrefixFrom(addr, 32)}, removed, "LCPDown must publish withdrawal of the existing route")
	_, ok := reg.Get(id)
	require.False(t, ok)
	require.Len(t, down, 1)
	require.Equal(t, addr, down[0].IPv4Addr)
	require.Nil(t, LoadSessionMetadata(id))
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.Same(t, transport, tun.lookupSession(sid))
	assertNoL2TPLifetimePacket(t, peer)
}

// TestL2TPFamilyReleaseWithdrawsLiveIPv4Routes proves the redistribution feed
// follows an IPv4 family release on the real actor chain: the PPP session's
// release reaches the reactor, the subscriber bridge republishes it as the
// subscriber release, and the route observer removes that family alone.
//
// Method: a dual-stack L2TP session on the actual PPP driver holds one
// Framed-Route and one Framed-IPv6-Route; the observer's batches are folded
// into liveRoutes, as TestL2TPSubscriberNetworkLifetime does. The peer's new
// IPCP Configure-Request takes IPCP out of Opened (RFC 1661 Section 4.4, tld),
// so the release is the one the PPP session publishes; the peer's
// Configure-Ack then reopens IPCP. The Terminate-Request trigger of the same
// release is TestNCPTerminateAfterSessionUpOnUnitFD (ppp).
//
// MUTATION: remove the subscriber release republication from
// subscriberBridge.onSessionIPReleased (subscriber_bridge.go): the /32 and the
// IPv4 framed route stay in liveRoutes and the release assertion goes red.
func TestL2TPFamilyReleaseWithdrawsLiveIPv4Routes(t *testing.T) {
	chain := newFamilyReleaseChain(t, 460, 461, "ppp46")

	chain.reactor.handlePPPEvent(chain.release(t, ppp.AddressFamilyIPv4))
	require.ElementsMatch(t, []netip.Prefix{chain.host, chain.v4Route}, chain.removed,
		"an IPv4 release removes the /32 and the IPv4 framed routes")
	require.Equal(t, map[netip.Prefix]bool{chain.v6Route: true}, chain.liveRoutes,
		"an IPv4 release leaves the IPv6 framed routes announced")

	chain.reactor.handlePPPEvent(chain.reopen(t, ppp.AddressFamilyIPv4))
	require.ElementsMatch(t, []netip.Prefix{chain.host, chain.v4Route}, chain.added,
		"the IPv4 reopen restores exactly the /32 and the IPv4 framed routes")
	require.Equal(t, chain.allRoutes(), chain.liveRoutes)
}

// TestL2TPFamilyReleaseWithdrawsLiveIPv6FramedRoutes is the IPv6 half of
// TestL2TPFamilyReleaseWithdrawsLiveIPv4Routes, on the same chain and session:
// an IPv6CP renegotiation releases IPv6, which removes the Framed-IPv6-Route
// prefix while the /32 and the IPv4 framed route stay; the reopen restores
// exactly the IPv6 framed route. There is no IPv6 host route to remove.
//
// MUTATION: remove the subscriber release republication from
// subscriberBridge.onSessionIPReleased (subscriber_bridge.go): the IPv6 framed
// route stays in liveRoutes and the release assertion goes red.
func TestL2TPFamilyReleaseWithdrawsLiveIPv6FramedRoutes(t *testing.T) {
	chain := newFamilyReleaseChain(t, 470, 471, "ppp47")

	chain.reactor.handlePPPEvent(chain.release(t, ppp.AddressFamilyIPv6))
	require.Equal(t, []netip.Prefix{chain.v6Route}, chain.removed,
		"an IPv6 release removes only the IPv6 framed routes")
	require.Equal(t, map[netip.Prefix]bool{chain.host: true, chain.v4Route: true}, chain.liveRoutes,
		"an IPv6 release leaves the /32 and the IPv4 framed routes announced")

	chain.reactor.handlePPPEvent(chain.reopen(t, ppp.AddressFamilyIPv6))
	require.Equal(t, []netip.Prefix{chain.v6Route}, chain.added,
		"the IPv6 reopen restores exactly the IPv6 framed routes")
	require.Equal(t, chain.allRoutes(), chain.liveRoutes)
}

// familyReleaseChain is one dual-stack L2TP session wired through the real
// reactor, subscriber bridge and route observer, with the observer's batches
// folded into liveRoutes. added and removed hold the prefixes of the batches
// since the last release or reopen began. Every bus delivery runs on the test
// goroutine, inside handlePPPEvent, so the fold needs no lock.
type familyReleaseChain struct {
	reactor    *l2tpReactor
	wire       *currentCoreWire
	liveRoutes map[netip.Prefix]bool
	added      []netip.Prefix
	removed    []netip.Prefix
	host       netip.Prefix
	v4Route    netip.Prefix
	v6Route    netip.Prefix
	// configure is the Configure-Request Ze sent when the peer's
	// renegotiation took the family out of Opened; reopen acknowledges it.
	configure ppp.LCPPacket
}

func newFamilyReleaseChain(t *testing.T, tid, sid uint16, ifname string) *familyReleaseChain {
	t.Helper()
	_, r, stop := newUnstartedReactor(t)
	t.Cleanup(stop)
	_, peerAddr := openPeerSocket(t)
	tun := mkTunnel(r, tid, tid+100, peerAddr)
	transport := addEstablishedSession(t, tun, sid, sid+100, true)
	id := transport.SubscriberID()
	transport.pppInterface = ifname
	t.Cleanup(func() { cancelSessionTimeouts(transport) })
	t.Cleanup(func() { ClearSessionMetadata(id) })

	bus := newTestBus()
	reg := subscriber.NewRegistry()
	bridge := newSubscriberBridge(reg, bus, r.logger)
	t.Cleanup(bridge.stop)
	r.eventBus = bus
	reader := StartSubscriberRouteObserver(r.logger, bus, subscriber.AccessL2TP, l2tpevents.ProtocolID, l2tpevents.RouteChange)
	t.Cleanup(reader.Stop)

	peer := netip.MustParseAddr("192.0.2.46")
	chain := &familyReleaseChain{
		reactor:    r,
		liveRoutes: make(map[netip.Prefix]bool),
		host:       netip.PrefixFrom(peer, 32),
		v4Route:    netip.MustParsePrefix("198.51.100.0/24"),
		v6Route:    netip.MustParsePrefix("2001:db8:46::/64"),
	}
	bus.Subscribe(l2tpevents.Namespace, redistevents.EventType, func(payload any) {
		batch, ok := payload.(*redistevents.RouteChangeBatch)
		if !ok {
			t.Errorf("route event payload is %T, want *redistevents.RouteChangeBatch", payload)
			return
		}
		for _, entry := range batch.Entries {
			switch entry.Action {
			case redistevents.ActionAdd:
				chain.liveRoutes[entry.Prefix] = true
				chain.added = append(chain.added, entry.Prefix)
			case redistevents.ActionRemove:
				delete(chain.liveRoutes, entry.Prefix)
				chain.removed = append(chain.removed, entry.Prefix)
			case redistevents.ActionUnspecified:
				t.Errorf("unexpected route action %v for %s", entry.Action, entry.Prefix)
			default:
				// Route actions are an open event-bus input; report unknown actions.
				t.Errorf("unexpected route action %v for %s", entry.Action, entry.Prefix)
			}
		}
	})
	StoreSessionMetadata(id, &AuthMetadata{FramedRoutes: []FramedRoute{{Prefix: chain.v4Route}, {Prefix: chain.v6Route}}})

	chain.wire = newCurrentCoreAdmission(t, r, &currentCoreAdmissionConfig{
		ids: currentCoreSessionIDs{tunnelID: tid, sessionID: sid}, ipv6: true, subscriberID: "",
	})
	chain.wire.prepareNetwork(t, "family-user", ppp.IPResponseArgs{Accept: true, Family: ppp.AddressFamilyIPv4,
		Local: netip.MustParseAddr("192.0.2.1"), Peer: peer}, [8]byte{2, 0, 0, 0, 0, 0, 0, 46})
	r.handlePPPEvent(chain.wire.assign(t, ppp.AddressFamilyIPv4))
	r.handlePPPEvent(chain.wire.assign(t, ppp.AddressFamilyIPv6))
	r.handlePPPEvent(chain.wire.nextUp(t))
	require.Equal(t, chain.allRoutes(), chain.liveRoutes, "each family's install announces its own routes")
	return chain
}

func (chain *familyReleaseChain) allRoutes() map[netip.Prefix]bool {
	return map[netip.Prefix]bool{chain.host: true, chain.v4Route: true, chain.v6Route: true}
}

// release sends the peer's new Configure-Request for the family and returns
// the release the PPP session publishes for it. RFC 1661 Section 4.4 orders
// the Opened RCR+ actions tld, scr, sca, so the release precedes Ze's frames.
func (chain *familyReleaseChain) release(t *testing.T, family ppp.AddressFamily) ppp.EventSessionIPReleased {
	t.Helper()
	protocol, options := chain.peerConfigure(family)
	chain.added, chain.removed = nil, nil
	chain.wire.write(t, protocol, ppp.LCPConfigureRequest, 0x63, options)
	event := chain.wire.event(t)
	released, ok := event.(ppp.EventSessionIPReleased)
	require.True(t, ok, "the %s renegotiation published %T, want the family release", family, event)
	require.Equal(t, family, released.Family)
	chain.configure = chain.wire.read(t, protocol, ppp.LCPConfigureRequest)
	chain.wire.read(t, protocol, ppp.LCPConfigureAck)
	return released
}

// reopen acknowledges Ze's Configure-Request, which takes the family to
// Opened again, and returns the assignment the PPP session publishes.
func (chain *familyReleaseChain) reopen(t *testing.T, family ppp.AddressFamily) ppp.EventSessionIPAssigned {
	t.Helper()
	protocol, _ := chain.peerConfigure(family)
	chain.added, chain.removed = nil, nil
	chain.wire.write(t, protocol, ppp.LCPConfigureAck, chain.configure.Identifier, chain.configure.Data)
	event := chain.wire.event(t)
	assigned, ok := event.(ppp.EventSessionIPAssigned)
	require.True(t, ok, "the %s reopen published %T, want the assignment", family, event)
	require.Equal(t, family, assigned.Family)
	return assigned
}

// peerConfigure answers the protocol and the peer's Configure-Request options
// for the family, from the addresses the pool answered.
func (chain *familyReleaseChain) peerConfigure(family ppp.AddressFamily) (uint16, []byte) {
	answer := chain.wire.answers[family]
	if family == ppp.AddressFamilyIPv4 {
		address := answer.Peer.As4()
		return ppp.ProtoIPCP, append([]byte{ppp.IPCPOptIPAddress, 6}, address[:]...)
	}
	return ppp.ProtoIPv6CP, append([]byte{ppp.IPv6CPOptInterfaceID, 10}, answer.PeerInterfaceID[:]...)
}
