// Design: docs/architecture/meta/role.md -- Learned role lifecycle
// Related: reactor_peers.go -- doRemovePeer, which issues the removal's generation

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestDoRemovePeerIssuesGenerationAboveEveryOpen checks the order the removal
// down gives a plugin between that down and an OPEN.
//
// GOAL: a plugin is delivered the removal down on its event loop and an OPEN
// on its callback loop, and orders the two by generation: an OPEN is stamped
// with the clock reading (Peer.validateOpen, rpc.ValidateOpenInput.Generation).
// METHOD: take the clock reading an OPEN of the peer would carry, with the
// peer never established since (its generation is the reading or older),
// remove it, and compare the down's generation with that reading and with the
// reading the next OPEN of any peer would carry.
//
// VALIDATES: the removal down names a generation above the old peer's OPEN
// stamp, and no later OPEN can be stamped below it.
// PREVENTS: a removal down still queued when a peer created again under the
// same name validated its OPEN deleting that new peer's learned role in
// bgp-role, and a removal of a never-established peer freeing nothing.
func TestDoRemovePeerIssuesGenerationAboveEveryOpen(t *testing.T) {
	r := newRemovalTestReactor()
	addr := netip.MustParseAddr("192.0.2.14")
	insertTestPeer(r, addr)
	peer := r.peers[peerKeyFromAddrPort(addr, DefaultBGPPort)]
	peer.advanceForwardGeneration()
	oldOpen := forwardGenerationClock.Load()

	removal, err := r.doRemovePeer(addr, message.NotifyCeasePeerDeconfigured)
	require.NoError(t, err)
	removed := removal.removed
	require.NotNil(t, removed)
	assert.Greater(t, removed.SessionGeneration, oldOpen,
		"the removal must name a generation above the removed peer's OPEN")
	assert.GreaterOrEqual(t, forwardGenerationClock.Load(), removed.SessionGeneration,
		"an OPEN validated after the removal must be stamped at or above it")
}
