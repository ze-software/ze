// Design: docs/architecture/plugin/rib-storage-design.md -- best-path candidates
// RFC: rfc/short/rfc8277.md -- RFC8277-3.1-1, comparability on one ADD-PATH session
// Related: rfc8277_test.go -- TestLabeledRoutesWithDifferentLabelsAreComparable, the two-session case

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC8277AddPathRoutesOnOneSessionAreComparable is the failing-first test for
// the second case of RFC 8277 Section 3.1: two UPDATEs "received on the same
// session, add-paths is used on that session, and the NLRIs of the two UPDATEs
// have different path identifiers. These two routes MUST be considered to be
// comparable, even if they specify different labels. Thus, the BGP best-path
// selection procedures (see Section 9.1 of [RFC4271]) are applied to select one
// of them as the better path."
//
// VALIDATES: one ADD-PATH peer sends 10.0.0.0/8 as path 7 (label 100, MED 10)
// and path 9 (label 200, MED 20); the candidates gathered for the prefix under
// either path's key hold both routes, and selection picks path 7's route.
// PREVENTS: best-path selection keyed on (path identifier, prefix), which never
// compares two paths of one session and elects one best route per identifier.
//
// RED at the time of writing: gatherCandidatesLocked looks up each peer's RIB
// with the path-id-keyed NLRI, so each key yields one candidate. Untagged until
// the fix lands (design-sized: best-path keying under ADD-PATH receive).
func TestRFC8277AddPathRoutesOnOneSessionAreComparable(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("192.0.2.31")
	ctxID, _ := bgpctx.Registry.Register(
		bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{labeledFamily: true}))

	pfx := netip.MustParsePrefix("10.0.0.0/8")
	feedReceived(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 1}, 10,
		labeledNLRI(7, true, pfx, []uint32{100})))
	feedReceived(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 2}, 20,
		labeledNLRI(9, true, pfx, []uint32{200})))
	require.Equal(t, 2, r.bgpPeers[peer].Len(), "precondition: two paths stored")

	for _, key := range [][]byte{{0, 0, 0, 7, 8, 10}, {0, 0, 0, 9, 8, 10}} {
		candidates := r.gatherCandidates(labeledFamily, key)
		assert.Len(t, candidates, 2, "both paths of the session are candidates for 10.0.0.0/8 (key %x)", key)
		if best := SelectBest(candidates); best != nil {
			assert.Equal(t, uint32(10), best.MED, "the MED 10 path wins (key %x)", key)
		}
	}
}
