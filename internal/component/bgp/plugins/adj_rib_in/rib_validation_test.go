package adj_rib_in

import (
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/seqmap"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestEnableValidation verifies enable-validation command sets the flag.
//
// VALIDATES: request bgp adj-rib-in enable-validation command sets validationEnabled=true.
// PREVENTS: Validation gate being permanently disabled.
func TestEnableValidation(t *testing.T) {
	r := newTestManager(t)

	assert.False(t, r.validationEnabled, "validation should be disabled by default")

	status, _, err := r.handleCommand("request bgp adj-rib-in enable-validation", nil, "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)
	assert.True(t, r.validationEnabled, "validation should be enabled after command")
}

// TestPendingRouteStorage verifies routes are stored as pending when validation is enabled.
//
// VALIDATES: Route stored as Pending when validationEnabled=true.
// PREVENTS: Routes being installed immediately when validation is active.
func TestPendingRouteStorage(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	event := &bgp.Event{
		Message:       &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 100},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[family.Family]string{family.IPv4Unicast: "180a0000"},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {
				{NextHop: "10.0.0.1", Action: routeaction.Add, NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	// Route should be in pending, NOT in installed ribIn
	r.mu.RLock()
	defer r.mu.RUnlock()

	assert.Empty(t, r.ribIn, "route should not be in installed ribIn when validation enabled")
	require.Equal(t, 1, len(r.pending), "route should be in pending map")

	key := pendingKey(netip.MustParseAddr("10.0.0.1"), routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0))
	pr, ok := r.pending[key]
	require.True(t, ok, "pending route should exist for key %s", key)
	assert.Equal(t, family.IPv4Unicast, pr.route.Family)
	assert.Equal(t, "40010100", pr.route.AttrHex)
	assert.Equal(t, "0a000001", pr.route.NHopHex)
	assert.Equal(t, "180a0000", pr.route.NLRIHex)
	assert.Equal(t, ValidationPending, pr.state)
}

// TestAcceptPendingRoute verifies accept-routes promotes pending to installed.
//
// VALIDATES: Pending route promoted to installed with correct validation state.
// PREVENTS: Routes stuck in pending forever.
func TestAcceptPendingRoute(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	// Add a pending route
	r.mu.Lock()
	key := pendingKey(netip.MustParseAddr("10.0.0.1"), routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0))
	r.pending[key] = &pendingRoute{
		peerAddr:   netip.MustParseAddr("10.0.0.1"),
		family:     family.IPv4Unicast,
		prefix:     "10.0.0.0/24",
		routeKey:   routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0),
		route:      &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now(),
		state:      ValidationPending,
	}
	r.mu.Unlock()

	status, _, err := r.handleCommand("request bgp adj-rib-in accept-routes", commandArgs("10.0.0.1 ipv4/unicast 10.0.0.0/24 0 1"), "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Should be removed from pending
	assert.Empty(t, r.pending, "pending map should be empty after accept")

	// Should be in installed ribIn
	require.Contains(t, r.ribIn, netip.MustParseAddr("10.0.0.1"))
	assert.Equal(t, 1, r.ribIn[netip.MustParseAddr("10.0.0.1")].Len(), "route should be in installed ribIn")

	// Check validation state
	var route *RawRoute
	r.ribIn[netip.MustParseAddr("10.0.0.1")].Range(func(_ compactRouteKey, _ uint64, rt *RawRoute) bool {
		route = rt
		return true
	})
	require.NotNil(t, route)
	assert.Equal(t, ValidationValid, route.ValidationState)
}

// TestRejectPendingRoute verifies reject-routes discards pending route.
//
// VALIDATES: Pending route discarded on reject (not stored).
// PREVENTS: Invalid routes entering the RIB.
func TestRejectPendingRoute(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	// Add a pending route
	r.mu.Lock()
	key := pendingKey(netip.MustParseAddr("10.0.0.1"), routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0))
	r.pending[key] = &pendingRoute{
		peerAddr:   netip.MustParseAddr("10.0.0.1"),
		family:     family.IPv4Unicast,
		prefix:     "10.0.0.0/24",
		routeKey:   routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0),
		route:      &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now(),
		state:      ValidationPending,
	}
	r.mu.Unlock()

	status, _, err := r.handleCommand("request bgp adj-rib-in reject-routes", commandArgs("10.0.0.1 ipv4/unicast 10.0.0.0/24 0"), "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Should be removed from pending
	assert.Empty(t, r.pending, "pending map should be empty after reject")
	// Should NOT be in installed ribIn
	assert.Empty(t, r.ribIn, "rejected route should not be in installed ribIn")
}

// TestAcceptPendingRouteOddPeerKey verifies that a peer key which is not an
// IP address (spaces, quotes, backslashes) is rejected at the accept-routes
// boundary: peer addresses parse once into netip.Addr, and a non-IP
// identifier fails closed with an error naming the offending value.
//
// VALIDATES: accept-routes rejects non-IP peer identifiers with an actionable error.
// PREVENTS: zero-Addr map keys silently aliasing unrelated peers.
func TestAcceptPendingRouteOddPeerKey(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true
	peer := `peer key "quoted"\with spaces`
	validPeer := netip.MustParseAddr("10.0.0.8")
	routeKey := routeKeyFromStrings(family.IPv4Unicast, "203.0.113.0/24", 0)

	r.mu.Lock()
	r.pending[pendingKey(validPeer, routeKey)] = &pendingRoute{
		peerAddr:   validPeer,
		family:     family.IPv4Unicast,
		prefix:     "203.0.113.0/24",
		routeKey:   routeKey,
		route:      &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "18cb0071"},
		receivedAt: time.Now(),
		state:      ValidationPending,
	}
	r.mu.Unlock()

	status, _, err := r.handleCommand("request bgp adj-rib-in accept-routes", []string{peer, "ipv4/unicast", "203.0.113.0/24", "0", "1"}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid peer address")
	assert.Contains(t, err.Error(), strconv.Quote(peer), "error must name the offending value")
	assert.Equal(t, statusError, status)

	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Len(t, r.pending, 1, "rejected command must not mutate pending state")
	assert.Empty(t, r.ribIn, "rejected command must not install routes")
}

// TestRejectPendingRouteOddPeerKey verifies that a peer key which is not an
// IP address (spaces, quotes, backslashes) is rejected at the reject-routes
// boundary: peer addresses parse once into netip.Addr, and a non-IP
// identifier fails closed with an error naming the offending value.
//
// VALIDATES: reject-routes rejects non-IP peer identifiers with an actionable error.
// PREVENTS: zero-Addr map keys silently aliasing unrelated peers.
func TestRejectPendingRouteOddPeerKey(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true
	peer := `peer key "quoted"\with spaces`
	validPeer := netip.MustParseAddr("10.0.0.8")
	routeKey := routeKeyFromStrings(family.IPv4Unicast, "198.51.100.0/24", 0)

	r.mu.Lock()
	r.pending[pendingKey(validPeer, routeKey)] = &pendingRoute{
		peerAddr:   validPeer,
		family:     family.IPv4Unicast,
		prefix:     "198.51.100.0/24",
		routeKey:   routeKey,
		route:      &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "18c63364"},
		receivedAt: time.Now(),
		state:      ValidationPending,
	}
	r.mu.Unlock()

	status, _, err := r.handleCommand("request bgp adj-rib-in reject-routes", []string{peer, "ipv4/unicast", "198.51.100.0/24", "0"}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid peer address")
	assert.Contains(t, err.Error(), strconv.Quote(peer), "error must name the offending value")
	assert.Equal(t, statusError, status)

	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Len(t, r.pending, 1, "rejected command must not mutate pending state")
	assert.Empty(t, r.ribIn, "rejected command must not install routes")
}

// TestPassthroughWithoutValidation verifies routes flow through unchanged without validation.
//
// VALIDATES: Route stored immediately as installed when validationEnabled=false.
// PREVENTS: Validation overhead when no validator is loaded.
func TestPassthroughWithoutValidation(t *testing.T) {
	r := newTestManager(t)

	assert.False(t, r.validationEnabled, "validation should be disabled by default")

	event := &bgp.Event{
		Message:       &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 100},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[family.Family]string{family.IPv4Unicast: "180a0000"},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {
				{NextHop: "10.0.0.1", Action: routeaction.Add, NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Route should be in installed ribIn directly
	require.Contains(t, r.ribIn, netip.MustParseAddr("10.0.0.1"))
	assert.Equal(t, 1, r.ribIn[netip.MustParseAddr("10.0.0.1")].Len())
	// No pending routes
	assert.Empty(t, r.pending)
}

// TestPendingTimeout verifies pending routes are auto-promoted after timeout.
//
// VALIDATES: Pending route promoted to installed after timeout (fail-open).
// PREVENTS: Routes being permanently stuck in pending.
func TestPendingTimeout(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true
	r.validationTimeout = 100 * time.Millisecond // Short timeout for test

	// Add a pending route with old receivedAt
	r.mu.Lock()
	key := pendingKey(netip.MustParseAddr("10.0.0.1"), routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0))
	r.pending[key] = &pendingRoute{
		peerAddr:   netip.MustParseAddr("10.0.0.1"),
		family:     family.IPv4Unicast,
		prefix:     "10.0.0.0/24",
		routeKey:   routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0),
		route:      &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now().Add(-200 * time.Millisecond), // Already expired
		state:      ValidationPending,
	}
	r.mu.Unlock()

	// Run the timeout scanner once
	r.sweepExpiredPending()

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Should be removed from pending
	assert.Empty(t, r.pending, "expired pending route should be promoted")
	// Should be in installed ribIn with NotValidated state
	require.Contains(t, r.ribIn, netip.MustParseAddr("10.0.0.1"))
	assert.Equal(t, 1, r.ribIn[netip.MustParseAddr("10.0.0.1")].Len())

	var route *RawRoute
	r.ribIn[netip.MustParseAddr("10.0.0.1")].Range(func(_ compactRouteKey, _ uint64, rt *RawRoute) bool {
		route = rt
		return true
	})
	require.NotNil(t, route)
	assert.Equal(t, ValidationNotValidated, route.ValidationState, "timeout should set NotValidated state")
}

// TestRevalidateInstalledRoute verifies revalidate returns route data for re-validation.
//
// VALIDATES: Revalidate command returns installed route data.
// PREVENTS: Stale validation state persisting after ROA cache change.
func TestRevalidateInstalledRoute(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	// Pre-populate an installed route
	m := seqmap.New[compactRouteKey, *RawRoute]()
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family:          family.IPv4Unicast,
		AttrHex:         "40010100",
		NHopHex:         "0a000001",
		NLRIHex:         "180a0000",
		ValidationState: ValidationValid,
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m

	status, data, err := r.handleCommand("request bgp adj-rib-in revalidate", commandArgs("ipv4/unicast 10.0.0.0/24"), "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)
	assert.Contains(t, string(mustMarshal(t, data)), "10.0.0.0/24", "revalidate should return route data")
}

// TestAcceptNonExistentRoute verifies early decision buffering.
//
// VALIDATES: accept-routes for non-existent pending route buffers an early decision.
// PREVENTS: Race between RPKI and adj-rib-in event delivery ordering.
func TestAcceptNonExistentRoute(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	status, data, err := r.handleCommand("request bgp adj-rib-in accept-routes", commandArgs("10.0.0.1 ipv4/unicast 10.0.0.0/24 0 1"), "")
	assert.Equal(t, statusDone, status)
	assert.NoError(t, err)
	assert.Contains(t, data, "early")

	rKey := routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0)
	key := pendingKey(netip.MustParseAddr("10.0.0.1"), rKey)
	r.mu.RLock()
	ed, ok := r.earlyDecisions[key]
	r.mu.RUnlock()
	require.True(t, ok, "early decision should be buffered")
	assert.Equal(t, earlyAccept, ed.action)
	assert.Equal(t, ValidationValid, ed.state)
}

// TestRejectAlreadyInstalled verifies early buffering when rejecting a non-pending route.
//
// VALIDATES: reject-routes for non-pending route buffers early decision, installed unchanged.
// PREVENTS: Installed routes being incorrectly removed by late reject.
func TestRejectAlreadyInstalled(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	// Pre-populate installed route (not pending)
	m := seqmap.New[compactRouteKey, *RawRoute]()
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m

	status, _, err := r.handleCommand("request bgp adj-rib-in reject-routes", commandArgs("10.0.0.1 ipv4/unicast 10.0.0.0/24 0"), "")
	assert.Equal(t, statusDone, status)
	assert.NoError(t, err)

	// Installed route should be unchanged
	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Equal(t, 1, r.ribIn[netip.MustParseAddr("10.0.0.1")].Len(), "installed route should not be removed")
}

// TestEarlyDecisionAppliedOnArrival verifies the store-and-reconcile flow:
// RPKI decision arrives before the route, route is promoted immediately on arrival.
//
// VALIDATES: early decision is consumed when the route arrives as pending.
// PREVENTS: Route stuck in pending when RPKI already decided.
func TestEarlyDecisionAppliedOnArrival(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	// Step 1: RPKI sends accept before the route exists.
	status, _, err := r.handleCommand("request bgp adj-rib-in accept-routes", commandArgs("10.0.0.1 ipv4/unicast 192.168.1.0/24 0 1"), "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	// Step 2: Route arrives and goes through the pending path.
	rKey := routeKeyFromStrings(family.IPv4Unicast, "192.168.1.0/24", 0)
	route := &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "18c0a801",
	}
	pr := &pendingRoute{
		peerAddr: netip.MustParseAddr("10.0.0.1"), family: family.IPv4Unicast,
		prefix: "192.168.1.0/24", routeKey: rKey, route: route,
	}
	r.mu.Lock()
	applied := r.applyEarlyDecision(netip.MustParseAddr("10.0.0.1"), rKey, pr)
	r.mu.Unlock()

	assert.True(t, applied, "early decision should have been applied")

	r.mu.RLock()
	_, edExists := r.earlyDecisions[pendingKey(netip.MustParseAddr("10.0.0.1"), rKey)]
	routes, ribExists := r.ribIn[netip.MustParseAddr("10.0.0.1")]
	r.mu.RUnlock()

	assert.False(t, edExists, "early decision should be consumed")
	require.True(t, ribExists, "route should be installed")
	assert.Equal(t, 1, routes.Len())
}

// TestEarlyRejectDropsRoute verifies early reject prevents route installation.
//
// VALIDATES: early reject discards route on arrival instead of leaving it pending.
// PREVENTS: Invalid routes being installed when RPKI rejects before delivery.
func TestEarlyRejectDropsRoute(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	status, _, err := r.handleCommand("request bgp adj-rib-in reject-routes", commandArgs("10.0.0.1 ipv4/unicast 172.16.0.0/24 0"), "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	rKey := routeKeyFromStrings(family.IPv4Unicast, "172.16.0.0/24", 0)
	route := &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "18ac1000",
	}
	pr := &pendingRoute{
		peerAddr: netip.MustParseAddr("10.0.0.1"), family: family.IPv4Unicast,
		prefix: "172.16.0.0/24", routeKey: rKey, route: route,
	}
	r.mu.Lock()
	applied := r.applyEarlyDecision(netip.MustParseAddr("10.0.0.1"), rKey, pr)
	r.mu.Unlock()

	assert.True(t, applied, "early reject should have been applied")

	r.mu.RLock()
	_, ribExists := r.ribIn[netip.MustParseAddr("10.0.0.1")]
	r.mu.RUnlock()
	assert.False(t, ribExists, "rejected route should not be installed")
}

// TestMultiplePendingRoutes verifies independent resolution of multiple pending routes.
//
// VALIDATES: Multiple pending routes resolved independently by accept/reject.
// PREVENTS: Accept/reject affecting wrong pending route.
func TestMultiplePendingRoutes(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	now := time.Now()

	// Add two pending routes
	r.mu.Lock()
	key1 := pendingKey(netip.MustParseAddr("10.0.0.1"), routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0))
	r.pending[key1] = &pendingRoute{
		peerAddr:   netip.MustParseAddr("10.0.0.1"),
		family:     family.IPv4Unicast,
		prefix:     "10.0.0.0/24",
		routeKey:   routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0),
		route:      &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: now,
		state:      ValidationPending,
	}
	key2 := pendingKey(netip.MustParseAddr("10.0.0.1"), routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0))
	r.pending[key2] = &pendingRoute{
		peerAddr:   netip.MustParseAddr("10.0.0.1"),
		family:     family.IPv4Unicast,
		prefix:     "10.0.1.0/24",
		routeKey:   routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0),
		route:      &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0001"},
		receivedAt: now,
		state:      ValidationPending,
	}
	r.mu.Unlock()

	// Accept first route
	status, _, err := r.handleCommand("request bgp adj-rib-in accept-routes", commandArgs("10.0.0.1 ipv4/unicast 10.0.0.0/24 0 1"), "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	// Reject second route
	status, _, err = r.handleCommand("request bgp adj-rib-in reject-routes", commandArgs("10.0.0.1 ipv4/unicast 10.0.1.0/24 0"), "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Pending should be empty
	assert.Empty(t, r.pending)

	// Only accepted route should be installed
	require.Contains(t, r.ribIn, netip.MustParseAddr("10.0.0.1"))
	assert.Equal(t, 1, r.ribIn[netip.MustParseAddr("10.0.0.1")].Len())

	rt, ok := r.ribIn[netip.MustParseAddr("10.0.0.1")].Get(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0))
	require.True(t, ok)
	assert.Equal(t, ValidationValid, rt.ValidationState)
}

