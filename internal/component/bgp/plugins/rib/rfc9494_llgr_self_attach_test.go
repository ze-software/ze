// Design: docs/architecture/route-selection.md -- Step 0, stale depreference
// RFC: rfc/short/rfc9494.md -- Section 4.3, a route Ze attached LLGR_STALE to is least preferred
// Related: rib_commands_community.go -- attachCommunityCommand raises the attached route's level
// Related: rfc9494_llgr_received_stale_test.go -- the received-route half of the same clause

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
)

// selfAttachLLGRPeer is the neighbor whose session is lost and whose routes Ze
// retains into the LLGR period. Its address is the lower one, so the final
// tie-break would also favor it.
var selfAttachLLGRPeer = netip.MustParseAddr("192.0.2.1")

// selfAttachFreshPeer is the competing neighbor whose session stays up.
var selfAttachFreshPeer = netip.MustParseAddr("192.0.2.2")

// selfAttachSetup stores 10.0.0.0/24 from both neighbors, the LLGR neighbor at
// LOCAL_PREF 200 and the fresh one at LOCAL_PREF 100, so every ordinary step
// prefers the LLGR neighbor's route. It then runs the two steps the GR plugin
// dispatches for that neighbor (gr.go): "mark-stale <peer> <restart-time>" at
// session loss, which leaves the level below storage.DepreferenceThreshold, and
// "attach-community <peer> ipv4/unicast ffff0006" at LLGR period start. When
// lateNLRI is set, that NLRI is stored from the LLGR neighbor between the two
// steps, so it is fresh when the attach sweep runs.
func selfAttachSetup(t *testing.T, lateNLRI []byte) *RIBManager {
	t.Helper()
	r := newTestRIBManager(t)

	preferred := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref200)
	ordinary := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref100)

	llgrRIB := storage.NewPeerRIB(selfAttachLLGRPeer.String())
	llgrRIB.Insert(family.IPv4Unicast, preferred, []byte{24, 10, 0, 0})
	r.bgpPeers[selfAttachLLGRPeer] = llgrRIB

	freshRIB := storage.NewPeerRIB(selfAttachFreshPeer.String())
	freshRIB.Insert(family.IPv4Unicast, ordinary, []byte{24, 10, 0, 0})
	if lateNLRI != nil {
		freshRIB.Insert(family.IPv4Unicast, ordinary, lateNLRI)
	}
	r.bgpPeers[selfAttachFreshPeer] = freshRIB

	_, _, err := r.handleCommand("request bgp rib mark-stale", "*", []string{selfAttachLLGRPeer.String(), "120"})
	require.NoError(t, err)

	if lateNLRI != nil {
		llgrRIB.Insert(family.IPv4Unicast, preferred, lateNLRI)
	}

	status, _, err := r.handleCommand("request bgp rib attach-community", "*",
		[]string{selfAttachLLGRPeer.String(), "ipv4/unicast", "ffff0006"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	return r
}

// selfAttachWinner runs the RIB's own candidate gather and best-path selection
// for nlri and returns the explanation of the one comparison between the two
// neighbors' routes.
func selfAttachWinner(t *testing.T, r *RIBManager, nlri []byte) *bestPathExplanation {
	t.Helper()
	candidates := gatherCandidatesHeld(t, r, family.IPv4Unicast, nlri, false)
	require.Len(t, candidates, 2, "both neighbors announced the prefix")
	explanation := SelectBestExplain(candidates)
	require.NotNil(t, explanation)
	require.Len(t, explanation.Steps, 1)
	return explanation
}

// TestRFC9494SelfAttachedLLGRStaleIsLeastPreferred verifies that a route Ze
// itself attached LLGR_STALE to, through the attach-community command the GR
// plugin issues at LLGR period start, loses route selection to a fresh route
// with worse ordinary attributes.
//
// VALIDATES: after mark-stale and attach-community ffff0006, the RIB's candidate
// gather and SelectBestExplain pick the fresh LOCAL_PREF 100 route over the
// retained LOCAL_PREF 200 route, decided at Step 0.
// PREVENTS: the attach step attaching the community without depreferencing the
// route, which leaves the retained route winning on LOCAL_PREF while the level
// mark-stale wrote stays below storage.DepreferenceThreshold.
//
// RFC requirement: RFC9494-4.3-1 positive -- after mark-stale 192.0.2.1 120 and attach-community 192.0.2.1 ipv4/unicast ffff0006, SelectBestExplain over the gathered candidates for 10.0.0.0/24 is decided at BestStepStale and picks the LOCAL_PREF 100 route from 192.0.2.2 over the LOCAL_PREF 200 route from 192.0.2.1.
func TestRFC9494SelfAttachedLLGRStaleIsLeastPreferred(t *testing.T) {
	t.Parallel()
	r := selfAttachSetup(t, nil)

	explanation := selfAttachWinner(t, r, []byte{24, 10, 0, 0})
	assert.Equal(t, BestStepStale, explanation.Steps[0].Step, "Step 0 decides")
	assert.Equal(t, selfAttachFreshPeer.String(), explanation.Winner.PeerAddr, "the fresh route wins")
}

// TestRFC9494SelfAttachSkipsRouteLearnedAfterLoss verifies the least-preferred
// treatment reaches only the routes the attach sweep attached LLGR_STALE to: a
// route the LLGR neighbor's table holds fresh when the sweep runs keeps
// competing on its attributes.
//
// VALIDATES: 10.0.1.0/24, stored from 192.0.2.1 after mark-stale, is decided at
// BestStepLocalPref and wins on LOCAL_PREF 200 after the attach sweep.
// PREVENTS: an attach sweep that depreferences every route of the neighbor,
// including one it never attached the community to.
//
// RFC requirement: RFC9494-4.3-1 negative -- 10.0.1.0/24 stored fresh from 192.0.2.1 after mark-stale and before attach-community ffff0006 is decided at BestStepLocalPref, and the LOCAL_PREF 200 route from 192.0.2.1 wins over the LOCAL_PREF 100 route from 192.0.2.2.
func TestRFC9494SelfAttachSkipsRouteLearnedAfterLoss(t *testing.T) {
	t.Parallel()
	late := []byte{24, 10, 0, 1}
	r := selfAttachSetup(t, late)

	explanation := selfAttachWinner(t, r, late)
	assert.Equal(t, BestStepLocalPref, explanation.Steps[0].Step, "LOCAL_PREF decides")
	assert.Equal(t, selfAttachLLGRPeer.String(), explanation.Winner.PeerAddr, "the fresh LOCAL_PREF 200 route wins")

	// The same sweep did depreference the retained route, so the scope is
	// what spared the late one.
	explanation = selfAttachWinner(t, r, []byte{24, 10, 0, 0})
	assert.Equal(t, BestStepStale, explanation.Steps[0].Step)
}
