// Design: docs/architecture/core-design.md -- reactor peer opQueue and the initial-sync gate.
// Related: peer.go -- queueGateClosedLocked, QueueAnnounce, QueueWithdraw.
// Related: peer_initial_sync.go -- drainAndCloseQueueGate, the only drainer.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestQueueRefusedAfterGateOpens replays the interleaving that parked a plugin
// route for the life of a session (test/plugin/event-predicate-wait.ci): the
// announce rail reads shouldQueue while the initial sync is still owed, the
// sync's final drain then empties the queue and opens the gate, and only then
// does the rail call QueueAnnounce. Nothing drains the queue after that point,
// so an append there is a route the peer never receives. The queue MUST refuse
// with errQueueGateOpen, leave the gate open, and let the rail send directly.
func TestQueueRefusedAfterGateOpens(t *testing.T) {
	peer := NewPeer(NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101))
	peer.setState(PeerStateEstablished)
	require.True(t, peer.shouldQueue(), "the rail reads the gate closed while the sync is owed")

	// The initial sync's final drain runs between the rail's read and its enqueue.
	require.False(t, peer.drainAndCloseQueueGate("192.0.2.1", 4096), "an empty drain tears nothing down")

	route := testRoute("203.0.113.0/24")
	require.ErrorIs(t, peer.QueueAnnounce(route, false, false), errQueueGateOpen)
	require.ErrorIs(t, peer.QueueWithdraw(route.NLRI()), errQueueGateOpen)
	peer.mu.RLock()
	parked := len(peer.opQueue)
	peer.mu.RUnlock()
	require.Zero(t, parked, "no operation may be parked behind a finished drain")
	require.False(t, peer.shouldQueue(), "the gate stays open, so the next announce goes out directly")
}
