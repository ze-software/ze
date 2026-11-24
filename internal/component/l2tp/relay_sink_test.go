package l2tp

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/callsink"
)

// TestRelaySink_UnmatchedServiceTerminatesLocally -- AC-3.
//
// VALIDATES: a PPPoE service with no relay binding is not relayed (Relay
// returns accepted=false), so pppoe terminates PPP locally.
func TestRelaySink_UnmatchedServiceTerminatesLocally(t *testing.T) {
	s := &Subsystem{
		params: Parameters{
			Remotes: []Remote{{Name: "lns1", Address: netip.MustParseAddrPort("10.0.0.1:1701")}},
			Relays:  []RelayBinding{{Service: "wholesale", Remote: "lns1"}},
		},
		logger: slog.Default(),
	}
	_, accepted, err := (&relaySink{s: s}).Relay(callsink.Request{Service: "retail"})
	require.NoError(t, err)
	require.False(t, accepted)
}

// TestRelaySink_MatchedButNoListener -- AC-3 fallback.
//
// VALIDATES: a matched relay with no running L2TP listener returns an error
// so pppoe can fall back to local termination instead of stranding the
// subscriber.
func TestRelaySink_MatchedButNoListener(t *testing.T) {
	s := &Subsystem{
		params: Parameters{
			Remotes: []Remote{{Name: "lns1", Address: netip.MustParseAddrPort("10.0.0.1:1701")}},
			Relays:  []RelayBinding{{Service: "wholesale", Remote: "lns1"}},
		},
		logger: slog.Default(),
	}
	_, accepted, err := (&relaySink{s: s}).Relay(callsink.Request{Service: "wholesale"})
	require.ErrorIs(t, err, errRelayNoListener)
	require.False(t, accepted)
}

// TestRelaySink_MatchedDialsRemote -- AC-3 relay origination.
//
// VALIDATES: a subscriber on a relay-bound service triggers a dial toward the
// bound remote -- an SCCRQ leaves ze -- and the sink reports accepted so the
// PPPoE server skips local termination.
func TestRelaySink_MatchedDialsRemote(t *testing.T) {
	ln, r, _, stop := buildLogReactor(t)
	defer stop()
	client := newClient(t, ln)
	defer client.Close()

	remoteAddr := clientAddrPort(t, client)
	s := &Subsystem{
		params: Parameters{
			Remotes: []Remote{{Name: "lns1", Address: remoteAddr}},
			Relays:  []RelayBinding{{Service: "wholesale", Remote: "lns1"}},
		},
		reactors: []*l2tpReactor{r},
		logger:   slog.Default(),
	}

	_, accepted, err := (&relaySink{s: s}).Relay(callsink.Request{
		Service: "wholesale", SubscriberMAC: "aa:bb:cc:dd:ee:ff", Channel: relayDescriptorForTest(7), Handoff: &fakeRealmHandoff{},
	})
	require.NoError(t, err)
	require.True(t, accepted, "matched relay hands the subscriber to L2TP")

	sccrq := readDatagram(t, client)
	require.Equal(t, MsgSCCRQ, msgTypeOf(t, sccrq), "ze dialed the relay remote")
}

// TestRelayTargetLocked_Resolution -- AC-3 binding resolution.
//
// VALIDATES: relayTargetLocked returns the bound remote for a configured
// service and reports no match for an unknown one.
func TestRelayTargetLocked_Resolution(t *testing.T) {
	s := &Subsystem{params: Parameters{
		Remotes: []Remote{{Name: "lns1", Address: netip.MustParseAddrPort("10.0.0.1:1701")}},
		Relays:  []RelayBinding{{Service: "wholesale", Remote: "lns1"}, {Service: "", Remote: "lns1"}},
	}}
	rem, ok := s.relayTargetLocked("wholesale")
	require.True(t, ok)
	require.Equal(t, "lns1", rem.Name)

	// Empty service (default) is a valid binding key.
	_, ok = s.relayTargetLocked("")
	require.True(t, ok)

	_, ok = s.relayTargetLocked("nope")
	require.False(t, ok)
}

