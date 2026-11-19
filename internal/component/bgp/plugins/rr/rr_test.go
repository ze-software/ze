// Related: rr.go -- handleStructuredState, the peer-up replay under test

package rr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRRSkipsReplayOnAttachedUp proves R-11 for the route reflector: an up with
// reason attached marks the peer up and starts no replay, so ReplayGen does not
// move and no replayForPeer goroutine runs.
//
// PREVENTS: a live attach reflecting every stored route into a session whose
// initial routing update already went out.
func TestRRSkipsReplayOnAttachedUp(t *testing.T) {
	const target = "10.0.0.2"
	original := rrOriginalFixture(t, target, 702, 42)
	rr := &routeReflector{peers: map[peerIdentity]*peerState{original: {Address: target, Identity: original}}}
	require.NoError(t, rr.handleStructuredState(&rpc.StructuredEvent{PeerAddress: target, PeerOwner: original.owner,
		ConnectionID: original.connection, DestinationSession: 142, SessionGeneration: 1042, State: rpc.SessionStateUp, Reason: rpc.ReasonAttached}))
	rr.mu.Lock()
	defer rr.mu.Unlock()
	assert.True(t, rr.peers[original].Up, "the session is up for the reflector")
	assert.Zero(t, rr.peers[original].ReplayGen, "an attached up starts no replay")
}

// TestRRTextSkipsReplayOnAttachedUp proves the same rule on the text event path
// (dispatchText): a state line carrying "reason attached" marks the peer up and
// starts no replay.
//
// PREVENTS: an external route reflector fed text events replaying every stored
// route into a session a reload only attached it to.
func TestRRTextSkipsReplayOnAttachedUp(t *testing.T) {
	const target = "10.0.0.2"
	original := rrOriginalFixture(t, target, 702, 42)
	rr := &routeReflector{peers: map[peerIdentity]*peerState{original: {Address: target, Identity: original}}}
	require.NoError(t, rr.dispatchText("peer "+target+" remote as 65001 state up peer-owner 702 connection-id 42 destination-session 142 session-generation 1042 reason "+rpc.ReasonAttached+"\n"))
	rr.mu.Lock()
	defer rr.mu.Unlock()
	assert.True(t, rr.peers[original].Up, "the session is up for the reflector")
	assert.Zero(t, rr.peers[original].ReplayGen, "an attached up starts no replay")
}
