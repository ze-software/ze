// Design: docs/guide/configuration.md -- AS-wide identifier claims follow each connection.
// Related: peer_session_factory.go -- validator attached before either TCP rail.
// Related: rfc4271_incoming_listener_test.go -- actual listener and Peer lifecycle.
// RFC 6286 Section 2.1 -- see rfc/short/rfc6286.md.
package reactor

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestCandidateValidationPreservesIncumbentRouterID negotiates a second actual
// Session while a listener-created incumbent remains Established. The candidate
// must claim its identifier without replacing the incumbent's identity or claim.
func TestCandidateValidationPreservesIncumbentRouterID(t *testing.T) {
	r, peer, incumbent, _, _ := incomingListenerPeer(t, false, 0)
	remote := incomingListenerDial(t, r)
	delayOpenReadType(t, remote, 1)
	incomingListenerEstablish(t, peer, remote)
	const incumbentID uint32 = 0x02020202
	require.Equal(t, incumbentID, peer.RemoteRouterID())

	const candidateID uint32 = 0x03030303
	candidate, _ := candidateRouterIDSession(t, peer, candidateID)
	require.Equal(t, fsm.StateOpenConfirm, candidate.State())
	require.Same(t, incumbent, peer.currentSession())
	require.Equal(t, fsm.StateEstablished, incumbent.State())
	require.Equal(t, incumbentID, peer.RemoteRouterID(), "an unselected connection cannot publish its remote identifier")
	owner, held := r.routerIDs.holder(65001, incumbentID)
	require.True(t, held, "candidate validation must retain the incumbent's claim")
	require.Same(t, peer, owner)
	owner, held = r.routerIDs.holder(65001, candidateID)
	require.True(t, held, "the independently validated candidate holds its own claim")
	require.Same(t, peer, owner)
}

// TestIncumbentCleanupPreservesCandidateRouterID closes the actual incumbent
// transport so runOnce performs its production cleanup. A validated candidate
// must continue excluding another Peer until the owning Peer itself releases it.
func TestIncumbentCleanupPreservesCandidateRouterID(t *testing.T) {
	for _, candidateID := range []uint32{0x02020202, 0x03030303} {
		t.Run(bgpIDString(candidateID), func(t *testing.T) {
			r, peer, incumbent, _, _ := incomingListenerPeer(t, false, 0)
			remote := incomingListenerDial(t, r)
			delayOpenReadType(t, remote, 1)
			incomingListenerEstablish(t, peer, remote)
			candidate, _ := candidateRouterIDSession(t, peer, candidateID)
			require.Equal(t, fsm.StateOpenConfirm, candidate.State())

			closeConnQuietly(remote)
			delayOpenAwait(t, func() bool { return peer.currentSession() != incumbent })
			require.Nil(t, incumbent.Conn(), "the old connection actually finished cleanup")
			owner, held := r.routerIDs.holder(65001, candidateID)
			require.True(t, held, "old-epoch cleanup cannot release another connection's claim")
			require.Same(t, peer, owner)

			_, err := claimPeer(t, r, "192.0.2.99", 65001, candidateID)
			var conflict *routerIDConflictError
			require.ErrorAs(t, err, &conflict, "another Peer remains excluded by the candidate")
			peer.releaseRouterIDClaim()
			_, err = claimPeer(t, r, "192.0.2.98", 65001, candidateID)
			require.NoError(t, err, "peer-wide removal releases its last connection claim")
		})
	}
}

// candidateRouterIDSession runs the shared factory and ordinary wire OPEN
// validator on a real TCP pair. It leaves the Session unselected in OpenConfirm;
// the caller owns the Peer publication decision, and cleanup owns its timers.
func candidateRouterIDSession(t *testing.T, peer *Peer, identifier uint32) (*Session, net.Conn) {
	t.Helper()
	session := peer.newSession()
	t.Cleanup(func() {
		session.closeConn()
		session.stopSessionTimers()
	})
	require.NoError(t, session.Start())
	remote, local := collisionProofTCPPair(t)
	require.NoError(t, session.Accept(local))
	delayOpenReadType(t, remote, 1)
	delayOpenWrite(t, remote, delayOpenRemote(identifier))
	require.NoError(t, session.ReadAndProcess())
	delayOpenReadType(t, remote, 4)
	return session, remote
}

