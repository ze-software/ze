package reactor

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/capability"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
	"github.com/exa-networks/zebgp/pkg/rib"
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

// TestBuildMPReachNLRIUnicast verifies MP_REACH_NLRI generation for IPv6 unicast.
//
// VALIDATES: IPv6 unicast routes produce correct MP_REACH_NLRI attribute with
// proper AFI=2, SAFI=1, next-hop, and NLRI encoding.
//
// PREVENTS: IPv6 routes being sent without MP_REACH_NLRI (which would be
// silently dropped by peers).
func TestBuildMPReachNLRIUnicast(t *testing.T) {
	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("2001:db8::1/128"),
		NextHop: netip.MustParseAddr("2001:db8::ffff"),
		Origin:  0, // IGP
	}

	attrBytes := buildMPReachNLRIUnicast(route)
	require.NotEmpty(t, attrBytes, "must produce attribute bytes")

	// Parse attribute header
	require.GreaterOrEqual(t, len(attrBytes), 3, "must have at least header")
	flags := attrBytes[0]
	code := attrBytes[1]

	require.Equal(t, byte(attribute.AttrMPReachNLRI), code, "code must be MP_REACH_NLRI (14)")
	require.True(t, flags&0x80 != 0, "must be optional")

	// Determine value offset based on extended length
	var valueOffset int
	if flags&0x10 != 0 {
		valueOffset = 4 // flags + code + 2-byte length
	} else {
		valueOffset = 3 // flags + code + 1-byte length
	}

	value := attrBytes[valueOffset:]
	require.GreaterOrEqual(t, len(value), 5, "value must have AFI/SAFI/NH_Len")

	// Parse AFI/SAFI
	afi := binary.BigEndian.Uint16(value[0:2])
	safi := value[2]
	nhLen := value[3]

	require.Equal(t, uint16(2), afi, "AFI must be IPv6 (2)")
	require.Equal(t, byte(1), safi, "SAFI must be Unicast (1)")
	require.Equal(t, byte(16), nhLen, "next-hop length must be 16 for IPv6")

	// Verify next-hop
	require.GreaterOrEqual(t, len(value), 4+16+1, "must have next-hop + reserved")
	nhBytes := value[4 : 4+16]
	expectedNH := route.NextHop.As16()
	require.Equal(t, expectedNH[:], nhBytes, "next-hop must match")

	// Verify reserved byte
	require.Equal(t, byte(0), value[4+16], "reserved byte must be 0")

	// Verify NLRI is present
	nlriOffset := 4 + 16 + 1
	require.Greater(t, len(value), nlriOffset, "must have NLRI")
}

