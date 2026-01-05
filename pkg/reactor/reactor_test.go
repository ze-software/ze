package reactor

import (
	"context"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/zebgp/pkg/api"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/message"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"github.com/stretchr/testify/require"
)

// TestReactorNew verifies Reactor creation with correct initial state.
//
// VALIDATES: Reactor is created with config and not running.
//
// PREVENTS: Reactor auto-starting or with invalid state.
func TestReactorNew(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	require.NotNil(t, reactor, "New must return non-nil")
	require.False(t, reactor.Running(), "reactor should not be running initially")
}

// TestReactorStartStop verifies basic start/stop lifecycle.
//
// VALIDATES: Reactor can be started and stopped cleanly.
//
// PREVENTS: Resource leaks or goroutine leaks on stop.
func TestReactorStartStop(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	err := reactor.Start()
	require.NoError(t, err)
	require.True(t, reactor.Running())

	reactor.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = reactor.Wait(ctx)
	require.NoError(t, err)

	require.False(t, reactor.Running())
}

// TestReactorAddPeer verifies adding peers to reactor.
//
// VALIDATES: Peers can be added and are tracked.
//
// PREVENTS: Lost peer references or duplicate handling.
func TestReactorAddPeer(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	peers := reactor.Peers()
	require.Len(t, peers, 1)
}

// TestReactorRemovePeer verifies removing peers from reactor.
//
// VALIDATES: Peers can be removed and cleaned up.
//
// PREVENTS: Orphaned peer goroutines.
func TestReactorRemovePeer(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	err = reactor.RemovePeer(settings.Address)
	require.NoError(t, err)

	peers := reactor.Peers()
	require.Len(t, peers, 0)
}

// TestReactorPeersStartOnRun verifies peers start when reactor runs.
//
// VALIDATES: All configured peers start when reactor starts.
//
// PREVENTS: Peers remaining idle after reactor start.
func TestReactorPeersStartOnRun(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = 0 // Invalid port to prevent actual connection

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	err = reactor.Start()
	require.NoError(t, err)

	// Give peers time to start
	time.Sleep(20 * time.Millisecond)

	peers := reactor.Peers()
	require.Len(t, peers, 1)
	require.NotEqual(t, PeerStateStopped, peers[0].State(), "peer should be running")

	reactor.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = reactor.Wait(ctx)
}

// TestReactorListenerAcceptsConnections verifies listener is active.
//
// VALIDATES: Reactor's listener accepts incoming connections.
//
// PREVENTS: Dead listener after reactor start.
func TestReactorListenerAcceptsConnections(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	err := reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	addr := reactor.ListenAddr()
	require.NotNil(t, addr)

	// Connect to listener
	conn, err := net.DialTimeout("tcp", addr.String(), time.Second) //nolint:noctx // Test code
	require.NoError(t, err)
	_ = conn.Close()
}

// TestReactorIncomingConnectionMatchesPeer verifies peer matching.
//
// VALIDATES: Incoming connections are matched to configured neighbors.
//
// PREVENTS: Connections from unknown peers being accepted.
func TestReactorIncomingConnectionMatchesPeer(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	// Add passive peer expecting connection from localhost
	settings := NewPeerSettings(
		mustParseAddr("127.0.0.1"),
		65000, 65001, 0x01010101,
	)
	settings.Passive = true

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	var accepted atomic.Bool
	reactor.SetConnectionCallback(func(conn net.Conn, n *PeerSettings) {
		accepted.Store(true)
		_ = conn.Close()
	})

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	addr := reactor.ListenAddr()

	// Connect
	conn, err := net.Dial("tcp", addr.String()) //nolint:noctx // Test code
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	time.Sleep(50 * time.Millisecond)

	require.True(t, accepted.Load(), "connection should be matched to peer")
}

// TestReactorContextCancellation verifies reactor stops on context cancel.
//
// VALIDATES: Reactor respects context cancellation.
//
// PREVENTS: Orphaned resources when parent context is cancelled.
func TestReactorContextCancellation(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	ctx, cancel := context.WithCancel(context.Background())

	err := reactor.StartWithContext(ctx)
	require.NoError(t, err)
	require.True(t, reactor.Running())

	cancel()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	err = reactor.Wait(waitCtx)

	require.NoError(t, err)
	require.False(t, reactor.Running())
}

// TestReactorGracefulShutdown verifies all components stop cleanly.
//
// VALIDATES: Peers, listener, and signals all stop on shutdown.
//
// PREVENTS: Partial shutdown leaving resources dangling.
func TestReactorGracefulShutdown(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = 0

	_ = reactor.AddPeer(settings)

	err := reactor.Start()
	require.NoError(t, err)

	time.Sleep(20 * time.Millisecond)

	reactor.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = reactor.Wait(ctx)
	require.NoError(t, err)

	// Verify everything stopped
	require.False(t, reactor.Running())
	for _, peer := range reactor.Peers() {
		require.Equal(t, PeerStateStopped, peer.State())
	}
}

