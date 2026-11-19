// Design: docs/architecture/behavior/peer-lifecycle.md -- truthful terminal owner removal.
package reactor

import (
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"net/netip"
	"testing"
)

// A deconfigured Peer can have no physical Session. Its departure still ends
// that exact Peer owner, while absence of physical provenance stays explicit.
func TestRemovalWithoutSessionKeepsTruthfulOriginalOwner(t *testing.T) {
	r := newRemovalTestReactor()
	address := netip.MustParseAddr("192.0.2.14")
	insertTestPeer(r, address)
	peer := r.peers[peerKeyFromAddrPort(address, DefaultBGPPort)]
	require.Nil(t, peer.currentSession())
	originalOwner := peer.sourceOwner
	require.NotZero(t, originalOwner)
	removal, err := r.doRemovePeer(address, message.NotifyCeasePeerDeconfigured)
	require.NoError(t, err)
	require.Equal(t, originalOwner, removal.removed.PeerOwner)
	require.Zero(t, removal.removed.ConnectionID, "no physical identity may be invented for owner-terminal removal")
	recreated := NewPeer(peer.Settings())
	require.NotEqual(t, originalOwner, recreated.sourceOwner)
	require.Equal(t, originalOwner, removal.removed.PeerOwner, "the delayed terminal event cannot name the recreated Peer")
}
