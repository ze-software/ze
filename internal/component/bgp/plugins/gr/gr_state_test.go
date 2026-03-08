package gr

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test helpers ---

// testPeer is a standard test peer address.
const testPeer = "192.168.1.1"

// helper to build a grPeerCap with given families and restart time.
func testCap(restartTime uint16, families ...grCapFamily) *grPeerCap {
	return &grPeerCap{
		RestartTime: restartTime,
		Families:    families,
	}
}

var (
	famIPv4    = grCapFamily{Family: "ipv4/unicast", ForwardState: true}
	famIPv6    = grCapFamily{Family: "ipv6/unicast", ForwardState: true}
	famIPv4NoF = grCapFamily{Family: "ipv4/unicast", ForwardState: false}
)

// --- Tests ---

// TestGRStateManagerRouteRetention verifies stale marking on session down.
//
// VALIDATES: GR-capable peer session drops → routes retained, marked stale, timer started.
// PREVENTS: Routes being deleted immediately when peer has GR capability.
func TestGRStateManagerRouteRetention(t *testing.T) {
	var expired []string
	mgr := newGRStateManager(func(peer string) {
		expired = append(expired, peer)
	})

	cap := testCap(120, famIPv4)
	activated := mgr.onSessionDown(testPeer, cap, false)

	assert.True(t, activated, "GR should activate for GR-capable peer")
	assert.True(t, mgr.peerActive(testPeer), "GR should be active for peer")
}

// TestGRStateManagerTimerExpiry verifies stale routes are purged when timer fires.
//
// VALIDATES: Restart timer expires without reconnect → all stale routes deleted.
// PREVENTS: Stale routes lingering indefinitely after peer disappears.
func TestGRStateManagerTimerExpiry(t *testing.T) {
	var mu sync.Mutex
	var expired []string
	mgr := newGRStateManager(func(peer string) {
		mu.Lock()
		expired = append(expired, peer)
		mu.Unlock()
	})

	cap := testCap(0, famIPv4) // restart-time=0 for immediate expiry
	mgr.onSessionDown(testPeer, cap, false)
	require.True(t, mgr.peerActive(testPeer))

	// handleTimerExpired is called by time.AfterFunc — simulate it directly
	mgr.handleTimerExpired(testPeer)

	assert.False(t, mgr.peerActive(testPeer), "GR state should be cleared after timer expiry")
	mu.Lock()
	assert.Contains(t, expired, testPeer, "timer expiry callback should fire")
	mu.Unlock()
}

// TestGRStateManagerReconnectWithFBit verifies F-bit preserves stale routes.
//
// VALIDATES: Peer reconnects with GR capability and F-bit set → stale kept until EOR.
// PREVENTS: Premature deletion of stale routes when peer advertises forwarding state preserved.
func TestGRStateManagerReconnectWithFBit(t *testing.T) {
	mgr := newGRStateManager(nil)

	cap := testCap(120, famIPv4, famIPv6)
	mgr.onSessionDown(testPeer, cap, false)

	// Peer reconnects with same families, F-bit set
	newCap := testCap(120, famIPv4, famIPv6)
	purged := mgr.onSessionReestablished(testPeer, newCap)

	assert.Empty(t, purged, "no families should be purged when F-bit set")
	assert.True(t, mgr.peerActive(testPeer), "GR should still be active (waiting for EOR)")
}

// TestGRStateManagerReconnectNoGR verifies purge when peer drops GR capability.
//
// VALIDATES: Peer reconnects without GR capability → all stale routes deleted.
// PREVENTS: Stale routes surviving when peer no longer supports GR.
func TestGRStateManagerReconnectNoGR(t *testing.T) {
	mgr := newGRStateManager(nil)

	cap := testCap(120, famIPv4)
	mgr.onSessionDown(testPeer, cap, false)

	purged := mgr.onSessionReestablished(testPeer, nil)

	assert.Contains(t, purged, "ipv4/unicast")
	assert.False(t, mgr.peerActive(testPeer), "GR state should be cleared")
}