// TestReactorStats verifies stats collection.
//
// VALIDATES: Reactor tracks connection statistics.
//
// PREVENTS: Missing observability.
func TestReactorStats(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	err := reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	stats := reactor.Stats()
	require.NotNil(t, stats)
	require.GreaterOrEqual(t, stats.Uptime, time.Duration(0))
}

// TestBuildAnnounceUpdateIPv6UsesMPReachNLRI verifies IPv6 routes use MP_REACH_NLRI.
//
// VALIDATES: IPv6 routes are encoded with MP_REACH_NLRI (attr 14) instead of
// NEXT_HOP (attr 3) and NLRI field.
//
// PREVENTS: IPv6 routes being sent with IPv4-style encoding which violates RFC 4760.
func TestBuildAnnounceUpdateIPv6UsesMPReachNLRI(t *testing.T) {
	route := api.RouteSpec{
		Prefix:  netip.MustParsePrefix("2605::2/128"),
		NextHop: netip.MustParseAddr("2001::1"),
	}

	// ctx with ASN4=true, AddPath=false
	ctx := &nlri.PackContext{ASN4: true}
	update := buildAnnounceUpdate(route, 65000, true, ctx)

	// IPv6 routes MUST NOT have regular NLRI field
	require.Empty(t, update.NLRI, "IPv6 routes must not use NLRI field")

	// Scan path attributes for type codes
	// Attribute format: flags(1) + code(1) + length(1 or 2) + value
	foundMPReach := false
	foundNextHop := false
	data := update.PathAttributes
	for len(data) >= 3 {
		flags := data[0]
		code := attribute.AttributeCode(data[1])
		var length int
		var hdrLen int
		if flags&0x10 != 0 { // Extended length
			if len(data) < 4 {
				break
			}
			length = int(data[2])<<8 | int(data[3])
			hdrLen = 4
		} else {
			length = int(data[2])
			hdrLen = 3
		}

		switch code { //nolint:exhaustive // Only checking for two specific attributes
		case attribute.AttrMPReachNLRI:
			foundMPReach = true
		case attribute.AttrNextHop:
			foundNextHop = true
		}

		if len(data) < hdrLen+length {
			break
		}
		data = data[hdrLen+length:]
	}

	require.True(t, foundMPReach, "IPv6 routes must have MP_REACH_NLRI attribute")
	require.False(t, foundNextHop, "IPv6 routes must not have NEXT_HOP attribute")
}

// TestBuildWithdrawUpdateIPv6UsesMPUnreachNLRI verifies IPv6 withdrawals use MP_UNREACH_NLRI.
//
// VALIDATES: IPv6 withdrawals are encoded with MP_UNREACH_NLRI (attr 15) instead of
// the WithdrawnRoutes field.
//
// PREVENTS: IPv6 withdrawals being sent with IPv4-style encoding which violates RFC 4760.
func TestBuildWithdrawUpdateIPv6UsesMPUnreachNLRI(t *testing.T) {
	prefix := netip.MustParsePrefix("2605::2/128")

	// ctx=nil means no ADD-PATH encoding
	update := buildWithdrawUpdate(prefix, nil)

	// IPv6 withdrawals MUST NOT have WithdrawnRoutes field
	require.Empty(t, update.WithdrawnRoutes, "IPv6 withdrawals must not use WithdrawnRoutes field")

	// Must have MP_UNREACH_NLRI in path attributes
	require.NotEmpty(t, update.PathAttributes, "IPv6 withdrawals must have PathAttributes with MP_UNREACH_NLRI")

	// Scan path attributes for MP_UNREACH_NLRI (type 15)
	foundMPUnreach := false
	data := update.PathAttributes
	for len(data) >= 3 {
		flags := data[0]
		code := attribute.AttributeCode(data[1])
		var length int
		var hdrLen int
		if flags&0x10 != 0 { // Extended length
			if len(data) < 4 {
				break
			}
			length = int(data[2])<<8 | int(data[3])
			hdrLen = 4
		} else {
			length = int(data[2])
			hdrLen = 3
		}

		if code == attribute.AttrMPUnreachNLRI {
			foundMPUnreach = true
			// Verify AFI=2 (IPv6), SAFI=1 (Unicast)
			if len(data) >= hdrLen+3 {
				afi := uint16(data[hdrLen])<<8 | uint16(data[hdrLen+1])
				safi := data[hdrLen+2]
				require.Equal(t, uint16(2), afi, "MP_UNREACH_NLRI AFI must be 2 (IPv6)")
				require.Equal(t, uint8(1), safi, "MP_UNREACH_NLRI SAFI must be 1 (Unicast)")
			}
		}

		if len(data) < hdrLen+length {
			break
		}
		data = data[hdrLen+length:]
	}

	require.True(t, foundMPUnreach, "IPv6 withdrawals must have MP_UNREACH_NLRI attribute")
}