// TestValidationStateField verifies validation state is stored on route entry.
//
// VALIDATES: ValidationState field stored on RawRoute.
// PREVENTS: Validation state being lost after accept.
func TestValidationStateField(t *testing.T) {
	tests := []struct {
		name     string
		stateArg string
		want     uint8
	}{
		{"Valid", "1", ValidationValid},
		{"NotFound", "2", ValidationNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestManager(t)
			r.validationEnabled = true

			r.mu.Lock()
			key := pendingKey(netip.MustParseAddr("10.0.0.1"), routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0))
			r.pending[key] = &pendingRoute{
				peerAddr:   netip.MustParseAddr("10.0.0.1"),
				family:     family.IPv4Unicast,
				prefix:     "10.0.0.0/24",
				routeKey:   routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0),
				route:      &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
				receivedAt: time.Now(),
				state:      ValidationPending,
			}
			r.mu.Unlock()

			status, _, err := r.handleCommand("request bgp adj-rib-in accept-routes", commandArgs("10.0.0.1 ipv4/unicast 10.0.0.0/24 0 "+tt.stateArg), "")
			require.NoError(t, err)
			assert.Equal(t, statusDone, status)

			r.mu.RLock()
			defer r.mu.RUnlock()

			rt, ok := r.ribIn[netip.MustParseAddr("10.0.0.1")].Get(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0))
			require.True(t, ok)
			assert.Equal(t, tt.want, rt.ValidationState)
		})
	}
}

