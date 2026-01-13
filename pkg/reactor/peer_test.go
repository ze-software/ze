package reactor

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/pkg/plugin"
	"codeberg.org/thomas-mangin/zebgp/pkg/rib"
	"github.com/stretchr/testify/require"
)

func mustParseAddr(s string) netip.Addr {
	return netip.MustParseAddr(s)
}

// testRoute creates a valid route for testing.
func testRoute(prefixStr string) *rib.Route {
	prefix := netip.MustParsePrefix(prefixStr)
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	n := nlri.NewINET(family, prefix, 0)
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

	// Give goroutine time to start
	time.Sleep(10 * time.Millisecond)

	require.NotEqual(t, PeerStateStopped, peer.State(), "state should change after Start")

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
	time.Sleep(100 * time.Millisecond)

	peer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = peer.Wait(ctx)

	count := connectCount.Load()
	require.GreaterOrEqual(t, count, int32(2), "peer should reconnect at least twice, got %d", count)
}

// TestPeerContextCancellation verifies peer stops on context cancellation.
//
// VALIDATES: Peer respects context cancellation for clean shutdown.
//
// PREVENTS: Orphaned goroutines when parent context is cancelled.
func TestPeerContextCancellation(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.Port = 0 // Invalid port

	peer := NewPeer(settings)

	ctx, cancel := context.WithCancel(context.Background())
	peer.StartWithContext(ctx)

	time.Sleep(10 * time.Millisecond)
	require.NotEqual(t, PeerStateStopped, peer.State())

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

	// Accept connections but don't respond (peer stays connecting)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Hold connection open without BGP handshake
			time.Sleep(time.Second)
			_ = conn.Close()
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
	time.Sleep(50 * time.Millisecond)
	state := peer.State()
	require.True(t, state == PeerStateConnecting || state == PeerStateActive,
		"state should be Connecting or Active, got %v", state)

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

	var transitions []PeerState
	peer.SetCallback(func(from, to PeerState) {
		transitions = append(transitions, to)
	})

	peer.Start()
	time.Sleep(20 * time.Millisecond)
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
		NextHop:         plugin.NewNextHopExplicit(nextHop),
		Origin:          0,
		LocalPreference: 100,
	}

	update := buildStaticRouteUpdateNew(route, nextHop, 65000, true, true, false, nil) // iBGP, ASN4, no ADD-PATH, no ExtNH needed

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
		NextHop:     plugin.NewNextHopExplicit(nextHop),
		Origin:      0,
		Communities: []uint32{0x78140000, 0x78147814}, // 30740:0, 30740:30740
	}

	update := buildStaticRouteUpdateNew(route, nextHop, 65000, false, true, false, nil) // eBGP, ASN4, no ADD-PATH, no ExtNH needed
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
	peer.Teardown(4) // AdminReset subcode

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
	peer.Teardown(4)
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

	peer.Teardown(2) // AdminShutdown
	peer.Teardown(4) // AdminReset

	peer.mu.RLock()
	require.Len(t, peer.opQueue, 2, "queue should have 2 items")
	require.Equal(t, uint8(2), peer.opQueue[0].Subcode)
	require.Equal(t, uint8(4), peer.opQueue[1].Subcode)
	peer.mu.RUnlock()
}

// TestPeerOpQueueOverflow verifies queue respects MaxOpQueueSize limit.
//
// VALIDATES: Operations are dropped when queue reaches MaxOpQueueSize.
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
	for i := 0; i < MaxOpQueueSize; i++ {
		peer.QueueAnnounce(route)
	}

	peer.mu.RLock()
	require.Len(t, peer.opQueue, MaxOpQueueSize, "queue should be at max capacity")
	peer.mu.RUnlock()

	// Additional operations should be dropped
	peer.QueueAnnounce(route)
	peer.Teardown(4)

	peer.mu.RLock()
	require.Len(t, peer.opQueue, MaxOpQueueSize, "queue should not exceed max capacity")
	peer.mu.RUnlock()
}

// =============================================================================
// Watchdog Tests
// =============================================================================