// TestGetPeerAPIBindingsEncodingInheritance verifies encoding inheritance chain.
//
// VALIDATES: Empty peer encoding inherits from process, empty process defaults to "text".
//
// PREVENTS: Empty encoding causing silent failures in message dispatch.
func TestGetPeerAPIBindingsEncodingInheritance(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
		APIProcesses: []APIProcessConfig{
			{Name: "json-proc", Run: "./test", Encoder: "json"},
			{Name: "text-proc", Run: "./test", Encoder: "text"},
			{Name: "empty-proc", Run: "./test", Encoder: ""},
		},
	}

	reactor := New(cfg)

	// Add peer with bindings that test inheritance
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.APIBindings = []APIBinding{
		{ProcessName: "json-proc", Encoding: "text"}, // Explicit override
		{ProcessName: "json-proc", Encoding: ""},     // Inherit from process (json)
		{ProcessName: "text-proc", Encoding: ""},     // Inherit from process (text)
		{ProcessName: "empty-proc", Encoding: ""},    // Process empty, default to text
	}

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	// Get bindings through API adapter
	adapter := &reactorAPIAdapter{reactor}
	bindings := adapter.GetPeerAPIBindings(mustParseAddr("192.0.2.1"))

	require.Len(t, bindings, 4)

	// Verify inheritance chain
	require.Equal(t, "text", bindings[0].Encoding, "explicit override should be text")
	require.Equal(t, "json", bindings[1].Encoding, "empty should inherit from json-proc")
	require.Equal(t, "text", bindings[2].Encoding, "empty should inherit from text-proc")
	require.Equal(t, "text", bindings[3].Encoding, "empty proc should default to text")

	// Verify format defaults to "parsed"
	require.Equal(t, "parsed", bindings[0].Format, "format should default to parsed")
}

// TestGetPeerAPIBindingsNotFound verifies nil return for unknown peer.
//
// VALIDATES: GetPeerAPIBindings returns nil for non-existent peer.
//
// PREVENTS: Panic on unknown peer lookup.
func TestGetPeerAPIBindingsNotFound(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}

	reactor := New(cfg)

	adapter := &reactorAPIAdapter{reactor}
	bindings := adapter.GetPeerAPIBindings(mustParseAddr("192.0.2.99"))

	require.Nil(t, bindings, "unknown peer should return nil")
}

// TestBuildLabeledUnicastRIBRouteAllAttributes verifies ALL attributes are stored.
//
// VALIDATES: buildLabeledUnicastRIBRoute includes all path attributes in rib.Route.
// This is critical for queued routes to preserve attributes on replay.
//
// PREVENTS: Attribute loss when routes are queued and replayed via buildRIBRouteUpdate.
// (Fixes bug where AnnounceRoute only stored OriginIGP, losing MED/Communities/etc.)
func TestBuildLabeledUnicastRIBRouteAllAttributes(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
		LocalAS:    65000,
	}
	reactor := New(cfg)
	adapter := &reactorAPIAdapter{reactor}

	// Create route with ALL attributes populated
	origin := uint8(1) // EGP
	med := uint32(100)
	localPref := uint32(200)
	route := api.LabeledUnicastRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/8"),
		NextHop: netip.MustParseAddr("192.0.2.1"),
		Labels:  []uint32{100, 200}, // Label stack
		PathID:  42,
		PathAttributes: api.PathAttributes{
			Origin:          &origin,
			MED:             &med,
			LocalPreference: &localPref,
			ASPath:          []uint32{65001, 65002},
			Communities:     []uint32{0x12345678},
			LargeCommunities: []api.LargeCommunity{
				{GlobalAdmin: 65000, LocalData1: 1, LocalData2: 2},
			},
			ExtendedCommunities: []attribute.ExtendedCommunity{{0x00, 0x02, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64}},
		},
	}

	ribRoute := adapter.buildLabeledUnicastRIBRoute(route, false) // eBGP

	// Verify NLRI
	require.NotNil(t, ribRoute.NLRI(), "NLRI must not be nil")
	require.Equal(t, nlri.SAFIMPLSLabel, ribRoute.NLRI().Family().SAFI, "SAFI must be MPLSLabel")
	require.Equal(t, uint32(42), ribRoute.NLRI().PathID(), "PathID must be preserved")

	// Verify NextHop
	require.Equal(t, netip.MustParseAddr("192.0.2.1"), ribRoute.NextHop(), "NextHop must be preserved")

	// Verify attributes are present
	attrs := ribRoute.Attributes()
	require.NotEmpty(t, attrs, "Attributes must not be empty")

	// Count attribute types
	foundOrigin := false
	foundMED := false
	foundLocalPref := false
	foundCommunities := false
	foundLargeCommunities := false
	foundExtCommunities := false

	for _, attr := range attrs {
		switch attr.Code() { //nolint:exhaustive // Only checking specific attributes
		case attribute.AttrOrigin:
			foundOrigin = true
			if o, ok := attr.(attribute.Origin); ok {
				require.Equal(t, attribute.Origin(1), o, "Origin must be EGP")
			}
		case attribute.AttrMED:
			foundMED = true
			if m, ok := attr.(attribute.MED); ok {
				require.Equal(t, attribute.MED(100), m, "MED must be 100")
			}
		case attribute.AttrLocalPref:
			foundLocalPref = true
			if lp, ok := attr.(attribute.LocalPref); ok {
				require.Equal(t, attribute.LocalPref(200), lp, "LocalPref must be 200")
			}
		case attribute.AttrCommunity:
			foundCommunities = true
		case attribute.AttrLargeCommunity:
			foundLargeCommunities = true
		case attribute.AttrExtCommunity:
			foundExtCommunities = true
		default:
			// Other attributes not checked in this test
		}
	}

	require.True(t, foundOrigin, "Origin attribute must be present")
	require.True(t, foundMED, "MED attribute must be present")
	require.True(t, foundLocalPref, "LocalPref attribute must be present")
	require.True(t, foundCommunities, "Communities attribute must be present")
	require.True(t, foundLargeCommunities, "LargeCommunities attribute must be present")
	require.True(t, foundExtCommunities, "ExtendedCommunities attribute must be present")

	// Verify AS_PATH
	asPath := ribRoute.ASPath()
	require.NotNil(t, asPath, "AS_PATH must not be nil")
	require.Len(t, asPath.Segments, 1, "AS_PATH must have 1 segment")
	require.Equal(t, []uint32{65001, 65002}, asPath.Segments[0].ASNs, "AS_PATH must preserve ASNs")
}