// TestCandidateOpenIdentitySurvivesDynamicPublication prepares a real candidate
// OPEN before the incumbent publishes its learned AS through the production
// dynamic-resolution path. The candidate must still admit a remote AS allowed
// by its own unresolved configuration and negotiate that AS over its own wire.
func TestCandidateOpenIdentitySurvivesDynamicPublication(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("127.0.0.1"), 65000, 0, 0x01010101)
	settings.Connection = ConnectionPassive
	settings.IsDynamic = true
	peer := NewPeer(settings)
	incumbent, incumbentRemote := candidateRouterIDSession(t, peer, 0x02020202)
	peer.mu.Lock()
	peer.session = incumbent
	peer.mu.Unlock()

	candidate := peer.newSession()
	t.Cleanup(func() {
		candidate.closeConn()
		candidate.stopSessionTimers()
	})
	require.NoError(t, candidate.Start())
	remote, local := collisionProofTCPPair(t)
	require.NoError(t, candidate.Accept(local))
	delayOpenReadType(t, remote, 1)

	delayOpenWrite(t, incumbentRemote, &message.Keepalive{})
	require.NoError(t, incumbent.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, incumbent.State())
	peer.resolveDynamicPeerSettings(incumbent)
	require.Equal(t, uint32(65001), peer.PeerAS(), "the incumbent actually published its learned AS")

	open := delayOpenRemote(0x03030303)
	open.MyAS = 65002
	delayOpenWrite(t, remote, open)
	require.NoError(t, candidate.ReadAndProcess(), "the candidate retains its own pre-publication OPEN admission facts")
	delayOpenReadType(t, remote, 4)
	require.Equal(t, fsm.StateOpenConfirm, candidate.State())
	require.NotNil(t, candidate.Negotiated())
	require.Equal(t, uint32(65002), candidate.Negotiated().PeerASN)
	require.Same(t, incumbent, peer.currentSession())
}

// TestCandidatePreparedAfterPublicationUsesCurrentAS supplies the opposite
// boundary: a candidate prepared after the incumbent's publication must refuse
// an unrelated AS and still accept the now-configured AS on a fresh connection.
func TestCandidatePreparedAfterPublicationUsesCurrentAS(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("127.0.0.1"), 65000, 0, 0x01010101)
	settings.Connection = ConnectionPassive
	settings.IsDynamic = true
	peer := NewPeer(settings)
	incumbent, incumbentRemote := candidateRouterIDSession(t, peer, 0x02020202)
	peer.mu.Lock()
	peer.session = incumbent
	peer.mu.Unlock()
	delayOpenWrite(t, incumbentRemote, &message.Keepalive{})
	require.NoError(t, incumbent.ReadAndProcess())
	peer.resolveDynamicPeerSettings(incumbent)
	require.Equal(t, uint32(65001), peer.PeerAS())

	candidate := peer.newSession()
	t.Cleanup(func() {
		candidate.closeConn()
		candidate.stopSessionTimers()
	})
	require.NoError(t, candidate.Start())
	remote, local := collisionProofTCPPair(t)
	require.NoError(t, candidate.Accept(local))
	delayOpenReadType(t, remote, 1)
	open := delayOpenRemote(0x03030303)
	open.MyAS = 65002
	delayOpenWrite(t, remote, open)
	require.ErrorIs(t, candidate.ReadAndProcess(), ErrPeerASMismatch)
	notification := delayOpenReadType(t, remote, 3)
	require.Equal(t, []byte{2, 2}, notification[19:21], "the configured AS mismatch retains OPEN/Bad Peer AS")
	require.Empty(t, notification[21:])
	require.Same(t, incumbent, peer.currentSession())
	require.Equal(t, fsm.StateEstablished, incumbent.State())

	accepted, _ := candidateRouterIDSession(t, peer, 0x04040404)
	require.Equal(t, fsm.StateOpenConfirm, accepted.State())
	require.NotNil(t, accepted.Negotiated())
	require.Equal(t, uint32(65001), accepted.Negotiated().PeerASN)
}
