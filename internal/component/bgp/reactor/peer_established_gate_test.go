// VALIDATES: publishing PeerStateEstablished closes the initial-sync gate FIRST,
//            so no observer of the state can see an established peer whose route
//            ops still bypass opQueue and whose initial sync reads as settled.
// PREVENTS:  the reordering that made test/plugin/mup-ipv4-announce.ci and
//            test/plugin/plugin-metrics-registered.ci fail on an oversubscribed
//            CI host. The gate used to be closed 39 lines after the publication
//            (peer_run.go's FSM callback), and everything in between --
//            setEstablishedNow, the GR EoR timer, resolveDynamicPeerSettings, a
//            synchronous Info log, resetAPISync, ResetPeerUpBarrier -- held that
//            window open. A plugin pushing a route inside it had the route
//            written DIRECT to the session, ahead of the End-of-RIB
//            sendInitialRoutes had not started emitting (RFC 4724 Section 2), and
//            `request quiesce` returned "settled" so the plugin sent its next
//            route into the same window. mup4's wire came out announce,
//            withdraw, EoR instead of announce, EoR, withdraw.
//
// Both tests drive the state through setState, the single publication point, and
// both fail if the store moves back after the publication.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSetStateEstablished_ClosesGateBeforePublishing observes the gate from
// inside the peer's own state callback. The callback is the production observer
// contract (Peer.SetCallback), and it runs synchronously from setState, so what
// it reads is exactly what any goroutine reads the moment Established becomes
// visible.
func TestSetStateEstablished_ClosesGateBeforePublishing(t *testing.T) {
	peer := newTestPeer()

	type observation struct {
		shouldQueue bool
		pendingSync bool
	}
	var seen []observation
	peer.SetCallback(func(_, to PeerState) {
		if to != PeerStateEstablished {
			return
		}
		seen = append(seen, observation{
			shouldQueue: peer.shouldQueue(),
			pendingSync: peer.pendingSync(),
		})
	})

	peer.setState(PeerStateEstablished)

	require.Len(t, seen, 1, "the state callback must fire once for the transition into Established")
	require.True(t, seen[0].shouldQueue,
		"shouldQueue must already be true when Established becomes visible, or a plugin route overtakes the initial-sync End-of-RIB")
	require.True(t, seen[0].pendingSync,
		"pendingSync must already be true when Established becomes visible, or DrainPeerSync reports a peer settled before its initial sync starts")
}

// TestPeersSynced_NotSyncedForJustEstablishedPeer pins the barrier half at the
// level the quiescer uses it. peersSynced backs DrainPeerSync (reactor_api.go),
// registered as the bgp-peer-sync quiescer, which is what `request quiesce` and
// ze_api.py's quiesce()/wait_for_ack() block on.
func TestPeersSynced_NotSyncedForJustEstablishedPeer(t *testing.T) {
	peer := newTestPeer()
	require.True(t, peersSynced([]*Peer{peer}),
		"an idle peer with an empty queue has no pending route work")

	peer.setState(PeerStateEstablished)
	require.False(t, peersSynced([]*Peer{peer}),
		"a peer that has just been published Established still owes its initial route sync")

	// What sendInitialRoutes does when it has drained the queue and put every
	// family's End-of-RIB on the wire.
	peer.sendingInitialRoutes.Store(0)
	require.True(t, peersSynced([]*Peer{peer}),
		"once the initial sync has finished the peer is settled")
}