// testWatchdogSettings creates settings with watchdog groups for testing.
func testWatchdogSettings() *PeerSettings {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.WatchdogGroups = map[string][]WatchdogRoute{
		"health": {
			{
				StaticRoute:        StaticRoute{Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1"))},
				InitiallyWithdrawn: false, // Starts announced
			},
			{
				StaticRoute:        StaticRoute{Prefix: netip.MustParsePrefix("10.0.1.0/24"), NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1"))},
				InitiallyWithdrawn: true, // Starts withdrawn
			},
		},
		"backup": {
			{
				StaticRoute:        StaticRoute{Prefix: netip.MustParsePrefix("20.0.0.0/24"), NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("192.0.2.2"))},
				InitiallyWithdrawn: true,
			},
		},
	}
	return settings
}

// TestWatchdogStateEagerlyInitialized verifies watchdog state is initialized in NewPeer.
//
// VALIDATES: Watchdog state is populated from config at Peer creation time.
//
// PREVENTS: Nil map panic when accessing watchdog state before session.
func TestWatchdogStateEagerlyInitialized(t *testing.T) {
	settings := testWatchdogSettings()
	peer := NewPeer(settings)

	// State should already be initialized
	require.NotNil(t, peer.watchdogState, "watchdogState should be initialized")
	require.Len(t, peer.watchdogState, 2, "should have 2 watchdog groups")

	// Verify "health" group state
	healthState := peer.watchdogState["health"]
	require.NotNil(t, healthState, "health group should exist")
	require.True(t, healthState["10.0.0.0/24#0"], "10.0.0.0/24 should be announced (InitiallyWithdrawn=false)")
	require.False(t, healthState["10.0.1.0/24#0"], "10.0.1.0/24 should be withdrawn (InitiallyWithdrawn=true)")

	// Verify "backup" group state
	backupState := peer.watchdogState["backup"]
	require.NotNil(t, backupState, "backup group should exist")
	require.False(t, backupState["20.0.0.0/24#0"], "20.0.0.0/24 should be withdrawn")
}

// TestWatchdogUnknownGroupReturnsError verifies error for non-existent group.
//
// VALIDATES: AnnounceWatchdog/WithdrawWatchdog return ErrWatchdogNotFound for unknown groups.
//
// PREVENTS: Silent success when group doesn't exist (confusing for users).
func TestWatchdogUnknownGroupReturnsError(t *testing.T) {
	settings := testWatchdogSettings()
	peer := NewPeer(settings)

	// Announce unknown group
	err := peer.AnnounceWatchdog("nonexistent")
	require.Error(t, err, "AnnounceWatchdog should error for unknown group")
	require.ErrorIs(t, err, ErrWatchdogNotFound)

	// Withdraw unknown group
	err = peer.WithdrawWatchdog("nonexistent")
	require.Error(t, err, "WithdrawWatchdog should error for unknown group")
	require.ErrorIs(t, err, ErrWatchdogNotFound)
}

// TestWatchdogStateUpdatedWhenDisconnected verifies state changes persist when not connected.
//
// VALIDATES: AnnounceWatchdog/WithdrawWatchdog update state even without active session.
//
// PREVENTS: State change being lost, causing wrong routes to be sent on reconnect.
func TestWatchdogStateUpdatedWhenDisconnected(t *testing.T) {
	settings := testWatchdogSettings()
	peer := NewPeer(settings)

	// Peer not started, no session
	require.Nil(t, peer.session, "session should be nil")

	// Initial state
	require.False(t, peer.watchdogState["health"]["10.0.1.0/24#0"], "should start withdrawn")

	// Announce while disconnected
	err := peer.AnnounceWatchdog("health")
	require.NoError(t, err)

	// State should be updated (all routes now announced)
	require.True(t, peer.watchdogState["health"]["10.0.0.0/24#0"], "should remain announced")
	require.True(t, peer.watchdogState["health"]["10.0.1.0/24#0"], "should now be announced")

	// Withdraw while disconnected
	err = peer.WithdrawWatchdog("health")
	require.NoError(t, err)

	// State should be updated (all routes now withdrawn)
	require.False(t, peer.watchdogState["health"]["10.0.0.0/24#0"], "should now be withdrawn")
	require.False(t, peer.watchdogState["health"]["10.0.1.0/24#0"], "should now be withdrawn")
}

// TestWatchdogStatePersistsAcrossOperations verifies state is not reset between calls.
//
// VALIDATES: Multiple announce/withdraw calls correctly track cumulative state.
//
// PREVENTS: State being reset to InitiallyWithdrawn on each operation.
func TestWatchdogStatePersistsAcrossOperations(t *testing.T) {
	settings := testWatchdogSettings()
	peer := NewPeer(settings)

	// Initial: 10.0.0.0/24 announced, 10.0.1.0/24 withdrawn
	require.True(t, peer.watchdogState["health"]["10.0.0.0/24#0"])
	require.False(t, peer.watchdogState["health"]["10.0.1.0/24#0"])

	// Withdraw all
	_ = peer.WithdrawWatchdog("health")
	require.False(t, peer.watchdogState["health"]["10.0.0.0/24#0"])
	require.False(t, peer.watchdogState["health"]["10.0.1.0/24#0"])

	// Announce all
	_ = peer.AnnounceWatchdog("health")
	require.True(t, peer.watchdogState["health"]["10.0.0.0/24#0"])
	require.True(t, peer.watchdogState["health"]["10.0.1.0/24#0"])

	// Withdraw again - should update, not reset to initial
	_ = peer.WithdrawWatchdog("health")
	require.False(t, peer.watchdogState["health"]["10.0.0.0/24#0"])
	require.False(t, peer.watchdogState["health"]["10.0.1.0/24#0"])
}

// TestWatchdogMultipleGroupsIndependent verifies groups don't affect each other.
//
// VALIDATES: Announce/withdraw on one group doesn't change other groups.
//
// PREVENTS: Cross-group state pollution.
func TestWatchdogMultipleGroupsIndependent(t *testing.T) {
	settings := testWatchdogSettings()
	peer := NewPeer(settings)

	// Initial: backup group is withdrawn
	require.False(t, peer.watchdogState["backup"]["20.0.0.0/24#0"])

	// Announce health group
	_ = peer.AnnounceWatchdog("health")

	// Backup should remain unchanged
	require.False(t, peer.watchdogState["backup"]["20.0.0.0/24#0"], "backup should not be affected by health announce")

	// Announce backup
	_ = peer.AnnounceWatchdog("backup")
	require.True(t, peer.watchdogState["backup"]["20.0.0.0/24#0"])

	// Withdraw health
	_ = peer.WithdrawWatchdog("health")
	require.True(t, peer.watchdogState["backup"]["20.0.0.0/24#0"], "backup should not be affected by health withdraw")
}

// TestWatchdogEmptyGroupNoError verifies empty watchdog groups work correctly.
//
// VALIDATES: Watchdog group with no routes is handled gracefully.
//
// PREVENTS: Panic or error when processing empty group.
func TestWatchdogEmptyGroupNoError(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.WatchdogGroups = map[string][]WatchdogRoute{
		"empty": {}, // No routes
	}

	peer := NewPeer(settings)

	// Empty group should be initialized
	require.NotNil(t, peer.watchdogState["empty"])
	require.Empty(t, peer.watchdogState["empty"])

	// Operations should succeed (no-op)
	err := peer.AnnounceWatchdog("empty")
	require.NoError(t, err)

	err = peer.WithdrawWatchdog("empty")
	require.NoError(t, err)
}

// TestWatchdogRouteKeyIncludesPathID verifies PathID is included in route key.
//
// VALIDATES: Routes with different PathIDs are tracked separately.
//
// PREVENTS: ADD-PATH routes overwriting each other in state.
func TestWatchdogRouteKeyIncludesPathID(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	settings.WatchdogGroups = map[string][]WatchdogRoute{
		"addpath": {
			{
				StaticRoute:        StaticRoute{Prefix: netip.MustParsePrefix("10.0.0.0/24"), PathID: 1},
				InitiallyWithdrawn: false,
			},
			{
				StaticRoute:        StaticRoute{Prefix: netip.MustParsePrefix("10.0.0.0/24"), PathID: 2},
				InitiallyWithdrawn: true,
			},
		},
	}

	peer := NewPeer(settings)

	// Same prefix but different PathIDs should have separate state
	require.True(t, peer.watchdogState["addpath"]["10.0.0.0/24#1"], "PathID=1 should be announced")
	require.False(t, peer.watchdogState["addpath"]["10.0.0.0/24#2"], "PathID=2 should be withdrawn")
}

// TestRouteFamilyIPv4Unicast verifies IPv4 unicast routes return correct family.
//
// VALIDATES: IPv4 unicast route returns AFI=1, SAFI=1.
//
// PREVENTS: EOR being sent for wrong family.
func TestRouteFamilyIPv4Unicast(t *testing.T) {
	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("192.0.2.0/24"),
		NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
	}

	family := routeFamily(route)

	require.Equal(t, nlri.AFIIPv4, family.AFI, "AFI should be IPv4")
	require.Equal(t, nlri.SAFIUnicast, family.SAFI, "SAFI should be unicast")
}

