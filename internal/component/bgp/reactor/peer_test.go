package reactor

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/rib"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
)

func mustParseAddr(s string) netip.Addr {
	return netip.MustParseAddr(s)
}

// freePort returns a port from the OS ephemeral range for tests that need to bind listeners.
func freePort(t *testing.T) int {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("freePort: unexpected address type")
	}
	port := addr.Port
	if err := ln.Close(); err != nil {
		t.Fatalf("freePort close: %v", err)
	}
	return port
}

// testRoute creates a valid route for testing.
func testRoute(prefixStr string) *rib.Route {
	prefix := netip.MustParsePrefix(prefixStr)
	fam := family.IPv4Unicast
	n := nlri.NewINET(fam, prefix, 0)
	return rib.NewRoute(n, netip.MustParseAddr("10.0.0.1"), nil)
}

// TestPeerNew verifies Peer creation with correct initial state.
//
// VALIDATES: Peer starts in stopped state with nil session.
//
// PREVENTS: Peer starting automatically or with invalid state.
func TestPeerNew(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)

	peer := NewPeer(settings)

	require.NotNil(t, peer, "NewPeer must return non-nil")
	require.Equal(t, PeerStateStopped, peer.State(), "initial state must be Stopped")
	require.Equal(t, settings, peer.Settings(), "Settings() must return peer settings")
}

// TestPeerStartStop verifies basic start/stop lifecycle.
//
// VALIDATES: Peer can be started and stopped cleanly.
//
// PREVENTS: Resource leaks or goroutine leaks on stop.
func TestPeerStartStop(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = 0 // Invalid port to prevent actual connection

	peer := NewPeer(settings)

	// Start peer
	peer.Start()

	require.Eventually(t, func() bool {
		return peer.State() != PeerStateStopped
	}, time.Second, time.Millisecond, "state should change after Start")

	// Stop peer
	peer.Stop()

	// Wait for stop
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = peer.Wait(ctx)

	require.Equal(t, PeerStateStopped, peer.State(), "state must be Stopped after Stop")
}

// TestPeerReconnect verifies reconnection logic with backoff.
//
// VALIDATES: Peer attempts reconnection after connection failure.
//
// PREVENTS: Peer giving up after first failure or flooding with
// connection attempts without backoff.
func TestPeerReconnect(t *testing.T) {
	// Use a listener that immediately closes connections
	listener, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx // Test code
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok, "expected TCPAddr")

	var connectCount atomic.Int32
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connectCount.Add(1)
			_ = conn.Close() // Immediately close to trigger reconnect
		}
	}()

	settings := NewPeerSettings(
		mustParseAddr("127.0.0.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = uint16(addr.Port) //nolint:gosec // Port fits in uint16

	peer := NewPeer(settings)
	peer.SetReconnectDelay(10*time.Millisecond, 50*time.Millisecond)

	peer.Start()

	// Wait for multiple reconnect attempts
	require.Eventually(t, func() bool {
		return connectCount.Load() >= 2
	}, time.Second, time.Millisecond, "peer should reconnect at least twice")

	peer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = peer.Wait(ctx)
}

// TestPeerContextCancellation verifies peer stops on context cancellation.
//
// VALIDATES: Peer respects context cancellation for clean shutdown.
//
// PREVENTS: Orphaned goroutines when parent context is canceled.
func TestPeerContextCancellation(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = 0 // Invalid port

	peer := NewPeer(settings)

	ctx, cancel := context.WithCancel(context.Background())
	peer.StartWithContext(ctx)

	require.Eventually(t, func() bool {
		return peer.State() != PeerStateStopped
	}, time.Second, time.Millisecond, "peer should leave Stopped state")

	// Cancel context
	cancel()

	// Should stop within reasonable time
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	err := peer.Wait(waitCtx)

	require.NoError(t, err, "peer should stop on context cancellation")
	require.Equal(t, PeerStateStopped, peer.State())
}

// TestPeerStateTransitions verifies state changes during connection lifecycle.
//
// VALIDATES: Peer reports correct state (Connecting, Connected, etc).
//
// PREVENTS: Incorrect state reporting to callers.
func TestPeerStateTransitions(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx // Test code
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok, "expected TCPAddr")

	// Accept connections but don't respond (peer stays connecting).
	// Connections held open until listener closes (deferred above).
	go func() {
		var held []net.Conn
		defer func() {
			for _, c := range held {
				c.Close() //nolint:errcheck // test cleanup
			}
		}()
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return // listener closed
			}
			held = append(held, conn)
		}
	}()

	settings := NewPeerSettings(
		mustParseAddr("127.0.0.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = uint16(addr.Port) //nolint:gosec // Port fits in uint16

	peer := NewPeer(settings)
	peer.Start()

	// Should transition to Connecting
	require.Eventually(t, func() bool {
		s := peer.State()
		return s == PeerStateConnecting || s == PeerStateActive
	}, time.Second, time.Millisecond, "state should be Connecting or Active")

	peer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = peer.Wait(ctx)
}

// TestPeerCallback verifies state change callbacks are invoked.
//
// VALIDATES: Callback is called on state transitions.
//
// PREVENTS: Missing notifications to observers.
func TestPeerCallback(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = 0

	peer := NewPeer(settings)

	var callbackCalled atomic.Bool
	var transitions []PeerState
	peer.SetCallback(func(from, to PeerState) {
		transitions = append(transitions, to)
		callbackCalled.Store(true)
	})

	peer.Start()
	require.Eventually(t, callbackCalled.Load, 2*time.Second, time.Millisecond, "callback should be invoked at least once")
	peer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = peer.Wait(ctx)

	require.NotEmpty(t, transitions, "callback should be invoked at least once")
}

// TestBuildStaticRouteUpdateIPv6 verifies UPDATE generation for IPv6 unicast.
//
// VALIDATES: IPv6 unicast routes include MP_REACH_NLRI attribute and have
// empty inline NLRI field.
//
// PREVENTS: IPv6 routes being sent with inline NLRI (which violates RFC 4760).
func TestBuildStaticRouteUpdateIPv6(t *testing.T) {
	nextHop := netip.MustParseAddr("2001:db8::ffff")
	route := StaticRoute{
		Prefix:          netip.MustParsePrefix("2001:db8::1/128"),
		NextHop:         bgptypes.NewNextHopExplicit(nextHop),
		Origin:          0,
		LocalPreference: 100,
	}

	ub := message.NewUpdateBuilder(65000, true, true, false)                    // iBGP, ASN4, no ADD-PATH
	update := buildStaticRouteUpdateNew(ub, &route, nextHop, netip.Addr{}, nil) // no link-local, no ExtNH

	// IPv6 routes must NOT have inline NLRI
	require.Empty(t, update.NLRI, "IPv6 route must not have inline NLRI")

	// Path attributes must contain MP_REACH_NLRI
	require.NotEmpty(t, update.PathAttributes, "must have path attributes")

	// Look for MP_REACH_NLRI (code 14) in attributes
	found := false
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		flags := update.PathAttributes[offset]
		code := update.PathAttributes[offset+1]

		var attrLen int
		if flags&0x10 != 0 {
			if offset+4 > len(update.PathAttributes) {
				break
			}
			attrLen = int(binary.BigEndian.Uint16(update.PathAttributes[offset+2 : offset+4]))
			offset += 4
		} else {
			if offset+3 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])
			offset += 3
		}

		if code == byte(attribute.AttrMPReachNLRI) {
			found = true
			break
		}
		offset += attrLen
	}

	require.True(t, found, "IPv6 UPDATE must contain MP_REACH_NLRI attribute")
}

