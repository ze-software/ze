// Design: docs/architecture/plugin/rib-storage-design.md -- best-path selection
// Related: bestpath.go -- firstASInPath, neighborAS, the MED step

package rib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// RFC 4271 Section 9.1.2.2 c), neighborAS for an IBGP-learned aggregate:
//
//	"If the route is learned via IBGP, and the other IBGP speaker either
//	 (a) originated the route, or (b) created the route by aggregation and the
//	 AS_PATH attribute of the aggregate route is either empty or begins with an
//	 AS_SET, it is the local AS."
//
// Each candidate's FirstAS and ASPathLen are read from AS_PATH wire bytes by
// firstASInPath and asPathLength, the functions extractCandidate uses, so the
// segment type reaches the MED step the way it does in production. Every path
// below counts 1 (an AS_SET counts 1, RFC 4271 Section 9.1.2.2 a), so the
// AS_PATH length step ties and the comparison reaches MED.

// aggregateCandidate is an IBGP-learned candidate (helper ibgpMEDCandidate)
// whose AS_PATH is the given wire bytes.
func aggregateCandidate(peer string, path []byte, med uint32, igpCost uint64) *Candidate {
	c := ibgpMEDCandidate(peer, firstASInPath(path), med, igpCost)
	c.ASPathLen = asPathLength(path)
	return c
}

// asSetLedPath is a four-octet AS_PATH whose only segment is the AS_SET
// {65010, 65020}: what an IBGP speaker's aggregate of two routes carries.
var asSetLedPath = []byte{
	1, 2, // AS_SET, two ASNs
	0, 0, 0xFD, 0xF2, // 65010
	0, 0, 0xFD, 0xFC, // 65020
}

// otherASSetLedPath is a second aggregate's AS_SET {65030, 65040}, built by
// another IBGP speaker.
var otherASSetLedPath = []byte{
	1, 2, // AS_SET, two ASNs
	0, 0, 0xFE, 0x06, // 65030
	0, 0, 0xFE, 0x10, // 65040
}

// asSequenceLedPath is a four-octet AS_PATH whose only segment is the
// AS_SEQUENCE [65010]: a route the IBGP speaker learned from neighbor AS 65010.
var asSequenceLedPath = []byte{
	2, 1, // AS_SEQUENCE, one ASN
	0, 0, 0xFD, 0xF2, // 65010
}

// VALIDATES: an IBGP-learned aggregate whose AS_PATH begins with an AS_SET has
// neighbor AS = local AS, so two such aggregates from different IBGP speakers
// (sets led by 65010 and 65030) are compared on MED: MED 10 beats MED 20 at the
// MED step although the MED 20 route has the lower IGP cost.
// PREVENTS: the AS_SET's first member standing in for the neighbor AS, which
// skips MED and lets the IGP cost decide.
//
// RFC requirement: RFC4271-9.1.2.2-3 positive -- two IBGP aggregates whose
// AS_PATHs begin with different AS_SETs share neighbor AS 65000 (the local AS),
// and the MED step decides between them.
func TestRFC4271IBGPAggregatesLedByASSetsCompareMED(t *testing.T) {
	lowMED := aggregateCandidate("192.0.2.1", asSetLedPath, 10, 50)
	highMED := aggregateCandidate("192.0.2.2", otherASSetLedPath, 20, 5)

	result, step := comparePair(lowMED, highMED)
	assert.Equal(t, -1, result, "the lower MED wins")
	assert.Equal(t, BestStepMED, step, "the MED step decides, not the IGP cost")
	assert.Same(t, lowMED, SelectBest([]*Candidate{highMED, lowMED}))
}

// VALIDATES: an IBGP-learned route whose AS_PATH begins with an AS_SEQUENCE
// keeps the leftmost AS as its neighbor AS, so against an IBGP aggregate whose
// AS_SET happens to list that same AS first (neighbor AS = local AS) the MED
// step is skipped and the IGP cost decides.
// PREVENTS: the AS_SET rule being over-applied to every IBGP route, which would
// compare MEDs set by different neighbor ASes.
//
// RFC requirement: RFC4271-9.1.2.2-3 negative -- the local-AS rule is confined
// to an empty or AS_SET-led path: an AS_SEQUENCE-led IBGP route (neighbor AS
// 65010) is not MED-compared with an aggregate led by the AS_SET {65010, 65020}
// (local AS), and the lower IGP cost wins although its MED is higher.
func TestRFC4271IBGPRouteLedByASSequenceKeepsItsNeighborAS(t *testing.T) {
	learned := aggregateCandidate("192.0.2.1", asSequenceLedPath, 10, 50)
	aggregate := aggregateCandidate("192.0.2.2", asSetLedPath, 20, 5)

	result, step := comparePair(learned, aggregate)
	assert.Equal(t, 1, result, "the lower IGP cost wins")
	assert.NotEqual(t, BestStepMED, step, "different neighbor ASes: MED is not compared")
	assert.Same(t, aggregate, SelectBest([]*Candidate{aggregate, learned}))
}