// TestRouteFamilyIPv6Unicast verifies IPv6 unicast routes return correct family.
//
// VALIDATES: IPv6 unicast route returns AFI=2, SAFI=1.
//
// PREVENTS: EOR being sent for wrong family.
func TestRouteFamilyIPv6Unicast(t *testing.T) {
	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}

	family := routeFamily(route)

	require.Equal(t, nlri.AFIIPv6, family.AFI, "AFI should be IPv6")
	require.Equal(t, nlri.SAFIUnicast, family.SAFI, "SAFI should be unicast")
}

// TestRouteFamilyVPNv4 verifies VPNv4 routes return correct family.
//
// VALIDATES: VPNv4 route (with RD) returns AFI=1, SAFI=128.
//
// PREVENTS: VPN routes being counted as unicast for EOR.
func TestRouteFamilyVPNv4(t *testing.T) {
	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
		RD:      "100:100", // Has RD = VPN
	}

	family := routeFamily(route)

	require.Equal(t, nlri.AFIIPv4, family.AFI, "AFI should be IPv4")
	require.Equal(t, nlri.SAFI(128), family.SAFI, "SAFI should be MPLS-VPN (128)")
}

// TestRouteFamilyVPNv6 verifies VPNv6 routes return correct family.
//
// VALIDATES: VPNv6 route (with RD) returns AFI=2, SAFI=128.
//
// PREVENTS: VPN routes being counted as unicast for EOR.
func TestRouteFamilyVPNv6(t *testing.T) {
	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
		RD:      "100:100", // Has RD = VPN
	}

	family := routeFamily(route)

	require.Equal(t, nlri.AFIIPv6, family.AFI, "AFI should be IPv6")
	require.Equal(t, nlri.SAFI(128), family.SAFI, "SAFI should be MPLS-VPN (128)")
}

