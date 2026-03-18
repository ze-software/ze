package adj_rib_in

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgp "codeberg.org/thomas-mangin/ze/internal/component/bgp"
	"codeberg.org/thomas-mangin/ze/internal/core/seqmap"
)

// TestEnableValidation verifies enable-validation command sets the flag.
//
// VALIDATES: adj-rib-in enable-validation command sets validationEnabled=true.
// PREVENTS: Validation gate being permanently disabled.
func TestEnableValidation(t *testing.T) {
	r := newTestManager(t)

	assert.False(t, r.validationEnabled, "validation should be disabled by default")

	status, _, err := r.handleCommand("adj-rib-in enable-validation", "")
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
		Message:       &bgp.MessageInfo{Type: "update", ID: 100},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]bgp.FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "10.0.0.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	// Route should be in pending, NOT in installed ribIn
	r.mu.RLock()
	defer r.mu.RUnlock()

	assert.Empty(t, r.ribIn, "route should not be in installed ribIn when validation enabled")
	require.Equal(t, 1, len(r.pending), "route should be in pending map")

	key := pendingKey("10.0.0.1", bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0))
	pr, ok := r.pending[key]
	require.True(t, ok, "pending route should exist for key %s", key)
	assert.Equal(t, "ipv4/unicast", pr.route.Family)
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
	key := pendingKey("10.0.0.1", bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0))
	r.pending[key] = &PendingRoute{
		peerAddr:   "10.0.0.1",
		family:     "ipv4/unicast",
		prefix:     "10.0.0.0/24",
		routeKey:   bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0),
		route:      &RawRoute{Family: "ipv4/unicast", AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now(),
		state:      ValidationPending,
	}
	r.mu.Unlock()

	status, _, err := r.handleCommand("adj-rib-in accept-routes", "10.0.0.1 ipv4/unicast 10.0.0.0/24 1")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Should be removed from pending
	assert.Empty(t, r.pending, "pending map should be empty after accept")

	// Should be in installed ribIn
	require.Contains(t, r.ribIn, "10.0.0.1")
	assert.Equal(t, 1, r.ribIn["10.0.0.1"].Len(), "route should be in installed ribIn")

	// Check validation state
	var route *RawRoute
	r.ribIn["10.0.0.1"].Range(func(_ string, _ uint64, rt *RawRoute) bool {
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
	key := pendingKey("10.0.0.1", bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0))
	r.pending[key] = &PendingRoute{
		peerAddr:   "10.0.0.1",
		family:     "ipv4/unicast",
		prefix:     "10.0.0.0/24",
		routeKey:   bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0),
		route:      &RawRoute{Family: "ipv4/unicast", AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now(),
		state:      ValidationPending,
	}
	r.mu.Unlock()

	status, _, err := r.handleCommand("adj-rib-in reject-routes", "10.0.0.1 ipv4/unicast 10.0.0.0/24")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Should be removed from pending
	assert.Empty(t, r.pending, "pending map should be empty after reject")
	// Should NOT be in installed ribIn
	assert.Empty(t, r.ribIn, "rejected route should not be in installed ribIn")
}

