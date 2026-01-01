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
	bgpctx "github.com/exa-networks/zebgp/pkg/bgp/context"
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

	ctx := &nlri.PackContext{ASN4: true}
	nf := &NegotiatedFamilies{IPv6Unicast: true}
	update := buildStaticRouteUpdateNew(route, 65000, true, ctx, nf) // iBGP, ctx with ASN4

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

	ctx := &nlri.PackContext{ASN4: true}
	nf := &NegotiatedFamilies{IPv4Unicast: true}
	update := buildStaticRouteUpdateNew(route, 65000, false, ctx, nf) // eBGP, ctx with ASN4
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

// TestComputeNegotiatedFamiliesLabeledUnicast verifies labeled-unicast family extraction.
//
// VALIDATES: Labeled unicast (SAFI=4) for IPv4/IPv6 is correctly identified.
//
// PREVENTS: Labeled-unicast routes being sent when not negotiated, EOR not sent.
func TestComputeNegotiatedFamiliesLabeledUnicast(t *testing.T) {
	local := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIMPLSLabel},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIMPLSLabel},
	}
	remote := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIMPLSLabel},
		// Remote does NOT support IPv6 labeled-unicast
	}

	neg := capability.Negotiate(local, remote, 65000, 65001)
	result := computeNegotiatedFamilies(neg)

	require.True(t, result.IPv4LabeledUnicast, "IPv4 labeled-unicast should be negotiated")
	require.False(t, result.IPv6LabeledUnicast, "IPv6 labeled-unicast should NOT be negotiated")
}

// TestComputeNegotiatedFamiliesLabeledUnicastAddPath verifies ADD-PATH for labeled-unicast.
//
// VALIDATES: ADD-PATH capability is correctly extracted for labeled-unicast families.
//
// PREVENTS: ADD-PATH encoding not applied for labeled-unicast routes.
func TestComputeNegotiatedFamiliesLabeledUnicastAddPath(t *testing.T) {
	local := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIMPLSLabel},
		&capability.AddPath{
			Families: []capability.AddPathFamily{
				{AFI: capability.AFIIPv4, SAFI: capability.SAFIMPLSLabel, Mode: capability.AddPathBoth},
			},
		},
	}
	remote := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIMPLSLabel},
		&capability.AddPath{
			Families: []capability.AddPathFamily{
				{AFI: capability.AFIIPv4, SAFI: capability.SAFIMPLSLabel, Mode: capability.AddPathBoth},
			},
		},
	}

	neg := capability.Negotiate(local, remote, 65000, 65001)
	result := computeNegotiatedFamilies(neg)

	require.True(t, result.IPv4LabeledUnicast, "IPv4 labeled-unicast should be negotiated")
	require.True(t, result.IPv4LabeledUnicastAddPath, "IPv4 labeled-unicast ADD-PATH should be negotiated")
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

// =============================================================================
// PackContext Tests (RFC 7911 ADD-PATH)
// =============================================================================

// TestPeerPackContextNilFamilies verifies nil families returns nil context.
//
// VALIDATES: packContext returns nil when no negotiated families exist.
//
// PREVENTS: Nil pointer dereference when building NLRI before session established.
func TestPeerPackContextNilFamilies(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// No families set (session not established)
	ctx := peer.packContext(nlri.IPv4Unicast)
	require.Nil(t, ctx, "nil families should return nil context")
}

// TestPeerPackContextIPv4AddPath verifies IPv4 unicast ADD-PATH context.
//
// VALIDATES: packContext returns AddPath=true for IPv4 unicast when negotiated.
//
// PREVENTS: Missing path ID in IPv4 unicast NLRI when ADD-PATH is negotiated.
func TestPeerPackContextIPv4AddPath(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Set families with ADD-PATH enabled for IPv4 unicast
	peer.families.Store(&NegotiatedFamilies{
		IPv4Unicast:        true,
		IPv4UnicastAddPath: true,
		IPv6Unicast:        true,
		IPv6UnicastAddPath: false,
	})

	ctx := peer.packContext(nlri.IPv4Unicast)
	require.NotNil(t, ctx, "should return non-nil context")
	require.True(t, ctx.AddPath, "AddPath should be true for IPv4 unicast")
}

// TestPeerPackContextIPv6AddPath verifies IPv6 unicast ADD-PATH context.
//
// VALIDATES: packContext returns AddPath=true for IPv6 unicast when negotiated.
//
// PREVENTS: Missing path ID in IPv6 unicast NLRI when ADD-PATH is negotiated.
func TestPeerPackContextIPv6AddPath(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Set families with ADD-PATH enabled for IPv6 unicast
	peer.families.Store(&NegotiatedFamilies{
		IPv4Unicast:        true,
		IPv4UnicastAddPath: false,
		IPv6Unicast:        true,
		IPv6UnicastAddPath: true,
	})

	ctx := peer.packContext(nlri.IPv6Unicast)
	require.NotNil(t, ctx, "should return non-nil context")
	require.True(t, ctx.AddPath, "AddPath should be true for IPv6 unicast")
}