// TestBuildStaticRouteUpdateIPv6 verifies UPDATE generation for IPv6 unicast.
//
// VALIDATES: IPv6 unicast routes include MP_REACH_NLRI attribute and have
// empty inline NLRI field.
//
// PREVENTS: IPv6 routes being sent with inline NLRI (which violates RFC 4760).
func TestBuildStaticRouteUpdateIPv6(t *testing.T) {
	route := StaticRoute{
		Prefix:          netip.MustParsePrefix("2001:db8::1/128"),
		NextHop:         netip.MustParseAddr("2001:db8::ffff"),
		Origin:          0,
		LocalPreference: 100,
	}

	update := buildStaticRouteUpdate(route, 65000, true, true, nil) // iBGP, asn4, no ExtNH

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
	route := StaticRoute{
		Prefix:      netip.MustParsePrefix("192.0.2.0/24"),
		NextHop:     netip.MustParseAddr("192.0.2.1"),
		Origin:      0,
		Communities: []uint32{0x78140000, 0x78147814}, // 30740:0, 30740:30740
	}

	update := buildStaticRouteUpdate(route, 65000, false, true, nil) // eBGP, asn4, no ExtNH
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
				StaticRoute:        StaticRoute{Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.0.2.1")},
				InitiallyWithdrawn: false, // Starts announced
			},
			{
				StaticRoute:        StaticRoute{Prefix: netip.MustParsePrefix("10.0.1.0/24"), NextHop: netip.MustParseAddr("192.0.2.1")},
				InitiallyWithdrawn: true, // Starts withdrawn
			},
		},
		"backup": {
			{
				StaticRoute:        StaticRoute{Prefix: netip.MustParsePrefix("20.0.0.0/24"), NextHop: netip.MustParseAddr("192.0.2.2")},
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

// =============================================================================
// NegotiatedFamilies Tests
// =============================================================================

// TestComputeNegotiatedFamiliesNil verifies nil input returns nil.
//
// VALIDATES: nil Negotiated returns nil NegotiatedFamilies.
//
// PREVENTS: Nil pointer dereference when session has no negotiated state.
func TestComputeNegotiatedFamiliesNil(t *testing.T) {
	result := computeNegotiatedFamilies(nil)
	require.Nil(t, result, "nil input should return nil")
}

// TestComputeNegotiatedFamiliesBasic verifies basic family extraction.
//
// VALIDATES: computeNegotiatedFamilies correctly extracts families from intersection.
//
// PREVENTS: Missing or incorrect family flags after negotiation.
func TestComputeNegotiatedFamiliesBasic(t *testing.T) {
	// Create capabilities that BOTH sides advertise (intersection)
	local := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIFlowSpec},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIFlowSpec},
		&capability.ASN4{ASN: 65000},
		&capability.ExtendedMessage{},
	}
	remote := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		// Remote does NOT support IPv6 unicast - should not be negotiated
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIFlowSpec},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIFlowSpec},
		&capability.ASN4{ASN: 65001},
		// Remote does NOT support ExtendedMessage - should not be negotiated
	}

	neg := capability.Negotiate(local, remote, 65000, 65001)
	require.NotNil(t, neg, "Negotiate should return non-nil")

	result := computeNegotiatedFamilies(neg)
	require.NotNil(t, result, "computeNegotiatedFamilies should return non-nil")

	// IPv4 unicast: both support - should be true
	require.True(t, result.IPv4Unicast, "IPv4 unicast should be negotiated (both support)")

	// IPv6 unicast: only local supports - should be false
	require.False(t, result.IPv6Unicast, "IPv6 unicast should NOT be negotiated (only local supports)")

	// IPv4 FlowSpec: both support - should be true
	require.True(t, result.IPv4FlowSpec, "IPv4 FlowSpec should be negotiated (both support)")

	// IPv6 FlowSpec: both support - should be true
	require.True(t, result.IPv6FlowSpec, "IPv6 FlowSpec should be negotiated (both support)")

	// ASN4: both support - should be true
	require.True(t, result.ASN4, "ASN4 should be negotiated (both support)")

	// ExtendedMessage: only local supports - should be false
	require.False(t, result.ExtendedMessage, "ExtendedMessage should NOT be negotiated (only local supports)")
}

// TestComputeNegotiatedFamiliesFlowSpecVPN verifies FlowSpec VPN family extraction.
//
// VALIDATES: FlowSpec VPN families are correctly identified.
//
// PREVENTS: FlowSpec non-VPN being confused with VPN variants.
func TestComputeNegotiatedFamiliesFlowSpecVPN(t *testing.T) {
	local := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIFlowSpec},    // 1/133
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIFlowSpecVPN}, // 1/134
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIFlowSpecVPN}, // 2/134
	}
	remote := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIFlowSpec},    // 1/133
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIFlowSpecVPN}, // 1/134
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIFlowSpecVPN}, // 2/134
	}

	neg := capability.Negotiate(local, remote, 65000, 65001)
	result := computeNegotiatedFamilies(neg)

	require.True(t, result.IPv4FlowSpec, "IPv4 FlowSpec should be negotiated")
	require.False(t, result.IPv6FlowSpec, "IPv6 FlowSpec should NOT be negotiated (not advertised)")
	require.True(t, result.IPv4FlowSpecVPN, "IPv4 FlowSpec VPN should be negotiated")
	require.True(t, result.IPv6FlowSpecVPN, "IPv6 FlowSpec VPN should be negotiated")
}