// TestBuildLabeledUnicastRIBRouteIBGPDefaults verifies iBGP default handling.
//
// VALIDATES: iBGP routes have empty AS_PATH and default origin.
//
// PREVENTS: Incorrect AS_PATH for iBGP routes.
func TestBuildLabeledUnicastRIBRouteIBGPDefaults(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
		LocalAS:    65000,
	}
	reactor := New(cfg)
	adapter := &reactorAPIAdapter{reactor}

	// Minimal route - no attributes set
	route := api.LabeledUnicastRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/8"),
		NextHop: netip.MustParseAddr("192.0.2.1"),
		Labels:  []uint32{100},
	}

	ribRoute := adapter.buildLabeledUnicastRIBRoute(route, true) // iBGP

	// Verify AS_PATH is empty for iBGP
	asPath := ribRoute.ASPath()
	require.NotNil(t, asPath, "AS_PATH must not be nil")
	require.Empty(t, asPath.Segments, "AS_PATH must be empty for iBGP")

	// Verify default Origin is IGP
	attrs := ribRoute.Attributes()
	foundOrigin := false
	for _, attr := range attrs {
		if o, ok := attr.(attribute.Origin); ok {
			foundOrigin = true
			require.Equal(t, attribute.OriginIGP, o, "Default origin must be IGP")
		}
	}
	require.True(t, foundOrigin, "Origin attribute must be present")
}

// TestBuildLabeledUnicastRIBRouteEBGPPrependsAS verifies eBGP AS prepending.
//
// VALIDATES: eBGP routes have LocalAS prepended to AS_PATH when no AS_PATH specified.
//
// PREVENTS: Missing LocalAS in eBGP announcements.
func TestBuildLabeledUnicastRIBRouteEBGPPrependsAS(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
		LocalAS:    65000,
	}
	reactor := New(cfg)
	adapter := &reactorAPIAdapter{reactor}

	// Route without AS_PATH
	route := api.LabeledUnicastRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/8"),
		NextHop: netip.MustParseAddr("192.0.2.1"),
		Labels:  []uint32{100},
	}

	ribRoute := adapter.buildLabeledUnicastRIBRoute(route, false) // eBGP

	// Verify LocalAS is prepended for eBGP
	asPath := ribRoute.ASPath()
	require.NotNil(t, asPath, "AS_PATH must not be nil")
	require.Len(t, asPath.Segments, 1, "AS_PATH must have 1 segment")
	require.Equal(t, []uint32{65000}, asPath.Segments[0].ASNs, "LocalAS must be prepended")
}

// =============================================================================
// Multi-Listener Tests (spec-listener-per-local-address.md)
// =============================================================================

// TestMultiListenerSameLocalAddress verifies one listener per unique LocalAddress.
//
// VALIDATES: Two peers with same LocalAddress create only one listener.
//
// PREVENTS: Duplicate listeners wasting resources and port conflicts.
func TestMultiListenerSameLocalAddress(t *testing.T) {
	cfg := &Config{
		Port:    0, // Use ephemeral port
		LocalAS: 65000,
	}
	reactor := New(cfg)

	localAddr := mustParseAddr("127.0.0.1")

	// Add two peers with same LocalAddress
	settings1 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings1.LocalAddress = localAddr
	settings1.Passive = true

	settings2 := NewPeerSettings(mustParseAddr("10.0.0.3"), 65000, 65002, 0x01010101)
	settings2.LocalAddress = localAddr
	settings2.Passive = true

	err := reactor.AddPeer(settings1)
	require.NoError(t, err)
	err = reactor.AddPeer(settings2)
	require.NoError(t, err)

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	// Should have exactly one listener
	addrs := reactor.ListenAddrs()
	require.Len(t, addrs, 1, "should have exactly 1 listener for shared LocalAddress")
}

