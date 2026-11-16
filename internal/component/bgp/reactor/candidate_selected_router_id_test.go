// Design: docs/architecture/behavior/peer-lifecycle.md -- selected incoming identity.
// Related: incoming_ingress_test.go -- actual listener and OpenConfirm incumbent.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestCandidateOpenPromotionPublishesRemoteRouterID wins a real listener
// collision before the candidate advances its validated OPEN. The same Session
// and socket must establish with that OPEN's identifier after incumbent cleanup.
func TestCandidateOpenPromotionPublishesRemoteRouterID(t *testing.T) {
	r, peer, original, outgoing := incomingIngressPeer(t, 0, fsm.StateOpenConfirm)
	const incumbentID uint32 = 0x02020202
	const candidateID uint32 = 0x03030303
	require.Equal(t, incumbentID, peer.RemoteRouterID(), "the incumbent published its own validated OPEN")

	incoming := incomingListenerDial(t, r)
	delayOpenReadType(t, incoming, 1)
	var candidate *incomingCandidate
	delayOpenAwait(t, func() bool {
		peer.mu.RLock()
		defer peer.mu.RUnlock()
		candidate, _ = peer.pending.(*incomingCandidate)
		return candidate != nil
	})
	selected := candidate.session
	selectedConn := selected.Conn()
	require.NotNil(t, selectedConn)
	require.Equal(t, fsm.StateOpenSent, selected.State(), "the winning receipt must still need OPEN advancement")
	require.Same(t, original, peer.currentSession())
	require.Equal(t, incumbentID, peer.RemoteRouterID(), "an unopened candidate leaves the incumbent identity intact")

	delayOpenWrite(t, incoming, delayOpenRemote(candidateID))
	notification := delayOpenReadType(t, outgoing, 3)
	require.Equal(t, []byte{byte(message.NotifyCease), message.NotifyCeaseConnectionCollision}, notification[message.HeaderLen:])
	closeConnQuietly(outgoing)
	delayOpenReadType(t, incoming, 4)
	delayOpenWrite(t, incoming, &message.Keepalive{})
	delayOpenAwait(t, func() bool { return peer.SessionState() == fsm.StateEstablished })
	require.Nil(t, original.Conn(), "the incumbent's connection completed cleanup")
	require.Same(t, selected, peer.currentSession(), "promotion retains the original incoming Session")
	require.Same(t, selectedConn, selected.Conn(), "promotion retains the original incoming socket")
	require.Equal(t, candidateID, peer.RemoteRouterID(), "selection publishes the validated receipt's identifier after old-epoch cleanup")
}
