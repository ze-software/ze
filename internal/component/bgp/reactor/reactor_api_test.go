package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPeerInfoPopulatesStats verifies that Peers() populates message and route counters.
//
// VALIDATES: reactorAPIAdapter.Peers() returns non-zero statistics from peer counters.
// PREVENTS: Stats fields remaining zero despite counter increments.
func TestPeerInfoPopulatesStats(t *testing.T) {
	r := New(&Config{})
	r.startTime = time.Now()

	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)
	peer.IncrMessageReceived()
	peer.IncrMessageReceived()
	peer.IncrMessageReceived()
	peer.IncrMessageSent()
	peer.IncrMessageSent()
	peer.IncrRoutesReceived(10)
	peer.IncrRoutesSent(5)
	peer.SetEstablishedNow()
	peer.state.Store(int32(PeerStateEstablished))

	r.peers[settings.PeerKey()] = peer

	adapter := &reactorAPIAdapter{r: r}
	peers := adapter.Peers()

	require.Len(t, peers, 1)
	p := peers[0]

	assert.Equal(t, uint64(3), p.MessagesReceived, "messages received")
	assert.Equal(t, uint64(2), p.MessagesSent, "messages sent")
	assert.Equal(t, uint32(10), p.RoutesReceived, "routes received")
	assert.Equal(t, uint32(5), p.RoutesSent, "routes sent")
	assert.True(t, p.Uptime > 0, "uptime should be non-zero for established peer")
}

// TestPeerInfoUptimeUsesEstablishedAt verifies per-peer uptime, not reactor start time.
//
// VALIDATES: Uptime comes from peer's EstablishedAt, not reactor.startTime.
// PREVENTS: All peers showing the same uptime regardless of when they established.
func TestPeerInfoUptimeUsesEstablishedAt(t *testing.T) {
	r := New(&Config{})
	r.startTime = time.Now().Add(-1 * time.Hour) // reactor started 1 hour ago

	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	// Peer established just now — uptime should be ~0, not ~1 hour
	peer.SetEstablishedNow()

	r.peers[settings.PeerKey()] = peer

	adapter := &reactorAPIAdapter{r: r}
	peers := adapter.Peers()

	require.Len(t, peers, 1)
	// Uptime should be close to 0, not close to 1 hour
	assert.Less(t, peers[0].Uptime, 10*time.Second, "uptime should reflect peer establishment, not reactor start")
}

// TestPeerInfoNonEstablishedZeroUptime verifies non-established peers have zero uptime.
//
// VALIDATES: Peers not in Established state have zero Uptime.
// PREVENTS: Non-established peers showing stale uptime from previous session.
func TestPeerInfoNonEstablishedZeroUptime(t *testing.T) {
	r := New(&Config{})
	r.startTime = time.Now()

	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)
	// Not established — state defaults to Idle (0)

	r.peers[settings.PeerKey()] = peer

	adapter := &reactorAPIAdapter{r: r}
	peers := adapter.Peers()

	require.Len(t, peers, 1)
	assert.Equal(t, time.Duration(0), peers[0].Uptime, "non-established peer should have zero uptime")
}