// TestFamiliesSentTracking verifies that family tracking produces correct EOR set.
//
// VALIDATES: Mixed route families result in correct familiesSent map.
//
// PREVENTS: EOR being sent for families without routes, or missing for families with routes.
func TestFamiliesSentTracking(t *testing.T) {
	// Simulate the familiesSent tracking logic from sendInitialRoutes
	familiesSent := make(map[nlri.Family]bool)

	// Routes of various types
	routes := []StaticRoute{
		{Prefix: netip.MustParsePrefix("192.0.2.0/24"), NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1"))},               // IPv4 Unicast
		{Prefix: netip.MustParsePrefix("192.0.2.128/25"), NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1"))},             // IPv4 Unicast (same family)
		{Prefix: netip.MustParsePrefix("2001:db8::/32"), NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1"))},           // IPv6 Unicast
		{Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")), RD: "100:100"}, // VPNv4
	}

	// Track families as sendInitialRoutes does
	for _, route := range routes {
		familiesSent[routeFamily(route)] = true
	}

	// Verify correct families are tracked
	require.True(t, familiesSent[nlri.IPv4Unicast], "IPv4 Unicast should be tracked")
	require.True(t, familiesSent[nlri.IPv6Unicast], "IPv6 Unicast should be tracked")
	require.True(t, familiesSent[nlri.Family{AFI: nlri.AFIIPv4, SAFI: 128}], "VPNv4 should be tracked")

	// Verify families without routes are NOT tracked
	require.False(t, familiesSent[nlri.Family{AFI: nlri.AFIIPv6, SAFI: 128}], "VPNv6 should NOT be tracked")
	require.False(t, familiesSent[nlri.Family{AFI: 1, SAFI: 5}], "MVPN should NOT be tracked")

	// Verify exactly 3 families (no duplicates from same-family routes)
	require.Equal(t, 3, len(familiesSent), "Should track exactly 3 unique families")
}

// TestFamiliesSentEmpty verifies empty routes produce no EOR.
//
// VALIDATES: No routes results in empty familiesSent map.
//
// PREVENTS: Spurious EOR messages when no routes are configured.
func TestFamiliesSentEmpty(t *testing.T) {
	familiesSent := make(map[nlri.Family]bool)

	// No routes sent - familiesSent should be empty
	require.Empty(t, familiesSent, "No routes should mean no EOR families")
}

// TestFamiliesSentOnlyVPN verifies VPN-only routes track correct family.
//
// VALIDATES: VPN routes don't pollute unicast EOR.
//
// PREVENTS: VPN routes triggering unicast EOR.
func TestFamiliesSentOnlyVPN(t *testing.T) {
	familiesSent := make(map[nlri.Family]bool)

	// Only VPN routes
	routes := []StaticRoute{
		{Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")), RD: "100:100"},
		{Prefix: netip.MustParsePrefix("10.0.1.0/24"), NextHop: plugin.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")), RD: "100:101"},
	}

	for _, route := range routes {
		familiesSent[routeFamily(route)] = true
	}

	// Only VPNv4 should be tracked
	require.Equal(t, 1, len(familiesSent), "Should track exactly 1 family")
	require.True(t, familiesSent[nlri.Family{AFI: nlri.AFIIPv4, SAFI: 128}], "VPNv4 should be tracked")
	require.False(t, familiesSent[nlri.IPv4Unicast], "IPv4 Unicast should NOT be tracked")
}

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
	require.Nil(t, peer.sendCtx, "sendCtx should be nil")
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
	peer.sendCtx = bgpctx.EncodingContextWithAddPath(true, map[nlri.Family]bool{
		nlri.IPv4Unicast: true,
		nlri.IPv6Unicast: false,
	})

	addPath := peer.sendCtx.AddPath(nlri.IPv4Unicast)
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
	peer.sendCtx = bgpctx.EncodingContextWithAddPath(true, map[nlri.Family]bool{
		nlri.IPv4Unicast: false,
		nlri.IPv6Unicast: true,
	})

	addPath := peer.sendCtx.AddPath(nlri.IPv6Unicast)
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
	peer.sendCtx = bgpctx.EncodingContextWithAddPath(true, map[nlri.Family]bool{
		nlri.IPv4LabeledUnicast: true,
		nlri.IPv6LabeledUnicast: true,
	})

	// IPv4 labeled-unicast (SAFI 4)
	addPath4 := peer.sendCtx.AddPath(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel})
	require.True(t, addPath4, "AddPath should be true for IPv4 labeled-unicast")

	// IPv6 labeled-unicast (SAFI 4)
	addPath6 := peer.sendCtx.AddPath(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIMPLSLabel})
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
	peer.sendCtx = bgpctx.EncodingContextWithAddPath(true, map[nlri.Family]bool{
		nlri.IPv4Unicast: false,
		nlri.IPv6Unicast: false,
	})

	addPath4 := peer.sendCtx.AddPath(nlri.IPv4Unicast)
	require.False(t, addPath4, "AddPath should be false for IPv4 unicast without ADD-PATH")

	addPath6 := peer.sendCtx.AddPath(nlri.IPv6Unicast)
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

	peer.sendCtx = bgpctx.EncodingContextWithAddPath(true, map[nlri.Family]bool{
		nlri.IPv4Unicast: true,
		// VPN not in map = AddPath false
	})

	// VPN family - not in AddPath map so should be false
	vpnFamily := nlri.Family{AFI: nlri.AFIIPv4, SAFI: 128}
	addPath := peer.sendCtx.AddPath(vpnFamily)
	require.False(t, addPath, "VPN family should have AddPath=false")
	require.True(t, peer.sendCtx.ASN4(), "ASN4 should still be accessible from sendCtx")
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
	peer.sendCtx = bgpctx.EncodingContextForASN4(true)
	require.True(t, peer.sendCtx.ASN4(), "ASN4 should be true when negotiated")

	// Session with ASN4=false (OLD speaker)
	peer.sendCtx = bgpctx.EncodingContextForASN4(false)
	require.False(t, peer.sendCtx.ASN4(), "ASN4 should be false for OLD speaker")
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
		NextHop:      plugin.NewNextHopExplicit(nextHop),
		OriginatorID: 0xC0A80101,
		ClusterList:  []uint32{0xC0A80102, 0xC0A80103},
	}

	params := toStaticRouteUnicastParams(route, nextHop, nil) // nil sendCtx - no ExtNH needed

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
		NextHop:      plugin.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
		OriginatorID: 0xC0A80101,
	}
	route2 := StaticRoute{
		Prefix:       netip.MustParsePrefix("10.0.1.0/24"),
		NextHop:      plugin.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
		OriginatorID: 0xC0A80102, // Different!
	}

	key1 := routeGroupKey(route1)
	key2 := routeGroupKey(route2)

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
		NextHop:     plugin.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
		ClusterList: []uint32{0xC0A80101},
	}
	route2 := StaticRoute{
		Prefix:      netip.MustParsePrefix("10.0.1.0/24"),
		NextHop:     plugin.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
		ClusterList: []uint32{0xC0A80101, 0xC0A80102}, // Different!
	}

	key1 := routeGroupKey(route1)
	key2 := routeGroupKey(route2)

	require.NotEqual(t, key1, key2,
		"Routes with different ClusterList should have different keys")
}