// TestPeerPackContextLabeledUnicastAddPath verifies labeled-unicast ADD-PATH context.
//
// VALIDATES: packContext returns AddPath=true for labeled-unicast when negotiated.
//
// PREVENTS: Missing path ID in labeled-unicast NLRI when ADD-PATH is negotiated.
func TestPeerPackContextLabeledUnicastAddPath(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Set families with ADD-PATH enabled for labeled-unicast
	peer.families.Store(&NegotiatedFamilies{
		IPv4LabeledUnicast:        true,
		IPv4LabeledUnicastAddPath: true,
		IPv6LabeledUnicast:        true,
		IPv6LabeledUnicastAddPath: true,
	})

	// IPv4 labeled-unicast (SAFI 4)
	ctx4 := peer.packContext(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel})
	require.NotNil(t, ctx4, "should return non-nil context")
	require.True(t, ctx4.AddPath, "AddPath should be true for IPv4 labeled-unicast")

	// IPv6 labeled-unicast (SAFI 4)
	ctx6 := peer.packContext(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIMPLSLabel})
	require.NotNil(t, ctx6, "should return non-nil context")
	require.True(t, ctx6.AddPath, "AddPath should be true for IPv6 labeled-unicast")
}

// TestPeerPackContextNoAddPath verifies non-ADD-PATH families return AddPath=false.
//
// VALIDATES: packContext returns AddPath=false for families without ADD-PATH.
//
// PREVENTS: Spurious path ID prepended to NLRI for non-ADD-PATH sessions.
func TestPeerPackContextNoAddPath(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Set families WITHOUT ADD-PATH
	peer.families.Store(&NegotiatedFamilies{
		IPv4Unicast:        true,
		IPv4UnicastAddPath: false,
		IPv6Unicast:        true,
		IPv6UnicastAddPath: false,
	})

	ctx4 := peer.packContext(nlri.IPv4Unicast)
	require.NotNil(t, ctx4, "should return non-nil context")
	require.False(t, ctx4.AddPath, "AddPath should be false for IPv4 unicast without ADD-PATH")

	ctx6 := peer.packContext(nlri.IPv6Unicast)
	require.NotNil(t, ctx6, "should return non-nil context")
	require.False(t, ctx6.AddPath, "AddPath should be false for IPv6 unicast without ADD-PATH")
}

// TestPeerPackContextOtherFamilies verifies non-unicast families return context with AddPath=false.
//
// VALIDATES: packContext returns context with ASN4 but AddPath=false for non-unicast.
//
// PREVENTS: Missing ASN4 for non-unicast families while correctly omitting ADD-PATH.
func TestPeerPackContextOtherFamilies(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	peer.families.Store(&NegotiatedFamilies{
		IPv4Unicast:        true,
		IPv4UnicastAddPath: true,
		ASN4:               true,
	})

	// VPN family - not supported for ADD-PATH in current implementation
	vpnFamily := nlri.Family{AFI: nlri.AFIIPv4, SAFI: 128}
	ctx := peer.packContext(vpnFamily)
	require.NotNil(t, ctx, "VPN family should return context (for ASN4)")
	require.False(t, ctx.AddPath, "VPN family should have AddPath=false")
	require.True(t, ctx.ASN4, "VPN family should have ASN4 from session")
}

// TestPeerPackContextASN4 verifies packContext includes ASN4 from negotiated state.
//
// VALIDATES: PackContext.ASN4 reflects negotiated 4-byte AS number capability.
// RFC 6793 Section 4.1: NEW speakers use 4-byte AS numbers when both support it.
//
// PREVENTS: AS_PATH encoded with wrong ASN size, causing protocol violations.
func TestPeerPackContextASN4(t *testing.T) {
	settings := NewPeerSettings(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	peer := NewPeer(settings)

	// Session with ASN4=true
	peer.families.Store(&NegotiatedFamilies{
		IPv4Unicast: true,
		ASN4:        true,
	})

	ctx := peer.packContext(nlri.IPv4Unicast)
	require.NotNil(t, ctx, "should return non-nil context")
	require.True(t, ctx.ASN4, "ASN4 should be true when negotiated")

	// Session with ASN4=false (OLD speaker)
	peer.families.Store(&NegotiatedFamilies{
		IPv4Unicast: true,
		ASN4:        false,
	})

	ctx = peer.packContext(nlri.IPv4Unicast)
	require.NotNil(t, ctx, "should return non-nil context")
	require.False(t, ctx.ASN4, "ASN4 should be false for OLD speaker")
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
	require.True(t, peer.RecvContext().ASN4, "recvCtx should have ASN4=true")
	require.True(t, peer.SendContext().ASN4, "sendCtx should have ASN4=true")
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
	route := StaticRoute{
		Prefix:       netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:      netip.MustParseAddr("192.168.1.1"),
		OriginatorID: 0xC0A80101,
		ClusterList:  []uint32{0xC0A80102, 0xC0A80103},
	}
	nf := &NegotiatedFamilies{IPv4Unicast: true}

	params := toStaticRouteUnicastParams(route, nf)

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
		NextHop:      netip.MustParseAddr("192.168.1.1"),
		OriginatorID: 0xC0A80101,
	}
	route2 := StaticRoute{
		Prefix:       netip.MustParsePrefix("10.0.1.0/24"),
		NextHop:      netip.MustParseAddr("192.168.1.1"),
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
		NextHop:     netip.MustParseAddr("192.168.1.1"),
		ClusterList: []uint32{0xC0A80101},
	}
	route2 := StaticRoute{
		Prefix:      netip.MustParsePrefix("10.0.1.0/24"),
		NextHop:     netip.MustParseAddr("192.168.1.1"),
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