// TestAcceptWithAddPathID verifies accept-routes works with non-zero pathID.
//
// VALIDATES: pathID is carried through the command and used in RouteKey lookup.
// PREVENTS: ADD-PATH sessions failing validation because pathID is dropped.
func TestAcceptWithAddPathID(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	r.mu.Lock()
	key := pendingKey(netip.MustParseAddr("10.0.0.1"), routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 42))
	r.pending[key] = &pendingRoute{
		peerAddr:   netip.MustParseAddr("10.0.0.1"),
		family:     family.IPv4Unicast,
		prefix:     "10.0.0.0/24",
		routeKey:   routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 42),
		route:      &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now(),
		state:      ValidationPending,
	}
	r.mu.Unlock()

	status, _, err := r.handleCommand("request bgp adj-rib-in accept-routes", commandArgs("10.0.0.1 ipv4/unicast 10.0.0.0/24 42 1"), "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Empty(t, r.pending)

	require.Contains(t, r.ribIn, netip.MustParseAddr("10.0.0.1"))
	rt, ok := r.ribIn[netip.MustParseAddr("10.0.0.1")].Get(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 42))
	require.True(t, ok, "route should be installed with pathID=42")
	assert.Equal(t, ValidationValid, rt.ValidationState)
}

// TestRejectWithAddPathID verifies reject-routes works with non-zero pathID.
//
// VALIDATES: pathID is used to look up the correct pending route for rejection.
// PREVENTS: Rejecting the wrong route when multiple pathIDs exist.
func TestRejectWithAddPathID(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	r.mu.Lock()
	key := pendingKey(netip.MustParseAddr("10.0.0.1"), routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 7))
	r.pending[key] = &pendingRoute{
		peerAddr:   netip.MustParseAddr("10.0.0.1"),
		family:     family.IPv4Unicast,
		prefix:     "10.0.0.0/24",
		routeKey:   routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 7),
		route:      &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now(),
		state:      ValidationPending,
	}
	r.mu.Unlock()

	status, _, err := r.handleCommand("request bgp adj-rib-in reject-routes", commandArgs("10.0.0.1 ipv4/unicast 10.0.0.0/24 7"), "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Empty(t, r.pending)
}