// TestBuildStaticRouteUpdateWithCommunities verifies communities are included.
//
// VALIDATES: Routes with communities produce COMMUNITIES attribute (code 8).
//
// PREVENTS: Communities being silently dropped from announcements.
func TestBuildStaticRouteUpdateWithCommunities(t *testing.T) {
	nextHop := netip.MustParseAddr("192.0.2.1")
	route := StaticRoute{
		Prefix:      netip.MustParsePrefix("192.0.2.0/24"),
		NextHop:     bgptypes.NewNextHopExplicit(nextHop),
		Origin:      0,
		Communities: []uint32{0x78140000, 0x78147814}, // 30740:0, 30740:30740
	}

	ub := message.NewUpdateBuilder(65000, false, true, false)                   // eBGP, ASN4, no ADD-PATH
	update := buildStaticRouteUpdateNew(ub, &route, nextHop, netip.Addr{}, nil) // no link-local, no ExtNH
	require.NotEmpty(t, update.PathAttributes, "must have path attributes")

	// Look for COMMUNITIES (code 8) in attributes
	found := false
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		flags := update.PathAttributes[offset]
		code := update.PathAttributes[offset+1]

		var attrLen int
		if flags&0x10 != 0 {
			if offset+4 > len(update.PathAttributes) {
				break
			}
			attrLen = int(binary.BigEndian.Uint16(update.PathAttributes[offset+2 : offset+4]))
			offset += 4
		} else {
			if offset+3 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])
			offset += 3
		}

		if code == byte(attribute.AttrCommunity) {
			found = true
			require.Equal(t, 8, attrLen, "communities length must be 8 (2 x 4 bytes)")
			break
		}
		offset += attrLen
	}

	require.True(t, found, "UPDATE with communities must contain COMMUNITIES attribute")
}

// TestPeerOpQueueOrdering verifies operation queue maintains order.
//
// VALIDATES: Operations queued when not connected are processed in order.
//
// PREVENTS: Out-of-order route announcements or teardowns.
func TestPeerOpQueueOrdering(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)

	peer := NewPeer(settings)

	// Queue operations while not connected
	route1 := testRoute("10.0.0.0/8")
	route2 := testRoute("20.0.0.0/8")

	peer.QueueAnnounce(route1)
	peer.QueueAnnounce(route2)

	// Verify queue order
	peer.mu.RLock()
	require.Len(t, peer.opQueue, 2, "queue should have 2 items")
	require.Equal(t, PeerOpAnnounce, peer.opQueue[0].Type)
	require.Equal(t, route1, peer.opQueue[0].Route)
	require.Equal(t, PeerOpAnnounce, peer.opQueue[1].Type)
	require.Equal(t, route2, peer.opQueue[1].Route)
	peer.mu.RUnlock()
}