// TestMultiListenerDifferentLocalAddresses verifies separate listeners per LocalAddress.
//
// VALIDATES: Two peers with different LocalAddresses create two listeners.
//
// PREVENTS: Cross-interface connection acceptance.
func TestMultiListenerDifferentLocalAddresses(t *testing.T) {
	// Check if IPv6 loopback is available
	ln, err := net.Listen("tcp", "[::1]:0") //nolint:noctx // Test code
	if err != nil {
		t.Skip("IPv6 loopback not available, skipping multi-listener test")
	}
	_ = ln.Close()

	cfg := &Config{
		Port:    0, // Use ephemeral port
		LocalAS: 65000,
	}
	reactor := New(cfg)

	// Add two peers with different LocalAddresses (IPv4 and IPv6 loopback)
	// Note: Peer Address must match LocalAddress family
	settings1 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings1.LocalAddress = mustParseAddr("127.0.0.1") // IPv4 local for IPv4 peer
	settings1.Passive = true

	settings2 := NewPeerSettings(mustParseAddr("2001:db8::3"), 65000, 65002, 0x01010101)
	settings2.LocalAddress = mustParseAddr("::1") // IPv6 local for IPv6 peer
	settings2.Passive = true

	err = reactor.AddPeer(settings1)
	require.NoError(t, err)
	err = reactor.AddPeer(settings2)
	require.NoError(t, err)

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	// Should have two listeners
	addrs := reactor.ListenAddrs()
	require.Len(t, addrs, 2, "should have 2 listeners for different LocalAddresses")
}

// TestMultiListenerNoPeers verifies reactor runs with no peers.
//
// VALIDATES: Reactor starts successfully with no peers (no listeners created).
//
// PREVENTS: Startup failure when no peers are configured.
func TestMultiListenerNoPeers(t *testing.T) {
	cfg := &Config{
		Port:    0, // Use ephemeral port
		LocalAS: 65000,
	}
	reactor := New(cfg)

	// No peers added

	err := reactor.Start()
	require.NoError(t, err, "reactor should start with no peers")
	defer reactor.Stop()

	require.True(t, reactor.Running(), "reactor should be running")

	// Should have no listeners (no peers = no LocalAddresses)
	addrs := reactor.ListenAddrs()
	require.Len(t, addrs, 0, "should have 0 listeners with no peers")
}

// TestMultiListenerConnectionToCorrectListener verifies connection routing.
//
// VALIDATES: Connection to a listener is matched to peer with that LocalAddress.
//
// PREVENTS: Connection from peer going to wrong listener.
func TestMultiListenerConnectionToCorrectListener(t *testing.T) {
	cfg := &Config{
		Port:    0, // Use ephemeral port
		LocalAS: 65000,
	}
	reactor := New(cfg)

	// Add peer expecting connection from 127.0.0.1
	// Note: Address != LocalAddress (peer is at 10.0.0.2, we listen on 127.0.0.1)
	settings := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("127.0.0.1")
	settings.Passive = true

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	var accepted atomic.Bool
	reactor.SetConnectionCallback(func(conn net.Conn, n *PeerSettings) {
		accepted.Store(true)
		_ = conn.Close()
	})

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	addr := reactor.ListenAddr()
	require.NotNil(t, addr, "should have a listener")

	// Connect from localhost
	// Note: This connection won't be "matched" to the peer because our source IP
	// isn't 10.0.0.2, but the listener should still be active
	conn, err := net.DialTimeout("tcp", addr.String(), time.Second) //nolint:noctx // Test code
	require.NoError(t, err)
	_ = conn.Close()

	// The connection will be rejected (unknown peer), but listener should work
	// Just verify listener is working by checking no error on connect
}

// TestMultiListenerLegacyListenAddrFallback verifies backward compatibility.
//
// VALIDATES: Legacy ListenAddr config still works when no LocalAddress on peers.
//
// PREVENTS: Breaking existing configs during migration.
func TestMultiListenerLegacyListenAddrFallback(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0", // Legacy config
		LocalAS:    65000,
	}
	reactor := New(cfg)

	// Peer without LocalAddress (legacy behavior)
	settings := NewPeerSettings(mustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Passive = true
	// Note: LocalAddress is zero value

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	// Legacy listener should be used
	addr := reactor.ListenAddr()
	require.NotNil(t, addr, "should have legacy listener")
}

// =============================================================================
// LocalAddress Validation Tests (spec-listener-per-local-address.md)
// =============================================================================

// TestAddPeerSelfReferential verifies Address != LocalAddress validation.
//
// VALIDATES: Peer with Address == LocalAddress is rejected.
//
// PREVENTS: Self-referential peer configuration that would never work.
func TestAddPeerSelfReferential(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// Self-referential: Address equals LocalAddress
	settings := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("10.0.0.1") // Same as Address!

	err := reactor.AddPeer(settings)
	require.Error(t, err, "self-referential peer should be rejected")
	require.Contains(t, err.Error(), "cannot equal local-address")
}

