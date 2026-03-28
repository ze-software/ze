package gr

import (
	"sync"
	"testing"
	"time"

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
	activated := mgr.onSessionDown(testPeer, cap, nil, false)

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
	mgr.onSessionDown(testPeer, cap, nil, false)
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
	mgr.onSessionDown(testPeer, cap, nil, false)

	// Peer reconnects with same families, F-bit set
	newCap := testCap(120, famIPv4, famIPv6)
	purged := mgr.onSessionReestablished(testPeer, newCap, nil)

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
	mgr.onSessionDown(testPeer, cap, nil, false)

	purged := mgr.onSessionReestablished(testPeer, nil, nil)

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
	mgr.onSessionDown(testPeer, cap, nil, false)

	// Reconnect: IPv4 F-bit=0, IPv6 F-bit=1
	newCap := testCap(120, famIPv4NoF, famIPv6)
	purged := mgr.onSessionReestablished(testPeer, newCap, nil)

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
	mgr.onSessionDown(testPeer, cap, nil, false)

	// Reconnect: only IPv4 (IPv6 missing)
	newCap := testCap(120, famIPv4)
	purged := mgr.onSessionReestablished(testPeer, newCap, nil)

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
	mgr.onSessionDown(testPeer, cap, nil, false)

	// Peer reconnects with F-bit set
	newCap := testCap(120, famIPv4, famIPv6)
	mgr.onSessionReestablished(testPeer, newCap, nil)

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
	activated := mgr.onSessionDown(testPeer, cap, nil, true) // wasNotification=true

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
	mgr.onSessionDown(testPeer, cap, nil, false)
	require.True(t, mgr.peerActive(testPeer))

	// Second session down (peer dropped again while GR active)
	activated := mgr.onSessionDown(testPeer, cap, nil, false)
	assert.True(t, activated, "GR should activate for new restart cycle")
	assert.True(t, mgr.peerActive(testPeer))
}

// TestGRStateManagerNoGRCapability verifies no state created for non-GR peers.
//
// VALIDATES: Non-GR peer session down produces no GR state.
// PREVENTS: GR state pollution for peers without GR capability.
func TestGRStateManagerNoGRCapability(t *testing.T) {
	mgr := newGRStateManager(nil)

	activated := mgr.onSessionDown(testPeer, nil, nil, false)

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
	activated := mgr.onSessionDown(testPeer, cap, nil, false)

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
	activated := mgr.onSessionDown(testPeer, cap, nil, false)

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

// --- LLGR state machine tests (RFC 9494) ---

// LLGR test helpers.
var (
	llgrCapIPv4 = &llgrPeerCap{
		Families: []llgrCapFamily{
			{Family: "ipv4/unicast", ForwardState: true, LLST: 3600},
		},
	}
	llgrCapMulti = &llgrPeerCap{
		Families: []llgrCapFamily{
			{Family: "ipv4/unicast", ForwardState: true, LLST: 3600},
			{Family: "ipv6/unicast", ForwardState: true, LLST: 7200},
		},
	}
	llgrCapPartial = &llgrPeerCap{
		Families: []llgrCapFamily{
			{Family: "ipv4/unicast", ForwardState: true, LLST: 3600},
			// ipv6/unicast not in LLGR cap -> will be purged on LLGR entry
		},
	}
)

// safeCollector collects strings from timer goroutines with synchronization.
type safeCollector struct {
	mu    sync.Mutex
	items []string
}

func (c *safeCollector) add(s string) { c.mu.Lock(); c.items = append(c.items, s); c.mu.Unlock() }
func (c *safeCollector) get() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string{}, c.items...)
}

// TestOnTimerExpired_WithLLGR verifies GR timer expiry transitions to LLGR.
//
// VALIDATES: When GR restart-time expires and LLGR is negotiated, state transitions
// to LLGR instead of purging. onLLGREnter callback fires per-family.
// PREVENTS: LLGR routes being purged when GR timer expires.
func TestOnTimerExpired_WithLLGR(t *testing.T) {
	t.Parallel()

	llgrEntries := &safeCollector{}
	expired := &safeCollector{}
	mgr := newGRStateManager(func(peer string) {
		expired.add(peer)
	})
	mgr.onLLGREnter = func(peer, family string, llst uint32) {
		llgrEntries.add(family)
	}

	cap := testCap(1, famIPv4) // restart-time=1s
	mgr.onSessionDown(testPeer, cap, llgrCapIPv4, false)

	// Wait for GR timer to fire and transition to LLGR
	require.Eventually(t, func() bool {
		return len(llgrEntries.get()) > 0
	}, 3*time.Second, 10*time.Millisecond, "should enter LLGR for ipv4/unicast")

	// Should NOT have called onTimerExpired (no purge)
	assert.Empty(t, expired.get(), "GR timer expiry should not purge when LLGR available")
	assert.Equal(t, []string{"ipv4/unicast"}, llgrEntries.get(), "should enter LLGR for ipv4/unicast")
	assert.True(t, mgr.peerActive(testPeer), "peer should still be active in LLGR")
}

