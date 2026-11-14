// Design: docs/architecture/api/commands.md -- the quiesce barrier, what a peer still owes
// Related: peer_sync_owed.go -- syncOwed, peerSyncOwedError
// Related: peer.go -- pendingSync, initialUpdateOwed, policyReplayHolds

package reactor

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
)

// syncOwedFenceOwner is the process the fenced peer in these tests waits for.
const syncOwedFenceOwner = "sync-owed-fence-fixture"

// registerSyncOwedFenceOwner registers a process that holds the replay fence,
// as bgp-rs, bgp-rr and bgp-rib do, and restores the registry after the test.
func registerSyncOwedFenceOwner(t *testing.T) {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:                syncOwedFenceOwner,
		Description:         "replay owner fixture that holds the replay fence",
		RunEngine:           func(net.Conn) int { return 0 },
		CLIHandler:          func([]string) int { return 0 },
		FencesLiveForwards:  true,
		SignalsSessionReady: true,
	}))
}

// establishSettledPeer brings a peer at address to Established through
// enterEstablished, the one production publisher of the state, then marks its
// initial sync finished: gate open, End-of-RIB written. A peer attached to the
// fence owner comes out with its replay fence up and nothing else owed.
func establishSettledPeer(t *testing.T, address string, fenced bool) (*Peer, *Session) {
	t.Helper()
	peer := NewPeer(NewPeerSettings(mustParseAddr(address), 65000, 65001, 0x01010101))
	if fenced {
		peer.mu.Lock()
		peer.settings.ProcessBindings = []ProcessBinding{{
			PluginName: syncOwedFenceOwner,
			Send:       map[string]bool{bgpevents.SendUpdate: true},
			Receive:    map[string]events.Direction{bgpevents.EventState: events.DirReceived},
		}}
		peer.mu.Unlock()
	}
	session := peer.testInitialSync()
	peer.enterEstablished(session)
	session.sendingInitialRoutes.Store(0)
	session.initialSyncEOROwed.Store(false)
	return peer, session
}

// TestPendingSyncWaitsForTheReplayFence proves that `request quiesce` waits
// for a peer-up replay that has not reported done.
//
// VALIDATES: an Established peer whose replay fence is up (Peer.initialUpdateOwed)
// reads as pending, with its initial sync finished and nothing queued, and
// reads as settled once the fence owner's "plugin session ready" lowers it.
// PREVENTS: test/plugin/rfc7999-blackhole-readvertise-agreement-forked.ci's
// failure under load. The End-of-RIB stopped waiting for route processes
// (owner ruling, 2026-09-18), so the initial-sync facts settle while a route
// server's replay to the peer is still passing; quiesce answered done, the
// fixture stopped ze, and the replay died with "no established peers to
// forward to" at the Cease.
func TestPendingSyncWaitsForTheReplayFence(t *testing.T) {
	registerSyncOwedFenceOwner(t)
	peer, session := establishSettledPeer(t, "192.0.2.1", true)
	require.True(t, peer.initialUpdateOwed.Load(), "setup: the fence owner must raise the replay fence at Established")

	require.True(t, peer.pendingSync(),
		"a peer whose peer-up replay has not reported done is still owed that replay")

	require.NoError(t, peer.SignalAPIReady(plugin.ProcessSender(syncOwedFenceOwner), session.initialReplay.Load()))
	require.False(t, peer.initialUpdateOwed.Load(), "setup: the owner's report must lower the fence")
	require.False(t, peer.pendingSync(),
		"once the replay owner has reported, the peer owes nothing more")
}

// TestPendingSyncWaitsForAPolicyReplay proves that `request quiesce` waits for
// a policy change that is re-sending routes to an established peer.
//
// VALIDATES: while a final policy restoration holds the peer
// (Peer.policyReplayHolds, beginPolicyReplay), the peer reads as pending, and
// it reads as settled once endPolicyReplay releases the hold.
// PREVENTS: a caller that changes a policy, quiesces and reads the wire, being
// told done while the restoration's pages are still being written.
func TestPendingSyncWaitsForAPolicyReplay(t *testing.T) {
	peer, session := establishSettledPeer(t, "192.0.2.1", false)
	require.False(t, peer.pendingSync(), "setup: an established peer with its sync done owes nothing")

	peer.beginPolicyReplay(session)
	require.True(t, peer.pendingSync(),
		"a peer whose routes a policy change is re-sending is still owed them")

	peer.endPolicyReplay()
	require.False(t, peer.pendingSync(),
		"once the restoration has released its hold, the peer owes nothing more")
}