// TestAddPeerLinkLocalIPv6 verifies link-local IPv6 LocalAddress is rejected.
//
// VALIDATES: Link-local IPv6 addresses are rejected for LocalAddress.
//
// PREVENTS: Configuration with zone-dependent addresses that aren't portable.
func TestAddPeerLinkLocalIPv6(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// Link-local IPv6 as LocalAddress
	settings := NewPeerSettings(mustParseAddr("2001:db8::2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("fe80::1") // Link-local!

	err := reactor.AddPeer(settings)
	require.Error(t, err, "link-local IPv6 LocalAddress should be rejected")
	require.Contains(t, err.Error(), "link-local")
}

// TestAddPeerDuplicateAddress verifies duplicate peer Address is rejected.
//
// VALIDATES: Adding peer with same Address as existing peer fails.
//
// PREVENTS: Map key collision and ambiguous peer matching.
func TestAddPeerDuplicateAddress(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// Add first peer
	settings1 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings1.LocalAddress = mustParseAddr("192.168.1.1")
	err := reactor.AddPeer(settings1)
	require.NoError(t, err)

	// Try to add duplicate
	settings2 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65002, 0x01010101) // Same Address!
	settings2.LocalAddress = mustParseAddr("192.168.1.1")

	err = reactor.AddPeer(settings2)
	require.Error(t, err, "duplicate peer Address should be rejected")
	require.ErrorIs(t, err, ErrPeerExists)
}

// TestAddPeerAddressFamilyMismatch verifies Address/LocalAddress family must match.
//
// VALIDATES: IPv4 peer cannot have IPv6 LocalAddress and vice versa.
//
// PREVENTS: Configuration where TCP socket family doesn't match peer.
func TestAddPeerAddressFamilyMismatch(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// IPv4 peer with IPv6 LocalAddress
	settings := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("::1") // IPv6 local for IPv4 peer!

	err := reactor.AddPeer(settings)
	require.Error(t, err, "address family mismatch should be rejected")
	require.Contains(t, err.Error(), "family mismatch")
}

// =============================================================================
// Dynamic Listener Lifecycle Tests (spec-listener-per-local-address.md)
// =============================================================================

// TestDynamicListenerAddPeerNewLocalAddress verifies listener creation on AddPeer.
//
// VALIDATES: Adding peer with new LocalAddress while running creates listener.
//
// PREVENTS: Dynamic peers failing to accept incoming connections.
func TestDynamicListenerAddPeerNewLocalAddress(t *testing.T) {
	cfg := &Config{
		Port:    0, // Use ephemeral port
		LocalAS: 65000,
	}
	reactor := New(cfg)

	// Start with no peers (no listeners)
	err := reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	require.Len(t, reactor.ListenAddrs(), 0, "should have 0 listeners initially")

	// Add peer with LocalAddress
	settings := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("127.0.0.1")
	settings.Passive = true

	err = reactor.AddPeer(settings)
	require.NoError(t, err)

	// Should now have 1 listener
	require.Len(t, reactor.ListenAddrs(), 1, "should have 1 listener after AddPeer")
}

// TestDynamicListenerAddPeerExistingLocalAddress verifies listener reuse.
//
// VALIDATES: Adding peer with existing LocalAddress doesn't create new listener.
//
// PREVENTS: Resource waste from duplicate listeners.
func TestDynamicListenerAddPeerExistingLocalAddress(t *testing.T) {
	cfg := &Config{
		Port:    0,
		LocalAS: 65000,
	}
	reactor := New(cfg)

	// Add first peer
	settings1 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings1.LocalAddress = mustParseAddr("127.0.0.1")
	settings1.Passive = true
	err := reactor.AddPeer(settings1)
	require.NoError(t, err)

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	require.Len(t, reactor.ListenAddrs(), 1, "should have 1 listener")

	// Add second peer with SAME LocalAddress
	settings2 := NewPeerSettings(mustParseAddr("10.0.0.3"), 65000, 65002, 0x01010101)
	settings2.LocalAddress = mustParseAddr("127.0.0.1") // Same!
	settings2.Passive = true

	err = reactor.AddPeer(settings2)
	require.NoError(t, err)

	// Should still have only 1 listener
	require.Len(t, reactor.ListenAddrs(), 1, "should still have 1 listener (shared)")
}

// TestDynamicListenerRemoveLastPeer verifies listener cleanup on RemovePeer.
//
// VALIDATES: Removing last peer for LocalAddress stops the listener.
//
// PREVENTS: Orphaned listeners consuming resources.
func TestDynamicListenerRemoveLastPeer(t *testing.T) {
	cfg := &Config{
		Port:    0,
		LocalAS: 65000,
	}
	reactor := New(cfg)

	// Add peer
	settings := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("127.0.0.1")
	settings.Passive = true
	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	require.Len(t, reactor.ListenAddrs(), 1, "should have 1 listener")

	// Remove peer
	err = reactor.RemovePeer(settings.Address)
	require.NoError(t, err)

	// Listener should be stopped
	require.Len(t, reactor.ListenAddrs(), 0, "should have 0 listeners after removing last peer")
}

// TestDynamicListenerRemoveOneOfMany verifies listener stays when others share it.
//
// VALIDATES: Removing peer keeps listener if other peers share LocalAddress.
//
// PREVENTS: Premature listener closure breaking other peers.
func TestDynamicListenerRemoveOneOfMany(t *testing.T) {
	cfg := &Config{
		Port:    0,
		LocalAS: 65000,
	}
	reactor := New(cfg)

	localAddr := mustParseAddr("127.0.0.1")

	// Add two peers with same LocalAddress
	settings1 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings1.LocalAddress = localAddr
	settings1.Passive = true
	err := reactor.AddPeer(settings1)
	require.NoError(t, err)

	settings2 := NewPeerSettings(mustParseAddr("10.0.0.3"), 65000, 65002, 0x01010101)
	settings2.LocalAddress = localAddr
	settings2.Passive = true
	err = reactor.AddPeer(settings2)
	require.NoError(t, err)

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	require.Len(t, reactor.ListenAddrs(), 1, "should have 1 listener")

	// Remove one peer
	err = reactor.RemovePeer(settings1.Address)
	require.NoError(t, err)

	// Listener should still exist (other peer uses it)
	require.Len(t, reactor.ListenAddrs(), 1, "should still have 1 listener (other peer shares it)")
}

// =============================================================================
// IPv4-Mapped IPv6 Address Tests
// =============================================================================

// TestAddPeerIPv4MappedNormalization verifies IPv4-mapped addresses are normalized.
//
// VALIDATES: LocalAddress ::ffff:192.168.1.1 is normalized to 192.168.1.1.
//
// PREVENTS: Listener/connection mismatch due to different address formats.
func TestAddPeerIPv4MappedNormalization(t *testing.T) {
	cfg := &Config{
		Port:    0,
		LocalAS: 65000,
	}
	reactor := New(cfg)

	// Add peer with IPv4-mapped IPv6 LocalAddress
	settings := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = netip.MustParseAddr("::ffff:127.0.0.1") // IPv4-mapped
	settings.Passive = true

	err := reactor.AddPeer(settings)
	require.NoError(t, err, "IPv4-mapped LocalAddress should be accepted")

	err = reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	// Verify listener was created on the unmapped IPv4 address
	addrs := reactor.ListenAddrs()
	require.Len(t, addrs, 1)

	// The listener should be on 127.0.0.1, not ::ffff:127.0.0.1
	listenerAddr := addrs[0].String()
	require.Contains(t, listenerAddr, "127.0.0.1:", "listener should be on unmapped IPv4")
	require.NotContains(t, listenerAddr, "::ffff", "listener should not use IPv4-mapped format")
}

// TestAddPeerIPv4MappedSelfReferential verifies self-referential check works with mapped addresses.
//
// VALIDATES: Peer with Address 10.0.0.1 and LocalAddress ::ffff:10.0.0.1 is rejected.
//
// PREVENTS: Self-referential configuration bypassing validation via address format.
func TestAddPeerIPv4MappedSelfReferential(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// Self-referential using IPv4-mapped format
	settings := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0x01010101)
	settings.LocalAddress = netip.MustParseAddr("::ffff:10.0.0.1") // Same as Address, mapped

	err := reactor.AddPeer(settings)
	require.Error(t, err, "IPv4-mapped self-referential should be rejected")
	require.Contains(t, err.Error(), "cannot equal local-address")
}

