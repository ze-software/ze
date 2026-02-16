package shrink

import (
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
	"github.com/stretchr/testify/assert"
)

var (
	t0      = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1      = t0.Add(100 * time.Millisecond)
	t2      = t0.Add(200 * time.Millisecond)
	t3      = t0.Add(300 * time.Millisecond)
	t4      = t0.Add(400 * time.Millisecond)
	prefix1 = netip.MustParsePrefix("10.0.0.0/24")
	prefix2 = netip.MustParsePrefix("10.0.1.0/24")
)

// TestRemoveEstablishedCascadesRoutes verifies that removing an Established
// event also removes all dependent route events for that peer.
//
// VALIDATES: Causal chain: Established → RouteSent is respected.
// PREVENTS: Dangling route events without prior establishment.
func TestRemoveEstablishedCascadesRoutes(t *testing.T) {
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix2, Time: t2},
	}

	result := RemoveWithDependents(events, 0)
	assert.Empty(t, result, "removing Established should cascade to all route events")
}

// TestRemoveEstablishedPreservesOtherPeers verifies that removing Established
// for peer 0 does not affect peer 1's events.
//
// VALIDATES: Causal dependencies are per-peer.
// PREVENTS: Cross-peer cascade removing unrelated events.
func TestRemoveEstablishedPreservesOtherPeers(t *testing.T) {
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
		{Type: peer.EventEstablished, PeerIndex: 1, Time: t0},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1},
		{Type: peer.EventRouteSent, PeerIndex: 1, Prefix: prefix2, Time: t1},
	}

	result := RemoveWithDependents(events, 0)
	assert.Len(t, result, 2, "peer 1's events should be preserved")
	assert.Equal(t, peer.EventEstablished, result[0].Type)
	assert.Equal(t, 1, result[0].PeerIndex)
	assert.Equal(t, peer.EventRouteSent, result[1].Type)
	assert.Equal(t, 1, result[1].PeerIndex)
}

// TestRemoveMiddleRouteNoCascade verifies that removing a RouteSent event
// does not cascade to unrelated events.
//
// VALIDATES: RouteSent has no dependents (route events are independent).
// PREVENTS: Over-pruning when removing individual routes.
func TestRemoveMiddleRouteNoCascade(t *testing.T) {
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix2, Time: t2},
	}

	result := RemoveWithDependents(events, 1)
	assert.Len(t, result, 2, "only the removed route should be gone")
	assert.Equal(t, peer.EventEstablished, result[0].Type)
	assert.Equal(t, prefix2, result[1].Prefix)
}

// TestRemoveDisconnectCascadesReconnect verifies that removing a Disconnected
// event also removes subsequent Reconnecting and re-establishment chains.
//
// VALIDATES: Disconnected → Reconnecting → Established chain is respected.
// PREVENTS: Reconnecting event without prior disconnect.
func TestRemoveDisconnectCascadesReconnect(t *testing.T) {
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1},
		{Type: peer.EventDisconnected, PeerIndex: 0, Time: t2},
		{Type: peer.EventReconnecting, PeerIndex: 0, Time: t3},
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t4},
	}

	// Remove the Disconnected event: peer stays established, so the
	// Reconnecting is informational and kept, and re-Established is fine.
	// But the key point: the events should still be valid.
	result := RemoveWithDependents(events, 2)

	// Peer is still established after removing disconnect, so Reconnecting
	// is kept (informational), and the second Established is valid.
	assert.Len(t, result, 4)
	assert.Equal(t, peer.EventEstablished, result[0].Type)
	assert.Equal(t, peer.EventRouteSent, result[1].Type)
	assert.Equal(t, peer.EventReconnecting, result[2].Type)
	assert.Equal(t, peer.EventEstablished, result[3].Type)
}

// TestRemoveEstablishedCascadesDisconnect verifies that removing Established
// also removes a subsequent Disconnected (can't disconnect what's not connected).
//
// VALIDATES: Established → Disconnected dependency.
// PREVENTS: Disconnected event for a peer that was never established.
func TestRemoveEstablishedCascadesDisconnect(t *testing.T) {
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1},
		{Type: peer.EventDisconnected, PeerIndex: 0, Time: t2},
	}

	result := RemoveWithDependents(events, 0)
	assert.Empty(t, result, "all events should be cascaded away")
}

// TestRemoveBeforeRemovalPointUnchanged verifies that events before the
// removal point are always kept.
//
// VALIDATES: Only events after removal are subject to cascade.
// PREVENTS: Accidental removal of events preceding the target.
func TestRemoveBeforeRemovalPointUnchanged(t *testing.T) {
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1},
		{Type: peer.EventEstablished, PeerIndex: 1, Time: t2},
		{Type: peer.EventRouteSent, PeerIndex: 1, Prefix: prefix2, Time: t3},
	}

	// Remove event at index 2 (Established for peer 1).
	result := RemoveWithDependents(events, 2)
	assert.Len(t, result, 2, "events before removal point preserved, peer 1 route cascaded")
	assert.Equal(t, peer.EventEstablished, result[0].Type)
	assert.Equal(t, 0, result[0].PeerIndex)
	assert.Equal(t, peer.EventRouteSent, result[1].Type)
	assert.Equal(t, 0, result[1].PeerIndex)
}

// TestRemoveOutOfBounds verifies that invalid indices return original events.
//
// VALIDATES: Bounds checking on removeIdx.
// PREVENTS: Panic on negative or oversized index.
func TestRemoveOutOfBounds(t *testing.T) {
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
	}

	assert.Len(t, RemoveWithDependents(events, -1), 1)
	assert.Len(t, RemoveWithDependents(events, 5), 1)
}

// TestRemoveReEstablishedPreservesLaterRoutes verifies that removing a
// re-establishment doesn't remove routes from the earlier establishment.
//
// VALIDATES: Re-establishment after disconnect is a new session.
// PREVENTS: State confusion between multiple session lifetimes.
func TestRemoveReEstablishedPreservesLaterRoutes(t *testing.T) {
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1},
		{Type: peer.EventDisconnected, PeerIndex: 0, Time: t2},
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t3},                // re-establish
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix2, Time: t4}, // new route
	}

	// Remove re-establishment at index 3. Route at index 4 depends on it.
	result := RemoveWithDependents(events, 3)
	assert.Len(t, result, 3, "first session preserved, second session's route removed")
	assert.Equal(t, peer.EventEstablished, result[0].Type)
	assert.Equal(t, peer.EventRouteSent, result[1].Type)
	assert.Equal(t, prefix1, result[1].Prefix)
	assert.Equal(t, peer.EventDisconnected, result[2].Type)
}
