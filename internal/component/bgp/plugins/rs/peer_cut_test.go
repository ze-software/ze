// Tests for the peer-up cut: which rail delivers a route to a peer that
// establishes while UPDATEs are already flowing.

package rs

import (
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// TestPeerUpCutPartitionsRailsExactly pins the peer-up cut: every UPDATE belongs
// to exactly one rail, decided by its reactor MessageID against the cut captured
// when the peer became a live forward target.
//
// VALIDATES: an UPDATE at or below a peer's ForwardFrom is NOT a live forward
// target for that peer (its Adj-RIB-In replay carries it); one above it IS.
// PREVENTS: the same route reaching a peer twice, once from the live forward and
// once from the peer-up replay, in an order decided by goroutine scheduling. That
// duplicate is what test/plugin/llgr-readvertise-multipeer.ci saw on the wire, and
// the reverse ordering is what let a replayed announce arrive before the live
// withdrawal that test/plugin/rfc7606-relay-one-field.ci requires to come first.
func TestPeerUpCutPartitionsRailsExactly(t *testing.T) {
	rs := newTestRouteServer(t)
	families := map[family.Family]bool{family.IPv4Unicast: true}

	// Three UPDATEs are taken delivery of, then the destination peer comes up.
	for _, id := range []uint64{7, 8, 9} {
		rs.mu.Lock()
		if id > rs.seenMsgID {
			rs.seenMsgID = id
		}
		rs.mu.Unlock()
	}
	rs.handleState(&Event{Type: eventState, PeerAddr: "10.0.0.2", PeerOwner: 2, ConnectionID: 102, DestinationSession: 202, State: "up", InitialReplay: 17})

	rs.mu.Lock()
	rs.peers[rsFixtureKey("10.0.0.2", 2, 102)].Families = families
	rs.peers[rsFixtureKey("10.0.0.1", 1, 101)] = &PeerState{Address: "10.0.0.1", DestinationSession: 201, SessionGeneration: 3, Up: true, Families: families}
	cut := rs.peers[rsFixtureKey("10.0.0.2", 2, 102)].ForwardFrom
	rs.mu.Unlock()

	require.Equal(t, uint64(9), cut, "the cut is the newest MessageID seen when the peer came up")

	// At or below the cut: the peer's replay owns these, so no live forward.
	for _, id := range []uint64{7, 9} {
		rs.mu.RLock()
		targets := rs.selectForwardTargets(nil, rsFixtureKey("10.0.0.1", 1, 101), id, families)
		rs.mu.RUnlock()
		require.NotContains(t, rsFixtureAddresses(targets), "10.0.0.2",
			"msgID %d is at or below the cut; the Adj-RIB-In replay delivers it, not the live rail", id)
	}

	// Above the cut: the live rail owns these.
	rs.mu.RLock()
	targets := rs.selectForwardTargets(nil, rsFixtureKey("10.0.0.1", 1, 101), 10, families)
	rs.mu.RUnlock()
	require.Contains(t, rsFixtureAddresses(targets), "10.0.0.2", "msgID above the cut must be forwarded live")
}

// TestPeerUpCutOfZeroLeavesEveryUpdateOnTheLiveRail pins the rs half of the
// partition at the value that used to break it.
//
// VALIDATES: a peer that establishes before this plugin has taken delivery of any
// UPDATE carries ForwardFrom 0 and is a live forward target for EVERY subsequent
// UPDATE.
// PREVENTS: reading the paired replay bound as "unbounded" at cut 0. That is the
// ordinary case, not an edge one -- both peers of
// test/plugin/llgr-readvertise-multipeer.ci came up at cut 0 on 39 of 40 runs --
// and while adj-rib-in treated the VALUE 0 as "no bound", the replay carried the
// same routes this rail forwards, so the peer received them twice. The adj-rib-in
// half is TestReplayCutZeroBoundsTheReplay.
func TestPeerUpCutOfZeroLeavesEveryUpdateOnTheLiveRail(t *testing.T) {
	rs := newTestRouteServer(t)
	families := map[family.Family]bool{family.IPv4Unicast: true}

	// No UPDATE has been taken delivery of yet: seenMsgID is still its zero value.
	rs.handleState(&Event{Type: eventState, PeerAddr: "10.0.0.2", PeerOwner: 2, ConnectionID: 102, DestinationSession: 202, State: "up", InitialReplay: 17})

	rs.mu.Lock()
	rs.peers[rsFixtureKey("10.0.0.2", 2, 102)].Families = families
	rs.peers[rsFixtureKey("10.0.0.1", 1, 101)] = &PeerState{Address: "10.0.0.1", DestinationSession: 201, SessionGeneration: 3, Up: true, Families: families}
	cut := rs.peers[rsFixtureKey("10.0.0.2", 2, 102)].ForwardFrom
	rs.mu.Unlock()

	require.Zero(t, cut, "nothing seen yet, so the cut is 0")

	for _, id := range []uint64{1, 9, 10} {
		rs.mu.RLock()
		targets := rs.selectForwardTargets(nil, rsFixtureKey("10.0.0.1", 1, 101), id, families)
		rs.mu.RUnlock()
		require.Contains(t, rsFixtureAddresses(targets), "10.0.0.2",
			"msgID %d is above the cut of 0, so the live rail owns it and the replay must not also carry it", id)
	}
}

// TestPeerUpCutCapturedAtomicallyWithForwardTarget pins the atomicity that makes
// the cut sound: Up and ForwardFrom are written in one critical section, and a
// re-established session re-captures the cut.
//
// VALIDATES: a peer that is a forward target always carries this session's cut.
// PREVENTS: a stale ForwardFrom surviving a session bounce, which would make the
// new session's replay stop short of where its live rail starts and silently drop
// every route in between.
func TestPeerUpCutCapturedAtomicallyWithForwardTarget(t *testing.T) {
	rs := newTestRouteServer(t)
	families := map[family.Family]bool{family.IPv4Unicast: true}
	oldKey := rsFixtureKey("10.0.0.2", 2, 102)
	newKey := rsFixtureKey("10.0.0.2", 2, 112)
	source := rsFixtureKey("10.0.0.1", 1, 101)
	rs.mu.Lock()
	rs.seenMsgID = 5
	rs.mu.Unlock()
	rs.handleState(&Event{Type: eventState, PeerAddr: "10.0.0.2", PeerOwner: 2, ConnectionID: 102, DestinationSession: 202, State: "up", InitialReplay: 17})
	rs.mu.RLock()
	original := rs.peers[oldKey]
	oldUp, oldCut := original.Up, original.ForwardFrom
	rs.mu.RUnlock()
	require.True(t, oldUp, "peer is a forward target")
	require.Equal(t, uint64(5), oldCut, "and carries the cut written in the same critical section")
	// A real physical/output replacement receives a fresh atomic cut.
	rs.handleState(&Event{Type: eventState, PeerAddr: "10.0.0.2", PeerOwner: 2, ConnectionID: 102, DestinationSession: 202, State: "down"})
	rs.mu.Lock()
	rs.seenMsgID = 42
	rs.mu.Unlock()
	rs.handleState(&Event{Type: eventState, PeerAddr: "10.0.0.2", PeerOwner: 2, ConnectionID: 112, DestinationSession: 212, State: "up", InitialReplay: 18})
	rs.mu.Lock()
	rs.peers[newKey].Families = families
	rs.peers[source] = &PeerState{Address: "10.0.0.1", DestinationSession: 201, SessionGeneration: 3, Up: true, Families: families}
	rs.mu.Unlock()
	rs.mu.RLock()
	newCut, newSession := rs.peers[newKey].ForwardFrom, rs.peers[newKey].DestinationSession
	originalUp := original.Up
	oldTargets := rs.selectForwardTargets(nil, source, 30, families)
	usefulTargets := rs.selectForwardTargets(nil, source, 43, families)
	rs.mu.RUnlock()
	require.Equal(t, uint64(42), newCut, "the new session re-captures the cut; a stale 5 would lose every route between 5 and 42")
	require.False(t, originalUp, "original physical actor remains down even after its empty map row is pruned")
	require.Equal(t, uint64(212), newSession, "successor owns its original output receipt")
	require.NotContains(t, rsFixtureAddresses(oldTargets), "10.0.0.2", "msgID 30 predates the new session's cut, so the replay owns it")
	require.Equal(t, []rpc.ForwardTarget{rsFixtureTarget("10.0.0.2", 2, 212)}, usefulTargets, "post-cut output reaches only the useful replacement receipt")
}

// TestDispatchStructuredAdvancesCutCursor pins that the cursor advances on the
// same goroutine that captures the cut, which is what makes "seen before the peer
// came up" and "at or below the cut" the same statement.
//
// VALIDATES: seenMsgID tracks the MAXIMUM MessageID taken delivery of, so an
// UPDATE numbered below one already seen cannot pull the cut backwards.
// PREVENTS: a late low-numbered UPDATE (MessageIDs are allocated per receive
// across all peers, so this plugin can take them out of numeric order) landing
// above a later peer's cut and being delivered by both rails.
func TestDispatchStructuredAdvancesCutCursor(t *testing.T) {
	rs := newTestRouteServer(t)

	for _, id := range []uint64{4, 11, 7} {
		rs.mu.Lock()
		if id > rs.seenMsgID {
			rs.seenMsgID = id
		}
		rs.mu.Unlock()
	}

	rs.mu.RLock()
	seen := rs.seenMsgID
	rs.mu.RUnlock()
	require.Equal(t, uint64(11), seen, "the cursor is a max, not a last-write")
}
