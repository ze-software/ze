package rib

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
)

// setupGRTestRIB creates a RIBManager with routes for two peers, each with two families.
// Peer "192.0.2.1" has: ipv4/unicast 10.0.0.0/24, ipv6/unicast 2001:db8::/32.
// Peer "192.0.2.2" has: ipv4/unicast 172.16.0.0/24.
func setupGRTestRIB(t *testing.T) *RIBManager {
	t.Helper()
	r := newTestRIBManager(t)

	ipv4Family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	ipv6Family := nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}

	attrBytes := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop)

	// Peer 1: two families
	peer1RIB := storage.NewPeerRIB("192.0.2.1")
	peer1RIB.Insert(ipv4Family, attrBytes, []byte{24, 10, 0, 0})               // 10.0.0.0/24
	peer1RIB.Insert(ipv6Family, attrBytes, []byte{32, 0x20, 0x01, 0x0d, 0xb8}) // 2001:db8::/32
	r.ribInPool["192.0.2.1"] = peer1RIB

	// Peer 2: one family
	peer2RIB := storage.NewPeerRIB("192.0.2.2")
	peer2RIB.Insert(ipv4Family, attrBytes, []byte{24, 172, 16, 0}) // 172.16.0.0/24
	r.ribInPool["192.0.2.2"] = peer2RIB

	return r
}

// TestRIBMarkStaleCommand verifies that "rib mark-stale" marks all routes for a specific peer as stale.
//
// VALIDATES: AC-1 — mark-stale marks all routes for peer, stores restart time.
// PREVENTS: mark-stale affecting other peers' routes or missing routes in some families.
func TestRIBMarkStaleCommand(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	// Mark peer 1 stale with restart-time=120
	status, data, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	// Parse response — should report how many routes were marked.
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	assert.Equal(t, float64(2), result["marked"], "should mark 2 routes for peer 1")

	// Verify peer 1 routes are stale.
	peer1RIB := r.ribInPool["192.0.2.1"]
	assert.Equal(t, 2, peer1RIB.StaleCount(), "peer 1 should have 2 stale routes")

	// Verify peer 2 routes are NOT stale.
	peer2RIB := r.ribInPool["192.0.2.2"]
	assert.Equal(t, 0, peer2RIB.StaleCount(), "peer 2 should have 0 stale routes")
}

// TestRIBMarkStaleCommandStoresGRState verifies that mark-stale stores per-peer GR metadata.
//
// VALIDATES: AC-10 — mark-stale records StaleAt, RestartTime, ExpiresAt for status display.
// PREVENTS: GR state not being stored, status showing no stale info.
func TestRIBMarkStaleCommandStoresGRState(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	_, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)

	// Verify GR state is stored.
	r.mu.RLock()
	state := r.grState["192.0.2.1"]
	r.mu.RUnlock()
	require.NotNil(t, state, "GR state should be stored for peer")
	assert.Equal(t, uint16(120), state.RestartTime)
	assert.False(t, state.StaleAt.IsZero(), "StaleAt should be set")
	assert.False(t, state.ExpiresAt.IsZero(), "ExpiresAt should be set")
}

