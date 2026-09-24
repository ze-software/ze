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
			transport := addEstablishedSession(tun, sid, 511, true)
			transport.pppInterface = "ppp41"
			t.Cleanup(func() { cancelSessionTimeouts(transport) })
			t.Cleanup(func() { ClearSessionMetadata(tid, sid) })

			bus := newTestBus()
			reg := subscriber.NewRegistry()
			bridge := newSubscriberBridge(reg, bus, r.logger)
			t.Cleanup(bridge.stop)
			r.eventBus = bus
			r.routeObserver = newSubscriberRouteObserver(r.logger, bus)
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
					default:
						t.Errorf("unexpected route action %v for %s", entry.Action, entry.Prefix)
					}
				}
			})
			id := l2tpSessionID(tid, sid)
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
			StoreSessionMetadata(tid, sid, &AuthMetadata{
				FramedPool: "old-pool", FilterID: "rate:20mbit/5mbit",
				SessionTimeout: 3600, IdleTimeout: 3600,
				FramedRoutes: []FramedRoute{{Prefix: oldRoute}, {Prefix: oldV6Route}},
			})
			first := ppp.EventSessionIPAssigned{
				TunnelID: tid, SessionID: sid, Family: ppp.AddressFamilyIPv4, Username: "old-user",
				Peer:       netip.MustParseAddr("192.0.2.41"),
				DNSPrimary: netip.MustParseAddr("192.0.2.53"), DNSSecondary: netip.MustParseAddr("192.0.2.54"),
			}
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
			oldPrefixes := []netip.Prefix{netip.PrefixFrom(first.Peer, 32), oldRoute, oldV6Route}
			require.Len(t, liveRoutes, len(oldPrefixes))
			for _, prefix := range oldPrefixes {
				require.True(t, liveRoutes[prefix], "missing published route %s", prefix)
			}

			var timeoutCtx, idleCtx context.Context
			if activateFirst {
				r.handlePPPEvent(firstV6)
				r.handlePPPEvent(ppp.EventSessionUp{TunnelID: tid, SessionID: sid})
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
			r.handlePPPEvent(ppp.EventLCPDown{TunnelID: tid, SessionID: sid, NetworkPhase: true, Reason: "peer restarted LCP"})
			_, ok = reg.Get(id)
			require.False(t, ok, "LCPDown must withdraw the old subscriber")
			require.Empty(t, liveRoutes, "LCPDown must publish route withdrawals")
			require.ElementsMatch(t, oldPrefixes, removed)
			require.Nil(t, LoadSessionMetadata(tid, sid), "old authorization must not survive replacement authentication")
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
			StoreSessionMetadata(tid, sid, &AuthMetadata{FramedPool: "new-pool", FramedRoutes: []FramedRoute{{Prefix: newRoute}}})
			second := first
			second.Username = "new-user"
			second.Peer = netip.MustParseAddr("192.0.2.42")
			second.DNSPrimary = netip.MustParseAddr("198.51.100.53")
			second.DNSSecondary = netip.MustParseAddr("198.51.100.54")
			secondV6 := firstV6
			secondV6.Username = "new-user"
			secondV6.InterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 42}
			r.handlePPPEvent(secondV6)
			configured, ok = reg.Get(id)
			require.True(t, ok)
			require.Equal(t, subscriber.StateConfiguring, configured.State)
			require.Equal(t, secondV6.InterfaceID, configured.IPv6InterfaceID)
			r.handlePPPEvent(second)
			r.handlePPPEvent(ppp.EventSessionUp{TunnelID: tid, SessionID: sid})
			latest := up[len(up)-1]
			assertL2TPLifetimeAddresses(t, &latest, second, secondV6)
			require.Equal(t, "new-user", latest.Username)
			require.Equal(t, "new-pool", latest.PoolName)
			require.Equal(t, baseline+1, l2tpActiveSubscribers(t))
			require.Equal(t, map[netip.Prefix]bool{netip.PrefixFrom(second.Peer, 32): true, newRoute: true}, liveRoutes)

			r.handlePPPEvent(ppp.EventSessionDown{TunnelID: tid, SessionID: sid, Reason: "peer finished"})
			require.Nil(t, tun.lookupSession(sid))
			_, ok = reg.Get(id)
			require.False(t, ok)
			require.Empty(t, liveRoutes)
			require.Nil(t, LoadSessionMetadata(tid, sid))
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
			transport := addEstablishedSession(tun, sid, 521, true)
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
			assigned := ppp.EventSessionIPAssigned{
				TunnelID: tid, SessionID: sid, Family: ppp.AddressFamilyIPv4,
				Username: tc.principal, Peer: netip.MustParseAddr("192.0.2.42"),
			}
			r.handlePPPEvent(assigned)
			sess, ok := reg.Get(l2tpSessionID(tid, sid))
			require.True(t, ok)
			require.Equal(t, tc.want, sess.Username)

			r.handlePPPEvent(ppp.EventLCPDown{TunnelID: tid, SessionID: sid, NetworkPhase: true})
			assigned.Username = ""
			r.handlePPPEvent(assigned)
			sess, ok = reg.Get(l2tpSessionID(tid, sid))
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
	addEstablishedSession(tun, 421, 521, true)
	publications := 0
	subevents.SessionIPAssigned.Subscribe(bus, func(*subevents.SessionIPAssignedPayload) { publications++ })
	for _, event := range []ppp.EventSessionIPAssigned{
		{TunnelID: 420, SessionID: 421, Family: ppp.AddressFamilyIPv4},
		{TunnelID: 420, SessionID: 421, Family: ppp.AddressFamilyIPv6},
		{TunnelID: 420, SessionID: 421, Family: ppp.AddressFamilyIPv4, Peer: netip.MustParseAddr("2001:db8::99")},
		{TunnelID: 420, SessionID: 999, Family: ppp.AddressFamilyIPv4, Peer: netip.MustParseAddr("192.0.2.99")},
		{TunnelID: 999, SessionID: 421, Family: ppp.AddressFamilyIPv4, Peer: netip.MustParseAddr("192.0.2.99")},
	} {
		r.handlePPPEvent(event)
		_, ok := reg.Get(l2tpSessionID(event.TunnelID, event.SessionID))
		require.False(t, ok)
	}
	// The bridge is also a bus consumer: malformed strings must not turn
	// into a registry entry carrying the zero address as a real assignment.
	_, err := l2tpevents.SessionIPAssigned.Emit(bus, &l2tpevents.SessionIPAssignedPayload{TunnelID: 420, SessionID: 421, PeerAddr: "not-an-address"})
	require.NoError(t, err)
	_, ok := reg.Get(l2tpSessionID(420, 421))
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
	transport := addEstablishedSession(tun, sid, 551, true)
	bus := newTestBus()
	reg := subscriber.NewRegistry()
	bridge := newSubscriberBridge(reg, bus, r.logger)
	t.Cleanup(bridge.stop)
	r.eventBus = bus
	observer := newSubscriberRouteObserver(r.logger, bus)
	r.routeObserver = observer
	addr := netip.MustParseAddr("192.0.2.45")
	reg.Add(&subscriber.Session{
		ID: l2tpSessionID(tid, sid), AccessType: subscriber.AccessL2TP,
		TunnelID: tid, SessionID: sid, State: subscriber.StateConfiguring, IPv4Addr: addr,
	})
	StoreSessionMetadata(tid, sid, &AuthMetadata{FramedPool: "old-pool"})
	t.Cleanup(func() { ClearSessionMetadata(tid, sid) })
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
	observer.OnSessionIPUp(tid, sid, "old-user", addr)
	var down []subscriber.Session
	subevents.SessionDown.Subscribe(bus, func(p *subevents.SessionDownPayload) { down = append(down, p.Session) })

	r.handlePPPEvent(ppp.EventLCPDown{TunnelID: tid, SessionID: sid, NetworkPhase: true})
	require.Equal(t, []netip.Prefix{netip.PrefixFrom(addr, 32)}, removed, "LCPDown must publish withdrawal of the existing route")
	_, ok := reg.Get(l2tpSessionID(tid, sid))
	require.False(t, ok)
	require.Len(t, down, 1)
	require.Equal(t, addr, down[0].IPv4Addr)
	require.Nil(t, LoadSessionMetadata(tid, sid))
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.Same(t, transport, tun.lookupSession(sid))
	assertNoL2TPLifetimePacket(t, peer)
}