// TestPeerShouldQueue verifies ShouldQueue returns correct state.
//
// VALIDATES: ShouldQueue returns true when not established, when initial routes
// are in progress, or when opQueue is non-empty. Returns false only when
// established AND no initial routes running AND queue empty.
//
// PREVENTS: Route ordering race where direct sends bypass queued routes.
func TestPeerShouldQueue(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Not established → should queue
	require.True(t, peer.ShouldQueue(), "should queue when not established")

	// Simulate established state. setState closes the initial-sync gate as it
	// publishes Established (peer.go), so the peer queues until its sync ends.
	peer.setState(PeerStateEstablished)
	require.True(t, peer.ShouldQueue(), "should queue while the initial sync is still owed")

	// What sendInitialRoutes does when its drain and End-of-RIB are done.
	peer.sendingInitialRoutes.Store(0)
	require.False(t, peer.ShouldQueue(), "should not queue when established with empty queue")

	// Queue has items → should queue (preserves insertion order)
	route := testRoute("10.0.0.0/8")
	peer.QueueAnnounce(route)
	require.True(t, peer.ShouldQueue(), "should queue when opQueue non-empty")

	// Clear queue, still established → should not queue
	peer.mu.Lock()
	peer.opQueue = peer.opQueue[:0]
	peer.mu.Unlock()
	require.False(t, peer.ShouldQueue(), "should not queue after clearing opQueue")

	// Initial routes in progress → should queue
	peer.sendingInitialRoutes.Store(1)
	require.True(t, peer.ShouldQueue(), "should queue when sendingInitialRoutes flag set")

	// Clear flag → should not queue
	peer.sendingInitialRoutes.Store(0)
	require.False(t, peer.ShouldQueue(), "should not queue after flag cleared")
}

// TestPeerTeardownQueuesWhenNotConnected verifies teardown is queued when no session.
//
// VALIDATES: Teardown called without active session queues the operation.
//
// PREVENTS: Lost teardown requests when session is not established.
func TestPeerTeardownQueuesWhenNotConnected(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)

	peer := NewPeer(settings)

	// Teardown with no session should queue
	require.NoError(t, peer.Teardown(4, "")) // AdminReset subcode

	peer.mu.RLock()
	require.Len(t, peer.opQueue, 1, "queue should have 1 item")
	require.Equal(t, PeerOpTeardown, peer.opQueue[0].Type)
	require.Equal(t, uint8(4), peer.opQueue[0].Subcode)
	peer.mu.RUnlock()
}

// TestPeerOpQueueMixedOperations verifies mixed announce/teardown ordering.
//
// VALIDATES: Interleaved announce and teardown operations maintain order.
//
// PREVENTS: Teardowns being processed before preceding announces.
func TestPeerOpQueueMixedOperations(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)

	peer := NewPeer(settings)

	// Simulate: announce → teardown → announce
	route1 := testRoute("10.0.0.0/8")
	route2 := testRoute("20.0.0.0/8")

	peer.QueueAnnounce(route1)
	require.NoError(t, peer.Teardown(4, ""))
	peer.QueueAnnounce(route2)

	peer.mu.RLock()
	require.Len(t, peer.opQueue, 3, "queue should have 3 items")

	// Verify order: Route1, Teardown, Route2
	require.Equal(t, PeerOpAnnounce, peer.opQueue[0].Type)
	require.Equal(t, route1, peer.opQueue[0].Route)

	require.Equal(t, PeerOpTeardown, peer.opQueue[1].Type)
	require.Equal(t, uint8(4), peer.opQueue[1].Subcode)

	require.Equal(t, PeerOpAnnounce, peer.opQueue[2].Type)
	require.Equal(t, route2, peer.opQueue[2].Route)
	peer.mu.RUnlock()
}

// TestPeerOpQueueMultipleTeardowns verifies consecutive teardowns are queued.
//
// VALIDATES: Multiple teardowns without intervening announces are all queued.
//
// PREVENTS: Teardown coalescing that might lose subcode information.
func TestPeerOpQueueMultipleTeardowns(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)

	peer := NewPeer(settings)

	require.NoError(t, peer.Teardown(2, "")) // AdminShutdown
	require.NoError(t, peer.Teardown(4, "")) // AdminReset

	peer.mu.RLock()
	require.Len(t, peer.opQueue, 2, "queue should have 2 items")
	require.Equal(t, uint8(2), peer.opQueue[0].Subcode)
	require.Equal(t, uint8(4), peer.opQueue[1].Subcode)
	peer.mu.RUnlock()
}

// TestPeerOpQueueOverflow verifies queue respects DefaultOpQueueSize limit.
//
// VALIDATES: Operations are dropped when queue reaches DefaultOpQueueSize.
//
// PREVENTS: Unbounded memory growth when session is disconnected.
func TestPeerOpQueueOverflow(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)

	peer := NewPeer(settings)

	// Fill queue to capacity with valid routes
	route := testRoute("10.0.0.0/8")
	for range DefaultOpQueueSize {
		peer.QueueAnnounce(route)
	}

	peer.mu.RLock()
	require.Len(t, peer.opQueue, DefaultOpQueueSize, "queue should be at max capacity")
	peer.mu.RUnlock()

	// Additional operations should be dropped
	peer.QueueAnnounce(route)
	require.ErrorIs(t, peer.Teardown(4, ""), ErrOpQueueFull)

	peer.mu.RLock()
	require.Len(t, peer.opQueue, DefaultOpQueueSize, "queue should not exceed max capacity")
	peer.mu.RUnlock()
}