// TestRIBMarkStaleCommandNonExistentPeer verifies mark-stale for unknown peer is a no-op.
//
// VALIDATES: mark-stale with non-existent peer returns 0 marked.
// PREVENTS: Panic or error on unknown peer address.
func TestRIBMarkStaleCommandNonExistentPeer(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	status, data, err := r.handleCommand("rib mark-stale", "*", []string{"10.10.10.10", "120"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	assert.Equal(t, float64(0), result["marked"])
}

// TestRIBMarkStaleCommandMissingArgs verifies mark-stale rejects missing arguments.
//
// VALIDATES: mark-stale requires peer and restart-time.
// PREVENTS: Panic on missing args.
func TestRIBMarkStaleCommandMissingArgs(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	// No args at all.
	_, _, err := r.handleCommand("rib mark-stale", "*", nil)
	assert.Error(t, err, "should error with no args")

	// Only peer, no restart time.
	_, _, err = r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1"})
	assert.Error(t, err, "should error with no restart time")
}

// TestRIBMarkStaleCommandExplicitLevel verifies mark-stale with an explicit stale level.
//
// VALIDATES: mark-stale [level] parameter sets StaleLevel on routes.
// PREVENTS: Optional level argument being silently ignored.
func TestRIBMarkStaleCommandExplicitLevel(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	// Mark with explicit level=2 (LLGR-stale / depreference threshold).
	status, data, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120", "2"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	assert.Equal(t, float64(2), result["marked"])

	// Verify routes have StaleLevel=2, not the default 1.
	ipv4Family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	entry, ok := r.ribInPool["192.0.2.1"].Lookup(ipv4Family, []byte{24, 10, 0, 0})
	require.True(t, ok)
	assert.Equal(t, uint8(2), entry.StaleLevel, "route should have StaleLevel=2")
}

// TestRIBMarkStaleCommandRejectsLevelZero verifies mark-stale rejects level=0.
//
// VALIDATES: Level 0 means fresh; mark-stale with level=0 is rejected.
// PREVENTS: Accidental unstaling via mark-stale command.
func TestRIBMarkStaleCommandRejectsLevelZero(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	status, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120", "0"})
	assert.Error(t, err, "level=0 should be rejected")
	assert.Equal(t, statusError, status)
	assert.Contains(t, err.Error(), "stale level must be > 0")
}

// TestRIBMarkStaleCommandInvalidRestartTime verifies mark-stale rejects non-numeric restart-time.
//
// VALIDATES: Invalid restart-time string produces a clear error.
// PREVENTS: Panic or silent default on malformed input.
func TestRIBMarkStaleCommandInvalidRestartTime(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	status, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "abc"})
	assert.Error(t, err, "non-numeric restart-time should be rejected")
	assert.Equal(t, statusError, status)
	assert.Contains(t, err.Error(), "invalid restart-time")
}

// TestRIBMarkStaleCommandInvalidLevel verifies mark-stale rejects non-numeric stale level.
//
// VALIDATES: Invalid stale level string produces a clear error.
// PREVENTS: Panic or silent default on malformed level input.
func TestRIBMarkStaleCommandInvalidLevel(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	// Non-numeric level.
	status, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120", "abc"})
	assert.Error(t, err, "non-numeric level should be rejected")
	assert.Equal(t, statusError, status)
	assert.Contains(t, err.Error(), "invalid stale level")

	// Level exceeding uint8 range.
	status, _, err = r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120", "256"})
	assert.Error(t, err, "level > 255 should be rejected")
	assert.Equal(t, statusError, status)
}

