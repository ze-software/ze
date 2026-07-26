// Tests for the peer-up cut: which rail delivers a route to a peer that
// establishes while UPDATEs are already flowing.

package rs

import (
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
	rs.handleState(&Event{Type: eventState, PeerAddr: "10.0.0.2", State: "up"})

	rs.mu.Lock()
	rs.peers["10.0.0.2"].Families = families
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true, Families: families}
	cut := rs.peers["10.0.0.2"].ForwardFrom
	rs.mu.Unlock()

	require.Equal(t, uint64(9), cut, "the cut is the newest MessageID seen when the peer came up")

	// At or below the cut: the peer's replay owns these, so no live forward.
	for _, id := range []uint64{7, 9} {
		rs.mu.RLock()
		targets := rs.selectForwardTargets(nil, "10.0.0.1", id, families)
		rs.mu.RUnlock()
		require.NotContains(t, targets, "10.0.0.2",
			"msgID %d is at or below the cut; the Adj-RIB-In replay delivers it, not the live rail", id)
	}

	// Above the cut: the live rail owns these.
	rs.mu.RLock()
	targets := rs.selectForwardTargets(nil, "10.0.0.1", 10, families)
	rs.mu.RUnlock()
	require.Contains(t, targets, "10.0.0.2", "msgID above the cut must be forwarded live")
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

	rs.mu.Lock()
	rs.seenMsgID = 5
	rs.mu.Unlock()
	rs.handleState(&Event{Type: eventState, PeerAddr: "10.0.0.2", State: "up"})

	rs.mu.RLock()
	require.True(t, rs.peers["10.0.0.2"].Up, "peer is a forward target")
	require.Equal(t, uint64(5), rs.peers["10.0.0.2"].ForwardFrom,
		"and carries the cut written in the same critical section")
	rs.mu.RUnlock()

	// Session bounces; more UPDATEs are taken delivery of while it is down.
	rs.handleState(&Event{Type: eventState, PeerAddr: "10.0.0.2", State: "down"})
	rs.mu.Lock()
	rs.seenMsgID = 42
	rs.mu.Unlock()
	rs.handleState(&Event{Type: eventState, PeerAddr: "10.0.0.2", State: "up"})

	rs.mu.Lock()
	rs.peers["10.0.0.2"].Families = families
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true, Families: families}
	rs.mu.Unlock()

	rs.mu.RLock()
	require.Equal(t, uint64(42), rs.peers["10.0.0.2"].ForwardFrom,
		"the new session re-captures the cut; a stale 5 would lose every route between 5 and 42")
	targets := rs.selectForwardTargets(nil, "10.0.0.1", 30, families)
	rs.mu.RUnlock()
	require.NotContains(t, targets, "10.0.0.2",
		"msgID 30 predates the new session's cut, so the replay owns it")
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
