package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

// RFC 4271 Section 9.1.2.2: "The criteria MUST be applied in the order specified."
//
// Each case below pits two ADJACENT criteria against each other: the earlier one
// favors x, the later one favors y, and every other attribute ties. The earlier
// criterion must decide. A Decision Process that swaps any adjacent pair among steps
// a) to f) (LOCAL_PREF, AS_PATH length, ORIGIN, MED, EBGP over IBGP, interior cost,
// BGP Identifier) picks y for that case.

// orderCandidates returns two EBGP candidates from one neighbor AS that tie on every
// criterion except the final BGP Identifier, which favors y.
func orderCandidates() (x, y *Candidate) {
	build := func(peer, routerID string) *Candidate {
		return &Candidate{
			PeerAddr:     peer,
			PeerIP:       netip.MustParseAddr(peer),
			PeerASN:      65001,
			LocalASN:     65000,
			LocalPref:    100,
			ASPathLen:    2,
			FirstAS:      65001,
			Origin:       OriginIGP,
			MED:          10,
			IGPCost:      10,
			OriginatorIP: netip.MustParseAddr(routerID),
		}
	}
	return build("192.0.2.1", "198.51.100.9"), build("192.0.2.2", "198.51.100.1")
}

// VALIDATES: for every adjacent pair of criteria from LOCAL_PREF to the BGP Identifier,
// the earlier criterion decides although the later one favors the other route.
// PREVENTS: a reordering among steps a) to f), such as AS_PATH length before
// LOCAL_PREF, which the step-f/g and neighbor-AS units alone do not catch.
//
// RFC requirement: RFC4271-9.1.2.2-1 positive -- for each adjacent pair (LOCAL_PREF /
// AS_PATH length, AS_PATH length / ORIGIN, ORIGIN / MED, MED / EBGP over IBGP, EBGP over
// IBGP / interior cost, interior cost / BGP Identifier) the earlier criterion decides, in
// both argument orders.
func TestRFC4271AdjacentCriteriaApplyInTheOrderSpecified(t *testing.T) {
	cases := []struct {
		name string
		step BestStep
		bias func(x, y *Candidate) // earlier criterion favors x, later favors y
	}{
		{"LOCAL_PREF before AS_PATH length", BestStepLocalPref, func(x, y *Candidate) {
			x.LocalPref = 200
			y.ASPathLen = 1
		}},
		{"AS_PATH length before ORIGIN", BestStepASPathLen, func(x, y *Candidate) {
			x.ASPathLen = 1
			x.Origin = OriginIncomplete
		}},
		{"ORIGIN before MED", BestStepOrigin, func(x, y *Candidate) {
			y.Origin = OriginEGP
			y.MED = 0
		}},
		{"MED before EBGP over IBGP", BestStepMED, func(x, y *Candidate) {
			x.MED = 0
			x.PeerASN = x.LocalASN // x is IBGP-learned, y stays EBGP
		}},
		{"EBGP over IBGP before interior cost", BestStepEBGPOverIBGP, func(x, y *Candidate) {
			y.PeerASN = y.LocalASN // y is IBGP-learned
			y.IGPCost = 1
		}},
		{"interior cost before BGP Identifier", BestStepIGPCost, func(x, y *Candidate) {
			x.IGPCost = 1 // y keeps the lower BGP Identifier
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x, y := orderCandidates()
			tc.bias(x, y)

			result, step := comparePair(x, y)
			assert.Equal(t, -1, result, "the earlier criterion's choice, x, wins")
			assert.Equal(t, tc.step, step, "the earlier criterion decides")

			result, step = comparePair(y, x)
			assert.Equal(t, 1, result, "argument order does not change the winner")
			assert.Equal(t, tc.step, step)
		})
	}
}