// TestRIBPurgeStaleCommand verifies that "rib purge-stale" deletes only stale routes.
//
// VALIDATES: AC-2 — purge-stale deletes stale routes, keeps fresh ones.
// PREVENTS: purge-stale deleting fresh routes or leaving stale routes.
func TestRIBPurgeStaleCommand(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	ipv4Family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	// Mark all peer 1 routes as stale.
	_, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)

	// Insert a fresh route for peer 1 (new NLRI, should have Stale=false).
	attrBytes := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop)
	r.ribInPool["192.0.2.1"].Insert(ipv4Family, attrBytes, []byte{24, 192, 168, 0}) // 192.168.0.0/24

	// Peer 1 now has 3 routes: 2 stale + 1 fresh.
	assert.Equal(t, 3, r.ribInPool["192.0.2.1"].Len())
	assert.Equal(t, 2, r.ribInPool["192.0.2.1"].StaleCount())

	// Purge stale for peer 1.
	status, data, err := r.handleCommand("rib purge-stale", "*", []string{"192.0.2.1"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	assert.Equal(t, float64(2), result["purged"], "should purge 2 stale routes")

	// Verify: only the fresh route survives.
	assert.Equal(t, 1, r.ribInPool["192.0.2.1"].Len(), "only fresh route should remain")
	assert.Equal(t, 0, r.ribInPool["192.0.2.1"].StaleCount(), "no stale routes should remain")

	// Verify: peer 2 is unaffected.
	assert.Equal(t, 1, r.ribInPool["192.0.2.2"].Len(), "peer 2 should be unaffected")
}

// TestRIBPurgeStaleFamilyCommand verifies per-family purge-stale.
//
// VALIDATES: AC-3 — purge-stale with family only affects that family.
// PREVENTS: Per-family purge deleting routes from other families.
func TestRIBPurgeStaleFamilyCommand(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	// Mark all peer 1 routes as stale (ipv4 and ipv6).
	_, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)
	assert.Equal(t, 2, r.ribInPool["192.0.2.1"].StaleCount())

	// Purge stale only for ipv4/unicast.
	status, data, err := r.handleCommand("rib purge-stale", "*", []string{"192.0.2.1", "ipv4/unicast"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	assert.Equal(t, float64(1), result["purged"], "should purge 1 stale ipv4 route")

	// Verify: ipv6 stale route still exists.
	assert.Equal(t, 1, r.ribInPool["192.0.2.1"].Len(), "ipv6 stale route should remain")
	assert.Equal(t, 1, r.ribInPool["192.0.2.1"].StaleCount(), "1 stale route should remain (ipv6)")
}

// TestRIBPurgeStalePreservesFresh verifies fresh routes survive purge-stale after implicit unstale.
//
// VALIDATES: AC-4, AC-7 — INSERT during GR clears stale; purge-stale keeps fresh routes.
// PREVENTS: Fresh re-announced routes being deleted by purge-stale.
func TestRIBPurgeStalePreservesFresh(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	ipv4Family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	// Mark all peer 1 routes as stale.
	_, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)
	assert.Equal(t, 2, r.ribInPool["192.0.2.1"].StaleCount())

	// Re-announce the IPv4 route with different attributes (implicit unstale via replacement).
	newAttrBytes := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireMED100)
	r.ribInPool["192.0.2.1"].Insert(ipv4Family, newAttrBytes, []byte{24, 10, 0, 0}) // 10.0.0.0/24

	// Now: ipv4 route is fresh (replaced), ipv6 route is still stale.
	assert.Equal(t, 1, r.ribInPool["192.0.2.1"].StaleCount(), "only ipv6 should be stale")

	// Purge stale for peer 1.
	status, data, err := r.handleCommand("rib purge-stale", "*", []string{"192.0.2.1"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	assert.Equal(t, float64(1), result["purged"], "should purge only the stale ipv6 route")

	// Verify: IPv4 route survives (it was refreshed).
	assert.Equal(t, 1, r.ribInPool["192.0.2.1"].Len(), "refreshed ipv4 route should survive")

	// Verify the surviving route is the IPv4 one via lookup.
	_, found := r.ribInPool["192.0.2.1"].Lookup(ipv4Family, []byte{24, 10, 0, 0})
	assert.True(t, found, "refreshed 10.0.0.0/24 should still be in RIB")
}

// TestGRFlowMarkAndPurge tests the full GR flow: mark → insert → purge → verify fresh kept.
//
// VALIDATES: AC-7 — End-to-end GR flow: disconnect → reconnect → fresh UPDATEs → EOR → purge.
// PREVENTS: Regression where purge-stale deletes fresh routes received during GR window.
func TestGRFlowMarkAndPurge(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	ipv4Family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	ipv6Family := nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}

	// Step 1: Peer goes down → mark-stale (simulating bgp-gr 3-step sequence)
	_, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)
	assert.Equal(t, 2, r.ribInPool["192.0.2.1"].StaleCount())

	// Step 2: Peer reconnects, sends fresh UPDATEs for IPv4 (same NLRI, different attrs)
	freshAttr := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref100)
	r.ribInPool["192.0.2.1"].Insert(ipv4Family, freshAttr, []byte{24, 10, 0, 0}) // re-announce 10.0.0.0/24

	// Also sends a brand new route
	r.ribInPool["192.0.2.1"].Insert(ipv4Family, freshAttr, []byte{24, 10, 1, 0}) // new 10.1.0.0/24

	// Step 3: EOR received for ipv4/unicast → purge stale for that family
	_, data, err := r.handleCommand("rib purge-stale", "*", []string{"192.0.2.1", "ipv4/unicast"})
	require.NoError(t, err)
	var purge1 map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &purge1))
	assert.Equal(t, float64(0), purge1["purged"], "no stale ipv4 routes (10.0.0.0/24 was refreshed)")

	// Step 4: EOR received for ipv6/unicast → purge stale for that family
	_, data, err = r.handleCommand("rib purge-stale", "*", []string{"192.0.2.1", "ipv6/unicast"})
	require.NoError(t, err)
	var purge2 map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &purge2))
	assert.Equal(t, float64(1), purge2["purged"], "1 stale ipv6 route purged")

	// Final state: 2 fresh IPv4 routes (re-announced + new), 0 IPv6.
	assert.Equal(t, 2, r.ribInPool["192.0.2.1"].Len(), "2 fresh routes should remain")
	assert.Equal(t, 0, r.ribInPool["192.0.2.1"].StaleCount(), "no stale routes should remain")

	// Verify specific routes.
	_, found := r.ribInPool["192.0.2.1"].Lookup(ipv4Family, []byte{24, 10, 0, 0})
	assert.True(t, found, "re-announced 10.0.0.0/24 should exist")
	_, found = r.ribInPool["192.0.2.1"].Lookup(ipv4Family, []byte{24, 10, 1, 0})
	assert.True(t, found, "new 10.1.0.0/24 should exist")
	_, found = r.ribInPool["192.0.2.1"].Lookup(ipv6Family, []byte{32, 0x20, 0x01, 0x0d, 0xb8})
	assert.False(t, found, "stale 2001:db8::/32 should be purged")
}

