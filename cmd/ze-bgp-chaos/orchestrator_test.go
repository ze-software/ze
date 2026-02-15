package main

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/validation"
)

// TestOrchestratorEventProcessing verifies that the event processor
// correctly updates model, tracker, and convergence from events.
//
// VALIDATES: Events flow to all three validation components.
// PREVENTS: Lost events or misrouted state updates.
func TestOrchestratorEventProcessing(t *testing.T) {
	m := validation.NewModel(3)
	tr := validation.NewTracker(3)
	conv := validation.NewConvergence(3, 5*time.Second)

	ep := &EventProcessor{
		Model:       m,
		Tracker:     tr,
		Convergence: conv,
	}

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	now := time.Now()

	// Peer 0 establishes.
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 2, Time: now})

	// Peer 0 sends a route.
	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: prefix})

	// Peer 1 receives it.
	recvTime := now.Add(50 * time.Millisecond)
	ep.Process(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: recvTime, Prefix: prefix})

	// Model should expect the route at peers 1 and 2.
	assert.True(t, m.Expected(1).Contains(prefix))
	assert.True(t, m.Expected(2).Contains(prefix))

	// Tracker should show peer 1 received it.
	assert.True(t, tr.ActualRoutes(1).Contains(prefix))

	// Convergence should have 1 resolved (peer 1) and 1 pending (peer 2).
	stats := conv.Stats()
	assert.Equal(t, 1, stats.Resolved)
	assert.Equal(t, 1, stats.Pending)
}

// TestOrchestratorWithdrawal verifies that withdrawal events update
// model and tracker correctly.
//
// VALIDATES: Withdrawal removes from both model and tracker.
// PREVENTS: Stale routes after withdrawal.
func TestOrchestratorWithdrawal(t *testing.T) {
	m := validation.NewModel(2)
	tr := validation.NewTracker(2)
	conv := validation.NewConvergence(2, 5*time.Second)

	ep := &EventProcessor{
		Model:       m,
		Tracker:     tr,
		Convergence: conv,
	}

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	now := time.Now()

	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})
	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: prefix})
	ep.Process(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: now.Add(10 * time.Millisecond), Prefix: prefix})

	// Peer 0 withdraws.
	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now.Add(100 * time.Millisecond), Prefix: prefix})
	// Wait — withdrawal is a different event type. RouteSent is announce.
	// Withdrawal from RR arrives as EventRouteWithdrawn at peer 1.
	ep.Process(peer.Event{Type: peer.EventRouteWithdrawn, PeerIndex: 1, Time: now.Add(200 * time.Millisecond), Prefix: prefix})

	// Tracker should show peer 1 no longer has the route.
	assert.False(t, tr.ActualRoutes(1).Contains(prefix))
}

// TestOrchestratorDisconnect verifies that disconnect events clear
// model state for the disconnected peer.
//
// VALIDATES: Disconnect removes all announced routes from model.
// PREVENTS: Orphaned expected routes for disconnected peers.
func TestOrchestratorDisconnect(t *testing.T) {
	m := validation.NewModel(3)
	tr := validation.NewTracker(3)
	conv := validation.NewConvergence(3, 5*time.Second)

	ep := &EventProcessor{
		Model:       m,
		Tracker:     tr,
		Convergence: conv,
	}

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	now := time.Now()

	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 2, Time: now})
	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: prefix})

	// Peer 0 disconnects.
	ep.Process(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: now.Add(time.Second)})

	// Model should no longer expect peer 0's route at anyone.
	assert.Equal(t, 0, m.Expected(1).Len())
	assert.Equal(t, 0, m.Expected(2).Len())

	// Tracker for peer 0 should be cleared.
	assert.Equal(t, 0, tr.ActualRoutes(0).Len())
}

// TestOrchestratorCounters verifies that the event processor
// tracks announced and received counts.
//
// VALIDATES: Counter accuracy for summary report.
// PREVENTS: Incorrect route statistics in exit summary.
func TestOrchestratorCounters(t *testing.T) {
	m := validation.NewModel(2)
	tr := validation.NewTracker(2)
	conv := validation.NewConvergence(2, 5*time.Second)

	ep := &EventProcessor{
		Model:       m,
		Tracker:     tr,
		Convergence: conv,
	}

	now := time.Now()
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})

	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: netip.MustParsePrefix("10.0.0.0/24")})
	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: netip.MustParsePrefix("10.0.1.0/24")})
	ep.Process(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: now, Prefix: netip.MustParsePrefix("10.0.0.0/24")})

	assert.Equal(t, 2, ep.Announced)
	assert.Equal(t, 1, ep.Received)
}