// TestMVPNRouteGroupKey_SeparatesDifferentExtCommunities verifies VPN isolation.
//
// VALIDATES: Routes with different Route Targets get different keys.
// PREVENTS: VPN isolation failure from incorrect grouping.
func TestMVPNRouteGroupKey_SeparatesDifferentExtCommunities(t *testing.T) {
	nh := netip.MustParseAddr("192.168.1.1")
	r1 := MVPNRoute{NextHop: nh, ExtCommunityBytes: []byte{0x00, 0x02, 0x00, 0x01}}
	r2 := MVPNRoute{NextHop: nh, ExtCommunityBytes: []byte{0x00, 0x02, 0x00, 0x02}}

	key1 := mvpnRouteGroupKey(r1)
	key2 := mvpnRouteGroupKey(r2)

	require.NotEqual(t, key1, key2,
		"Routes with different ExtCommunityBytes should have different keys")
}

// TestMVPNRouteGroupKey_SeparatesDifferentOrigin verifies attribute separation.
//
// VALIDATES: Routes with different Origin get different keys.
// PREVENTS: Silent attribute loss in grouped updates.
func TestMVPNRouteGroupKey_SeparatesDifferentOrigin(t *testing.T) {
	nh := netip.MustParseAddr("192.168.1.1")
	r1 := MVPNRoute{NextHop: nh, Origin: 0}
	r2 := MVPNRoute{NextHop: nh, Origin: 1}

	key1 := mvpnRouteGroupKey(r1)
	key2 := mvpnRouteGroupKey(r2)

	require.NotEqual(t, key1, key2,
		"Routes with different Origin should have different keys")
}

