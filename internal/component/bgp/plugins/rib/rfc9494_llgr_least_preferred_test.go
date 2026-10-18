package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
)

// TestRFC9494LLGRStaleLosesEveryLaterTieBreak isolates the least-preferred rule
// from the tie-breaks after it. TestComparePair_LLGRStale gives the fresh route
// the lower peer address and equal attributes, so the final tie-break picks it
// with or without the stale step; here the LLGR-stale route wins every later
// step, so only the stale step can make it lose.
//
// RFC 9494 Section 4.3: "A BGP speaker that has advertised the Long-Lived
// Graceful Restart Capability to a neighbor MUST perform the following upon
// receiving a route from that neighbor with the LLGR_STALE community or upon
// attaching the LLGR_STALE community itself per Section 4.2: * Treat the route
// as the least preferred in route selection (see below)."
//
// VALIDATES: a candidate at storage.DepreferenceThreshold with the higher
// LOCAL_PREF, the shorter AS_PATH and the lower peer address loses to a fresh
// candidate, in both argument orders of ComparePair and in both input orders of
// SelectBest.
// PREVENTS: the stale step being removed or inverted while a later tie-break
// happens to produce the same winner.
// MUTATION: make the aDepref != bDepref branch of compareBeforeMED never fire,
// and ComparePair(fresh, stale) and SelectBest over {stale, fresh} pick the
// stale route on LOCAL_PREF. The order matters because the level comparison after
// that branch still ranks the stale route lower when it is the first argument,
// which is why both orders are asserted.
func TestRFC9494LLGRStaleLosesEveryLaterTieBreak(t *testing.T) {
	t.Parallel()
	// RFC requirement: RFC9494-4.3-1 positive -- a route at the LLGR_STALE depreference level loses route selection to a fresh route even though it has the higher LOCAL_PREF, the shorter AS_PATH and the lower peer address, in both comparison orders
	stale := &Candidate{
		PeerAddr: "10.0.0.1", PeerIP: netip.MustParseAddr("10.0.0.1"),
		LocalPref: 200, ASPathLen: 1, StaleLevel: storage.DepreferenceThreshold,
	}
	fresh := &Candidate{
		PeerAddr: "10.0.0.2", PeerIP: netip.MustParseAddr("10.0.0.2"),
		LocalPref: 100, ASPathLen: 3, StaleLevel: storage.StaleLevelFresh,
	}

	assert.Equal(t, 1, ComparePair(stale, fresh), "the LLGR-stale route is the worse of the pair")
	assert.Equal(t, -1, ComparePair(fresh, stale), "the fresh route is the better of the pair")
	assert.Equal(t, "10.0.0.2", SelectBest([]*Candidate{stale, fresh}).PeerAddr)
	assert.Equal(t, "10.0.0.2", SelectBest([]*Candidate{fresh, stale}).PeerAddr)
}
