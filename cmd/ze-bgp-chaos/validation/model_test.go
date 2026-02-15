package validation

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func p(s string) netip.Prefix { return netip.MustParsePrefix(s) }

// TestModelAnnounce verifies that announcing a route from peer A makes it
// expected at peers B, C, D but not at A itself.
//
// VALIDATES: Announce propagates to all peers except the source.
// PREVENTS: Route appearing in source peer's expected set.
func TestModelAnnounce(t *testing.T) {
	m := NewModel(4)
	m.SetEstablished(0, true)
	m.SetEstablished(1, true)
	m.SetEstablished(2, true)
	m.SetEstablished(3, true)

	m.Announce(0, p("10.0.0.0/24"))

	// Peer 0 (source) should NOT expect the route.
	assert.False(t, m.Expected(0).Contains(p("10.0.0.0/24")))

	// Peers 1, 2, 3 should expect the route.
	assert.True(t, m.Expected(1).Contains(p("10.0.0.0/24")))
	assert.True(t, m.Expected(2).Contains(p("10.0.0.0/24")))
	assert.True(t, m.Expected(3).Contains(p("10.0.0.0/24")))
}

// TestModelWithdraw verifies that withdrawing a route removes it from
// all peers' expected sets.
//
// VALIDATES: Withdraw removes from expected at all non-source peers.
// PREVENTS: Stale routes remaining in expected set after withdrawal.
func TestModelWithdraw(t *testing.T) {
	m := NewModel(3)
	m.SetEstablished(0, true)
	m.SetEstablished(1, true)
	m.SetEstablished(2, true)

	m.Announce(0, p("10.0.0.0/24"))
	require.True(t, m.Expected(1).Contains(p("10.0.0.0/24")))

	m.Withdraw(0, p("10.0.0.0/24"))

	assert.False(t, m.Expected(1).Contains(p("10.0.0.0/24")))
	assert.False(t, m.Expected(2).Contains(p("10.0.0.0/24")))
}

// TestModelDisconnect verifies that disconnecting a peer removes all its
// announced routes from other peers' expected sets.
//
// VALIDATES: Disconnect removes all routes sourced by that peer.
// PREVENTS: Orphaned routes remaining after peer disconnects.
func TestModelDisconnect(t *testing.T) {
	m := NewModel(3)
	m.SetEstablished(0, true)
	m.SetEstablished(1, true)
	m.SetEstablished(2, true)

	m.Announce(0, p("10.0.0.0/24"))
	m.Announce(0, p("10.0.1.0/24"))
	m.Announce(1, p("172.16.0.0/24"))

	// Verify initial state.
	require.True(t, m.Expected(2).Contains(p("10.0.0.0/24")))
	require.True(t, m.Expected(2).Contains(p("10.0.1.0/24")))
	require.True(t, m.Expected(2).Contains(p("172.16.0.0/24")))

	// Disconnect peer 0.
	m.Disconnect(0)

	// Peer 0's routes should be gone from everyone.
	assert.False(t, m.Expected(1).Contains(p("10.0.0.0/24")))
	assert.False(t, m.Expected(1).Contains(p("10.0.1.0/24")))
	assert.False(t, m.Expected(2).Contains(p("10.0.0.0/24")))
	assert.False(t, m.Expected(2).Contains(p("10.0.1.0/24")))

	// Peer 1's routes should remain at peer 2.
	assert.True(t, m.Expected(2).Contains(p("172.16.0.0/24")))
	// Peer 0 is disconnected — expects nothing.
	assert.Equal(t, 0, m.Expected(0).Len())
}

// TestModelReconnect verifies that reconnecting a peer makes it expect
// all routes from other established peers.
//
// VALIDATES: Reconnected peer receives all current routes from others.
// PREVENTS: Reconnected peer having an empty expected set.
func TestModelReconnect(t *testing.T) {
	m := NewModel(3)
	m.SetEstablished(0, true)
	m.SetEstablished(1, true)
	m.SetEstablished(2, true)

	m.Announce(0, p("10.0.0.0/24"))
	m.Announce(1, p("172.16.0.0/24"))

	// Disconnect and reconnect peer 2.
	m.Disconnect(2)
	m.SetEstablished(2, true)

	// Peer 2 should expect routes from 0 and 1.
	assert.True(t, m.Expected(2).Contains(p("10.0.0.0/24")))
	assert.True(t, m.Expected(2).Contains(p("172.16.0.0/24")))
}

// TestModelDisconnectedPeerNoExpected verifies that a disconnected peer
// does not receive announcements from others.
//
// VALIDATES: Routes are not propagated to disconnected peers.
// PREVENTS: Validation expecting routes at unreachable peers.
func TestModelDisconnectedPeerNoExpected(t *testing.T) {
	m := NewModel(3)
	m.SetEstablished(0, true)
	m.SetEstablished(1, true)
	// Peer 2 is NOT established.

	m.Announce(0, p("10.0.0.0/24"))

	assert.True(t, m.Expected(1).Contains(p("10.0.0.0/24")))
	assert.Equal(t, 0, m.Expected(2).Len(), "disconnected peer should have no expected routes")
}

// TestModelExpectedLen verifies the count of expected routes per peer.
//
// VALIDATES: Expected set size is accurate.
// PREVENTS: Double-counting or missing routes in expected set.
func TestModelExpectedLen(t *testing.T) {
	m := NewModel(3)
	m.SetEstablished(0, true)
	m.SetEstablished(1, true)
	m.SetEstablished(2, true)

	m.Announce(0, p("10.0.0.0/24"))
	m.Announce(0, p("10.0.1.0/24"))
	m.Announce(1, p("172.16.0.0/24"))

	// Peer 2 should expect 3 routes (2 from peer 0 + 1 from peer 1).
	assert.Equal(t, 3, m.Expected(2).Len())
	// Peer 0 should expect 1 route (from peer 1).
	assert.Equal(t, 1, m.Expected(0).Len())
	// Peer 1 should expect 2 routes (from peer 0).
	assert.Equal(t, 2, m.Expected(1).Len())
}

// TestModelAnnouncedRoutes verifies that AnnouncedRoutes returns the set
// of routes a peer has announced.
//
// VALIDATES: Source tracking is accurate.
// PREVENTS: Lost track of which peer announced what.
func TestModelAnnouncedRoutes(t *testing.T) {
	m := NewModel(2)
	m.SetEstablished(0, true)
	m.SetEstablished(1, true)

	m.Announce(0, p("10.0.0.0/24"))
	m.Announce(0, p("10.0.1.0/24"))

	announced := m.AnnouncedRoutes(0)
	assert.Equal(t, 2, announced.Len())
	assert.True(t, announced.Contains(p("10.0.0.0/24")))
	assert.True(t, announced.Contains(p("10.0.1.0/24")))

	// Peer 1 announced nothing.
	assert.Equal(t, 0, m.AnnouncedRoutes(1).Len())
}
