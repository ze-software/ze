package validation

import (
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	t0      = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1      = t0.Add(100 * time.Millisecond)
	t2      = t0.Add(200 * time.Millisecond)
	t3      = t0.Add(300 * time.Millisecond)
	prefix1 = netip.MustParsePrefix("10.0.0.0/24")
	prefix2 = netip.MustParsePrefix("10.0.1.0/24")
)

// --- RouteConsistency ---

// TestRouteConsistencyPass verifies no violations when all routes propagated.
//
// VALIDATES: Routes announced by peer 0 are received by peer 1.
// PREVENTS: False positives from route-consistency property.
func TestRouteConsistencyPass(t *testing.T) {
	p := NewRouteConsistency(2)
	p.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1})
	p.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Prefix: prefix1, Time: t2})

	assert.Empty(t, p.Violations())
}

// TestRouteConsistencyFail verifies violation when route not received.
//
// VALIDATES: Missing route generates violation with peer index.
// PREVENTS: Undetected route propagation failures.
func TestRouteConsistencyFail(t *testing.T) {
	p := NewRouteConsistency(2)
	p.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1})
	// Peer 1 does NOT receive the route.

	v := p.Violations()
	require.NotEmpty(t, v)
	assert.Equal(t, "route-consistency", v[0].Property)
	assert.Contains(t, v[0].Message, "missing")
	assert.Equal(t, 1, v[0].PeerIndex)
}

// TestRouteConsistencyAfterDisconnect verifies state transitions on disconnect.
//
// VALIDATES: After disconnect, source's routes are no longer expected but may
// remain in receiver's tracker as "extra" until explicitly withdrawn by RR.
// PREVENTS: Misunderstanding of disconnect semantics in route-consistency.
func TestRouteConsistencyAfterDisconnect(t *testing.T) {
	p := NewRouteConsistency(2)
	p.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1})
	p.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Prefix: prefix1, Time: t2})
	// Peer 0 disconnects — its routes are cleared from model (no longer expected).
	// But peer 1's tracker still has the route → it's now "extra".
	p.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: t3})
	// RR sends withdrawal to peer 1.
	p.ProcessEvent(peer.Event{Type: peer.EventRouteWithdrawn, PeerIndex: 1, Prefix: prefix1, Time: t3})

	assert.Empty(t, p.Violations())
}

// --- ConvergenceDeadline ---

// TestConvergenceDeadlinePass verifies no violation when routes converge in time.
//
// VALIDATES: Route received within deadline produces no violation.
// PREVENTS: False violations when convergence is fast enough.
func TestConvergenceDeadlinePass(t *testing.T) {
	p := NewConvergenceDeadline(2, 5*time.Second)
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Prefix: prefix1, Time: t1})

	assert.Empty(t, p.Violations())
}

// TestConvergenceDeadlineFail verifies violation when route exceeds deadline.
//
// VALIDATES: Route not received within deadline generates violation.
// PREVENTS: Undetected slow convergence.
func TestConvergenceDeadlineFail(t *testing.T) {
	p := NewConvergenceDeadline(2, 1*time.Second)
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t0})
	// Advance time past deadline without receiving.
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix2, Time: t0.Add(2 * time.Second)})

	v := p.Violations()
	require.NotEmpty(t, v)
	assert.Equal(t, "convergence-deadline", v[0].Property)
	assert.Contains(t, v[0].Message, "not received")
}

// --- NoDuplicateRoutes ---

// TestNoDuplicateRoutesPass verifies no violation for clean announce pattern.
//
// VALIDATES: Single announce per prefix produces no violation.
// PREVENTS: False positives on normal announcement pattern.
func TestNoDuplicateRoutesPass(t *testing.T) {
	p := NewNoDuplicateRoutes(2)
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix2, Time: t1})

	assert.Empty(t, p.Violations())
}

