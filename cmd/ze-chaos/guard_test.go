package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/chaos"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/route"
)

// TestGuardAllowsChaosOnFreshPeer verifies that all chaos actions are
// allowed on a newly-established peer with no pending state.
//
// VALIDATES: Fresh established peer accepts all chaos action types.
// PREVENTS: Guard over-blocking on normal peers.
func TestGuardAllowsChaosOnFreshPeer(t *testing.T) {
	g := newPeerGuard(2)
	g.OnEstablished(0)

	actions := []chaos.ActionType{
		chaos.ActionTCPDisconnect,
		chaos.ActionNotificationCease,
		chaos.ActionHoldTimerExpiry,
		chaos.ActionDisconnectDuringBurst,
		chaos.ActionReconnectStorm,
		chaos.ActionConnectionCollision,
		chaos.ActionMalformedUpdate,
		chaos.ActionConfigReload,
	}

	for _, a := range actions {
		ok, reason := g.AllowChaos(0, a)
		assert.True(t, ok, "action %s should be allowed, got blocked: %s", a, reason)
	}
}

// TestGuardBlocksDuplicateHoldTimerExpiry verifies that a second hold-timer
// expiry action is rejected when one is already pending.
//
// VALIDATES: Duplicate hold-timer expiry blocked with reason.
// PREVENTS: Redundant stop-keepalive on a peer already waiting for expiry.
func TestGuardBlocksDuplicateHoldTimerExpiry(t *testing.T) {
	g := newPeerGuard(2)
	g.OnEstablished(0)

	// First hold-timer expiry is allowed.
	ok, _ := g.AllowChaos(0, chaos.ActionHoldTimerExpiry)
	assert.True(t, ok)

	// Mark it as pending.
	g.OnHoldTimerExpiry(0)

	// Second is blocked.
	ok, reason := g.AllowChaos(0, chaos.ActionHoldTimerExpiry)
	assert.False(t, ok)
	assert.Contains(t, reason, "already pending")

	// Other chaos actions still allowed (disconnect is valid even with pending expiry).
	ok, _ = g.AllowChaos(0, chaos.ActionTCPDisconnect)
	assert.True(t, ok)
}

// TestGuardBlocksRouteActionsWhenNoRoutes verifies that route actions are
// blocked after a full withdrawal leaves no routes to operate on.
//
// VALIDATES: Withdraw/churn blocked when routesLive is false.
// PREVENTS: Withdrawing routes that were never announced (or already withdrawn).
func TestGuardBlocksRouteActionsWhenNoRoutes(t *testing.T) {
	g := newPeerGuard(2)
	g.OnEstablished(0)

	// Routes are live after establishment — all route actions allowed.
	for _, a := range []route.ActionType{route.ActionChurn, route.ActionPartialWithdraw, route.ActionFullWithdraw} {
		ok, _ := g.AllowRoute(0, a)
		assert.True(t, ok, "action %s should be allowed with live routes", a)
	}

	// Full withdraw clears routes.
	g.OnFullWithdraw(0)

	// All route actions now blocked.
	for _, a := range []route.ActionType{route.ActionChurn, route.ActionPartialWithdraw, route.ActionFullWithdraw} {
		ok, reason := g.AllowRoute(0, a)
		assert.False(t, ok, "action %s should be blocked with no routes", a)
		assert.NotEmpty(t, reason)
	}
}

// TestGuardBlocksRouteActionsWhenHoldTimerPending verifies that route actions
// are rejected when a hold-timer expiry is pending (session is about to die).
//
// VALIDATES: All route actions blocked during pending hold-timer expiry.
// PREVENTS: Pointless route operations on a dying session.
func TestGuardBlocksRouteActionsWhenHoldTimerPending(t *testing.T) {
	g := newPeerGuard(2)
	g.OnEstablished(0)
	g.OnHoldTimerExpiry(0)

	for _, a := range []route.ActionType{route.ActionChurn, route.ActionPartialWithdraw, route.ActionFullWithdraw} {
		ok, reason := g.AllowRoute(0, a)
		assert.False(t, ok, "action %s should be blocked with pending expiry", a)
		assert.Contains(t, reason, "hold-timer")
	}
}

// TestGuardResetsOnDisconnect verifies that disconnection clears all
// pending state, so a reconnected peer starts fresh.
//
// VALIDATES: Disconnect resets holdTimerPending and routesLive.
// PREVENTS: Stale guard state carrying over across sessions.
func TestGuardResetsOnDisconnect(t *testing.T) {
	g := newPeerGuard(2)
	g.OnEstablished(0)
	g.OnHoldTimerExpiry(0)
	g.OnFullWithdraw(0)

	// Everything blocked.
	ok, _ := g.AllowChaos(0, chaos.ActionHoldTimerExpiry)
	assert.False(t, ok)
	ok, _ = g.AllowRoute(0, route.ActionChurn)
	assert.False(t, ok)

	// Disconnect + re-establish resets.
	g.OnDisconnected(0)
	g.OnEstablished(0)

	ok, _ = g.AllowChaos(0, chaos.ActionHoldTimerExpiry)
	assert.True(t, ok)
	ok, _ = g.AllowRoute(0, route.ActionChurn)
	assert.True(t, ok)
}

// TestGuardRoutesRestoredAfterChurn verifies that churn re-announce
// restores the routesLive flag.
//
// VALIDATES: OnRoutesRestored re-enables route actions after withdrawal.
// PREVENTS: Permanent route-action lockout after a full withdraw + churn.
func TestGuardRoutesRestoredAfterChurn(t *testing.T) {
	g := newPeerGuard(2)
	g.OnEstablished(0)
	g.OnFullWithdraw(0)

	ok, _ := g.AllowRoute(0, route.ActionPartialWithdraw)
	assert.False(t, ok)

	g.OnRoutesRestored(0)

	ok, _ = g.AllowRoute(0, route.ActionPartialWithdraw)
	assert.True(t, ok)
}

// TestGuardPeerIsolation verifies that guard state for one peer
// doesn't affect another peer.
//
// VALIDATES: Per-peer state isolation.
// PREVENTS: Cross-peer state contamination via shared slice.
func TestGuardPeerIsolation(t *testing.T) {
	g := newPeerGuard(3)
	g.OnEstablished(0)
	g.OnEstablished(1)
	g.OnEstablished(2)

	// Block peer 0 only.
	g.OnHoldTimerExpiry(0)
	g.OnFullWithdraw(1)

	// Peer 2 unaffected.
	ok, _ := g.AllowChaos(2, chaos.ActionHoldTimerExpiry)
	assert.True(t, ok)
	ok, _ = g.AllowRoute(2, route.ActionFullWithdraw)
	assert.True(t, ok)

	// Peer 0: chaos blocked, routes still live.
	ok, _ = g.AllowChaos(0, chaos.ActionHoldTimerExpiry)
	assert.False(t, ok)
	ok, _ = g.AllowRoute(0, route.ActionChurn)
	assert.False(t, ok, "hold-timer pending should block route actions")

	// Peer 1: chaos allowed, routes blocked.
	ok, _ = g.AllowChaos(1, chaos.ActionHoldTimerExpiry)
	assert.True(t, ok)
	ok, _ = g.AllowRoute(1, route.ActionChurn)
	assert.False(t, ok, "no routes should block churn")
}
