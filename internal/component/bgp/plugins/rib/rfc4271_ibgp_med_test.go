package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

// RFC 4271 Section 9.1.2.2 c), the MULTI_EXIT_DISC step for IBGP-learned routes.
//
//	"For IBGP-learned routes, the MULTI_EXIT_DISC MUST be used in route comparisons
//	 that reach this step in the Decision Process."
//
// neighborAS for an IBGP-learned route that the other IBGP speaker originated, whose
// AS_PATH is therefore empty, "is the local AS". Two such routes share a neighbor AS,
// so the MED step compares them.

// ibgpMEDCandidate is an IBGP-learned candidate (PeerASN equals LocalASN 65000) that
// ties on every step before MED. The later steps are left for each test to bias.
func ibgpMEDCandidate(peer string, firstAS, med uint32, igpCost uint64) *Candidate {
	return &Candidate{
		PeerAddr:     peer,
		PeerIP:       netip.MustParseAddr(peer),
		PeerASN:      65000,
		LocalASN:     65000,
		LocalPref:    100,
		ASPathLen:    0,
		FirstAS:      firstAS,
		Origin:       OriginIGP,
		MED:          med,
		IGPCost:      igpCost,
		OriginatorIP: netip.MustParseAddr(peer),
	}
}

// VALIDATES: two IBGP-learned routes originated inside the local AS (empty AS_PATH)
// are compared on MED, and the lower MED wins at BestStepMED.
// PREVENTS: the MED step being skipped because the empty AS_PATH yields no leftmost
// AS, which let the IGP cost decide for a route with the higher MED.
//
// RFC requirement: RFC4271-9.1.2.2-3 positive -- for IBGP-learned routes with an empty
// AS_PATH (neighborAS is the local AS), MED is used: MED 10 beats MED 20 at the MED step
// although the MED 20 route has the lower IGP cost.
func TestRFC4271IBGPLocallyOriginatedRoutesCompareMED(t *testing.T) {
	lowMED := ibgpMEDCandidate("192.0.2.1", 0, 10, 50)
	highMED := ibgpMEDCandidate("192.0.2.2", 0, 20, 5)

	result, step := comparePair(lowMED, highMED)
	assert.Equal(t, -1, result, "the lower MED wins")
	assert.Equal(t, BestStepMED, step, "the MED step decides, not the IGP cost")
	assert.Same(t, lowMED, SelectBest([]*Candidate{highMED, lowMED}))
}

// VALIDATES: two IBGP-learned routes from the same neighbor AS are decided on MED even
// when every later step (IGP cost, then the BGP Identifier) favors the route with the
// higher MED.
// PREVENTS: a Decision Process that treats MED as an eBGP-only criterion.
//
// RFC requirement: RFC4271-9.1.2.2-3 negative -- inputs are biased toward skipping MED
// for IBGP (every later criterion prefers the MED 20 route), and the MED 10 route still
// wins at the MED step.
func TestRFC4271IBGPMEDIsNotSkippedWhenLaterStepsDisagree(t *testing.T) {
	lowMED := ibgpMEDCandidate("192.0.2.9", 65010, 10, 50)
	highMED := ibgpMEDCandidate("192.0.2.1", 65010, 20, 5)

	result, step := comparePair(lowMED, highMED)
	assert.Equal(t, -1, result, "the lower MED wins")
	assert.Equal(t, BestStepMED, step)
	assert.Same(t, lowMED, SelectBest([]*Candidate{highMED, lowMED}))
}