// TestOnTimerExpired_WithoutLLGR verifies GR timer expiry purges without LLGR.
//
// VALIDATES: Existing behavior preserved: GR timer expiry without LLGR purges all stale.
// PREVENTS: Regression in GR-only behavior.
func TestOnTimerExpired_WithoutLLGR(t *testing.T) {
	t.Parallel()

	expired := &safeCollector{}
	mgr := newGRStateManager(func(peer string) {
		expired.add(peer)
	})

	cap := testCap(1, famIPv4)
	mgr.onSessionDown(testPeer, cap, nil, false) // no LLGR

	require.Eventually(t, func() bool {
		return len(expired.get()) > 0
	}, 3*time.Second, 10*time.Millisecond, "should purge on GR timer expiry without LLGR")

	assert.Equal(t, []string{testPeer}, expired.get())
	assert.False(t, mgr.peerActive(testPeer), "peer should no longer be active")
}

// TestOnTimerExpired_MixedFamilies verifies mixed LLGR/non-LLGR families on GR expiry.
//
// VALIDATES: Families with LLST>0 enter LLGR; families without LLGR cap are purged.
// PREVENTS: All families entering LLGR when only some have LLST configured.
func TestOnTimerExpired_MixedFamilies(t *testing.T) {
	t.Parallel()

	llgrEntries := &safeCollector{}
	familyExpired := &safeCollector{}
	mgr := newGRStateManager(nil)
	mgr.onLLGREnter = func(peer, family string, llst uint32) {
		llgrEntries.add(family)
	}
	mgr.onLLGRFamilyExpired = func(peer, family string) {
		familyExpired.add(family)
	}

	// GR cap has both families, LLGR cap only has ipv4
	cap := testCap(1, famIPv4, famIPv6)
	mgr.onSessionDown(testPeer, cap, llgrCapPartial, false)

	require.Eventually(t, func() bool {
		return len(llgrEntries.get()) > 0 && len(familyExpired.get()) > 0
	}, 3*time.Second, 10*time.Millisecond, "GR timer should fire and transition to LLGR")

	assert.Equal(t, []string{"ipv4/unicast"}, llgrEntries.get(), "only ipv4 should enter LLGR")
	assert.Contains(t, familyExpired.get(), "ipv6/unicast", "ipv6 should be purged (no LLGR)")
	assert.True(t, mgr.peerActive(testPeer), "peer should still be active for ipv4 LLGR")
}

// TestLLSTTimerExpiry_SingleFamily verifies LLST timer expiry for one family.
//
// VALIDATES: When LLST timer fires, that family's stale routes are purged.
// PREVENTS: All families being purged when only one LLST timer fires.
func TestLLSTTimerExpiry_SingleFamily(t *testing.T) {
	t.Parallel()

	familyExpired := &safeCollector{}
	completed := &safeCollector{}
	mgr := newGRStateManager(nil)
	mgr.onLLGREnter = func(peer, family string, llst uint32) {}
	mgr.onLLGRFamilyExpired = func(peer, family string) {
		familyExpired.add(family)
	}
	mgr.onLLGRComplete = func(peer string) {
		completed.add(peer)
	}

	// Both families, different LLST: ipv4=1s, ipv6=100s
	llgrCap := &llgrPeerCap{
		Families: []llgrCapFamily{
			{Family: "ipv4/unicast", ForwardState: true, LLST: 1},
			{Family: "ipv6/unicast", ForwardState: true, LLST: 100},
		},
	}
	cap := testCap(1, famIPv4, famIPv6)
	mgr.onSessionDown(testPeer, cap, llgrCap, false)

	// Wait for GR timer (1s) + LLST ipv4 timer (1s) to fire
	require.Eventually(t, func() bool {
		return len(familyExpired.get()) > 0
	}, 5*time.Second, 10*time.Millisecond, "ipv4 LLST should have fired")

	assert.Contains(t, familyExpired.get(), "ipv4/unicast", "ipv4 LLST should have fired")
	assert.Empty(t, completed.get(), "should not be complete (ipv6 still active)")
	assert.True(t, mgr.peerActive(testPeer), "peer should still be active for ipv6")
}

