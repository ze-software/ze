// Design: docs/architecture/chaos-web-dashboard.md — chaos action compatibility guard

package guard

import (
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/chaos/engine"
	"codeberg.org/thomas-mangin/ze/internal/chaos/route"
)

// peerState tracks per-peer conditions used for action compatibility checks.
type peerState struct {
	// holdTimerPending is true when a hold-timer expiry action has been sent
	// but the peer hasn't disconnected yet. Prevents duplicate expiry actions
	// and pointless route actions on a dying session.
	holdTimerPending bool

	// routesLive is true when the peer has announced routes that haven't been
	// fully withdrawn. Set to true on establishment, set to false on full
	// withdrawal, restored on reconnect or churn re-announce.
	//
	// Limitation: this is a boolean, not a count. After a partial withdrawal,
	// routesLive stays true because some routes remain. However, the simulator's
	// withdrawFraction picks from the full routes slice regardless of what was
	// previously withdrawn, so overlapping partial withdrawals can withdraw
	// already-withdrawn routes. Fixing this requires tracking the announced
	// prefix set per peer (like validation/model.go does), which is a larger
	// change than the guard is designed for.
	routesLive bool
}

// Guard enforces action compatibility by filtering out actions that are
// invalid given the peer's current state. It is written by three goroutines:
// the event-processing loop (authoritative state from peer events), the chaos
// scheduler (dispatch-time hold-timer update), and the route scheduler
// (dispatch-time full-withdraw update). All access is mutex-protected.
type Guard struct {
	mu    sync.RWMutex
	peers []peerState
}

// New creates a guard for n peers, all starting in idle state.
func New(n int) *Guard {
	return &Guard{peers: make([]peerState, n)}
}

// OnEstablished marks a peer as having live routes and clears pending state.
// Called when EventEstablished is received. Routes are sent after this event
// but before the keepalive loop reads from the action channels, so
// routesLive=true is safe: no action can execute until routes are sent.
func (g *Guard) OnEstablished(idx int) {
	g.mu.Lock()
	g.peers[idx] = peerState{routesLive: true}
	g.mu.Unlock()
}

// OnDisconnected resets the peer to idle state.
func (g *Guard) OnDisconnected(idx int) {
	g.mu.Lock()
	g.peers[idx] = peerState{}
	g.mu.Unlock()
}

// OnHoldTimerExpiry marks the peer as having a pending hold-timer expiry.
func (g *Guard) OnHoldTimerExpiry(idx int) {
	g.mu.Lock()
	g.peers[idx].holdTimerPending = true
	g.mu.Unlock()
}

// OnFullWithdraw marks the peer as having no live routes.
func (g *Guard) OnFullWithdraw(idx int) {
	g.mu.Lock()
	g.peers[idx].routesLive = false
	g.mu.Unlock()
}

// OnRoutesRestored marks the peer as having live routes again
// (e.g., after churn re-announces).
func (g *Guard) OnRoutesRestored(idx int) {
	g.mu.Lock()
	g.peers[idx].routesLive = true
	g.mu.Unlock()
}

// AllowChaos returns true if the chaos action is compatible with the peer's
// current state. Returns a reason string when rejected (empty on allow).
func (g *Guard) AllowChaos(idx int, action engine.ActionType) (bool, string) {
	g.mu.RLock()
	s := g.peers[idx]
	g.mu.RUnlock()

	switch action {
	case engine.ActionHoldTimerExpiry:
		if s.holdTimerPending {
			return false, "hold-timer expiry already pending"
		}
	case engine.ActionTCPDisconnect,
		engine.ActionNotificationCease,
		engine.ActionDisconnectDuringBurst,
		engine.ActionReconnectStorm,
		engine.ActionConnectionCollision,
		engine.ActionMalformedUpdate,
		engine.ActionConfigReload:
		// No additional guards — these are always valid on an established peer.
	}

	return true, ""
}

// AllowRoute returns true if the route action is compatible with the peer's
// current state. Returns a reason string when rejected (empty on allow).
func (g *Guard) AllowRoute(idx int, action route.ActionType) (bool, string) {
	g.mu.RLock()
	s := g.peers[idx]
	g.mu.RUnlock()

	if s.holdTimerPending {
		return false, "hold-timer expiry pending"
	}

	switch action {
	case route.ActionPartialWithdraw, route.ActionFullWithdraw:
		if !s.routesLive {
			return false, "no routes to withdraw"
		}
	case route.ActionChurn:
		if !s.routesLive {
			return false, "no routes to churn"
		}
	}

	return true, ""
}
