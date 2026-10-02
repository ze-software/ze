// VALIDATES: RFC 5286 Section 3.5 for a neighbor reached over a broadcast (transit)
// network: the reverse cost neighborLinks gives that candidate is the neighbor's own
// Router-LSA transit link onto the network (reverseTransitCost), and selectLFA refuses the
// neighbor as an alternate when that only link back costs 0xffff.
// PREVENTS: the transit candidate path taking its reverse cost from anywhere but the
// neighbor's link onto the LAN, so a costed-out LAN neighbor is used as an alternate.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// transitReverseLAN is the Network-LSA ID (the DR interface address) of the LAN S and N share.
const transitReverseLAN = "10.0.13.254"

// transitReverseGraph builds S (1.1.1.1) with a point-to-point link to E (2.2.2.2), the
// primary, and a transit link onto a LAN where N (3.3.3.3) is the only other router. N's
// own transit link onto the LAN, its only link back to S, costs nReverse.
func transitReverseGraph(t *testing.T, nReverse uint16) *Graph {
	t.Helper()
	g := NewGraph(testArea())
	s, e, n := testRID(t, srcS), testRID(t, nbrE), testRID(t, altN)
	g.Routers[s] = &RouterVertex{ID: s, Links: []packet.RouterLink{
		p2pLink(t, nbrE, "10.0.12.1", 10),
		transitLinkDR(t, transitReverseLAN, "10.0.13.1", 10),
	}}
	g.Routers[e] = &RouterVertex{ID: e, Links: []packet.RouterLink{p2pLink(t, srcS, addrE, 10)}}
	g.Routers[n] = &RouterVertex{ID: n, Links: []packet.RouterLink{
		transitLinkDR(t, transitReverseLAN, addrN, nReverse),
	}}
	lan := testLSID(t, transitReverseLAN)
	g.Networks[lan] = &NetworkVertex{ID: lan, AdvertisingDR: s, AttachedRouters: []types.RouterID{s, n}}
	return g
}

// transitReverseSelect enumerates S's candidates with the OSPFv2 next-hop source, returns
// N's candidate, and runs the Section 3.6 selection protecting D through E over exactly
// those candidates, with N loop-free for D: D_opt(N,D)=10 < D_opt(N,S)+D_opt(S,D)=10+15.
func transitReverseSelect(t *testing.T, g *Graph) (candLink, bool) {
	t.Helper()
	cands := neighborLinks(g, testRID(t, srcS), v4NextHop{}, nil)
	candByAddr := make(map[netip.Addr]candLink, len(cands))
	var lanN candLink
	for _, c := range cands {
		candByAddr[c.addr] = c
		if c.neighbor == testRID(t, altN) {
			lanN = c
		}
	}
	if lanN.addr != netip.MustParseAddr(addrN) || !lanN.broadcast {
		t.Fatalf("no broadcast candidate for N at %s: %+v", addrN, cands)
	}
	primary := NextHop{Addr: netip.MustParseAddr(addrE), Router: testRID(t, nbrE)}
	if _, ok := candByAddr[primary.Addr]; !ok {
		t.Fatalf("no point-to-point candidate for E at %s: %+v", addrE, cands)
	}
	res := mkResult(t, srcS, map[string]uint64{srcS: 0, nbrE: 10, altN: 10, destD: 15})
	spt := map[types.RouterID]*Result{
		testRID(t, altN): mkResult(t, altN, map[string]uint64{altN: 0, srcS: 10, nbrE: 15, destD: 10}),
		testRID(t, nbrE): mkResult(t, nbrE, map[string]uint64{nbrE: 0, destD: 5}),
	}
	_, ok := selectLFA(routerVertex(testRID(t, destD)), primary, []NextHop{primary}, cands, candByAddr, spt, res, FastRerouteConfig{})
	return lanN, ok
}

// RFC requirement: RFC5286-x-3 positive -- N's only link back to S is its transit link onto
// the shared LAN at cost 10, so the candidate neighborLinks builds for N carries reverse cost
// 10 and selectLFA uses N as the alternate.
func TestRFC5286TransitNeighborFiniteReverseIsAlternate(t *testing.T) {
	// Goal: a LAN neighbor with a finite way back is usable. Method: the real candidate
	// enumeration over a graph, then the selection over the candidates it returned.
	lanN, ok := transitReverseSelect(t, transitReverseGraph(t, 10))
	if lanN.reverseCost != 10 {
		t.Fatalf("N's reverse cost over the LAN = %d, want 10 (its own transit link)", lanN.reverseCost)
	}
	if !ok {
		t.Fatalf("LAN neighbor N with a finite reverse cost was rejected as an alternate")
	}
}

// RFC requirement: RFC5286-x-3 negative -- N's only link back to S, its transit link onto
// the shared LAN, costs 0xffff (LSInfinity for a Router-LSA link), so every link from S to N
// has a reverse cost of LSInfinity and S MUST NOT use N as an alternate.
func TestRFC5286TransitNeighborCostedOutReverseNotAlternate(t *testing.T) {
	// Goal: the costed-out LAN neighbor is refused. Method: the same graph with only N's
	// transit metric changed to 0xffff.
	lanN, ok := transitReverseSelect(t, transitReverseGraph(t, 0xffff))
	if lanN.reverseCost != 0xffff {
		t.Fatalf("N's reverse cost over the LAN = %#x, want 0xffff (its own transit link)", lanN.reverseCost)
	}
	if ok {
		t.Fatalf("LAN neighbor N whose only link back to S costs 0xffff was used; RFC 5286 Section 3.5 forbids it")
	}
}
