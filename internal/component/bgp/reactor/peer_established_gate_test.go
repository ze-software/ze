// VALIDATES: publishing PeerStateEstablished closes the session's initial-sync
//            gate FIRST, so no observer of the state can see an established peer
//            whose route ops still bypass opQueue and whose initial sync reads as
//            settled.
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
// The tests drive the state through enterEstablished, the one production
// publisher of PeerStateEstablished, which holds the session the gate belongs
// to. They fail if the gate store moves back after the publication.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSetStateEstablished_ClosesGateBeforePublishing observes the gate from
// inside the peer's own state callback. The callback is the production observer
// contract (Peer.SetCallback), and it runs synchronously from setState, which
// enterEstablished calls, so what it reads is exactly what any goroutine reads
// the moment Established becomes visible.
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

	peer.enterEstablished(peer.testInitialSync())

	require.Len(t, seen, 1, "the state callback must fire once for the transition into Established")
	require.True(t, seen[0].shouldQueue,
		"shouldQueue must already be true when Established becomes visible, or a plugin route overtakes the initial-sync End-of-RIB")
	require.True(t, seen[0].pendingSync,
		"pendingSync must already be true when Established becomes visible, or DrainPeerSync reports a peer settled before its initial sync starts")
}

// TestEstablishedPublishedWithGateClosed observes, from the same state callback,
// the two facts the lock-free readers depend on: the session published as the
// peer's established one (Peer.establishedSession) is the session the peer holds
// (Peer.session), and that session's own gate is already closed.
//
// VALIDATES: A-3 and the forward worker's half of AC-6. enterEstablished stores
// the session's gate and publishes the session before it publishes Established.
// PREVENTS: the forward worker's hold (forwardOrderHold, which reads the gate
// through Peer.establishedSession and never takes p.mu) reading no session, or a
// session the peer no longer holds, at the instant Established is visible, and
// so releasing a forwarded UPDATE ahead of the initial sync.
func TestEstablishedPublishedWithGateClosed(t *testing.T) {
	peer := newTestPeer()
	session := peer.testInitialSync()
	peer.establishedSession.Store(nil)

	type observation struct {
		published *Session
		held      *Session
		gate      int32
	}
	var seen []observation
	peer.SetCallback(func(_, to PeerState) {
		if to != PeerStateEstablished {
			return
		}
		published := peer.establishedSession.Load()
		var gate int32 = -1
		if published != nil {
			gate = published.sendingInitialRoutes.Load()
		}
		seen = append(seen, observation{published: published, held: peer.currentSession(), gate: gate})
	})

	peer.enterEstablished(session)

	require.Len(t, seen, 1, "the state callback must fire once for the transition into Established")
	require.NotNil(t, seen[0].published,
		"no session was published as established when Established became visible")
	require.Same(t, seen[0].held, seen[0].published,
		"the session published as established is not the session the peer holds")
	require.Equal(t, int32(1), seen[0].gate,
		"the established session's initial-sync gate was not closed when Established became visible")
}

// TestPeersSynced_NotSyncedForJustEstablishedPeer pins the barrier half at the
// level the quiescer uses it. peersSynced backs DrainPeerSync (reactor_api.go),
// registered as the bgp-peer-sync quiescer, which is what `request quiesce` and
// ze_api.py's quiesce()/wait_for_ack() block on.
func TestPeersSynced_NotSyncedForJustEstablishedPeer(t *testing.T) {
	peer := newTestPeer()
	require.True(t, peersSynced([]*Peer{peer}),
		"an idle peer with an empty queue has no pending route work")

	session := peer.testInitialSync()
	peer.enterEstablished(session)
	require.False(t, peersSynced([]*Peer{peer}),
		"a peer that has just been published Established still owes its initial route sync")

	// What sendInitialRoutes does when it has drained the queue: it opens the
	// session's queueing gate BEFORE it writes the End-of-RIB, so the gate alone
	// does not settle the peer.
	session.sendingInitialRoutes.Store(0)
	require.False(t, peersSynced([]*Peer{peer}),
		"a peer whose End-of-RIB is still owed is not settled, or `request quiesce` returns "+
			"while the marker that closes the initial update is still to come")

	// And what it does once every family's End-of-RIB is on the wire.
	session.initialSyncEOROwed.Store(false)
	require.True(t, peersSynced([]*Peer{peer}),
		"once the initial sync has finished the peer is settled")
}