// TestPendingSyncWaitsForAForwardOnTheChannel proves that `request quiesce`
// waits for a forwarded item that sits on the peer's worker channel.
//
// VALIDATES: an item TryDispatch put on the destination's channel
// (Peer.fwdChannelPending) keeps the peer pending until the worker has written
// it, and the peer reads as settled after.
// PREVENTS: the bgp-forward-pool barrier settling before a replay enqueues its
// routes (the quiescers run concurrently, quiesceAll), and the peer-level
// answer then calling the peer settled with those routes not written.
//
// Method: the destination's worker is stopped inside the batch that carries
// the item, before it takes session.writeMu, so the item is provably unwritten.
func TestPendingSyncWaitsForAForwardOnTheChannel(t *testing.T) {
	const announceID uint64 = 7900

	rail := newChannelOrderRail(t)
	dst := rail.dst
	dst.testInitialSync().initialSyncEOROwed.Store(false)
	require.False(t, dst.pendingSync(), "setup: the destination owes nothing before the forward")

	dst.mu.RLock()
	session := dst.session
	dst.mu.RUnlock()
	require.NotNil(t, session, "setup: the destination must hold a session")

	session.writeMu.Lock()
	_, dispatched := reactorForwardRS(rail.r, rail.publish(announceID, syncOrderAnnounceBody), announceID,
		netip.MustParseAddr(forwardSourceAddr), rail.src)
	session.writeMu.Unlock()
	require.Equal(t, 1, dispatched, "setup: the announce must be dispatched")

	awaitEntered(t, rail.entered)
	require.True(t, dst.forwardChannelPending(), "setup: the announce must be on the channel, unwritten")
	require.True(t, dst.pendingSync(),
		"a peer with a forwarded route still on its worker channel is owed that route")

	rail.release()
	require.Eventually(t, func() bool { return !dst.pendingSync() }, 5*time.Second, time.Millisecond,
		"once the worker has written the route, the peer owes nothing more")
}

// TestPendingSyncWaitsForAForwardInOverflow proves that `request quiesce`
// waits for a forwarded item that took the peer's overflow queue.
//
// VALIDATES: an item dispatchOverflow parked for the destination
// (Peer.fwdOverflowPending) keeps the peer pending while it is unwritten, with
// the channel count at zero, and the peer reads as settled once it is written.
// PREVENTS: the channel count alone answering for the queue: an item that
// leaves overflow for the channel is not counted there.
//
// Method: the worker is stopped inside the batch that carries the item, so the
// overflow count (which the worker re-derives only after that batch) is the
// only fact still naming it.
func TestPendingSyncWaitsForAForwardInOverflow(t *testing.T) {
	rail := newChannelOrderRail(t)
	dst := rail.dst
	dst.testInitialSync().initialSyncEOROwed.Store(false)
	require.False(t, dst.pendingSync(), "setup: the destination owes nothing before the forward")

	dst.mu.RLock()
	session := dst.session
	dst.mu.RUnlock()
	require.NotNil(t, session, "setup: the destination must hold a session")

	// A flush item carries no bytes, as handOffForwardFlush builds it, and it
	// is a real item of this destination's queue.
	item := fwdItem{peer: dst, session: session}
	require.True(t, rail.r.fwdPool.dispatchOverflow(fwdKey{peerAddr: dst.Settings().PeerKey()}, &item),
		"setup: the item must be parked")

	awaitEntered(t, rail.entered)
	require.False(t, dst.forwardChannelPending(), "setup: an item drained from overflow is not counted on the channel")
	require.True(t, dst.pendingSync(),
		"a peer with a forwarded item from its overflow queue still unwritten is owed that item")

	rail.release()
	require.Eventually(t, func() bool { return !dst.pendingSync() }, 5*time.Second, time.Millisecond,
		"once the worker has written the item, the peer owes nothing more")
}

// TestDrainPeerSyncErrorNamesWhatEachPeerOwes proves that a caller whose
// quiesce timed out can tell a replay still passing from an End-of-RIB still
// owed, and which process the replay waits for.
//
// VALIDATES: peerSyncOwedError names each owed peer by address with the plain
// words of its peerSyncOwed, the unreported fence owner beside a replay, and
// keeps the drain's context error as its cause.
// PREVENTS: "quiesce failed for: bgp-peer-sync" as the whole answer, which
// says neither which peer nor what work.
func TestDrainPeerSyncErrorNamesWhatEachPeerOwes(t *testing.T) {
	registerSyncOwedFenceOwner(t)
	fenced, _ := establishSettledPeer(t, "192.0.2.1", true)
	marker, markerSession := establishSettledPeer(t, "192.0.2.2", false)
	markerSession.initialSyncEOROwed.Store(true)
	settled, _ := establishSettledPeer(t, "192.0.2.3", false)

	err := peerSyncOwedError([]*Peer{fenced, marker, settled}, context.DeadlineExceeded)
	require.ErrorIs(t, err, context.DeadlineExceeded, "the drain's own error stays the cause")
	text := err.Error()
	require.Contains(t, text, "peer 192.0.2.1: "+peerSyncOwedReplay.String()+" (waiting for "+syncOwedFenceOwner+")")
	require.Contains(t, text, "peer 192.0.2.2: "+peerSyncOwedEndOfRIB.String())
	require.NotContains(t, text, "192.0.2.3", "a settled peer is not named")
}