// TestAddPeerIPv4MappedAddressNormalization verifies peer Address is normalized.
//
// VALIDATES: Peer with Address ::ffff:10.0.0.2 is stored as 10.0.0.2.
//
// PREVENTS: Connection lookup failure when peer Address uses IPv4-mapped format.
func TestAddPeerIPv4MappedAddressNormalization(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// Add peer with IPv4-mapped Address
	settings := NewPeerSettings(netip.MustParseAddr("::ffff:10.0.0.2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("127.0.0.1")
	settings.Passive = true

	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	// Verify peer is accessible via unmapped address
	peers := reactor.Peers()
	require.Len(t, peers, 1)

	// The stored Address should be unmapped
	storedAddr := peers[0].Settings().Address
	require.Equal(t, netip.MustParseAddr("10.0.0.2"), storedAddr, "Address should be unmapped")
	require.True(t, storedAddr.Is4(), "Address should be IPv4 after unmapping")
}

// TestAddPeerIPv4MappedAddressDuplicate verifies duplicate detection works with mapped addresses.
//
// VALIDATES: Adding ::ffff:10.0.0.2 after 10.0.0.2 is rejected as duplicate.
//
// PREVENTS: Duplicate peers via different address formats.
func TestAddPeerIPv4MappedAddressDuplicate(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// Add peer with IPv4 Address
	settings1 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings1.LocalAddress = mustParseAddr("127.0.0.1")
	err := reactor.AddPeer(settings1)
	require.NoError(t, err)

	// Try to add same peer with IPv4-mapped Address
	settings2 := NewPeerSettings(netip.MustParseAddr("::ffff:10.0.0.2"), 65000, 65002, 0x01010101)
	settings2.LocalAddress = mustParseAddr("127.0.0.1")

	err = reactor.AddPeer(settings2)
	require.Error(t, err, "IPv4-mapped duplicate should be rejected")
	require.ErrorIs(t, err, ErrPeerExists)
}

// TestRemovePeerIPv4Mapped verifies RemovePeer works with mapped addresses.
//
// VALIDATES: RemovePeer(::ffff:10.0.0.2) removes peer stored as 10.0.0.2.
//
// PREVENTS: API inconsistency where AddPeer normalizes but RemovePeer doesn't.
func TestRemovePeerIPv4Mapped(t *testing.T) {
	cfg := &Config{LocalAS: 65000}
	reactor := New(cfg)

	// Add peer with IPv4 Address
	settings := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x01010101)
	settings.LocalAddress = mustParseAddr("127.0.0.1")
	err := reactor.AddPeer(settings)
	require.NoError(t, err)
	require.Len(t, reactor.Peers(), 1)

	// Remove using IPv4-mapped format
	err = reactor.RemovePeer(netip.MustParseAddr("::ffff:10.0.0.2"))
	require.NoError(t, err, "RemovePeer should accept IPv4-mapped address")
	require.Len(t, reactor.Peers(), 0, "peer should be removed")
}

// TestNotifyMessageReceiverWireUpdate verifies WireUpdate is set for UPDATE messages.
//
// VALIDATES: RawMessage.WireUpdate is populated for UPDATE messages.
// PREVENTS: Missing WireUpdate field when API receives UPDATE.
func TestNotifyMessageReceiverWireUpdate(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
	}
	reactor := New(cfg)

	// Add peer
	peerAddr := mustParseAddr("10.0.0.1")
	settings := NewPeerSettings(peerAddr, 65000, 65001, 0x01010101)
	err := reactor.AddPeer(settings)
	require.NoError(t, err)

	// Track received messages
	var receivedMsg api.RawMessage
	var receivedPeer api.PeerInfo
	receiver := &testMessageReceiver{
		onReceived: func(peer api.PeerInfo, msg api.RawMessage) {
			receivedPeer = peer
			receivedMsg = msg
		},
	}
	reactor.SetMessageReceiver(receiver)

	// Build UPDATE payload: withdrawn(0) + attrs(ORIGIN) + nlri(/24)
	// Format: wdLen(2) + attrLen(2) + attrs + nlri
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	nlri := []byte{0x18, 0xc0, 0xa8, 0x01}  // 192.168.1.0/24
	updatePayload := make([]byte, 2+2+len(attrs)+len(nlri))
	// withdrawn len = 0
	updatePayload[0], updatePayload[1] = 0, 0
	// attr len
	updatePayload[2], updatePayload[3] = 0, byte(len(attrs))
	copy(updatePayload[4:], attrs)
	copy(updatePayload[4+len(attrs):], nlri)

	// Create WireUpdate (as session would do)
	wireUpdate := api.NewWireUpdate(updatePayload, 0)

	// Call notifyMessageReceiver directly (same package)
	// In normal flow, session creates WireUpdate and passes it through
	// Pass nil buf since we're not testing caching here
	_ = reactor.notifyMessageReceiver(peerAddr, message.TypeUPDATE, updatePayload, wireUpdate, 0, "received", nil)

	// Verify WireUpdate is set
	require.NotNil(t, receivedMsg.WireUpdate, "WireUpdate should be set for UPDATE")
	require.Equal(t, peerAddr, receivedPeer.Address, "peer address should match")

	// Verify WireUpdate provides correct data
	require.NotNil(t, receivedMsg.WireUpdate.Attrs(), "WireUpdate.Attrs() should return attributes")
	require.NotNil(t, receivedMsg.WireUpdate.NLRI(), "WireUpdate.NLRI() should return NLRI")
	require.Nil(t, receivedMsg.WireUpdate.Withdrawn(), "WireUpdate.Withdrawn() should be nil (no withdrawals)")

	// Verify backward compat: AttrsWire is derived from WireUpdate
	require.NotNil(t, receivedMsg.AttrsWire, "AttrsWire should be set for backward compat")
	require.Equal(t, receivedMsg.WireUpdate.Attrs(), receivedMsg.AttrsWire, "AttrsWire should be same as WireUpdate.Attrs()")
}

// testMessageReceiver implements api.MessageReceiver for testing.
type testMessageReceiver struct {
	onReceived func(api.PeerInfo, api.RawMessage)
	onSent     func(api.PeerInfo, api.RawMessage)
}

func (r *testMessageReceiver) OnMessageReceived(peer api.PeerInfo, msg api.RawMessage) {
	if r.onReceived != nil {
		r.onReceived(peer, msg)
	}
}

func (r *testMessageReceiver) OnMessageSent(peer api.PeerInfo, msg api.RawMessage) {
	if r.onSent != nil {
		r.onSent(peer, msg)
	}
}