// TestGRConsecutiveRestart tests the 3-step sequence for consecutive restarts.
//
// VALIDATES: AC-11 — Consecutive restart: old stale purged, fresh routes re-marked stale.
// PREVENTS: Old stale routes surviving into new GR cycle.
func TestGRConsecutiveRestart(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	ipv4Family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	// First disconnect: mark all routes as stale.
	_, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)
	assert.Equal(t, 2, r.ribInPool["192.0.2.1"].StaleCount())

	// Peer reconnects, re-announces only IPv4 (IPv6 stays stale).
	freshAttr := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref100)
	r.ribInPool["192.0.2.1"].Insert(ipv4Family, freshAttr, []byte{24, 10, 0, 0}) // refreshes 10.0.0.0/24
	assert.Equal(t, 1, r.ribInPool["192.0.2.1"].StaleCount(), "only ipv6 stale after refresh")

	// Second disconnect before EOR! Consecutive restart.
	// Step 1: purge-stale (delete old stale routes from previous cycle)
	_, data, err := r.handleCommand("rib purge-stale", "*", []string{"192.0.2.1"})
	require.NoError(t, err)
	var purgeResult map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &purgeResult))
	assert.Equal(t, float64(1), purgeResult["purged"], "should purge 1 old stale (ipv6)")

	// Step 2: retain-routes (already tested elsewhere)
	// Step 3: mark-stale again (marks the fresh IPv4 route as stale for new cycle)
	_, _, err = r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "90"})
	require.NoError(t, err)

	// Now: only the refreshed IPv4 route exists, and it's marked stale for the new cycle.
	assert.Equal(t, 1, r.ribInPool["192.0.2.1"].Len(), "1 route should remain")
	assert.Equal(t, 1, r.ribInPool["192.0.2.1"].StaleCount(), "1 route should be stale (new cycle)")

	// GR state should reflect the new restart time.
	r.mu.RLock()
	state := r.grState["192.0.2.1"]
	r.mu.RUnlock()
	require.NotNil(t, state)
	assert.Equal(t, uint16(90), state.RestartTime, "restart time should be updated to new value")
}

