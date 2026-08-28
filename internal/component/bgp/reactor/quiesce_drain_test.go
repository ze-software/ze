package reactor

import (
	"context"
	"testing"
	"time"
)

// establishedPeer builds a minimal Peer in the Established state, optionally
// still in initial-route-sync (shouldQueue true). Only the fields State()/
// shouldQueue() read are set, which is all peersSynced inspects.
func establishedPeer(syncing bool) *Peer {
	p := &Peer{}
	p.state.Store(int32(PeerStateEstablished))
	if syncing {
		p.sendingInitialRoutes.Store(1)
	}
	return p
}

// TestPeersSyncedWaitsForSyncingPeer: an Established peer still draining its
// initial-sync opQueue is NOT synced.
//
// VALIDATES: AC-1 -- the barrier waits while a peer is mid initial-sync.
// PREVENTS: quiesce replying before opQueue routes reach the wire (nexthop race).
func TestPeersSyncedWaitsForSyncingPeer(t *testing.T) {
	peers := []*Peer{establishedPeer(false), establishedPeer(true)}
	if peersSynced(peers) {
		t.Fatal("peersSynced = true, want false (one peer still syncing)")
	}
}

// TestPeersSyncedTrueWhenAllDrained: all Established peers finished sync.
//
// VALIDATES: AC-2 -- drained peers report synced immediately.
func TestPeersSyncedTrueWhenAllDrained(t *testing.T) {
	peers := []*Peer{establishedPeer(false), establishedPeer(false)}
	if !peersSynced(peers) {
		t.Fatal("peersSynced = false, want true (all drained)")
	}
}

// TestPeersSyncedSkipsIdleNonEstablished: a non-Established peer with an EMPTY
// opQueue (down/idle, no pending routes) is skipped, not waited on.
//
// VALIDATES: AC-3 -- a down peer with no pending work never hangs the barrier.
// PREVENTS: DrainPeerSync blocking forever on an idle peer.
func TestPeersSyncedSkipsIdleNonEstablished(t *testing.T) {
	// A zero-value Peer is non-Established with an empty opQueue: nothing to drain.
	peers := []*Peer{{}, establishedPeer(false)}
	if !peersSynced(peers) {
		t.Fatal("peersSynced = false, want true (idle non-established peer must be skipped)")
	}
}

// queuedPeer builds a not-yet-Established peer with a route queued in its opQueue,
// as happens when a plugin send() arrives before the session establishes.
func queuedPeer() *Peer {
	p := &Peer{}
	p.opQueue = append(p.opQueue, peerOp{Type: PeerOpAnnounce})
	return p
}

// TestPeersSyncedWaitsForQueuedRoutesWhileEstablishing: a peer not yet
// Established but with routes queued IS waited on. This is the send()-during-
// establishment case the fixed sleep used to mask -- skipping it let the next
// send race ahead (the nexthop failure).
//
// VALIDATES: the fix -- the barrier waits for queued routes regardless of state.
// PREVENTS: DrainPeerSync returning before a queued route reaches the wire.
func TestPeersSyncedWaitsForQueuedRoutesWhileEstablishing(t *testing.T) {
	if peersSynced([]*Peer{queuedPeer()}) {
		t.Fatal("peersSynced = true, want false (peer has queued routes to deliver)")
	}
}

// TestWaitForConditionImmediateWhenTrue: returns nil at once when the condition
// already holds (no ticker wait).
//
// VALIDATES: AC-2 -- immediate return when drained.
func TestWaitForConditionImmediateWhenTrue(t *testing.T) {
	start := time.Now()
	if err := waitForCondition(context.Background(), time.Second, func() bool { return true }); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("took %v, should return immediately", elapsed)
	}
}

// TestWaitForConditionWaitsThenSucceeds: polls until the condition flips true.
//
// VALIDATES: AC-1 -- the poll returns as soon as the condition holds.
func TestWaitForConditionWaitsThenSucceeds(t *testing.T) {
	calls := 0
	cond := func() bool { calls++; return calls >= 3 }
	if err := waitForCondition(context.Background(), time.Millisecond, cond); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if calls < 3 {
		t.Errorf("condition checked %d times, want >= 3", calls)
	}
}

// TestWaitForConditionRespectsContext: a never-true condition is bounded by ctx.
//
// VALIDATES: R-2 -- a stuck drain cannot hang the barrier past the deadline.
// PREVENTS: DrainPeerSync blocking the daemon indefinitely.
func TestWaitForConditionRespectsContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := waitForCondition(ctx, time.Millisecond, func() bool { return false })
	if err == nil {
		t.Fatal("err = nil, want context deadline error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("took %v, should be bounded near the 50ms deadline", elapsed)
	}
}