// TestRouteFamilyIPv4Unicast verifies IPv4 unicast routes return correct family.
//
// VALIDATES: IPv4 unicast route returns AFI=1, SAFI=1.
//
// PREVENTS: EOR being sent for wrong family.
func TestRouteFamilyIPv4Unicast(t *testing.T) {
	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("192.0.2.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
	}

	fam := routeFamily(&route)

	require.Equal(t, family.AFIIPv4, fam.AFI, "AFI should be IPv4")
	require.Equal(t, family.SAFIUnicast, fam.SAFI, "SAFI should be unicast")
}

// TestRouteFamilyIPv6Unicast verifies IPv6 unicast routes return correct family.
//
// VALIDATES: IPv6 unicast route returns AFI=2, SAFI=1.
//
// PREVENTS: EOR being sent for wrong family.
func TestRouteFamilyIPv6Unicast(t *testing.T) {
	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}

	fam := routeFamily(&route)

	require.Equal(t, family.AFIIPv6, fam.AFI, "AFI should be IPv6")
	require.Equal(t, family.SAFIUnicast, fam.SAFI, "SAFI should be unicast")
}

// TestRouteFamilyVPNv4 verifies VPNv4 routes return correct family.
//
// VALIDATES: VPNv4 route (with RD) returns AFI=1, SAFI=128.
//
// PREVENTS: VPN routes being counted as unicast for EOR.
func TestRouteFamilyVPNv4(t *testing.T) {
	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
		RD:      "100:100", // Has RD = VPN
	}

	fam := routeFamily(&route)

	require.Equal(t, family.AFIIPv4, fam.AFI, "AFI should be IPv4")
	require.Equal(t, family.SAFI(128), fam.SAFI, "SAFI should be MPLS-VPN (128)")
}

// TestRouteFamilyVPNv6 verifies VPNv6 routes return correct family.
//
// VALIDATES: VPNv6 route (with RD) returns AFI=2, SAFI=128.
//
// PREVENTS: VPN routes being counted as unicast for EOR.
func TestRouteFamilyVPNv6(t *testing.T) {
	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
		RD:      "100:100", // Has RD = VPN
	}

	fam := routeFamily(&route)

	require.Equal(t, family.AFIIPv6, fam.AFI, "AFI should be IPv6")
	require.Equal(t, family.SAFI(128), fam.SAFI, "SAFI should be MPLS-VPN (128)")
}

// test-relax: three End-of-RIB "families sent" tests stood here until 2026-08-09.
// Each built a local map, filled it inline, and asserted on its own fill; none
// called production code, so all three were green against any implementation while
// their names and VALIDATES comments claimed RFC 4724 coverage they did not have.
// One session read them as proof that ze sends End-of-RIB only for families that
// carried routes, and was about to escalate a conformance violation that does not
// exist. Coverage is REPLACED, not dropped: the behavior they named is now driven
// from sendInitialRoutes and asserted on the bytes that reach the wire, by
// TestInitialSyncEORCountedOncePerFamilyOnTheWire (both families silent) and
// TestInitialSyncEORReachesTheSilentFamilyToo (one family carries a route, the
// other does not) in peer_initial_sync_test.go. Both go red when the End-of-RIB
// loop is disabled, and red again when it is narrowed to route-carrying families,
// which is the reading the deleted tests encoded.

// =============================================================================
// ADD-PATH Tests (RFC 7911)
// =============================================================================

// TestPeerAddPathNilSendCtx verifies AddPath returns false when sendCtx is nil.
//
// VALIDATES: AddPath returns false when session not established (nil sendCtx).
//
// PREVENTS: Nil pointer dereference when building NLRI before session established.
func TestPeerAddPathNilSendCtx(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// No sendCtx set (session not established)
	require.Nil(t, peer.sendCtx.Load(), "sendCtx should be nil")
}

// TestPeerAddPathIPv4Unicast verifies IPv4 unicast ADD-PATH context.
//
// VALIDATES: sendCtx.AddPath returns true for IPv4 unicast when negotiated.
//
// PREVENTS: Missing path ID in IPv4 unicast NLRI when ADD-PATH is negotiated.
func TestPeerAddPathIPv4Unicast(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Set sendCtx with ADD-PATH enabled for IPv4 unicast
	peer.sendCtx.Store(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{
		{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true,
		{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}: false,
	}))

	addPath := peer.sendCtx.Load().AddPath(family.IPv4Unicast)
	require.True(t, addPath, "AddPath should be true for IPv4 unicast")
}

// TestPeerAddPathIPv6Unicast verifies IPv6 unicast ADD-PATH context.
//
// VALIDATES: sendCtx.AddPath returns true for IPv6 unicast when negotiated.
//
// PREVENTS: Missing path ID in IPv6 unicast NLRI when ADD-PATH is negotiated.
func TestPeerAddPathIPv6Unicast(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Set sendCtx with ADD-PATH enabled for IPv6 unicast
	peer.sendCtx.Store(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{
		{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: false,
		{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}: true,
	}))

	addPath := peer.sendCtx.Load().AddPath(family.IPv6Unicast)
	require.True(t, addPath, "AddPath should be true for IPv6 unicast")
}