// TestLLSTTimerExpiry_LastFamily verifies cleanup when the last LLST timer fires.
//
// VALIDATES: When all LLST timers fire, peer state is cleaned up and onLLGRComplete called.
// PREVENTS: Peer state leaking after all LLGR families expire.
func TestLLSTTimerExpiry_LastFamily(t *testing.T) {
	t.Parallel()

	completed := &safeCollector{}
	mgr := newGRStateManager(nil)
	mgr.onLLGREnter = func(peer, family string, llst uint32) {}
	mgr.onLLGRFamilyExpired = func(peer, family string) {}
	mgr.onLLGRComplete = func(peer string) {
		completed.add(peer)
	}

	llgrCap := &llgrPeerCap{
		Families: []llgrCapFamily{
			{Family: "ipv4/unicast", ForwardState: true, LLST: 1},
		},
	}
	cap := testCap(1, famIPv4)
	mgr.onSessionDown(testPeer, cap, llgrCap, false)

	// Wait for GR (1s) + LLST (1s) to fire
	require.Eventually(t, func() bool {
		return len(completed.get()) > 0
	}, 5*time.Second, 10*time.Millisecond, "onLLGRComplete should fire")

	assert.Equal(t, []string{testPeer}, completed.get(), "onLLGRComplete should fire")
	assert.False(t, mgr.peerActive(testPeer), "peer should no longer be active")
}

// TestOnSessionDown_SkipGR_DirectLLGR verifies restart-time=0 skips GR.
//
// VALIDATES: RFC 9494: restart-time=0 + LLST>0 enters LLGR immediately.
// PREVENTS: Peer being stuck in GR with zero-duration timer.
func TestOnSessionDown_SkipGR_DirectLLGR(t *testing.T) {
	t.Parallel()

	var llgrEntries []string
	mgr := newGRStateManager(nil)
	mgr.onLLGREnter = func(peer, family string, llst uint32) {
		llgrEntries = append(llgrEntries, family)
	}

	cap := testCap(0, famIPv4) // restart-time=0
	activated := mgr.onSessionDown(testPeer, cap, llgrCapIPv4, false)

	assert.True(t, activated, "should activate with restart-time=0 and LLGR")
	assert.Equal(t, []string{"ipv4/unicast"}, llgrEntries, "should enter LLGR immediately")
	assert.True(t, mgr.peerActive(testPeer), "peer should be active in LLGR")
}

// TestOnSessionDown_ZeroGR_ZeroLLST verifies no GR/LLGR with both zero.
//
// VALIDATES: restart-time=0 and LLST=0 means neither GR nor LLGR.
// PREVENTS: Accidentally entering LLGR with zero LLST.
func TestOnSessionDown_ZeroGR_ZeroLLST(t *testing.T) {
	t.Parallel()

	mgr := newGRStateManager(nil)

	llgrCapZero := &llgrPeerCap{
		Families: []llgrCapFamily{
			{Family: "ipv4/unicast", ForwardState: true, LLST: 0},
		},
	}
	cap := testCap(0, famIPv4) // restart-time=0
	activated := mgr.onSessionDown(testPeer, cap, llgrCapZero, false)

	// restart-time=0 triggers immediate LLGR, but LLST=0 means family has no LLGR
	// So the family gets purged and LLGR completes immediately
	assert.True(t, activated, "should activate (GR cap present with families)")
	assert.False(t, mgr.peerActive(testPeer), "peer should not be active (LLST=0)")
}

// TestOnSessionReestablished_DuringLLGR verifies reconnect during LLGR.
//
// VALIDATES: Reconnect during LLGR: stop LLST timers, validate new caps.
// PREVENTS: LLST timers running after session re-established.
func TestOnSessionReestablished_DuringLLGR(t *testing.T) {
	t.Parallel()

	mgr := newGRStateManager(nil)
	mgr.onLLGREnter = func(peer, family string, llst uint32) {}
	mgr.onLLGRFamilyExpired = func(peer, family string) {}

	cap := testCap(0, famIPv4) // skip GR, enter LLGR immediately
	mgr.onSessionDown(testPeer, cap, llgrCapIPv4, false)

	assert.True(t, mgr.peerActive(testPeer), "should be in LLGR")

	// Reconnect with both GR and LLGR caps, F-bit=1
	newCap := testCap(120, famIPv4)
	purged := mgr.onSessionReestablished(testPeer, newCap, llgrCapIPv4)

	assert.Empty(t, purged, "F-bit=1 in both caps, nothing to purge")
}

