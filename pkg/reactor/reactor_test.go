package reactor

import (
	"context"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/exa-networks/zebgp/pkg/api"
	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
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