// TestValidationStateConstants verifies boundary values.
//
// VALIDATES: Validation state constants have correct values.
// PREVENTS: Off-by-one in validation state encoding.
func TestValidationStateConstants(t *testing.T) {
	assert.Equal(t, uint8(0), ValidationNotValidated)
	assert.Equal(t, uint8(1), ValidationValid)
	assert.Equal(t, uint8(2), ValidationNotFound)
	assert.Equal(t, uint8(3), ValidationInvalid)
}

// TestPeerDownClearsPending verifies peer-down clears pending routes for that peer.
//
// VALIDATES: Peer state=down clears both installed and pending routes.
// PREVENTS: Orphaned pending routes after peer disconnect.
func TestPeerDownClearsPending(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	// Add a pending route
	r.mu.Lock()
	key := pendingKey(netip.MustParseAddr("10.0.0.1"), routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0))
	r.pending[key] = &pendingRoute{
		peerAddr:   netip.MustParseAddr("10.0.0.1"),
		family:     family.IPv4Unicast,
		prefix:     "10.0.0.0/24",
		routeKey:   routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0),
		route:      &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now(),
		state:      ValidationPending,
	}
	r.mu.Unlock()

	// Peer goes down
	downEvent := &bgp.Event{
		Type:  "state",
		Peer:  mustMarshal(t, bgp.PeerInfoJSON{Remote: bgp.PeerRemoteInfo{Address: "10.0.0.1", AS: 65001}}),
		State: "down",
	}
	r.handleState(downEvent)

	r.mu.RLock()
	defer r.mu.RUnlock()

	assert.Empty(t, r.pending, "pending routes should be cleared on peer down")
}