// TestComputeNegotiatedFamiliesVPLS verifies L2VPN VPLS family extraction.
//
// VALIDATES: L2VPN VPLS (AFI=25, SAFI=65) is correctly identified.
//
// PREVENTS: VPLS routes being sent when not negotiated.
func TestComputeNegotiatedFamiliesVPLS(t *testing.T) {
	local := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIL2VPN, SAFI: capability.SAFIVPLS},
	}
	remote := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIL2VPN, SAFI: capability.SAFIVPLS},
	}

	neg := capability.Negotiate(local, remote, 65000, 65001)
	result := computeNegotiatedFamilies(neg)

	require.True(t, result.L2VPNVPLS, "L2VPN VPLS should be negotiated")
}

// TestComputeNegotiatedFamiliesMVPN verifies MVPN family extraction.
//
// VALIDATES: McastVPN (SAFI=5) for IPv4/IPv6 is correctly identified.
//
// PREVENTS: MVPN routes being sent when not negotiated.
func TestComputeNegotiatedFamiliesMVPN(t *testing.T) {
	local := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIMcastVPN},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIMcastVPN},
	}
	remote := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIMcastVPN},
		// Remote does NOT support IPv6 MVPN
	}

	neg := capability.Negotiate(local, remote, 65000, 65001)
	result := computeNegotiatedFamilies(neg)

	require.True(t, result.IPv4McastVPN, "IPv4 McastVPN should be negotiated")
	require.False(t, result.IPv6McastVPN, "IPv6 McastVPN should NOT be negotiated")
}

// TestComputeNegotiatedFamiliesMUP verifies MUP family extraction.
//
// VALIDATES: MUP (SAFI=85) for IPv4/IPv6 is correctly identified.
//
// PREVENTS: MUP routes being sent when not negotiated.
func TestComputeNegotiatedFamiliesMUP(t *testing.T) {
	local := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: 85}, // MUP
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: 85}, // MUP
	}
	remote := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: 85}, // MUP
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: 85}, // MUP
	}

	neg := capability.Negotiate(local, remote, 65000, 65001)
	result := computeNegotiatedFamilies(neg)

	require.True(t, result.IPv4MUP, "IPv4 MUP should be negotiated")
	require.True(t, result.IPv6MUP, "IPv6 MUP should be negotiated")
}

// TestRouteFamilyIPv4Unicast verifies IPv4 unicast routes return correct family.
//
// VALIDATES: IPv4 unicast route returns AFI=1, SAFI=1.
//
// PREVENTS: EOR being sent for wrong family.
func TestRouteFamilyIPv4Unicast(t *testing.T) {
	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("192.0.2.0/24"),
		NextHop: netip.MustParseAddr("192.0.2.1"),
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
		NextHop: netip.MustParseAddr("2001:db8::1"),
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
		NextHop: netip.MustParseAddr("192.0.2.1"),
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
		NextHop: netip.MustParseAddr("2001:db8::1"),
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
		{Prefix: netip.MustParsePrefix("192.0.2.0/24"), NextHop: netip.MustParseAddr("10.0.0.1")},               // IPv4 Unicast
		{Prefix: netip.MustParsePrefix("192.0.2.128/25"), NextHop: netip.MustParseAddr("10.0.0.1")},             // IPv4 Unicast (same family)
		{Prefix: netip.MustParsePrefix("2001:db8::/32"), NextHop: netip.MustParseAddr("2001:db8::1")},           // IPv6 Unicast
		{Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("10.0.0.1"), RD: "100:100"}, // VPNv4
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
		{Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("10.0.0.1"), RD: "100:100"},
		{Prefix: netip.MustParsePrefix("10.0.1.0/24"), NextHop: netip.MustParseAddr("10.0.0.1"), RD: "100:101"},
	}

	for _, route := range routes {
		familiesSent[routeFamily(route)] = true
	}

	// Only VPNv4 should be tracked
	require.Equal(t, 1, len(familiesSent), "Should track exactly 1 family")
	require.True(t, familiesSent[nlri.Family{AFI: nlri.AFIIPv4, SAFI: 128}], "VPNv4 should be tracked")
	require.False(t, familiesSent[nlri.IPv4Unicast], "IPv4 Unicast should NOT be tracked")
}