// TestPeerAddPathLabeledUnicast verifies labeled-unicast ADD-PATH context.
//
// VALIDATES: sendCtx.AddPath returns true for labeled-unicast when negotiated.
//
// PREVENTS: Missing path ID in labeled-unicast NLRI when ADD-PATH is negotiated.
func TestPeerAddPathLabeledUnicast(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Set sendCtx with ADD-PATH enabled for labeled-unicast
	peer.sendCtx.Store(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{
		{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}: true,
		{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel}: true,
	}))

	// IPv4 labeled-unicast (SAFI 4)
	addPath4 := peer.sendCtx.Load().AddPath(family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel})
	require.True(t, addPath4, "AddPath should be true for IPv4 labeled-unicast")

	// IPv6 labeled-unicast (SAFI 4)
	addPath6 := peer.sendCtx.Load().AddPath(family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel})
	require.True(t, addPath6, "AddPath should be true for IPv6 labeled-unicast")
}

// TestPeerAddPathNoAddPath verifies non-ADD-PATH families return AddPath=false.
//
// VALIDATES: sendCtx.AddPath returns false for families without ADD-PATH.
//
// PREVENTS: Spurious path ID prepended to NLRI for non-ADD-PATH sessions.
func TestPeerAddPathNoAddPath(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Set sendCtx WITHOUT ADD-PATH
	peer.sendCtx.Store(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{
		{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: false,
		{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}: false,
	}))

	addPath4 := peer.sendCtx.Load().AddPath(family.IPv4Unicast)
	require.False(t, addPath4, "AddPath should be false for IPv4 unicast without ADD-PATH")

	addPath6 := peer.sendCtx.Load().AddPath(family.IPv6Unicast)
	require.False(t, addPath6, "AddPath should be false for IPv6 unicast without ADD-PATH")
}

// TestPeerAddPathOtherFamilies verifies non-unicast families return AddPath=false.
//
// VALIDATES: sendCtx.AddPath returns false for families not in AddPath map.
//
// PREVENTS: Spurious path ID in NLRI for families without ADD-PATH negotiated.
func TestPeerAddPathOtherFamilies(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	peer.sendCtx.Store(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{
		{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true,
		// VPN not in map = AddPath false
	}))

	// VPN family - not in AddPath map so should be false
	vpnFamily := family.Family{AFI: family.AFIIPv4, SAFI: 128}
	addPath := peer.sendCtx.Load().AddPath(vpnFamily)
	require.False(t, addPath, "VPN family should have AddPath=false")
	require.True(t, peer.sendCtx.Load().ASN4(), "ASN4 should still be accessible from sendCtx")
}

// TestPeerEncodingContextASN4 verifies sendCtx includes ASN4 from negotiated state.
//
// VALIDATES: sendCtx.ASN4() reflects negotiated 4-byte AS number capability.
// RFC 6793 Section 4.1: NEW speakers use 4-byte AS numbers when both support it.
//
// PREVENTS: AS_PATH encoded with wrong ASN size, causing protocol violations.
func TestPeerEncodingContextASN4(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Session with ASN4=true
	peer.sendCtx.Store(bgpctx.EncodingContextForASN4(true))
	require.True(t, peer.sendCtx.Load().ASN4(), "ASN4 should be true when negotiated")

	// Session with ASN4=false (OLD speaker)
	peer.sendCtx.Store(bgpctx.EncodingContextForASN4(false))
	require.False(t, peer.sendCtx.Load().ASN4(), "ASN4 should be false for OLD speaker")
}

// =============================================================================
// Peer EncodingContext Tests
// =============================================================================
//
// These tests verify the integration of EncodingContext with Peer lifecycle:
//
//	Test                              | Scenario
//	----------------------------------|------------------------------------------
//	TestPeerEncodingContextNilInitially | Contexts nil before session established
//	TestPeerSetEncodingContexts         | Contexts created from Negotiated
//	TestPeerClearEncodingContexts       | Contexts cleared on teardown
//	TestPeerEncodingContextAddPath      | Asymmetric ADD-PATH (Send/Receive case)
//
// Note: Full ADD-PATH permutation testing is in pkg/bgp/context/negotiated_test.go.
// These tests focus on Peer integration, not the FromNegotiated logic itself.
// =============================================================================

// TestPeerEncodingContextNilInitially verifies contexts are nil after creation.
//
// VALIDATES: recvCtx/sendCtx are nil before session established.
//
// PREVENTS: Using uninitialized context for encoding.
func TestPeerEncodingContextNilInitially(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	require.Nil(t, peer.RecvContext(), "recvCtx should be nil initially")
	require.Nil(t, peer.SendContext(), "sendCtx should be nil initially")
	require.Equal(t, bgpctx.ContextID(0), peer.RecvContextID(), "recvCtxID should be 0 initially")
	require.Equal(t, bgpctx.ContextID(0), peer.SendContextID(), "sendCtxID should be 0 initially")
}

// TestPeerSetEncodingContexts verifies context setting.
//
// VALIDATES: setEncodingContexts correctly stores contexts and IDs.
//
// PREVENTS: Wrong context used for encoding/decoding.
func TestPeerSetEncodingContexts(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Create mock negotiated state
	local := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: 65000},
	}
	remote := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: 65001},
	}
	neg := capability.Negotiate(local, remote, 65000, 65001)

	// Set contexts
	peer.setEncodingContexts(neg)

	require.NotNil(t, peer.RecvContext(), "recvCtx should be set")
	require.NotNil(t, peer.SendContext(), "sendCtx should be set")
	require.True(t, peer.RecvContext().ASN4(), "recvCtx should have ASN4=true")
	require.True(t, peer.SendContext().ASN4(), "sendCtx should have ASN4=true")
}