// TestPassthroughWithoutValidation verifies routes flow through unchanged without validation.
//
// VALIDATES: Route stored immediately as installed when validationEnabled=false.
// PREVENTS: Validation overhead when no validator is loaded.
func TestPassthroughWithoutValidation(t *testing.T) {
	r := newTestManager(t)

	assert.False(t, r.validationEnabled, "validation should be disabled by default")

	event := &bgp.Event{
		Message:       &bgp.MessageInfo{Type: "update", ID: 100},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]bgp.FamilyOperation{
			"ipv4/unicast": {
				{NextHop: "10.0.0.1", Action: "add", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}

	r.handleReceived(event)

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Route should be in installed ribIn directly
	require.Contains(t, r.ribIn, "10.0.0.1")
	assert.Equal(t, 1, r.ribIn["10.0.0.1"].Len())
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
	key := pendingKey("10.0.0.1", bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0))
	r.pending[key] = &PendingRoute{
		peerAddr:   "10.0.0.1",
		family:     "ipv4/unicast",
		prefix:     "10.0.0.0/24",
		routeKey:   bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0),
		route:      &RawRoute{Family: "ipv4/unicast", AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
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
	require.Contains(t, r.ribIn, "10.0.0.1")
	assert.Equal(t, 1, r.ribIn["10.0.0.1"].Len())

	var route *RawRoute
	r.ribIn["10.0.0.1"].Range(func(_ string, _ uint64, rt *RawRoute) bool {
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
	m := seqmap.New[string, *RawRoute]()
	m.Put(bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0), 1, &RawRoute{
		Family:          "ipv4/unicast",
		AttrHex:         "40010100",
		NHopHex:         "0a000001",
		NLRIHex:         "180a0000",
		ValidationState: ValidationValid,
	})
	r.ribIn["10.0.0.1"] = m

	status, data, err := r.handleCommand("adj-rib-in revalidate", "ipv4/unicast 10.0.0.0/24")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)
	assert.Contains(t, data, "10.0.0.0/24", "revalidate should return route data")
}

// TestAcceptNonExistentRoute verifies error for unknown pending route.
//
// VALIDATES: accept-routes for non-existent pending route returns error.
// PREVENTS: Panic or silent success on invalid accept.
func TestAcceptNonExistentRoute(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	status, _, err := r.handleCommand("adj-rib-in accept-routes", "10.0.0.1 ipv4/unicast 10.0.0.0/24 1")
	assert.Equal(t, statusError, status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pending route")
}

// TestRejectAlreadyInstalled verifies error when rejecting an already-installed route.
//
// VALIDATES: reject-routes for non-pending route returns error, no state change.
// PREVENTS: Installed routes being incorrectly removed by late reject.
func TestRejectAlreadyInstalled(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true

	// Pre-populate installed route (not pending)
	m := seqmap.New[string, *RawRoute]()
	m.Put(bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0), 1, &RawRoute{
		Family: "ipv4/unicast", AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn["10.0.0.1"] = m

	status, _, err := r.handleCommand("adj-rib-in reject-routes", "10.0.0.1 ipv4/unicast 10.0.0.0/24")
	assert.Equal(t, statusError, status)
	assert.Error(t, err)

	// Installed route should be unchanged
	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Equal(t, 1, r.ribIn["10.0.0.1"].Len(), "installed route should not be removed")
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
	key1 := pendingKey("10.0.0.1", bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0))
	r.pending[key1] = &PendingRoute{
		peerAddr:   "10.0.0.1",
		family:     "ipv4/unicast",
		prefix:     "10.0.0.0/24",
		routeKey:   bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0),
		route:      &RawRoute{Family: "ipv4/unicast", AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: now,
		state:      ValidationPending,
	}
	key2 := pendingKey("10.0.0.1", bgp.RouteKey("ipv4/unicast", "10.0.1.0/24", 0))
	r.pending[key2] = &PendingRoute{
		peerAddr:   "10.0.0.1",
		family:     "ipv4/unicast",
		prefix:     "10.0.1.0/24",
		routeKey:   bgp.RouteKey("ipv4/unicast", "10.0.1.0/24", 0),
		route:      &RawRoute{Family: "ipv4/unicast", AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0001"},
		receivedAt: now,
		state:      ValidationPending,
	}
	r.mu.Unlock()

	// Accept first route
	status, _, err := r.handleCommand("adj-rib-in accept-routes", "10.0.0.1 ipv4/unicast 10.0.0.0/24 1")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	// Reject second route
	status, _, err = r.handleCommand("adj-rib-in reject-routes", "10.0.0.1 ipv4/unicast 10.0.1.0/24")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Pending should be empty
	assert.Empty(t, r.pending)

	// Only accepted route should be installed
	require.Contains(t, r.ribIn, "10.0.0.1")
	assert.Equal(t, 1, r.ribIn["10.0.0.1"].Len())

	rt, ok := r.ribIn["10.0.0.1"].Get(bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0))
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
			key := pendingKey("10.0.0.1", bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0))
			r.pending[key] = &PendingRoute{
				peerAddr:   "10.0.0.1",
				family:     "ipv4/unicast",
				prefix:     "10.0.0.0/24",
				routeKey:   bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0),
				route:      &RawRoute{Family: "ipv4/unicast", AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
				receivedAt: time.Now(),
				state:      ValidationPending,
			}
			r.mu.Unlock()

			status, _, err := r.handleCommand("adj-rib-in accept-routes", "10.0.0.1 ipv4/unicast 10.0.0.0/24 "+tt.stateArg)
			require.NoError(t, err)
			assert.Equal(t, statusDone, status)

			r.mu.RLock()
			defer r.mu.RUnlock()

			rt, ok := r.ribIn["10.0.0.1"].Get(bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0))
			require.True(t, ok)
			assert.Equal(t, tt.want, rt.ValidationState)
		})
	}
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
	key := pendingKey("10.0.0.1", bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0))
	r.pending[key] = &PendingRoute{
		peerAddr:   "10.0.0.1",
		family:     "ipv4/unicast",
		prefix:     "10.0.0.0/24",
		routeKey:   bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0),
		route:      &RawRoute{Family: "ipv4/unicast", AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now(),
		state:      ValidationPending,
	}
	r.mu.Unlock()

	// Peer goes down
	downEvent := &bgp.Event{
		Type:  "state",
		Peer:  mustMarshal(t, bgp.PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
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
	key1 := pendingKey("10.0.0.1", bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0))
	r.pending[key1] = &PendingRoute{
		peerAddr:   "10.0.0.1",
		family:     "ipv4/unicast",
		prefix:     "10.0.0.0/24",
		routeKey:   bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0),
		route:      &RawRoute{Family: "ipv4/unicast", AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now().Add(-200 * time.Millisecond),
		state:      ValidationPending,
	}
	// Not-yet-expired route
	key2 := pendingKey("10.0.0.1", bgp.RouteKey("ipv4/unicast", "10.0.1.0/24", 0))
	r.pending[key2] = &PendingRoute{
		peerAddr:   "10.0.0.1",
		family:     "ipv4/unicast",
		prefix:     "10.0.1.0/24",
		routeKey:   bgp.RouteKey("ipv4/unicast", "10.0.1.0/24", 0),
		route:      &RawRoute{Family: "ipv4/unicast", AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0001"},
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
	require.Contains(t, r.ribIn, "10.0.0.1")
	assert.Equal(t, 1, r.ribIn["10.0.0.1"].Len(), "only expired route should be installed")
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
	key1 := pendingKey("10.0.0.1", bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0))
	r.pending[key1] = &PendingRoute{
		peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.0.0/24",
		routeKey: bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0),
		route:    &RawRoute{Family: "ipv4/unicast", AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		state:    ValidationPending,
	}
	// Pending route for peer 2
	key2 := pendingKey("10.0.0.2", bgp.RouteKey("ipv4/unicast", "10.0.1.0/24", 0))
	r.pending[key2] = &PendingRoute{
		peerAddr: "10.0.0.2", family: "ipv4/unicast", prefix: "10.0.1.0/24",
		routeKey: bgp.RouteKey("ipv4/unicast", "10.0.1.0/24", 0),
		route:    &RawRoute{Family: "ipv4/unicast", AttrHex: "40010100", NHopHex: "0a000002", NLRIHex: "180a0001"},
		state:    ValidationPending,
	}
	r.mu.Unlock()

	// Peer 1 goes down
	downEvent := &bgp.Event{
		Type:  "state",
		Peer:  mustMarshal(t, bgp.PeerInfoFlat{Address: "10.0.0.1", ASN: 65001}),
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
	key := pendingKey("10.0.0.1", bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0))
	r.pending[key] = &PendingRoute{
		peerAddr:   "10.0.0.1",
		family:     "ipv4/unicast",
		prefix:     "10.0.0.0/24",
		routeKey:   bgp.RouteKey("ipv4/unicast", "10.0.0.0/24", 0),
		route:      &RawRoute{Family: "ipv4/unicast", AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000"},
		receivedAt: time.Now(),
		state:      ValidationPending,
	}
	r.mu.Unlock()

	// Receive withdrawal
	withdraw := &bgp.Event{
		Message:      &bgp.MessageInfo{Type: "update", ID: 101},
		Peer:         testPeerJSON(t),
		RawWithdrawn: map[string]string{"ipv4/unicast": "180a0000"},
		FamilyOps: map[string][]bgp.FamilyOperation{
			"ipv4/unicast": {
				{Action: "del", NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	}
	r.handleReceived(withdraw)

	r.mu.RLock()
	defer r.mu.RUnlock()

	assert.Empty(t, r.pending, "pending route should be removed on withdrawal")
}