// TestParseValidationState verifies all valid and invalid state values.
//
// VALIDATES: parseValidationState accepts "1" and "2", rejects all others.
// PREVENTS: Invalid validation states being accepted.
func TestParseValidationState(t *testing.T) {
	tests := []struct {
		input   string
		want    uint8
		wantErr bool
	}{
		{"1", ValidationValid, false},
		{"2", ValidationNotFound, false},
		{"0", 0, true},
		{"3", 0, true},
		{"4", 0, true},
		{"abc", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run("state_"+tt.input, func(t *testing.T) {
			got, err := parseValidationState(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "should reject state %q", tt.input)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestSweepExpiredMixed verifies sweep promotes only expired routes.
//
// VALIDATES: Expired routes promoted, non-expired routes preserved.
// PREVENTS: Sweep promoting routes that haven't timed out.
func TestSweepExpiredMixed(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true
	r.validationTimeout = 100 * time.Millisecond

	r.mu.Lock()
	// Expired route
	key1 := pendingKey(netip.MustParseAddr("10.0.0.1"), routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0))
	r.pending[key1] = &pendingRoute{
		peerAddr:   netip.MustParseAddr("10.0.0.1"),
		family:     family.IPv4Unicast,
		prefix:     "10.0.0.0/24",
		routeKey:   routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0),
		route:      &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now().Add(-200 * time.Millisecond),
		state:      ValidationPending,
	}
	// Not-yet-expired route
	key2 := pendingKey(netip.MustParseAddr("10.0.0.1"), routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0))
	r.pending[key2] = &pendingRoute{
		peerAddr:   netip.MustParseAddr("10.0.0.1"),
		family:     family.IPv4Unicast,
		prefix:     "10.0.1.0/24",
		routeKey:   routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0),
		route:      &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0001"},
		receivedAt: time.Now().Add(10 * time.Second), // Far in the future
		state:      ValidationPending,
	}
	r.mu.Unlock()

	r.sweepExpiredPending()

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Only expired route should be promoted
	assert.Equal(t, 1, len(r.pending), "non-expired route should remain pending")
	_, stillPending := r.pending[key2]
	assert.True(t, stillPending, "non-expired route key should still be in pending")

	// Expired route should be in installed
	require.Contains(t, r.ribIn, netip.MustParseAddr("10.0.0.1"))
	assert.Equal(t, 1, r.ribIn[netip.MustParseAddr("10.0.0.1")].Len(), "only expired route should be installed")
}

// TestClearPeerPendingPreservesOthers verifies clearing one peer's pending routes
// does not affect another peer's pending routes.
//
// VALIDATES: clearPeerPending only removes routes for the specified peer.
// PREVENTS: Accidentally clearing all pending routes on any peer-down.
func TestClearPeerPendingPreservesOthers(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	r.mu.Lock()
	// Pending route for peer 1
	key1 := pendingKey(netip.MustParseAddr("10.0.0.1"), routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0))
	r.pending[key1] = &pendingRoute{
		peerAddr: netip.MustParseAddr("10.0.0.1"), family: family.IPv4Unicast, prefix: "10.0.0.0/24",
		routeKey: routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0),
		route:    &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		state:    ValidationPending,
	}
	// Pending route for peer 2
	key2 := pendingKey(netip.MustParseAddr("10.0.0.2"), routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0))
	r.pending[key2] = &pendingRoute{
		peerAddr: netip.MustParseAddr("10.0.0.2"), family: family.IPv4Unicast, prefix: "10.0.1.0/24",
		routeKey: routeKeyFromStrings(family.IPv4Unicast, "10.0.1.0/24", 0),
		route:    &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000002", NLRIHex: "180a0001"},
		state:    ValidationPending,
	}
	r.mu.Unlock()

	// Peer 1 goes down
	downEvent := &bgp.Event{
		Type:  "state",
		Peer:  mustMarshal(t, bgp.PeerInfoJSON{Remote: bgp.PeerRemoteInfo{Address: "10.0.0.1", AS: 65001}}),
		State: "down",
	}
	r.handleState(downEvent)

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Peer 2's pending route should be preserved
	assert.Equal(t, 1, len(r.pending), "peer 2 pending route should be preserved")
	_, ok := r.pending[key2]
	assert.True(t, ok, "peer 2 pending route should still exist")
}

// TestWithdrawalRemovesPending verifies withdrawal removes pending route.
//
// VALIDATES: Withdrawal for a pending route removes it from pending.
// PREVENTS: Stale pending routes after withdrawal received.
func TestWithdrawalRemovesPending(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	// Add a pending route
	r.mu.Lock()
	key := pendingKey(netip.MustParseAddr("10.0.0.1"), routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0))
	r.pending[key] = &pendingRoute{
		peerAddr:   netip.MustParseAddr("10.0.0.1"),
		family:     family.IPv4Unicast,
		prefix:     "10.0.0.0/24",
		routeKey:   routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0),
		route:      &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now(),
		state:      ValidationPending,
	}
	r.mu.Unlock()

	// Receive withdrawal
	withdraw := &bgp.Event{
		Message:      &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 101},
		Peer:         testPeerJSON(t),
		RawWithdrawn: map[family.Family]string{family.IPv4Unicast: "180a0000"},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {
				{Action: routeaction.Del, NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleReceived(withdraw)

	r.mu.RLock()
	defer r.mu.RUnlock()

	assert.Empty(t, r.pending, "pending route should be removed on withdrawal")
}