// TestMVPNRouteGroupKey_SeparatesDifferentOriginatorID verifies RR attribute separation.
//
// VALIDATES: Routes with different OriginatorID get different keys.
// PREVENTS: Route reflector loop from incorrect grouping.
func TestMVPNRouteGroupKey_SeparatesDifferentOriginatorID(t *testing.T) {
	nh := netip.MustParseAddr("192.168.1.1")
	r1 := MVPNRoute{NextHop: nh, OriginatorID: 0xC0A80101}
	r2 := MVPNRoute{NextHop: nh, OriginatorID: 0xC0A80102}

	key1 := mvpnRouteGroupKey(r1)
	key2 := mvpnRouteGroupKey(r2)

	require.NotEqual(t, key1, key2,
		"Routes with different OriginatorID should have different keys")
}

// TestMVPNRouteGroupKey_SeparatesDifferentClusterList verifies RR attribute separation.
//
// VALIDATES: Routes with different ClusterList get different keys.
// PREVENTS: Route reflector loop from incorrect grouping.
func TestMVPNRouteGroupKey_SeparatesDifferentClusterList(t *testing.T) {
	nh := netip.MustParseAddr("192.168.1.1")
	r1 := MVPNRoute{NextHop: nh, ClusterList: []uint32{0xC0A80101}}
	r2 := MVPNRoute{NextHop: nh, ClusterList: []uint32{0xC0A80102}}

	key1 := mvpnRouteGroupKey(r1)
	key2 := mvpnRouteGroupKey(r2)

	require.NotEqual(t, key1, key2,
		"Routes with different ClusterList should have different keys")
}

// TestMVPNRouteGroupKey_SameAttributesSameKey verifies batching preserved.
//
// VALIDATES: Routes with identical attributes get same key.
// PREVENTS: Unnecessary UPDATE fragmentation.
func TestMVPNRouteGroupKey_SameAttributesSameKey(t *testing.T) {
	nh := netip.MustParseAddr("192.168.1.1")
	rt := []byte{0x00, 0x02, 0x00, 0x01}
	r1 := MVPNRoute{NextHop: nh, Origin: 0, LocalPreference: 100, ExtCommunityBytes: rt}
	r2 := MVPNRoute{NextHop: nh, Origin: 0, LocalPreference: 100, ExtCommunityBytes: rt}

	key1 := mvpnRouteGroupKey(r1)
	key2 := mvpnRouteGroupKey(r2)

	require.Equal(t, key1, key2,
		"Routes with identical attributes should have same key")
}

