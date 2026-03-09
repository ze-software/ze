package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPeerStatsInitialZero verifies counters start at zero.
//
// VALIDATES: New peers have zero update, keepalive, and EOR counters.
// PREVENTS: Counters starting with garbage values.
func TestPeerStatsInitialZero(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	stats := peer.Stats()
	assert.Equal(t, uint32(0), stats.UpdatesReceived)
	assert.Equal(t, uint32(0), stats.UpdatesSent)
	assert.Equal(t, uint32(0), stats.KeepalivesReceived)
	assert.Equal(t, uint32(0), stats.KeepalivesSent)
	assert.Equal(t, uint32(0), stats.EORReceived)
	assert.Equal(t, uint32(0), stats.EORSent)
}

// TestPeerStatsIncrement verifies counter increment methods.
//
// VALIDATES: IncrUpdatesReceived/Sent, IncrKeepalivesReceived/Sent, IncrEORReceived/Sent update counters.
// PREVENTS: Counters not being updated or updating the wrong counter.
func TestPeerStatsIncrement(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	peer.IncrUpdatesReceived()
	peer.IncrUpdatesReceived()
	peer.IncrUpdatesSent()
	peer.IncrKeepalivesReceived()
	peer.IncrKeepalivesReceived()
	peer.IncrKeepalivesReceived()
	peer.IncrKeepalivesSent()
	peer.IncrKeepalivesSent()
	peer.IncrEORReceived()
	peer.IncrEORSent()

	stats := peer.Stats()
	assert.Equal(t, uint32(2), stats.UpdatesReceived)
	assert.Equal(t, uint32(1), stats.UpdatesSent)
	assert.Equal(t, uint32(3), stats.KeepalivesReceived)
	assert.Equal(t, uint32(2), stats.KeepalivesSent)
	assert.Equal(t, uint32(1), stats.EORReceived)
	assert.Equal(t, uint32(1), stats.EORSent)
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

	peer.IncrUpdatesReceived()
	peer.IncrUpdatesSent()
	peer.IncrKeepalivesReceived()
	peer.IncrKeepalivesSent()
	peer.IncrEORReceived()
	peer.IncrEORSent()
	peer.SetEstablishedNow()

	peer.ClearStats()

	stats := peer.Stats()
	assert.Equal(t, uint32(0), stats.UpdatesReceived)
	assert.Equal(t, uint32(0), stats.UpdatesSent)
	assert.Equal(t, uint32(0), stats.KeepalivesReceived)
	assert.Equal(t, uint32(0), stats.KeepalivesSent)
	assert.Equal(t, uint32(0), stats.EORReceived)
	assert.Equal(t, uint32(0), stats.EORSent)
	assert.True(t, peer.EstablishedAt().IsZero())
}