// TestNoDuplicateRoutesFail verifies violation on double-announce.
//
// VALIDATES: Same prefix announced twice without withdrawal is detected.
// PREVENTS: Undetected duplicate route announcements.
func TestNoDuplicateRoutesFail(t *testing.T) {
	p := NewNoDuplicateRoutes(2)
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1}) // duplicate!

	v := p.Violations()
	require.NotEmpty(t, v)
	assert.Equal(t, "no-duplicate-routes", v[0].Property)
	assert.Contains(t, v[0].Message, "twice")
}

// TestNoDuplicateRoutesAfterDisconnect verifies clean state after disconnect.
//
// VALIDATES: Re-announce after disconnect is not flagged as duplicate.
// PREVENTS: False violations after reconnection cycle.
func TestNoDuplicateRoutesAfterDisconnect(t *testing.T) {
	p := NewNoDuplicateRoutes(2)
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: t1})
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t2}) // OK after disconnect

	assert.Empty(t, p.Violations())
}

// TestNoDuplicateRoutesAfterWithdrawal verifies that re-announce after withdrawal is allowed.
//
// VALIDATES: Withdrawal clears the announced state for a prefix.
// PREVENTS: False duplicate violation after explicit withdrawal.
func TestNoDuplicateRoutesAfterWithdrawal(t *testing.T) {
	p := NewNoDuplicateRoutes(2)
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventRouteWithdrawn, PeerIndex: 0, Prefix: prefix1, Time: t1})
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t2}) // re-announce after withdrawal

	assert.Empty(t, p.Violations())
}

// --- HoldTimerEnforcement ---

// TestHoldTimerEnforcementPass verifies no violation when session tears down after hold-timer chaos.
//
// VALIDATES: Disconnect after hold-timer-expiry chaos clears the pending state.
// PREVENTS: False violations when hold-timer is correctly enforced.
func TestHoldTimerEnforcementPass(t *testing.T) {
	p := NewHoldTimerEnforcement(2)
	p.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 0, ChaosAction: "hold-timer-expiry", Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: t1})

	assert.Empty(t, p.Violations())
}

// TestHoldTimerEnforcementFail verifies violation when session survives hold-timer chaos.
//
// VALIDATES: Hold-timer-expiry without subsequent disconnect generates violation.
// PREVENTS: Undetected hold-timer enforcement failures.
func TestHoldTimerEnforcementFail(t *testing.T) {
	p := NewHoldTimerEnforcement(2)
	p.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 0, ChaosAction: "hold-timer-expiry", Time: t0})
	// No disconnect follows.

	v := p.Violations()
	require.NotEmpty(t, v)
	assert.Equal(t, "hold-timer-enforcement", v[0].Property)
	assert.Contains(t, v[0].Message, "not torn down")
}

// TestHoldTimerIgnoresOtherChaos verifies that non-hold-timer chaos events are ignored.
//
// VALIDATES: Only hold-timer-expiry chaos triggers the enforcement check.
// PREVENTS: False violations from unrelated chaos events.
func TestHoldTimerIgnoresOtherChaos(t *testing.T) {
	p := NewHoldTimerEnforcement(2)
	p.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 0, ChaosAction: "tcp-disconnect", Time: t0})

	assert.Empty(t, p.Violations())
}

// TestHoldTimerClearedOnReEstablished verifies that re-establishment clears stale pending expiry.
//
// VALIDATES: EventEstablished defensively clears pending hold-timer expiry.
// PREVENTS: Stale violations when disconnect event was lost but peer re-establishes.
func TestHoldTimerClearedOnReEstablished(t *testing.T) {
	p := NewHoldTimerEnforcement(2)
	p.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 0, ChaosAction: "hold-timer-expiry", Time: t0})
	// No disconnect observed, but peer re-establishes (implies disconnect happened).
	p.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: t1})

	assert.Empty(t, p.Violations())
}

// --- MessageOrdering ---

// TestMessageOrderingPass verifies no violation when routes follow establishment.
//
// VALIDATES: Route events after established produce no violation.
// PREVENTS: False positives on normal session ordering.
func TestMessageOrderingPass(t *testing.T) {
	p := NewMessageOrdering(2)
	p.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1})

	assert.Empty(t, p.Violations())
}

