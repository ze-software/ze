// Design: docs/architecture/behavior/peer-lifecycle.md -- exact physical departures.
package reactor

import (
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"net/netip"
	"testing"
)

// The actual TCP session close carries the same immutable token as its UP.
// MUTATION: omit closed.connectionID in sessionDown.
func TestPhysicalConnectionDepartureUsesEndedSocket(t *testing.T) {
	link := startMPLinkNeighbor(t)
	original := link.peer.currentSession()
	require.NotNil(t, original)
	require.NotZero(t, original.connectionID)
	require.Equal(t, original.connectionID, establishedPeerInfo(link.peer).ConnectionID)
	require.NoError(t, link.conn.Close())
	closed := awaitClose(t, link)
	require.Equal(t, original.connectionID, closed.connectionID)
	require.NotEqual(t, original.initialReplay.Load(), closed.connectionID, "initial replay rotated on actual establishment")
}

// Removal captures before graph publication/stop; changing the pointer later
// must not change the already-captured departure value.
// MUTATION: omit ConnectionID in removalLocked.
func TestPhysicalConnectionRemovalCapturesOriginalBeforeReplacement(t *testing.T) {
	r := newRemovalTestReactor()
	address := netip.MustParseAddr("192.0.2.14")
	insertTestPeer(r, address)
	peer := r.peers[peerKeyFromAddrPort(address, DefaultBGPPort)]
	original := peer.newSession()
	peer.session = original
	removal, err := r.doRemovePeer(address, message.NotifyCeasePeerDeconfigured)
	require.NoError(t, err)
	require.Equal(t, original.connectionID, removal.removed.ConnectionID)
	replacement := peer.newSession()
	peer.session = replacement
	require.NotEqual(t, original.connectionID, replacement.connectionID)
	require.Equal(t, original.connectionID, removal.removed.ConnectionID)
}