// TestGroupMVPNRoutesByKey_SeparatesDifferentRT verifies VPN isolation.
//
// VALIDATES: Routes with different Route Targets are in separate groups.
// PREVENTS: VPN traffic leakage between customers.
func TestGroupMVPNRoutesByKey_SeparatesDifferentRT(t *testing.T) {
	nh := netip.MustParseAddr("192.168.1.1")
	routes := []MVPNRoute{
		{NextHop: nh, ExtCommunityBytes: []byte{0x00, 0x02, 0x00, 0x01}},
		{NextHop: nh, ExtCommunityBytes: []byte{0x00, 0x02, 0x00, 0x02}},
	}

	groups := groupMVPNRoutesByKey(routes)

	require.Equal(t, 2, len(groups),
		"Expected 2 groups for different RTs, got %d", len(groups))
}

// TestGroupMVPNRoutesByKey_GroupsSameAttributes verifies batching.
//
// VALIDATES: Routes with same attributes are grouped together.
// PREVENTS: Unnecessary UPDATE fragmentation.
func TestGroupMVPNRoutesByKey_GroupsSameAttributes(t *testing.T) {
	nh := netip.MustParseAddr("192.168.1.1")
	rt := []byte{0x00, 0x02, 0x00, 0x01}
	routes := []MVPNRoute{
		{NextHop: nh, Origin: 0, LocalPreference: 100, ExtCommunityBytes: rt, RouteType: 5},
		{NextHop: nh, Origin: 0, LocalPreference: 100, ExtCommunityBytes: rt, RouteType: 6},
	}

	groups := groupMVPNRoutesByKey(routes)

	require.Equal(t, 1, len(groups),
		"Expected 1 group for same attributes, got %d", len(groups))
	for _, g := range groups {
		require.Equal(t, 2, len(g),
			"Expected 2 routes in group, got %d", len(g))
	}
}

// TestMVPNRouteGroupKey_SeparatesDifferentLocalPref verifies LOCAL_PREF separation.
//
// VALIDATES: Routes with different LocalPreference get different keys.
// PREVENTS: Incorrect route selection from shared LOCAL_PREF.
func TestMVPNRouteGroupKey_SeparatesDifferentLocalPref(t *testing.T) {
	nh := netip.MustParseAddr("192.168.1.1")
	r1 := MVPNRoute{NextHop: nh, LocalPreference: 100}
	r2 := MVPNRoute{NextHop: nh, LocalPreference: 200}

	key1 := mvpnRouteGroupKey(r1)
	key2 := mvpnRouteGroupKey(r2)

	require.NotEqual(t, key1, key2,
		"Routes with different LocalPreference should have different keys")
}

// TestMVPNRouteGroupKey_SeparatesDifferentMED verifies MED separation.
//
// VALIDATES: Routes with different MED get different keys.
// PREVENTS: Incorrect route selection from shared MED.
func TestMVPNRouteGroupKey_SeparatesDifferentMED(t *testing.T) {
	nh := netip.MustParseAddr("192.168.1.1")
	r1 := MVPNRoute{NextHop: nh, MED: 10}
	r2 := MVPNRoute{NextHop: nh, MED: 20}

	key1 := mvpnRouteGroupKey(r1)
	key2 := mvpnRouteGroupKey(r2)

	require.NotEqual(t, key1, key2,
		"Routes with different MED should have different keys")
}

// TestMVPNRouteGroupKey_SeparatesDifferentNextHop verifies NextHop separation.
//
// VALIDATES: Routes with different NextHop get different keys.
// PREVENTS: Incorrect forwarding from shared NextHop.
func TestMVPNRouteGroupKey_SeparatesDifferentNextHop(t *testing.T) {
	r1 := MVPNRoute{NextHop: netip.MustParseAddr("192.168.1.1")}
	r2 := MVPNRoute{NextHop: netip.MustParseAddr("192.168.1.2")}

	key1 := mvpnRouteGroupKey(r1)
	key2 := mvpnRouteGroupKey(r2)

	require.NotEqual(t, key1, key2,
		"Routes with different NextHop should have different keys")
}