// TestRelaySink_SelectsReachingReactorForServiceAndRealm proves both Service-Name
// and realm relays pick the reactor whose listener reaches the bound remote
// rather than always dialing from s.reactors[0].
//
// VALIDATES: AC-7 on both relaySink.Relay and relaySink.RelayRealm: with IPv4
// listener 0 and IPv6 listener 1, relaying to an IPv6 remote dials from the
// IPv6 reactor; relaying to an unreachable off-host IPv4 remote refuses with
// ErrDialFromLoopback and returns accepted=false.
// PREVENTS: PPPoE relay calls failing when listener 0 cannot reach the remote.
func TestRelaySink_SelectsReachingReactorForServiceAndRealm(t *testing.T) {
	ctx := context.Background()
	v4Ln := newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), slog.Default())
	require.NoError(t, v4Ln.Start(ctx))
	t.Cleanup(func() { _ = v4Ln.Stop() })

	v6Ln := newUDPListener(netip.MustParseAddrPort("[::1]:0"), slog.Default())
	require.NoError(t, v6Ln.Start(ctx))
	t.Cleanup(func() { _ = v6Ln.Stop() })

	v4Reactor := newL2TPReactor(v4Ln, slog.Default(), reactorParams{
		ServerName: "v4",
		Defaults:   TunnelDefaults{HostName: "ze-v4", FramingCapabilities: 0x3, RecvWindow: 16},
	})
	require.NoError(t, v4Reactor.Start())
	t.Cleanup(v4Reactor.Stop)

	v6Reactor := newL2TPReactor(v6Ln, slog.Default(), reactorParams{
		ServerName: "v6",
		Defaults:   TunnelDefaults{HostName: "ze-v6", FramingCapabilities: 0x3, RecvWindow: 16},
	})
	require.NoError(t, v6Reactor.Start())
	t.Cleanup(v6Reactor.Stop)

	peerConn, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.IPv6loopback, Port: 0})
	require.NoError(t, err)
	t.Cleanup(func() { _ = peerConn.Close() })
	udpAddr, ok := peerConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("peerConn.LocalAddr() has type %T, want *net.UDPAddr", peerConn.LocalAddr())
	}
	peerAddr := udpAddr.AddrPort()

	s := &Subsystem{
		params: Parameters{
			Remotes: []Remote{
				{Name: "lns-v6", Address: peerAddr},
				{Name: "lns-offhost", Address: netip.MustParseAddrPort("192.0.2.1:1701")},
			},
			Relays: []RelayBinding{
				{Service: "v6-svc", Remote: "lns-v6"},
				{Service: "offhost-svc", Remote: "lns-offhost"},
			},
			RelayRealms: []RealmRelayBinding{
				{Realm: "v6.example", Remote: "lns-v6"},
				{Realm: "offhost.example", Remote: "lns-offhost"},
			},
		},
		reactors: []*l2tpReactor{v4Reactor, v6Reactor},
		logger:   slog.Default(),
	}
	sink := &relaySink{s: s}

	// Service-Name relay dials through v6Reactor (index 1).
	_, accepted, err := sink.Relay(callsink.Request{
		Service: "v6-svc", SubscriberMAC: "aa:bb:cc:dd:ee:01",
		Channel: relayDescriptorForTest(7), Handoff: &fakeRealmHandoff{},
	})
	require.NoError(t, err)
	require.True(t, accepted)

	buf := make([]byte, 2048)
	require.NoError(t, peerConn.SetReadDeadline(time.Now().Add(2*time.Second)))
	n, from, err := peerConn.ReadFromUDPAddrPort(buf)
	require.NoError(t, err)
	require.Equal(t, MsgSCCRQ, msgTypeOf(t, buf[:n]))
	require.Equal(t, v6Ln.Addr(), from, "Service-Name relay must dial from IPv6 reactor[1]")

	// Realm relay also picks v6Reactor (joins the connecting tunnel on v6Reactor).
	_, accepted, err = sink.RelayRealm(callsink.RealmRequest{
		Realm: "v6.example", Username: "cpe@v6.example", SubscriberMAC: "aa:bb:cc:dd:ee:02",
		Channel: relayDescriptorForTest(8), Handoff: &fakeRealmHandoff{},
		Auth: callsink.ProxyAuth{Kind: callsink.AuthKindCHAP, Identifier: 1, Challenge: []byte{1}, Response: []byte{2}},
	})
	require.NoError(t, err)
	require.True(t, accepted)

	v4Reactor.tunnelsMu.Lock()
	v4Tunnels := len(v4Reactor.tunnelsByLocalID)
	v4Reactor.tunnelsMu.Unlock()
	require.Zero(t, v4Tunnels, "IPv4 reactor[0] must remain unused for IPv6 relays")

	// Unreachable off-host IPv4 remote is refused on both Relay and RelayRealm.
	_, accepted, err = sink.Relay(callsink.Request{
		Service: "offhost-svc", SubscriberMAC: "aa:bb:cc:dd:ee:03",
		Channel: relayDescriptorForTest(9), Handoff: &fakeRealmHandoff{},
	})
	require.ErrorIs(t, err, ErrDialFromLoopback)
	require.False(t, accepted)

	_, accepted, err = sink.RelayRealm(callsink.RealmRequest{
		Realm: "offhost.example", Username: "cpe@offhost.example", SubscriberMAC: "aa:bb:cc:dd:ee:04",
		Channel: relayDescriptorForTest(10), Handoff: &fakeRealmHandoff{},
		Auth: callsink.ProxyAuth{Kind: callsink.AuthKindCHAP, Identifier: 1, Challenge: []byte{1}, Response: []byte{2}},
	})
	require.ErrorIs(t, err, ErrDialFromLoopback)
	require.False(t, accepted)
}