// TestOnSessionReestablished_DuringLLGR_NoLLGRCap verifies reconnect without LLGR.
//
// VALIDATES: Reconnect during LLGR with no LLGR cap: delete all stale.
// PREVENTS: Stale routes persisting when peer no longer supports LLGR.
func TestOnSessionReestablished_DuringLLGR_NoLLGRCap(t *testing.T) {
	t.Parallel()

	mgr := newGRStateManager(nil)
	mgr.onLLGREnter = func(peer, family string, llst uint32) {}
	mgr.onLLGRFamilyExpired = func(peer, family string) {}

	cap := testCap(0, famIPv4)
	mgr.onSessionDown(testPeer, cap, llgrCapIPv4, false)

	// Reconnect with GR only (no LLGR), F-bit=1
	newCap := testCap(120, famIPv4)
	purged := mgr.onSessionReestablished(testPeer, newCap, nil)

	// GR F-bit=1 for ipv4, so stale routes are kept until EOR
	assert.Empty(t, purged, "GR F-bit=1, stale kept until EOR")
}

// TestOnSessionReestablished_DuringLLGR_NoCaps verifies reconnect with no caps.
//
// VALIDATES: Reconnect during LLGR with neither GR nor LLGR: delete all stale.
// PREVENTS: Stale routes persisting when peer has no GR/LLGR support.
func TestOnSessionReestablished_DuringLLGR_NoCaps(t *testing.T) {
	t.Parallel()

	mgr := newGRStateManager(nil)
	mgr.onLLGREnter = func(peer, family string, llst uint32) {}
	mgr.onLLGRFamilyExpired = func(peer, family string) {}

	cap := testCap(0, famIPv4)
	mgr.onSessionDown(testPeer, cap, llgrCapIPv4, false)

	// Reconnect with no GR cap
	purged := mgr.onSessionReestablished(testPeer, nil, nil)

	assert.Len(t, purged, 1, "all stale should be purged")
	assert.Contains(t, purged, "ipv4/unicast")
	assert.False(t, mgr.peerActive(testPeer), "peer should no longer be active")
}

// TestOnEORReceived_DuringLLGR verifies EOR stops LLST timer.
//
// VALIDATES: EOR during LLGR stops LLST timer for that family and purges stale.
// PREVENTS: LLST timer firing after EOR already received.
func TestOnEORReceived_DuringLLGR(t *testing.T) {
	t.Parallel()

	mgr := newGRStateManager(nil)
	mgr.onLLGREnter = func(peer, family string, llst uint32) {}
	mgr.onLLGRFamilyExpired = func(peer, family string) {}

	cap := testCap(0, famIPv4, famIPv6)
	mgr.onSessionDown(testPeer, cap, llgrCapMulti, false)

	shouldPurge := mgr.onEORReceived(testPeer, "ipv4/unicast")
	assert.True(t, shouldPurge, "should purge stale for EOR family")
	assert.True(t, mgr.peerActive(testPeer), "peer should still be active (ipv6 remaining)")
}

// TestOnEORReceived_DuringLLGR_LastFamily verifies EOR for last family completes LLGR.
//
// VALIDATES: EOR for last LLGR family cleans up all state.
// PREVENTS: Peer state leaking after all EORs received during LLGR.
func TestOnEORReceived_DuringLLGR_LastFamily(t *testing.T) {
	t.Parallel()

	mgr := newGRStateManager(nil)
	mgr.onLLGREnter = func(peer, family string, llst uint32) {}
	mgr.onLLGRFamilyExpired = func(peer, family string) {}

	cap := testCap(0, famIPv4)
	mgr.onSessionDown(testPeer, cap, llgrCapIPv4, false)

	shouldPurge := mgr.onEORReceived(testPeer, "ipv4/unicast")
	assert.True(t, shouldPurge, "should purge stale for EOR family")
	assert.False(t, mgr.peerActive(testPeer), "peer should no longer be active")
}

// TestConsecutiveRestart_DuringLLGR verifies new session drop while in LLGR.
//
// VALIDATES: New session drop during LLGR cleans old LLGR state and starts fresh.
// PREVENTS: LLST timers from old cycle interfering with new GR cycle.
func TestConsecutiveRestart_DuringLLGR(t *testing.T) {
	t.Parallel()

	var llgrCount int
	mgr := newGRStateManager(nil)
	mgr.onLLGREnter = func(peer, family string, llst uint32) {
		llgrCount++
	}
	mgr.onLLGRFamilyExpired = func(peer, family string) {}

	cap := testCap(0, famIPv4)
	mgr.onSessionDown(testPeer, cap, llgrCapIPv4, false)
	assert.Equal(t, 1, llgrCount, "first LLGR entry")

	// New session drop during LLGR (consecutive restart)
	mgr.onSessionDown(testPeer, cap, llgrCapIPv4, false)
	assert.Equal(t, 2, llgrCount, "second LLGR entry after consecutive restart")
	assert.True(t, mgr.peerActive(testPeer), "peer should still be active")
}