// TestRIBShowInStaleFlag verifies rib show in includes stale flag on stale routes.
//
// VALIDATES: AC-9 — rib show in with stale routes shows stale flag per route.
// PREVENTS: Operators unable to identify stale routes in show output.
func TestRIBShowInStaleFlag(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	ipv4Family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	// Before marking stale, show received should not have "stale" field.
	_, data, err := r.handleCommand("rib show", "192.0.2.1", []string{"received"})
	require.NoError(t, err)
	var before map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &before))
	adjInRaw, ok := before["adj-rib-in"]
	require.True(t, ok, "response should have adj-rib-in")
	adjIn, ok := adjInRaw.(map[string]any)
	require.True(t, ok, "adj-rib-in should be a map")
	routesRaw, ok := adjIn["192.0.2.1"]
	require.True(t, ok, "should have routes for peer")
	routes, ok := routesRaw.([]any)
	require.True(t, ok, "routes should be an array")
	for _, rt := range routes {
		rtMap, ok := rt.(map[string]any)
		require.True(t, ok)
		_, hasStale := rtMap["stale"]
		assert.False(t, hasStale, "non-stale routes should not have stale field")
	}

	// Mark all peer 1 routes as stale.
	_, _, err = r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)

	// Insert a fresh route so we can verify mixed output.
	attrBytes := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop)
	r.ribInPool["192.0.2.1"].Insert(ipv4Family, attrBytes, []byte{24, 192, 168, 0}) // fresh 192.168.0.0/24

	// Show received should have "stale": true on stale routes, no "stale" on fresh.
	_, data, err = r.handleCommand("rib show", "192.0.2.1", []string{"received"})
	require.NoError(t, err)
	var after map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &after))
	adjInRaw, ok = after["adj-rib-in"]
	require.True(t, ok)
	adjIn, ok = adjInRaw.(map[string]any)
	require.True(t, ok)
	routesRaw, ok = adjIn["192.0.2.1"]
	require.True(t, ok)
	routes, ok = routesRaw.([]any)
	require.True(t, ok)

	staleCount := 0
	freshCount := 0
	for _, rt := range routes {
		rtMap, ok := rt.(map[string]any)
		require.True(t, ok)
		if stale, has := rtMap["stale"]; has {
			if s, ok := stale.(bool); ok && s {
				staleCount++
				// Verify stale-level is also present with numeric value.
				lvl, hasLevel := rtMap["stale-level"]
				assert.True(t, hasLevel, "stale routes should have stale-level field")
				assert.Equal(t, float64(1), lvl, "default stale level should be 1")
				continue
			}
		}
		freshCount++
		_, hasLevel := rtMap["stale-level"]
		assert.False(t, hasLevel, "fresh routes should not have stale-level field")
	}
	assert.Equal(t, 2, staleCount, "should have 2 stale routes")
	assert.Equal(t, 1, freshCount, "should have 1 fresh route")
}

// TestRIBMarkStaleStartsExpiryTimer verifies that mark-stale starts an expiry timer.
//
// VALIDATES: AC-1 — mark-stale starts expiry timer as safety net.
// PREVENTS: Stale routes persisting forever if bgp-gr never sends purge-stale.
func TestRIBMarkStaleStartsExpiryTimer(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	_, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)

	r.mu.RLock()
	state := r.grState["192.0.2.1"]
	r.mu.RUnlock()
	require.NotNil(t, state)
	assert.NotNil(t, state.expiryTimer, "mark-stale should start expiry timer")
}

