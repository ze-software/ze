// Related: rr.go -- handleStructuredState, the peer-up replay under test

package rr

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
	rr := &routeReflector{peers: map[string]*peerState{target: {Address: target}}}

	rr.handleStructuredState(&rpc.StructuredEvent{PeerAddress: target, State: rpc.SessionStateUp, Reason: rpc.ReasonAttached})

	rr.mu.Lock()
	defer rr.mu.Unlock()
	assert.True(t, rr.peers[target].Up, "the session is up for the reflector")
	assert.Zero(t, rr.peers[target].ReplayGen, "an attached up starts no replay")
}

// TestRRTextSkipsReplayOnAttachedUp proves the same rule on the text event path
// (dispatchText): a state line carrying "reason attached" marks the peer up and
// starts no replay.
//
// PREVENTS: an external route reflector fed text events replaying every stored
// route into a session a reload only attached it to.
func TestRRTextSkipsReplayOnAttachedUp(t *testing.T) {
	const target = "10.0.0.2"
	rr := &routeReflector{peers: map[string]*peerState{target: {Address: target}}}

	rr.dispatchText("peer " + target + " remote as 65001 state up reason " + rpc.ReasonAttached + "\n")

	rr.mu.Lock()
	defer rr.mu.Unlock()
	assert.True(t, rr.peers[target].Up, "the session is up for the reflector")
	assert.Zero(t, rr.peers[target].ReplayGen, "an attached up starts no replay")
}