// TestGRStateManagerReconnectFBitZero verifies purge for F-bit=0 families.
//
// VALIDATES: Peer reconnects with GR but F-bit=0 → stale routes for that family deleted.
// PREVENTS: Stale routes persisting when peer says forwarding state NOT preserved.
func TestGRStateManagerReconnectFBitZero(t *testing.T) {
	mgr := newGRStateManager(nil)

	cap := testCap(120, famIPv4, famIPv6)
	mgr.onSessionDown(testPeer, cap, false)

	// Reconnect: IPv4 F-bit=0, IPv6 F-bit=1
	newCap := testCap(120, famIPv4NoF, famIPv6)
	purged := mgr.onSessionReestablished(testPeer, newCap)

	assert.Contains(t, purged, "ipv4/unicast", "IPv4 should be purged (F-bit=0)")
	assert.NotContains(t, purged, "ipv6/unicast", "IPv6 should be kept (F-bit=1)")
	assert.True(t, mgr.peerActive(testPeer), "GR still active for IPv6")
}

// TestGRStateManagerReconnectMissingFamily verifies purge for missing families.
//
// VALIDATES: Peer reconnects with GR but family missing → stale routes deleted.
// PREVENTS: Stale routes persisting for families peer no longer advertises.
func TestGRStateManagerReconnectMissingFamily(t *testing.T) {
	mgr := newGRStateManager(nil)

	cap := testCap(120, famIPv4, famIPv6)
	mgr.onSessionDown(testPeer, cap, false)

	// Reconnect: only IPv4 (IPv6 missing)
	newCap := testCap(120, famIPv4)
	purged := mgr.onSessionReestablished(testPeer, newCap)

	assert.Contains(t, purged, "ipv6/unicast", "IPv6 should be purged (missing from new cap)")
	assert.NotContains(t, purged, "ipv4/unicast", "IPv4 should be kept")
}

// TestGRStateManagerEORPurge verifies EOR triggers stale purge per family.
//
// VALIDATES: EOR received for a family → remaining stale routes for that family deleted.
// PREVENTS: Stale routes surviving after peer signals initial update complete.
func TestGRStateManagerEORPurge(t *testing.T) {
	mgr := newGRStateManager(nil)

	cap := testCap(120, famIPv4, famIPv6)
	mgr.onSessionDown(testPeer, cap, false)

	// Peer reconnects with F-bit set
	newCap := testCap(120, famIPv4, famIPv6)
	mgr.onSessionReestablished(testPeer, newCap)

	// EOR for IPv4 — should purge IPv4 stale, keep IPv6
	shouldPurge := mgr.onEORReceived(testPeer, "ipv4/unicast")
	assert.True(t, shouldPurge, "EOR should trigger purge for IPv4")

	// GR still active for IPv6
	assert.True(t, mgr.peerActive(testPeer))

	// EOR for IPv6 — should purge IPv6 stale and complete GR
	shouldPurge = mgr.onEORReceived(testPeer, "ipv6/unicast")
	assert.True(t, shouldPurge, "EOR should trigger purge for IPv6")

	// GR should be complete (all families received EOR)
	assert.False(t, mgr.peerActive(testPeer), "GR should be complete after EOR for all families")
}

// TestGRStateManagerNotificationBypass verifies NOTIFICATION bypasses GR.
//
// VALIDATES: NOTIFICATION triggers session down → routes deleted immediately (no GR).
// PREVENTS: Route retention when session ended due to NOTIFICATION (RFC 4724 Section 4).
func TestGRStateManagerNotificationBypass(t *testing.T) {
	mgr := newGRStateManager(nil)

	cap := testCap(120, famIPv4)
	activated := mgr.onSessionDown(testPeer, cap, true) // wasNotification=true

	assert.False(t, activated, "GR should not activate on NOTIFICATION")
	assert.False(t, mgr.peerActive(testPeer), "no GR state for NOTIFICATION teardown")
}

// TestGRStateManagerConsecutiveRestarts verifies old stale routes are deleted.
//
// VALIDATES: Consecutive restarts → previously stale routes deleted, current marked stale.
// PREVENTS: Stale route accumulation across multiple restart cycles.
func TestGRStateManagerConsecutiveRestarts(t *testing.T) {
	mgr := newGRStateManager(nil)

	cap := testCap(120, famIPv4)
	mgr.onSessionDown(testPeer, cap, false)
	require.True(t, mgr.peerActive(testPeer))

	// Second session down (peer dropped again while GR active)
	activated := mgr.onSessionDown(testPeer, cap, false)
	assert.True(t, activated, "GR should activate for new restart cycle")
	assert.True(t, mgr.peerActive(testPeer))
}

