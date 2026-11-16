package reactor

import (
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// VALIDATES: Peer connection management (pending, inbound, collision detection).
// PREVENTS: Connection leaks, double-accept, nil panics on connection operations.

// TestPeerPendingConnection_SetAndHas verifies setPendingConnection stores connection.
func TestPeerPendingConnection_SetAndHas(t *testing.T) {
	peer := newTestPeer()
	client, server := net.Pipe()
	defer client.Close() //nolint:errcheck // test cleanup
	defer server.Close() //nolint:errcheck // test cleanup

	require.False(t, peer.hasPendingConnection())

	err := peer.setPendingConnection(client)
	require.NoError(t, err)
	assert.True(t, peer.hasPendingConnection())
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

	require.NoError(t, peer.setPendingConnection(c1))
	err := peer.setPendingConnection(c2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// TestPeerPendingConnection_Clear verifies clearPendingConnection resets state.
func TestPeerPendingConnection_Clear(t *testing.T) {
	peer := newTestPeer()
	client, server := net.Pipe()
	defer client.Close() //nolint:errcheck // test cleanup
	defer server.Close() //nolint:errcheck // test cleanup

	require.NoError(t, peer.setPendingConnection(client))
	assert.True(t, peer.hasPendingConnection())

	peer.clearPendingConnection()
	assert.False(t, peer.hasPendingConnection())
}

// TestPeerInboundConnection_SetAndTake verifies inbound connection round-trip.
func TestPeerInboundConnection_SetAndTake(t *testing.T) {
	peer := newTestPeer()
	client, server := net.Pipe()
	defer client.Close() //nolint:errcheck // test cleanup
	defer server.Close() //nolint:errcheck // test cleanup

	peer.setInboundConnection(client)

	taken := peer.takeInboundConnection()
	assert.Equal(t, client, inboundConn(taken))

	// Second take returns nil.
	taken2 := peer.takeInboundConnection()
	assert.Nil(t, inboundConn(taken2))
}

// TestPeerInboundConnection_ReplaceClosesOld verifies replacement closes previous connection.
func TestPeerInboundConnection_ReplaceClosesOld(t *testing.T) {
	peer := newTestPeer()
	old, oldServer := net.Pipe()
	defer oldServer.Close() //nolint:errcheck // test cleanup

	newConn, newServer := net.Pipe()
	defer newConn.Close()   //nolint:errcheck // test cleanup
	defer newServer.Close() //nolint:errcheck // test cleanup

	peer.setInboundConnection(old)
	peer.setInboundConnection(newConn)

	// Old connection should be closed — writing to old server side should fail.
	buf := make([]byte, 1)
	_, err := oldServer.Read(buf)
	require.Error(t, err, "old connection should be closed after replacement")

	// New connection should be takeable.
	taken := peer.takeInboundConnection()
	assert.Equal(t, newConn, inboundConn(taken))
}

// TestPeerResolvePendingCollision_NoPending removes an admitted candidate before
// its remote OPEN arrives. Its stale reader must not transfer an unowned winner
// or replace the original Session; the actual caller retires that exact socket.
func TestPeerResolvePendingCollision_NoPending(t *testing.T) {
	session, existing, current := setupOpenConfirmSession(t, 0x01020301)
	t.Cleanup(func() { closeConnQuietly(existing); closeConnQuietly(current) })
	peer := NewPeer(session.settings)
	peer.session = session
	client, server, candidate := collisionCandidateInput(t, peer)
	peer.clearPendingConnection()
	delayOpenWrite(t, client, &message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0xffffffff})
	requireCollisionCease(t, client, "the canceled admission")
	var octet [1]byte
	n, err := client.Read(octet[:])
	require.Zero(t, n)
	require.ErrorIs(t, err, io.EOF)
	assert.Same(t, server, candidate.conn, "the stale owner retires its exact socket")
	assert.False(t, peer.hasPendingConnection())
	peer.mu.RLock()
	retained := inboundConn(peer.inbound)
	peer.mu.RUnlock()
	assert.Nil(t, retained, "no pending admission means no winner")
	assert.Same(t, session, peer.currentSession())
	assert.Same(t, current, session.Conn())
	assert.Equal(t, fsm.StateOpenConfirm, session.State())
}

// TestPeerResolvePendingCollision_NoSession retires the incumbent after incoming
// admission but before its OPEN. With no selected Session at resolution, the
// actual caller rejects and closes the exact incoming socket without a handoff.
func TestPeerResolvePendingCollision_NoSession(t *testing.T) {
	session, existing, current := setupOpenConfirmSession(t, 0x01020301)
	t.Cleanup(func() { closeConnQuietly(existing); closeConnQuietly(current) })
	peer := NewPeer(session.settings)
	peer.session = session
	client, server, candidate := collisionCandidateInput(t, peer)
	require.NoError(t, session.Stop())
	session.closeConn()
	peer.mu.Lock()
	peer.session = nil
	peer.mu.Unlock()
	delayOpenWrite(t, client, &message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0xffffffff})
	requireCollisionCease(t, client, "the incoming connection with no incumbent")
	var octet [1]byte
	n, err := client.Read(octet[:])
	require.Zero(t, n)
	require.ErrorIs(t, err, io.EOF, "the caller handles ownership by closing its rejected socket")
	assert.Same(t, server, candidate.conn, "the actual reader owns the exact rejected socket")
	assert.Nil(t, peer.currentSession(), "no replacement Session is published")
	peer.mu.RLock()
	retained := inboundConn(peer.inbound)
	peer.mu.RUnlock()
	assert.Nil(t, retained, "no incoming winner is transferred")
	assert.False(t, peer.hasPendingConnection(), "pending should be cleared")
}