// TestRIBExpiryTimerAutoExpires verifies timer auto-purges stale routes.
//
// VALIDATES: AC-1 — expiry timer fires and purges stale routes.
// PREVENTS: Stale routes surviving past restart-time if EOR never arrives.
func TestRIBExpiryTimerAutoExpires(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	// Use a very short restart time (1s) so the timer fires quickly.
	// Timer margin is 5s, so total wait is ~6s. To avoid slow test,
	// call autoExpireStale directly instead of waiting for timer.
	_, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "1"})
	require.NoError(t, err)
	assert.Equal(t, 2, r.ribInPool["192.0.2.1"].StaleCount())

	// Simulate timer firing by calling autoExpireStale directly.
	r.mu.RLock()
	state := r.grState["192.0.2.1"]
	r.mu.RUnlock()
	r.autoExpireStale("192.0.2.1", state)

	// All stale routes should be purged.
	assert.Equal(t, 0, r.ribInPool["192.0.2.1"].Len(), "auto-expire should purge all stale routes")

	// GR state should be cleaned up.
	r.mu.RLock()
	_, hasState := r.grState["192.0.2.1"]
	r.mu.RUnlock()
	assert.False(t, hasState, "GR state should be cleared after auto-expire")
}

// TestRIBPurgeStaleStopsTimer verifies purge-stale cancels the expiry timer
// when all stale routes are cleared.
//
// VALIDATES: Expiry timer canceled when stale routes fully purged.
// PREVENTS: Timer firing after stale routes already purged by normal path.
func TestRIBPurgeStaleStopsTimer(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	_, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)

	// Purge all stale routes.
	_, _, err = r.handleCommand("rib purge-stale", "*", []string{"192.0.2.1"})
	require.NoError(t, err)

	// GR state (and timer) should be cleaned up.
	r.mu.RLock()
	_, hasState := r.grState["192.0.2.1"]
	r.mu.RUnlock()
	assert.False(t, hasState, "GR state should be cleared after full purge")
}

// TestRIBConsecutiveRestartResetsTimer verifies consecutive restart resets the timer.
//
// VALIDATES: AC-11 — new mark-stale cancels old timer and starts new one.
// PREVENTS: Old timer firing during new GR cycle with wrong expiry time.
func TestRIBConsecutiveRestartResetsTimer(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	// First mark-stale with 120s.
	_, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)

	r.mu.RLock()
	state1 := r.grState["192.0.2.1"]
	timer1 := state1.expiryTimer
	r.mu.RUnlock()
	require.NotNil(t, timer1)

	// Second mark-stale with 90s (consecutive restart).
	_, _, err = r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "90"})
	require.NoError(t, err)

	r.mu.RLock()
	state2 := r.grState["192.0.2.1"]
	r.mu.RUnlock()
	require.NotNil(t, state2)
	assert.NotNil(t, state2.expiryTimer, "new timer should be set")
	assert.Equal(t, uint16(90), state2.RestartTime, "restart time should be updated")
}

// TestRIBStatusWithStale verifies rib status includes stale route information.
//
// VALIDATES: AC-10 — rib status shows stale count, stale-at, expires-at.
// PREVENTS: Status output missing GR stale information.
func TestRIBStatusWithStale(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	// Mark peer 1 stale.
	_, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)

	// Check status.
	status, data, err := r.handleCommand("rib status", "*", nil)
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))

	// Status should include stale information.
	staleCount, hasStale := result["stale-routes"]
	assert.True(t, hasStale, "status should include stale-routes count")
	assert.Equal(t, float64(2), staleCount, "should show 2 stale routes")

	// AC-10: GR state should include per-peer stale-at and expires-at absolute times.
	grStateRaw, hasGR := result["gr-state"]
	require.True(t, hasGR, "status should include gr-state")
	grState, ok := grStateRaw.(map[string]any)
	require.True(t, ok, "gr-state should be a map")
	peerState, ok := grState["192.0.2.1"].(map[string]any)
	require.True(t, ok, "gr-state should have entry for peer")
	_, hasStaleAt := peerState["stale-at"]
	assert.True(t, hasStaleAt, "peer gr-state should include stale-at")
	_, hasExpiresAt := peerState["expires-at"]
	assert.True(t, hasExpiresAt, "peer gr-state should include expires-at")
	restartTime, hasRT := peerState["restart-time"]
	assert.True(t, hasRT, "peer gr-state should include restart-time")
	assert.Equal(t, float64(120), restartTime, "restart-time should be 120")
}