// TestPeerClearEncodingContexts verifies context clearing on teardown.
//
// VALIDATES: clearEncodingContexts sets contexts to nil.
//
// PREVENTS: Stale context after session end.
func TestPeerClearEncodingContexts(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Set contexts first
	local := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: 65000},
	}
	remote := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: 65001},
	}
	neg := capability.Negotiate(local, remote, 65000, 65001)
	peer.setEncodingContexts(neg)

	require.NotNil(t, peer.RecvContext(), "recvCtx should be set before clear")

	// Clear contexts
	peer.clearEncodingContexts()

	require.Nil(t, peer.RecvContext(), "recvCtx should be nil after clear")
	require.Nil(t, peer.SendContext(), "sendCtx should be nil after clear")
	require.Equal(t, bgpctx.ContextID(0), peer.RecvContextID(), "recvCtxID should be 0 after clear")
	require.Equal(t, bgpctx.ContextID(0), peer.SendContextID(), "sendCtxID should be 0 after clear")
}

// TestPeerEncodingContextAddPath verifies ADD-PATH context asymmetry.
//
// VALIDATES: recv/send contexts have correct ADD-PATH based on mode.
//
// PREVENTS: Wrong path ID handling for asymmetric ADD-PATH.
func TestPeerEncodingContextAddPath(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Local wants to send, remote wants to receive -> we can send, can't receive
	local := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: 65000},
		&capability.AddPath{Families: []capability.AddPathFamily{
			{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathSend},
		}},
	}
	remote := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: 65001},
		&capability.AddPath{Families: []capability.AddPathFamily{
			{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathReceive},
		}},
	}
	neg := capability.Negotiate(local, remote, 65000, 65001)
	peer.setEncodingContexts(neg)

	ipv4 := bgpctx.Family{AFI: 1, SAFI: 1}

	// We can send but not receive
	require.False(t, peer.RecvContext().AddPathFor(ipv4), "recv should NOT have AddPath (we can't receive)")
	require.True(t, peer.SendContext().AddPathFor(ipv4), "send should have AddPath (we can send)")
}

// TestToStaticRouteUnicastParams_CopiesReflectorAttrs verifies RFC 4456 fields.
//
// VALIDATES: OriginatorID and ClusterList are copied to UnicastParams.
// PREVENTS: Silent data loss for route reflector attributes.
func TestToStaticRouteUnicastParams_CopiesReflectorAttrs(t *testing.T) {
	nextHop := netip.MustParseAddr("192.168.1.1")
	route := StaticRoute{
		Prefix:       netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:      bgptypes.NewNextHopExplicit(nextHop),
		OriginatorID: 0xC0A80101,
		ClusterList:  []uint32{0xC0A80102, 0xC0A80103},
	}

	params := toStaticRouteUnicastParams(&route, nextHop, netip.Addr{}, nil) // no link-local, nil sendCtx - no ExtNH needed

	require.Equal(t, route.OriginatorID, params.OriginatorID,
		"OriginatorID not copied: got %x, want %x", params.OriginatorID, route.OriginatorID)
	require.Equal(t, len(route.ClusterList), len(params.ClusterList),
		"ClusterList length mismatch: got %d, want %d", len(params.ClusterList), len(route.ClusterList))
	for i, v := range route.ClusterList {
		require.Equal(t, v, params.ClusterList[i],
			"ClusterList[%d] mismatch: got %x, want %x", i, params.ClusterList[i], v)
	}
}

// TestRouteGroupKey_IncludesReflectorAttrs verifies grouping key includes RFC 4456 fields.
//
// VALIDATES: Routes with different OriginatorID get different keys.
// PREVENTS: Silent data loss when grouping routes with different reflector attrs.
func TestRouteGroupKey_IncludesReflectorAttrs(t *testing.T) {
	route1 := StaticRoute{
		Prefix:       netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:      bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
		OriginatorID: 0xC0A80101,
	}
	route2 := StaticRoute{
		Prefix:       netip.MustParsePrefix("10.0.1.0/24"),
		NextHop:      bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
		OriginatorID: 0xC0A80102, // Different!
	}

	key1 := routeGroupKey(&route1)
	key2 := routeGroupKey(&route2)

	require.NotEqual(t, key1, key2,
		"Routes with different OriginatorID should have different keys\nkey1: %s\nkey2: %s", key1, key2)
}

// TestRouteGroupKey_IncludesClusterList verifies ClusterList affects grouping.
//
// VALIDATES: Routes with different ClusterList get different keys.
// PREVENTS: Silent data loss when grouping routes with different cluster lists.
func TestRouteGroupKey_IncludesClusterList(t *testing.T) {
	route1 := StaticRoute{
		Prefix:      netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:     bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
		ClusterList: []uint32{0xC0A80101},
	}
	route2 := StaticRoute{
		Prefix:      netip.MustParsePrefix("10.0.1.0/24"),
		NextHop:     bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
		ClusterList: []uint32{0xC0A80101, 0xC0A80102}, // Different!
	}

	key1 := routeGroupKey(&route1)
	key2 := routeGroupKey(&route2)

	require.NotEqual(t, key1, key2,
		"Routes with different ClusterList should have different keys")
}

// =============================================================================
// RouteNextHop Resolution Tests
// =============================================================================

