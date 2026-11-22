// Design: docs/architecture/plugin/rib-storage-design.md -- best-path selection
// RFC: rfc/short/rfc4360.md -- Section 6, Extended Communities must not alter best-path selection into loops
// Related: bestpath.go -- comparePair, SelectBest

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC4360ExtendedCommunitiesDoNotAlterBestPath verifies that Ze's BGP
// decision process (extractCandidate + SelectBest / comparePair) does not use
// the Extended Communities attribute (type 16) to modify best-path selection.
//
// RFC 4360 Section 6: "The Extended Community attribute MUST NOT be used to
// modify the BGP best path selection algorithm in a way that leads to
// forwarding loops."
//
// VALIDATES: two routes identical in every RFC 4271 best-path criterion select
// the same winner regardless of differing or absent Extended Communities, while
// differing standard criteria (LOCAL_PREF) do decide the winner.
// PREVENTS: introducing an Extended Community comparison step into comparePair
// that could diverge from RFC 4271 best-path ordering and induce loops.
//
// RFC requirement: RFC4360-6-1 positive -- routes carrying different RFC 4360
// Extended Communities (Route Target, Route Origin, or absent) with equal RFC
// 4271 criteria select the exact same winner via comparePair (lowest peer IP),
// proving Extended Communities do not alter the decision process.
// RFC requirement: RFC4360-6-1 negative -- changing a standard RFC 4271
// criterion (LOCAL_PREF) flips the winner in comparePair even when Extended
// Communities are unchanged, proving comparePair actively selects on RFC 4271
// attributes rather than ignoring candidate differences.
func TestRFC4360ExtendedCommunitiesDoNotAlterBestPath(t *testing.T) {
	t.Parallel()

	r := newTestRIBManager(t)
	buildAttrs := func(localPref uint32, extComm []byte) []byte {
		attrs := []byte{
			0x40, 1, 1, 0, // ORIGIN = IGP
			0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9, // AS_PATH = [65001]
			0x40, 3, 4, 192, 0, 2, 1, // NEXT_HOP
			0x40, 5, 4, byte(localPref >> 24), byte(localPref >> 16), byte(localPref >> 8), byte(localPref), // LOCAL_PREF
		}
		if len(extComm) > 0 {
			attrs = append(attrs, 0xc0, 16, byte(len(extComm)))
			attrs = append(attrs, extComm...)
		}
		return attrs
	}

	extract := func(owner uint64, peerStr string, attrs []byte) *Candidate {
		entry, _, _, err := storage.ParseRouteEntry(attrs)
		require.NoError(t, err)
		t.Cleanup(entry.Release)
		peer := netip.MustParseAddr(peerStr)
		return r.extractCandidate(family.IPv4Unicast, owner, peer, peerStr, entry)
	}

	rtHigh := []byte{0x00, 0x02, 0xfd, 0xe9, 0x00, 0x00, 0x00, 0x01} // target:65001:1
	rtLow := []byte{0x00, 0x02, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01}  // target:1:1

	// Positive: peer 10.0.0.1 wins on peer-address tiebreak regardless of Extended Communities.
	c1 := extract(1, "10.0.0.1", buildAttrs(100, rtHigh))
	c2 := extract(2, "10.0.0.2", buildAttrs(100, rtLow))
	c3 := extract(3, "10.0.0.2", buildAttrs(100, nil))
	require.Equal(t, -1, ComparePair(c1, c2))
	require.Same(t, c1, SelectBest([]*Candidate{c2, c1}))
	require.Equal(t, -1, ComparePair(c1, c3))
	require.Same(t, c1, SelectBest([]*Candidate{c3, c1}))

	// Swapping the Extended Communities leaves the exact same winner.
	c1Swapped := extract(1, "10.0.0.1", buildAttrs(100, rtLow))
	c2Swapped := extract(2, "10.0.0.2", buildAttrs(100, rtHigh))
	require.Equal(t, -1, ComparePair(c1Swapped, c2Swapped))
	require.Same(t, c1Swapped, SelectBest([]*Candidate{c2Swapped, c1Swapped}))

	// Negative control: higher LOCAL_PREF on 10.0.0.2 flips the winner.
	c2HigherLP := extract(2, "10.0.0.2", buildAttrs(200, rtHigh))
	require.Equal(t, 1, ComparePair(c1, c2HigherLP))
	require.Same(t, c2HigherLP, SelectBest([]*Candidate{c1, c2HigherLP}))
}