// --- Generic community command tests ---

// Community wire values used in tests (not LLGR-specific in the RIB).
// testCommunityA is a 4-byte community used in generic tests (same value as LLGR_STALE).
var testCommunityA = []byte{0xFF, 0xFF, 0x00, 0x06}

// testWireCommunityB is a COMMUNITIES attribute containing testCommunityB.
var testWireCommunityB = []byte{0xC0, 0x08, 0x04, 0xFF, 0xFF, 0x00, 0x07}

// TestAttachCommunity verifies rib attach-community attaches a community to stale routes.
//
// VALIDATES: attach-community adds a 4-byte community to stale routes and raises StaleLevel.
// PREVENTS: Routes missing community marker after LLGR entry.
func TestAttachCommunity(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	ipv4Family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	// Mark peer 1 stale
	_, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)

	// Attach community ffff0006 to stale ipv4 routes
	status, data, err := r.handleCommand("rib attach-community", "*", []string{"192.0.2.1", "ipv4/unicast", "ffff0006"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	assert.Equal(t, float64(1), result["attached"], "should attach to 1 stale ipv4 route")

	// Verify community attached and StaleLevel raised
	entry, found := r.ribInPool["192.0.2.1"].Lookup(ipv4Family, []byte{24, 10, 0, 0})
	require.True(t, found)
	assert.True(t, entry.StaleLevel >= storage.DepreferenceThreshold, "StaleLevel should be raised")
	assert.True(t, entry.HasCommunities(), "route should have communities")
	commData, err := pool.Communities.Get(entry.Communities)
	require.NoError(t, err)
	assert.True(t, containsCommunity(commData, testCommunityA), "community should be attached")
}

// TestDeleteWithCommunity verifies rib delete-with-community deletes matching stale routes.
//
// VALIDATES: delete-with-community removes stale routes that contain the specified community.
// PREVENTS: Routes with NO_LLGR persisting into LLGR period.
func TestDeleteWithCommunity(t *testing.T) {
	t.Parallel()
	r := newTestRIBManager(t)

	ipv4Family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	// Insert route WITH testCommunityB
	attrWithComm := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireCommunityB)
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(ipv4Family, attrWithComm, []byte{24, 10, 0, 0})
	r.ribInPool["192.0.2.1"] = peerRIB

	// Mark stale
	_, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)

	// Delete routes with community ffff0007
	status, data, err := r.handleCommand("rib delete-with-community", "*", []string{"192.0.2.1", "ipv4/unicast", "ffff0007"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	assert.Equal(t, float64(1), result["deleted"])

	assert.Equal(t, 0, peerRIB.Len(), "route should be deleted")
}

// TestAttachCommunity_Idempotent verifies calling attach twice doesn't duplicate.
//
// VALIDATES: attachCommunity is idempotent.
// PREVENTS: Community duplicated on repeated attach calls.
func TestAttachCommunity_Idempotent(t *testing.T) {
	t.Parallel()
	r := newTestRIBManager(t)

	ipv4Family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	attrBytes := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop)
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(ipv4Family, attrBytes, []byte{24, 10, 0, 0})
	r.ribInPool["192.0.2.1"] = peerRIB

	nlriBytes := []byte{24, 10, 0, 0}

	// Attach same community twice via ModifyFamilyEntry (attachCommunity mutates entry).
	var ok1, ok2 bool
	peerRIB.ModifyFamilyEntry(ipv4Family, nlriBytes, func(entry *storage.RouteEntry) {
		ok1 = r.attachCommunity(entry, testCommunityA)
	})
	assert.True(t, ok1, "first attach should succeed")

	peerRIB.ModifyFamilyEntry(ipv4Family, nlriBytes, func(entry *storage.RouteEntry) {
		ok2 = r.attachCommunity(entry, testCommunityA)
	})
	assert.True(t, ok2, "second attach should succeed (already present)")

	// Verify only one community (4 bytes, not 8)
	entry, found := peerRIB.Lookup(ipv4Family, nlriBytes)
	require.True(t, found)
	commData, err := pool.Communities.Get(entry.Communities)
	require.NoError(t, err)
	assert.Equal(t, 4, len(commData), "community data should be exactly 4 bytes")
}

