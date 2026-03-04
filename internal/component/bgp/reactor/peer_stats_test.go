package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPeerStatsInitialZero verifies counters start at zero.
//
// VALIDATES: New peers have zero message and route counters.
// PREVENTS: Counters starting with garbage values.
func TestPeerStatsInitialZero(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	stats := peer.Stats()
	assert.Equal(t, uint64(0), stats.MessagesReceived)
	assert.Equal(t, uint64(0), stats.MessagesSent)
	assert.Equal(t, uint32(0), stats.RoutesReceived)
	assert.Equal(t, uint32(0), stats.RoutesSent)
}

// TestPeerStatsIncrement verifies counter increment methods.
//
// VALIDATES: IncrMessageReceived/Sent and IncrRoutesReceived/Sent update counters.
// PREVENTS: Counters not being updated or updating the wrong counter.
func TestPeerStatsIncrement(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	peer.IncrMessageReceived()
	peer.IncrMessageReceived()
	peer.IncrMessageSent()
	peer.IncrRoutesReceived(5)
	peer.IncrRoutesSent(3)

	stats := peer.Stats()
	assert.Equal(t, uint64(2), stats.MessagesReceived)
	assert.Equal(t, uint64(1), stats.MessagesSent)
	assert.Equal(t, uint32(5), stats.RoutesReceived)
	assert.Equal(t, uint32(3), stats.RoutesSent)
}

// TestPeerEstablishedAt verifies per-peer uptime tracking.
//
// VALIDATES: SetEstablishedNow records a timestamp, EstablishedAt returns it.
// PREVENTS: Uptime using reactor start time instead of per-peer session time.
func TestPeerEstablishedAt(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	require.True(t, peer.EstablishedAt().IsZero(), "should be zero before establishment")

	peer.SetEstablishedNow()

	require.False(t, peer.EstablishedAt().IsZero(), "should be non-zero after establishment")
}

// TestPeerStatsClearOnReset verifies counters can be cleared.
//
// VALIDATES: ClearStats resets all counters and established timestamp.
// PREVENTS: Stale counters surviving session reset.
func TestPeerStatsClearOnReset(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	peer.IncrMessageReceived()
	peer.IncrMessageSent()
	peer.IncrRoutesReceived(10)
	peer.IncrRoutesSent(5)
	peer.SetEstablishedNow()

	peer.ClearStats()

	stats := peer.Stats()
	assert.Equal(t, uint64(0), stats.MessagesReceived)
	assert.Equal(t, uint64(0), stats.MessagesSent)
	assert.Equal(t, uint32(0), stats.RoutesReceived)
	assert.Equal(t, uint32(0), stats.RoutesSent)
	assert.True(t, peer.EstablishedAt().IsZero())
}
