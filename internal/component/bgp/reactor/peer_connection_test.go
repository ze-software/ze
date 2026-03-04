package reactor

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: Peer connection management (pending, inbound, collision detection).
// PREVENTS: Connection leaks, double-accept, nil panics on connection operations.

// TestPeerPendingConnection_SetAndHas verifies SetPendingConnection stores connection.
func TestPeerPendingConnection_SetAndHas(t *testing.T) {
	peer := newTestPeer()
	client, server := net.Pipe()
	defer client.Close() //nolint:errcheck // test cleanup
	defer server.Close() //nolint:errcheck // test cleanup

	require.False(t, peer.HasPendingConnection())

	err := peer.SetPendingConnection(client)
	require.NoError(t, err)
	assert.True(t, peer.HasPendingConnection())
}

// TestPeerPendingConnection_AlreadyExists verifies error on double-set.
func TestPeerPendingConnection_AlreadyExists(t *testing.T) {
	peer := newTestPeer()
	c1, s1 := net.Pipe()
	defer c1.Close() //nolint:errcheck // test cleanup
	defer s1.Close() //nolint:errcheck // test cleanup
	c2, s2 := net.Pipe()
	defer c2.Close() //nolint:errcheck // test cleanup
	defer s2.Close() //nolint:errcheck // test cleanup

	require.NoError(t, peer.SetPendingConnection(c1))
	err := peer.SetPendingConnection(c2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// TestPeerPendingConnection_Clear verifies ClearPendingConnection resets state.
func TestPeerPendingConnection_Clear(t *testing.T) {
	peer := newTestPeer()
	client, server := net.Pipe()
	defer client.Close() //nolint:errcheck // test cleanup
	defer server.Close() //nolint:errcheck // test cleanup

	require.NoError(t, peer.SetPendingConnection(client))
	assert.True(t, peer.HasPendingConnection())

	peer.ClearPendingConnection()
	assert.False(t, peer.HasPendingConnection())
}

// TestPeerInboundConnection_SetAndTake verifies inbound connection round-trip.
func TestPeerInboundConnection_SetAndTake(t *testing.T) {
	peer := newTestPeer()
	client, server := net.Pipe()
	defer client.Close() //nolint:errcheck // test cleanup
	defer server.Close() //nolint:errcheck // test cleanup

	peer.SetInboundConnection(client)

	taken := peer.takeInboundConnection()
	assert.Equal(t, client, taken)

	// Second take returns nil.
	taken2 := peer.takeInboundConnection()
	assert.Nil(t, taken2)
}

// TestPeerInboundConnection_ReplaceClosesOld verifies replacement closes previous connection.
func TestPeerInboundConnection_ReplaceClosesOld(t *testing.T) {
	peer := newTestPeer()
	old, oldServer := net.Pipe()
	defer oldServer.Close() //nolint:errcheck // test cleanup

	newConn, newServer := net.Pipe()
	defer newConn.Close()   //nolint:errcheck // test cleanup
	defer newServer.Close() //nolint:errcheck // test cleanup

	peer.SetInboundConnection(old)
	peer.SetInboundConnection(newConn)

	// Old connection should be closed — writing to old server side should fail.
	buf := make([]byte, 1)
	_, err := oldServer.Read(buf)
	require.Error(t, err, "old connection should be closed after replacement")

	// New connection should be takeable.
	taken := peer.takeInboundConnection()
	assert.Equal(t, newConn, taken)
}

// TestPeerResolvePendingCollision_NoPending verifies no-op when no pending connection.
func TestPeerResolvePendingCollision_NoPending(t *testing.T) {
	peer := newTestPeer()

	accept, conn, open, wait := peer.ResolvePendingCollision(nil)
	assert.False(t, accept)
	assert.Nil(t, conn)
	assert.Nil(t, open)
	assert.Nil(t, wait)
}

// TestPeerResolvePendingCollision_NoSession verifies rejection when session is nil.
func TestPeerResolvePendingCollision_NoSession(t *testing.T) {
	peer := newTestPeer()
	client, server := net.Pipe()
	defer client.Close() //nolint:errcheck // test cleanup
	defer server.Close() //nolint:errcheck // test cleanup

	// Manually set pending connection (bypassing session check).
	peer.mu.Lock()
	peer.pendingConn = client
	peer.mu.Unlock()

	accept, conn, _, _ := peer.ResolvePendingCollision(nil)
	assert.False(t, accept)
	assert.Equal(t, client, conn, "should return the pending connection for caller to handle")
	assert.False(t, peer.HasPendingConnection(), "pending should be cleared")
}

// TestPeerAcceptConnectionWithOpen_NoSession verifies error when no session.
func TestPeerAcceptConnectionWithOpen_NoSession(t *testing.T) {
	peer := newTestPeer()
	err := peer.AcceptConnectionWithOpen(nil, nil)
	require.ErrorIs(t, err, ErrNotConnected)
}