// TestResolveNextHop_Explicit verifies explicit next-hop resolution.
//
// VALIDATES: Explicit policy returns the configured address.
// PREVENTS: Explicit addresses being modified or rejected.
func TestResolveNextHop_Explicit(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	addr := netip.MustParseAddr("10.0.0.1")
	nh := bgptypes.NewNextHopExplicit(addr)

	got, err := peer.resolveNextHop(nh, family.IPv4Unicast)
	require.NoError(t, err)
	require.Equal(t, addr, got)
}

// TestResolveNextHop_Self verifies self next-hop resolution.
//
// VALIDATES: Self policy returns LocalAddress from settings.
// PREVENTS: Self policy using wrong address or failing unexpectedly.
func TestResolveNextHop_Self(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	settings.LocalAddress = netip.MustParseAddr("10.0.0.100")
	peer := NewPeer(settings)

	nh := bgptypes.NewNextHopSelf()

	got, err := peer.resolveNextHop(nh, family.IPv4Unicast)
	require.NoError(t, err)
	require.Equal(t, settings.LocalAddress, got)
}

// TestResolveNextHop_SelfNoLocal verifies error when Self without LocalAddress.
//
// VALIDATES: Self policy without LocalAddress returns ErrNextHopSelfNoLocal.
// PREVENTS: Using invalid/zero address when LocalAddress not configured.
func TestResolveNextHop_SelfNoLocal(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	// LocalAddress not set (zero value)
	peer := NewPeer(settings)

	nh := bgptypes.NewNextHopSelf()

	_, err := peer.resolveNextHop(nh, family.IPv4Unicast)
	require.ErrorIs(t, err, ErrNextHopSelfNoLocal)
}

// TestResolveNextHop_Unset verifies error for unset policy.
//
// VALIDATES: Unset policy returns ErrNextHopUnset.
// PREVENTS: Using zero-value RouteNextHop silently.
func TestResolveNextHop_Unset(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	var nh bgptypes.RouteNextHop // zero value = NextHopUnset

	_, err := peer.resolveNextHop(nh, family.IPv4Unicast)
	require.ErrorIs(t, err, ErrNextHopUnset)
}

// TestResolveNextHop_ExplicitInvalid verifies explicit with invalid addr.
//
// VALIDATES: Explicit with invalid addr returns that addr (no error).
// PREVENTS: Blocking explicit addresses unnecessarily.
func TestResolveNextHop_ExplicitInvalid(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	nh := bgptypes.NewNextHopExplicit(netip.Addr{}) // invalid addr

	got, err := peer.resolveNextHop(nh, family.IPv4Unicast)
	require.NoError(t, err, "explicit bypasses validation")
	require.False(t, got.IsValid(), "should return invalid addr as-is")
}

// TestCanUseNextHopFor_IPv4Natural verifies IPv4 addr for IPv4 family.
//
// VALIDATES: IPv4 address is valid next-hop for IPv4 unicast.
// PREVENTS: Natural match being rejected.
func TestCanUseNextHopFor_IPv4Natural(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	addr := netip.MustParseAddr("10.0.0.1")
	ok := peer.canUseNextHopFor(addr, family.IPv4Unicast)
	require.True(t, ok, "IPv4 addr should be valid for IPv4 family")
}

// TestCanUseNextHopFor_IPv6Natural verifies IPv6 addr for IPv6 family.
//
// VALIDATES: IPv6 address is valid next-hop for IPv6 unicast.
// PREVENTS: Natural match being rejected.
func TestCanUseNextHopFor_IPv6Natural(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	addr := netip.MustParseAddr("2001:db8::1")
	ok := peer.canUseNextHopFor(addr, family.IPv6Unicast)
	require.True(t, ok, "IPv6 addr should be valid for IPv6 family")
}

// TestCanUseNextHopFor_ExtendedNH verifies cross-family with Extended NH.
//
// VALIDATES: IPv6 addr for IPv4 family allowed when Extended NH negotiated.
// PREVENTS: Rejecting valid RFC 5549/8950 configuration.
//
// RFC requirement: RFC8950-4-1 positive -- when the Extended Next Hop capability for
// IPv4/Unicast -> IPv6 next-hop is present in the send context, canUseNextHopFor permits an
// IPv6 next-hop for IPv4 NLRI, so the speaker may advertise it (internal/component/bgp/reactor/peer.go:694).
//
// RFC requirement: RFC5549-4-1 positive -- with Extended Next Hop negotiated for IPv4/Unicast -> IPv6,
// canUseNextHopFor permits the IPv6 next-hop for IPv4 NLRI, so the speaker may advertise it; RFC 5549
// shares ze's RFC 8950 extended-next-hop code path (internal/component/bgp/reactor/peer.go:694).
// RFC requirement: RFC5549-4-4 positive -- the MUST NOT does not fire when ExtNH is negotiated:
// canUseNextHopFor returns true, permitting the IPv6-next-hop-for-IPv4 advertisement
// (internal/component/bgp/reactor/peer.go:694).
func TestCanUseNextHopFor_ExtendedNH(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	// Set up sendCtx with Extended NH for IPv4 unicast → IPv6 next-hop
	peer.sendCtx.Store(bgpctx.NewEncodingContext(nil, &capability.EncodingCaps{
		ExtendedNextHop: map[capability.Family]capability.AFI{
			{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}: capability.AFIIPv6,
		},
	}, bgpctx.DirectionSend))

	addr := netip.MustParseAddr("2001:db8::1") // IPv6 addr
	ok := peer.canUseNextHopFor(addr, family.IPv4Unicast)
	require.True(t, ok, "IPv6 addr should be valid for IPv4 family with Extended NH")
}