// TestMessageOrderingFail verifies violation when route sent before established.
//
// VALIDATES: Route event before established generates violation.
// PREVENTS: Undetected protocol ordering violations.
func TestMessageOrderingFail(t *testing.T) {
	p := NewMessageOrdering(2)
	// Route before established.
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t0})

	v := p.Violations()
	require.NotEmpty(t, v)
	assert.Equal(t, "message-ordering", v[0].Property)
	assert.Contains(t, v[0].Message, "before established")
}

// TestMessageOrderingAfterReconnect verifies violation detection after reconnect cycle.
//
// VALIDATES: Route event after disconnect (but before re-establish) is caught.
// PREVENTS: Ordering violations slipping through reconnect cycles.
func TestMessageOrderingAfterReconnect(t *testing.T) {
	p := NewMessageOrdering(2)
	p.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: t1})
	// Route after disconnect but before re-establish.
	p.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 0, Prefix: prefix1, Time: t2})

	v := p.Violations()
	require.NotEmpty(t, v)
	assert.Contains(t, v[0].Message, "before established")
}

// --- PropertyEngine ---

// TestPropertyEngineResults verifies per-property pass/fail aggregation.
//
// VALIDATES: Engine produces correct PropertyResult for each registered property.
// PREVENTS: Incorrect aggregation of property results.
func TestPropertyEngineResults(t *testing.T) {
	engine := NewPropertyEngine(AllProperties(2, 5*time.Second))

	// Feed a passing scenario.
	engine.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: t0})
	engine.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: t0})
	engine.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1})
	engine.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Prefix: prefix1, Time: t2})

	results := engine.Results()
	require.Len(t, results, 5)
	for _, r := range results {
		assert.True(t, r.Pass, "property %s should pass", r.Name)
	}
}

// TestPropertyEngineDetectsViolation verifies engine reports violations from failing properties.
//
// VALIDATES: At least one property reports violations when routes are missing.
// PREVENTS: Engine silently ignoring property violations.
func TestPropertyEngineDetectsViolation(t *testing.T) {
	engine := NewPropertyEngine(AllProperties(2, 5*time.Second))

	// Peer 0 announces but peer 1 never receives.
	engine.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: t0})
	engine.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: t0})
	engine.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1})

	violations := engine.AllViolations()
	assert.NotEmpty(t, violations)
}

// TestSelectProperties verifies property selection by name.
//
// VALIDATES: Only named properties are returned, unknown names cause error.
// PREVENTS: Silent selection of wrong properties.
func TestSelectProperties(t *testing.T) {
	all := AllProperties(2, 5*time.Second)

	selected, err := SelectProperties(all, []string{"route-consistency", "message-ordering"})
	require.NoError(t, err)
	require.Len(t, selected, 2)
	assert.Equal(t, "route-consistency", selected[0].Name())
	assert.Equal(t, "message-ordering", selected[1].Name())
}

// TestSelectPropertiesUnknown verifies error for unknown property names.
//
// VALIDATES: Unknown property name produces descriptive error.
// PREVENTS: Silent acceptance of misspelled property names.
func TestSelectPropertiesUnknown(t *testing.T) {
	all := AllProperties(2, 5*time.Second)

	_, err := SelectProperties(all, []string{"nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown property")
}

// TestListProperties verifies property listing format.
//
// VALIDATES: All properties listed with name and description.
// PREVENTS: Missing properties in --properties list output.
func TestListProperties(t *testing.T) {
	all := AllProperties(2, 5*time.Second)
	lines := ListProperties(all)

	require.Len(t, lines, 5)
	assert.Contains(t, lines[0], "route-consistency")
	assert.Contains(t, lines[0], "eligible peer")
}

// TestPropertyReset verifies that Reset clears internal state.
//
// VALIDATES: After Reset, violations from previous run are cleared.
// PREVENTS: Violation carryover between replay runs.
func TestPropertyReset(t *testing.T) {
	p := NewNoDuplicateRoutes(2)
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t0})
	p.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1})
	require.NotEmpty(t, p.Violations())

	p.Reset()
	assert.Empty(t, p.Violations())
}
