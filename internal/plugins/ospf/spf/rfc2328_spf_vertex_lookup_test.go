// VALIDATES: RFC 2328 Section 16.1 step (2)(b): for each link of vertex V the calculation
// looks up W's router-LSA or network-LSA and skips the link when that LSA does not exist,
// is at MaxAge, or has no link back to V, then examines V's next link.
// PREVENTS: a missing or MaxAge LSA pulled into the tree, and a skip that abandons the rest
// of V's links; the tagged units covered only the link-back clause.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// atMaxAge returns lsa with LS age MaxAge.
func atMaxAge(lsa packet.LSA) packet.LSA {
	lsa.Header.Age = types.LSAge(types.MaxAge)
	return lsa
}

// RFC requirement: RFC2328-16.1-1 positive -- the root lists four links that must be skipped (2.2.2.2 with no router-LSA, 3.3.3.3 whose router-LSA links back but is at MaxAge, 4.4.4.4 with no link back, a transit link to 10.0.9.254 with no network-LSA) BEFORE its link to 5.5.5.5: the calculation goes on to that next link, so 5.5.5.5 is reached at cost 10 and its stub 192.0.2.0/24 is routed, while none of the skipped vertices is reached and no route runs through them (Compute, spf.go; BuildGraph, graph.go).
func TestRFC2328SPFExaminesNextLinkAfterSkip(t *testing.T) {
	// Goal: every skip clause in one LSA, and the next link still examined.
	// Method: Compute over a hand-built area database, root 1.1.1.1.
	area := testArea()
	db := testSource(t, area,
		routerLSA(t, "1.1.1.1",
			p2pLink(t, "2.2.2.2", "10.0.0.1", 10),
			p2pLink(t, "3.3.3.3", "10.0.1.1", 10),
			p2pLink(t, "4.4.4.4", "10.0.2.1", 10),
			transitLinkDR(t, "10.0.9.254", "10.0.9.1", 10),
			p2pLink(t, "5.5.5.5", "10.0.5.1", 10),
		),
		atMaxAge(routerLSA(t, "3.3.3.3", p2pLink(t, "1.1.1.1", "10.0.1.3", 10), stubLink(t, "198.51.100.0", 1))),
		routerLSA(t, "4.4.4.4", stubLink(t, "203.0.113.0", 1)),
		routerLSA(t, "5.5.5.5", p2pLink(t, "1.1.1.1", "10.0.5.5", 10), stubLink(t, "192.0.2.0", 5)),
	)
	res := Compute(BuildGraph(db, area), testRID(t, "1.1.1.1"), 8)
	w := res.Nodes[routerVertex(testRID(t, "5.5.5.5"))]
	if w == nil || w.Metric != 10 {
		t.Fatalf("5.5.5.5 = %+v, want reached at cost 10 after the skipped links", w)
	}
	for _, skipped := range []string{"2.2.2.2", "3.3.3.3", "4.4.4.4"} {
		if n := res.Nodes[routerVertex(testRID(t, skipped))]; n != nil {
			t.Errorf("%s reached: %+v", skipped, n)
		}
	}
	if n := res.Nodes[networkVertex(testLSID(t, "10.0.9.254"))]; n != nil {
		t.Errorf("transit network 10.0.9.254 with no network-LSA reached: %+v", n)
	}
	routes := BuildRoutes(res, 8, nil)
	if len(routes) != 1 || routes[0].Prefix != netip.MustParsePrefix("192.0.2.0/24") || routes[0].Metric != 15 {
		t.Fatalf("routes = %+v, want only 192.0.2.0/24 at 15", routes)
	}
}

// RFC requirement: RFC2328-16.1-1 negative -- a vertex W whose LSA is missing or at MaxAge is refused even though everything else about the link is valid: a router W with no router-LSA, a router W at MaxAge that links back, a transit network with no network-LSA, and a transit network whose network-LSA is at MaxAge (both routers linking to it) are never reached, and no route is built through them (BuildGraph, graph.go; Compute, spf.go).
func TestRFC2328SPFSkipsMissingOrMaxAgeVertex(t *testing.T) {
	// Goal: the missing-LSA and MaxAge clauses, each on its own.
	// Method: root 1.1.1.1 with one link; W carries a stub that would be routed if reached.
	root := func(link packet.RouterLink) packet.LSA { return routerLSA(t, "1.1.1.1", link) }
	lan := func() packet.LSA {
		return networkLSA(t, "10.0.0.254", "2.2.2.2", "255.255.255.0", "1.1.1.1", "2.2.2.2")
	}
	lanRouter := routerLSA(t, "2.2.2.2", transitLink(t, "10.0.0.2"), stubLink(t, "198.51.100.0", 1))
	cases := []struct {
		name   string
		lsas   []packet.LSA
		vertex VertexID
	}{
		{"router-LSA missing", []packet.LSA{root(p2pLink(t, "2.2.2.2", "10.0.0.1", 10))}, routerVertex(testRID(t, "2.2.2.2"))},
		{"router-LSA at MaxAge", []packet.LSA{root(p2pLink(t, "2.2.2.2", "10.0.0.1", 10)), atMaxAge(routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.0.2", 10), stubLink(t, "198.51.100.0", 1)))}, routerVertex(testRID(t, "2.2.2.2"))},
		{"network-LSA missing", []packet.LSA{root(transitLink(t, "10.0.0.1")), lanRouter}, networkVertex(testLSID(t, "10.0.0.254"))},
		{"network-LSA at MaxAge", []packet.LSA{root(transitLink(t, "10.0.0.1")), lanRouter, atMaxAge(lan())}, networkVertex(testLSID(t, "10.0.0.254"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			area := testArea()
			res := Compute(BuildGraph(testSource(t, area, tc.lsas...), area), testRID(t, "1.1.1.1"), 8)
			if n := res.Nodes[tc.vertex]; n != nil {
				t.Fatalf("vertex %+v reached: %+v", tc.vertex, n)
			}
			if n := res.Nodes[routerVertex(testRID(t, "2.2.2.2"))]; n != nil {
				t.Fatalf("2.2.2.2 reached through the refused vertex: %+v", n)
			}
			if routes := BuildRoutes(res, 8, nil); len(routes) != 0 {
				t.Fatalf("routes = %+v, want none", routes)
			}
		})
	}
	// Control: the same router and network LSAs below MaxAge are reached, so the MaxAge
	// subtests fail on the age alone.
	area := testArea()
	for _, lsas := range [][]packet.LSA{
		{root(p2pLink(t, "2.2.2.2", "10.0.0.1", 10)), routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.0.2", 10), stubLink(t, "198.51.100.0", 1))},
		{root(transitLink(t, "10.0.0.1")), lanRouter, lan()},
	} {
		res := Compute(BuildGraph(testSource(t, area, lsas...), area), testRID(t, "1.1.1.1"), 8)
		if n := res.Nodes[routerVertex(testRID(t, "2.2.2.2"))]; n == nil {
			t.Fatalf("control: 2.2.2.2 not reached with every LSA below MaxAge")
		}
	}
}