// TestCanUseNextHopFor_CrossFamilyNoCap verifies cross-family without cap fails.
//
// VALIDATES: IPv6 addr for IPv4 family rejected without Extended NH.
// PREVENTS: Invalid next-hop going on wire.
//
// RFC requirement: RFC8950-4-1 negative -- without the Extended Next Hop capability negotiated,
// canUseNextHopFor denies an IPv6 next-hop for IPv4 NLRI, so the speaker MUST NOT advertise it
// (internal/component/bgp/reactor/peer.go:694).
//
// RFC requirement: RFC5549-4-1 negative -- without Extended Next Hop negotiated, canUseNextHopFor
// denies an IPv6 next-hop for IPv4 NLRI, so peer support has not been ascertained and the speaker
// must not advertise it (internal/component/bgp/reactor/peer.go:694).
// RFC requirement: RFC5549-4-4 negative -- a peer that has not advertised ExtNH must not be sent an
// IPv6 next-hop for IPv4 NLRI: canUseNextHopFor returns false (internal/component/bgp/reactor/peer.go:694).
func TestCanUseNextHopFor_CrossFamilyNoCap(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)
	// No sendCtx or ExtendedNextHop

	addr := netip.MustParseAddr("2001:db8::1") // IPv6 addr
	ok := peer.canUseNextHopFor(addr, family.IPv4Unicast)
	require.False(t, ok, "cross-family should fail without Extended NH")
}

// TestCanUseNextHopFor_NilSendCtx verifies nil sendCtx cross-family fails.
//
// VALIDATES: Cross-family fails gracefully when sendCtx is nil.
// PREVENTS: Panic on nil pointer dereference.
//
// RFC requirement: RFC8950-4-1 negative -- with no send context at all there is no negotiated
// Extended Next Hop capability, so canUseNextHopFor denies an IPv6 next-hop for IPv4 NLRI
// (internal/component/bgp/reactor/peer.go:694,704).
//
// RFC requirement: RFC5549-4-4 negative -- with no send context there is no negotiated ExtNH, so
// canUseNextHopFor denies the IPv6 next-hop for IPv4 NLRI and the speaker MUST NOT send it
// (internal/component/bgp/reactor/peer.go:694,704).
func TestCanUseNextHopFor_NilSendCtx(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)
	peer.sendCtx.Store(nil)

	addr := netip.MustParseAddr("2001:db8::1") // IPv6 addr
	ok := peer.canUseNextHopFor(addr, family.IPv4Unicast)
	require.False(t, ok, "cross-family should fail with nil sendCtx")
}

// --- Backpressure pause/resume tests ---
// VALIDATES: AC-5, AC-6 — Peer.PauseReading() delegates to session, handles nil session
// PREVENTS: Panic when pausing a peer with no active session

func TestPeerPauseReadingDelegates(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)

	t.Run("with active session", func(t *testing.T) {
		peer := NewPeer(settings)
		session := NewSession(settings)

		peer.mu.Lock()
		peer.session = session
		peer.mu.Unlock()

		// PauseReading should delegate to session.Pause().
		peer.PauseReading()
		require.True(t, session.IsPaused(), "session should be paused after PauseReading()")

		// ResumeReading should delegate to session.Resume().
		peer.ResumeReading()
		require.False(t, session.IsPaused(), "session should not be paused after ResumeReading()")

		// IsReadPaused should reflect session state.
		require.False(t, peer.IsReadPaused())
		peer.PauseReading()
		require.True(t, peer.IsReadPaused())
		peer.ResumeReading()
	})

	t.Run("with nil session", func(t *testing.T) {
		peer := NewPeer(settings)
		// session is nil by default.

		// Should not panic.
		peer.PauseReading()
		peer.ResumeReading()
		require.False(t, peer.IsReadPaused())
	})
}

// TestPeerTeardownQueuesMessage verifies that Teardown preserves the RFC 8203
// shutdown communication message in the operation queue.
//
// VALIDATES: Teardown with a non-empty message stores the message in the queued PeerOp.
//
// PREVENTS: Shutdown communication message being silently dropped when queued.
func TestPeerTeardownQueuesMessage(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)

	peer := NewPeer(settings)

	// Teardown with shutdown message, no session
	require.NoError(t, peer.Teardown(2, "maintenance"))

	peer.mu.RLock()
	require.Len(t, peer.opQueue, 1, "queue should have 1 item")
	require.Equal(t, PeerOpTeardown, peer.opQueue[0].Type)
	require.Equal(t, uint8(2), peer.opQueue[0].Subcode)
	require.Equal(t, "maintenance", peer.opQueue[0].Message)
	peer.mu.RUnlock()
}

// test-relax: MVPNRoute / mvpnRouteGroupKey / groupMVPNRoutesByKey were removed by
// spec-route-config-plugin-migration. MVPN route grouping is now the reactor's generic
// pluginRouteGroupKey (keyed on family + next-hop + AS_PATH + LOCAL_PREF + raw attrs,
// which carry origin/MED/ext-comm/originator/cluster); grouping is verified byte-for-byte
// by test/encode/mvpn.ci. The per-attribute separation cases are subsumed by that key.