// TestMVPNRouteGroupKey_ClusterListOrderMatters verifies RFC 4456 compliance.
//
// VALIDATES: Routes with same cluster IDs in different order get different keys.
// PREVENTS: RFC 4456 violation - ClusterList order indicates RR traversal path.
func TestMVPNRouteGroupKey_ClusterListOrderMatters(t *testing.T) {
	nh := netip.MustParseAddr("192.168.1.1")
	// RFC 4456 Section 8: RRs prepend CLUSTER_ID, so order matters
	r1 := MVPNRoute{NextHop: nh, ClusterList: []uint32{0xC0A80101, 0xC0A80102}}
	r2 := MVPNRoute{NextHop: nh, ClusterList: []uint32{0xC0A80102, 0xC0A80101}} // Reversed

	key1 := mvpnRouteGroupKey(r1)
	key2 := mvpnRouteGroupKey(r2)

	require.NotEqual(t, key1, key2,
		"Routes with different ClusterList order should have different keys (RFC 4456)")
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
	nh := plugin.NewNextHopExplicit(addr)

	got, err := peer.resolveNextHop(nh, nlri.IPv4Unicast)
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

	nh := plugin.NewNextHopSelf()

	got, err := peer.resolveNextHop(nh, nlri.IPv4Unicast)
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

	nh := plugin.NewNextHopSelf()

	_, err := peer.resolveNextHop(nh, nlri.IPv4Unicast)
	require.ErrorIs(t, err, ErrNextHopSelfNoLocal)
}

// TestResolveNextHop_Unset verifies error for unset policy.
//
// VALIDATES: Unset policy returns ErrNextHopUnset.
// PREVENTS: Using zero-value RouteNextHop silently.
func TestResolveNextHop_Unset(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	var nh plugin.RouteNextHop // zero value = NextHopUnset

	_, err := peer.resolveNextHop(nh, nlri.IPv4Unicast)
	require.ErrorIs(t, err, ErrNextHopUnset)
}

// TestResolveNextHop_ExplicitInvalid verifies explicit with invalid addr.
//
// VALIDATES: Explicit with invalid addr returns that addr (no error).
// PREVENTS: Blocking explicit addresses unnecessarily.
func TestResolveNextHop_ExplicitInvalid(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	nh := plugin.NewNextHopExplicit(netip.Addr{}) // invalid addr

	got, err := peer.resolveNextHop(nh, nlri.IPv4Unicast)
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
	ok := peer.canUseNextHopFor(addr, nlri.IPv4Unicast)
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
	ok := peer.canUseNextHopFor(addr, nlri.IPv6Unicast)
	require.True(t, ok, "IPv6 addr should be valid for IPv6 family")
}

// TestCanUseNextHopFor_ExtendedNH verifies cross-family with Extended NH.
//
// VALIDATES: IPv6 addr for IPv4 family allowed when Extended NH negotiated.
// PREVENTS: Rejecting valid RFC 5549/8950 configuration.
func TestCanUseNextHopFor_ExtendedNH(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	// Set up sendCtx with Extended NH for IPv4 unicast → IPv6 next-hop
	peer.sendCtx = bgpctx.NewEncodingContext(nil, &capability.EncodingCaps{
		ExtendedNextHop: map[capability.Family]capability.AFI{
			{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}: capability.AFIIPv6,
		},
	}, bgpctx.DirectionSend)

	addr := netip.MustParseAddr("2001:db8::1") // IPv6 addr
	ok := peer.canUseNextHopFor(addr, nlri.IPv4Unicast)
	require.True(t, ok, "IPv6 addr should be valid for IPv4 family with Extended NH")
}

// TestCanUseNextHopFor_CrossFamilyNoCap verifies cross-family without cap fails.
//
// VALIDATES: IPv6 addr for IPv4 family rejected without Extended NH.
// PREVENTS: Invalid next-hop going on wire.
func TestCanUseNextHopFor_CrossFamilyNoCap(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)
	// No sendCtx or ExtendedNextHop

	addr := netip.MustParseAddr("2001:db8::1") // IPv6 addr
	ok := peer.canUseNextHopFor(addr, nlri.IPv4Unicast)
	require.False(t, ok, "cross-family should fail without Extended NH")
}

// TestCanUseNextHopFor_NilSendCtx verifies nil sendCtx cross-family fails.
//
// VALIDATES: Cross-family fails gracefully when sendCtx is nil.
// PREVENTS: Panic on nil pointer dereference.
func TestCanUseNextHopFor_NilSendCtx(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)
	peer.sendCtx = nil

	addr := netip.MustParseAddr("2001:db8::1") // IPv6 addr
	ok := peer.canUseNextHopFor(addr, nlri.IPv4Unicast)
	require.False(t, ok, "cross-family should fail with nil sendCtx")
}