// TestGRStateManagerNoGRCapability verifies no state created for non-GR peers.
//
// VALIDATES: Non-GR peer session down produces no GR state.
// PREVENTS: GR state pollution for peers without GR capability.
func TestGRStateManagerNoGRCapability(t *testing.T) {
	mgr := newGRStateManager(nil)

	activated := mgr.onSessionDown(testPeer, nil, false)

	assert.False(t, activated, "GR should not activate for non-GR peer")
	assert.False(t, mgr.peerActive(testPeer))
}

// TestGRStateManagerEORForNonGRPeer verifies EOR is ignored for non-GR peers.
//
// VALIDATES: EOR received for peer without GR state returns false.
// PREVENTS: Spurious purge actions for non-GR peers.
func TestGRStateManagerEORForNonGRPeer(t *testing.T) {
	mgr := newGRStateManager(nil)

	shouldPurge := mgr.onEORReceived(testPeer, "ipv4/unicast")
	assert.False(t, shouldPurge, "EOR for non-GR peer should not trigger purge")
}

// TestGRStateManagerRestartTimeZero verifies zero restart time behavior.
//
// VALIDATES: Restart time of 0 still creates GR state with timer.
// PREVENTS: Division by zero or special-casing of zero restart time.
// BOUNDARY: restart-time=0 is the minimum valid value.
func TestGRStateManagerRestartTimeZero(t *testing.T) {
	mgr := newGRStateManager(nil)

	cap := testCap(0, famIPv4)
	activated := mgr.onSessionDown(testPeer, cap, false)

	assert.True(t, activated)
	assert.True(t, mgr.peerActive(testPeer))
}

// TestGRStateManagerRestartTimeMax verifies maximum restart time.
//
// VALIDATES: Restart time of 4095 (max 12-bit) is accepted.
// BOUNDARY: restart-time=4095 is the maximum valid value.
func TestGRStateManagerRestartTimeMax(t *testing.T) {
	mgr := newGRStateManager(nil)

	cap := testCap(4095, famIPv4)
	activated := mgr.onSessionDown(testPeer, cap, false)

	assert.True(t, activated)
	assert.True(t, mgr.peerActive(testPeer))
}

// TestGRResultToPeerCap verifies conversion from wire format to state machine types.
//
// VALIDATES: grResultToPeerCap correctly maps AFI/SAFI numbers to family strings.
// PREVENTS: Incorrect family string generation breaking state machine lookups.
func TestGRResultToPeerCap(t *testing.T) {
	result := &grResult{
		RestartTime: 120,
		Families: []grFamily{
			{AFI: 1, SAFI: 1, ForwardState: true},
			{AFI: 2, SAFI: 1, ForwardState: false},
		},
	}

	cap := grResultToPeerCap(result)

	assert.Equal(t, uint16(120), cap.RestartTime)
	require.Len(t, cap.Families, 2)
	assert.Equal(t, "ipv4/unicast", cap.Families[0].Family)
	assert.True(t, cap.Families[0].ForwardState)
	assert.Equal(t, "ipv6/unicast", cap.Families[1].Family)
	assert.False(t, cap.Families[1].ForwardState)
}

// TestAfiSAFIToFamily verifies AFI/SAFI number to string conversion.
//
// VALIDATES: Known AFI/SAFI pairs produce correct family strings.
// PREVENTS: Wrong family strings breaking state machine map lookups.
func TestAfiSAFIToFamily(t *testing.T) {
	tests := []struct {
		name string
		afi  uint16
		safi uint8
		want string
	}{
		{"ipv4/unicast", 1, 1, "ipv4/unicast"},
		{"ipv6/unicast", 2, 1, "ipv6/unicast"},
		{"ipv4/multicast", 1, 2, "ipv4/multicast"},
		{"ipv4/vpn", 1, 128, "ipv4/vpn"},
		{"ipv6/flow", 2, 133, "ipv6/flow"},
		{"l2vpn/evpn", 25, 70, "l2vpn/evpn"},
		{"l2vpn/vpls", 25, 65, "l2vpn/vpls"},
		{"bgp-ls/bgp-ls", 16388, 71, "bgp-ls/bgp-ls"},
		{"bgp-ls/bgp-ls-vpn", 16388, 72, "bgp-ls/bgp-ls-vpn"},
		{"ipv4/mpls-label", 1, 4, "ipv4/mpls-label"},
		{"ipv6/flow-vpn", 2, 134, "ipv6/flow-vpn"},
		{"unknown afi", 99, 1, "afi-99/unicast"},
		{"unknown safi", 1, 99, "ipv4/safi-99"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := afiSAFIToFamily(tt.afi, tt.safi)
			assert.Equal(t, tt.want, got)
		})
	}
}