// TestAttachCommunity_NoCommunities verifies community created when route has none.
//
// VALIDATES: attachCommunity creates community attribute from scratch.
// PREVENTS: Panic when route has no community attribute.
func TestAttachCommunity_NoCommunities(t *testing.T) {
	t.Parallel()
	r := newTestRIBManager(t)

	ipv4Family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	attrBytes := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop)
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(ipv4Family, attrBytes, []byte{24, 10, 0, 0})
	r.ribInPool["192.0.2.1"] = peerRIB

	// Mark stale + attach community
	_, _, err := r.handleCommand("rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)

	_, data, err := r.handleCommand("rib attach-community", "*", []string{"192.0.2.1", "ipv4/unicast", "ffff0006"})
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	assert.Equal(t, float64(1), result["attached"])

	entry, found := peerRIB.Lookup(ipv4Family, []byte{24, 10, 0, 0})
	require.True(t, found)
	assert.True(t, entry.HasCommunities(), "community should be created")
	commData, err := pool.Communities.Get(entry.Communities)
	require.NoError(t, err)
	assert.Equal(t, 4, len(commData))
	assert.True(t, containsCommunity(commData, testCommunityA))
}

// TestContainsCommunity verifies community scanning helper.
//
// VALIDATES: containsCommunity correctly identifies 4-byte communities in wire data.
// PREVENTS: False positives/negatives in community detection.
func TestContainsCommunity(t *testing.T) {
	t.Parallel()

	commA := []byte{0xFF, 0xFF, 0x00, 0x06}
	commB := []byte{0xFF, 0xFF, 0x00, 0x07}

	tests := []struct {
		name      string
		data      []byte
		community []byte
		want      bool
	}{
		{"empty data", nil, commA, false},
		{"single match", commA, commA, true},
		{"single no match", commB, commA, false},
		{"match at end", append([]byte{0xFD, 0xE8, 0x00, 0x64}, commB...), commB, true},
		{"match at start", append(commA, 0xFD, 0xE8, 0x00, 0x64), commA, true},
		{"non-aligned data", []byte{1, 2, 3}, commA, false},
		{"wrong community size", []byte{0xFF, 0xFF, 0x00, 0x06}, []byte{0xFF, 0xFF}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := containsCommunity(tt.data, tt.community)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestAttachCommunity_MissingArgs verifies error on bad input.
//
// VALIDATES: attach-community rejects missing arguments.
// PREVENTS: Panic or unclear error on malformed input.
func TestAttachCommunity_MissingArgs(t *testing.T) {
	t.Parallel()
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("rib attach-community", "*", nil)
	assert.Equal(t, statusError, status)
	assert.Error(t, err)
}

// TestDeleteWithCommunity_MissingArgs verifies error on bad input.
//
// VALIDATES: delete-with-community rejects missing arguments.
// PREVENTS: Panic or unclear error on malformed input.
func TestDeleteWithCommunity_MissingArgs(t *testing.T) {
	t.Parallel()
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("rib delete-with-community", "*", nil)
	assert.Equal(t, statusError, status)
	assert.Error(t, err)
}
