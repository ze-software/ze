// VALIDATES: lfa.go reverseTransitCost (RFC 5286 Section 3.5 reverse-cost gate
// over a broadcast pseudo-node) and pseudoNodeDist (D_opt(PN,D): the minimum over
// the pseudo-node's attached routers plus the root S, RFC 2328 broadcast model).
// PREVENTS: a broadcast alternate scored against the wrong reverse cost or an
// over-counted pseudo-node distance that would misclassify the LFA.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestReverseTransitCost(t *testing.T) {
	nb := testRID(t, "2.2.2.2")
	nw := testLSID(t, "10.0.0.254")
	g := NewGraph(testArea())
	g.Routers[nb] = &RouterVertex{ID: nb, Links: []packet.RouterLink{
		transitLinkDR(t, "10.0.0.254", "10.0.0.2", 7),
		stubLink(t, "192.0.2.0", 3),
	}}

	// The neighbor's own transit link onto the pseudo-node carries cost N->PN = 7.
	if got := reverseTransitCost(g, nb, nw); got != 7 {
		t.Fatalf("reverseTransitCost = %d, want 7", got)
	}
	// A router not in the graph is unreachable: LSInfinity.
	if got := reverseTransitCost(g, testRID(t, "9.9.9.9"), nw); got != LSInfinity {
		t.Fatalf("reverseTransitCost(absent router) = %d, want LSInfinity", got)
	}
	// A router present but with no transit link onto THIS pseudo-node: LSInfinity.
	if got := reverseTransitCost(g, nb, testLSID(t, "10.9.9.254")); got != LSInfinity {
		t.Fatalf("reverseTransitCost(other network) = %d, want LSInfinity", got)
	}
}

func TestPseudoNodeDist(t *testing.T) {
	nw := testLSID(t, "10.0.0.254")
	dest := routerVertex(testRID(t, "9.9.9.9"))
	n1 := testRID(t, "2.2.2.2")
	n2 := testRID(t, "3.3.3.3")
	addr1 := netip.MustParseAddr("10.0.0.2")
	addr2 := netip.MustParseAddr("10.0.0.3")
	// Decoys that MUST be ignored: a broadcast candidate on a DIFFERENT network, and
	// a NON-broadcast candidate on the right network. Both are given a tiny distance
	// so that if the filter leaked, the minimum would drop below the real answer.
	addrOtherNet := netip.MustParseAddr("10.0.0.4")
	addrNotBcast := netip.MustParseAddr("10.0.0.5")

	candByAddr := map[netip.Addr]candLink{
		addr1:        {neighbor: n1, addr: addr1, broadcast: true, network: nw},
		addr2:        {neighbor: n2, addr: addr2, broadcast: true, network: nw},
		addrOtherNet: {neighbor: testRID(t, "4.4.4.4"), addr: addrOtherNet, broadcast: true, network: testLSID(t, "10.9.9.254")},
		addrNotBcast: {neighbor: testRID(t, "5.5.5.5"), addr: addrNotBcast, broadcast: false, network: nw},
	}
	spt := map[types.RouterID]*Result{
		n1:                    mkResult(t, "2.2.2.2", map[string]uint64{"9.9.9.9": 5}),
		n2:                    mkResult(t, "3.3.3.3", map[string]uint64{"9.9.9.9": 8}),
		testRID(t, "4.4.4.4"): mkResult(t, "4.4.4.4", map[string]uint64{"9.9.9.9": 1}), // decoy: closest, must be ignored
		testRID(t, "5.5.5.5"): mkResult(t, "5.5.5.5", map[string]uint64{"9.9.9.9": 2}), // decoy: closest, must be ignored
	}

	// Root S reaches D at 12; the two on-network attached routers reach it at 5 and 8.
	root := mkResult(t, "1.1.1.1", map[string]uint64{"9.9.9.9": 12})
	if got := pseudoNodeDist(candByAddr, nw, dest, spt, root); got != 5 {
		t.Fatalf("pseudoNodeDist = %d, want 5 (min over attached routers, decoys excluded)", got)
	}

	// When the root S is the closest attachment, D_opt(PN,D) takes S's distance.
	rootClosest := mkResult(t, "1.1.1.1", map[string]uint64{"9.9.9.9": 3})
	if got := pseudoNodeDist(candByAddr, nw, dest, spt, rootClosest); got != 3 {
		t.Fatalf("pseudoNodeDist = %d, want 3 (root is closest to D over the PN)", got)
	}

	// A candidate whose per-neighbor SPT is missing is skipped; the distance then
	// comes from the root fallback only.
	sptMissing := map[types.RouterID]*Result{}
	onlyBcast := map[netip.Addr]candLink{addr1: {neighbor: n1, addr: addr1, broadcast: true, network: nw}}
	if got := pseudoNodeDist(onlyBcast, nw, dest, sptMissing, mkResult(t, "1.1.1.1", map[string]uint64{"9.9.9.9": 7})); got != 7 {
		t.Fatalf("pseudoNodeDist(nil neighbor SPT) = %d, want 7 (root fallback)", got)
	}

	// No usable attachment reaches D: LSInfinity.
	if got := pseudoNodeDist(onlyBcast, nw, dest, sptMissing, mkResult(t, "1.1.1.1", map[string]uint64{})); got != LSInfinity {
		t.Fatalf("pseudoNodeDist(unreachable) = %d, want LSInfinity", got)
	}
}
