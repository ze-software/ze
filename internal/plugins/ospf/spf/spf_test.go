// VALIDATES: RFC 2328 Section 16.1 intra-area SPF -- shortest-path tree over
// router and transit-network vertices, two-way check, next-hop derivation, ECMP,
// stub attachment, and transit (broadcast LAN) subnet route installation.
// PREVENTS: regressions where SPF drops transit topology, picks the wrong
// next-hop, or fails to install a remote broadcast LAN's own prefix.
package spf

import (
	"net/netip"
	"testing"
)

// RFC requirement: RFC2328-16.1-1 positive -- a transit link whose neighbor LSA exists, is not MaxAge, and carries a link back to the current vertex passes the two-way check and enters the shortest-path tree (twoWayRouterLink, spf.go:290-302).
func TestOSPFSPFShortestPath(t *testing.T) {
	area := testArea()
	db := baseP2PSource(t, area)
	res := Compute(BuildGraph(db, area), testRID(t, "1.1.1.1"), 8)
	b := res.Nodes[routerVertex(testRID(t, "2.2.2.2"))]
	if b == nil || b.Metric != 10 {
		t.Fatalf("B result = %+v, want metric 10", b)
	}
	if len(b.NextHops) != 1 || b.NextHops[0].Addr != netip.MustParseAddr("10.0.0.2") {
		t.Fatalf("B next-hops = %+v, want 10.0.0.2", b.NextHops)
	}
	routes := BuildRoutes(res, 8, nil)
	if len(routes) != 1 || routes[0].Prefix != netip.MustParsePrefix("192.0.2.0/24") || routes[0].Metric != 15 {
		t.Fatalf("routes = %+v, want 192.0.2.0/24 metric 15", routes)
	}
}

// RFC requirement: RFC2328-16.1-1 negative -- a one-way link (the neighbor's Router-LSA has no link back) fails the two-way check, so the neighbor vertex is never reached and no route is installed through it (twoWayRouterLink, spf.go:290-302).
func TestOSPFTwoWayCheck(t *testing.T) {
	area := testArea()
	db := testSource(t, area,
		routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.0.1", 10)),
		routerLSA(t, "2.2.2.2", stubLink(t, "192.0.2.0", 5)),
	)
	res := Compute(BuildGraph(db, area), testRID(t, "1.1.1.1"), 8)
	if got := res.Nodes[routerVertex(testRID(t, "2.2.2.2"))]; got != nil {
		t.Fatalf("one-way neighbor reached: %+v", got)
	}
	if routes := BuildRoutes(res, 8, nil); len(routes) != 0 {
		t.Fatalf("one-way topology installed routes: %+v", routes)
	}
}

func TestOSPFTransitNetworkSPF(t *testing.T) {
	area := testArea()
	db := testSource(t, area,
		routerLSA(t, "1.1.1.1", transitLink(t, "10.0.0.1")),
		routerLSA(t, "2.2.2.2", transitLink(t, "10.0.0.2"), stubLink(t, "198.51.100.0", 5)),
		networkLSA(t, "10.0.0.254", "2.2.2.2", "255.255.255.0", "1.1.1.1", "2.2.2.2"),
	)
	res := Compute(BuildGraph(db, area), testRID(t, "1.1.1.1"), 8)
	lan := res.Nodes[networkVertex(testLSID(t, "10.0.0.254"))]
	if lan == nil || lan.Metric != 1 {
		t.Fatalf("transit network result = %+v, want metric 1", lan)
	}
	b := res.Nodes[routerVertex(testRID(t, "2.2.2.2"))]
	if b == nil || b.Metric != 1 {
		t.Fatalf("B over transit result = %+v, want metric 1", b)
	}
	routes := BuildRoutes(res, 8, nil)
	if len(routes) != 1 || routes[0].Prefix != netip.MustParsePrefix("198.51.100.0/24") || routes[0].Metric != 6 {
		t.Fatalf("routes = %+v, want LAN stub metric 6", routes)
	}
}

func TestOSPFTransitNetworkRoute(t *testing.T) {
	area := testArea()
	// root 1.1.1.1 --p2p(cost 10)--> 2.2.2.2; routers 2.2.2.2 and 3.3.3.3 share a
	// transit LAN 10.0.1.0/25 (DR interface 10.0.1.126) that is REMOTE from the
	// root. RFC 2328 Section 16.1 step (4): the LAN's own subnet is an intra-area
	// route reached via the next-hop router on the SPT. A /25 mask also proves the
	// prefix length comes from the Network-LSA, not a hard-coded /24.
	db := testSource(t, area,
		routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.0.1", 10)),
		routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.0.2", 10), transitLinkDR(t, "10.0.1.126", "10.0.1.2", 1)),
		routerLSA(t, "3.3.3.3", transitLinkDR(t, "10.0.1.126", "10.0.1.3", 1)),
		networkLSA(t, "10.0.1.126", "3.3.3.3", "255.255.255.128", "2.2.2.2", "3.3.3.3"),
	)
	res := Compute(BuildGraph(db, area), testRID(t, "1.1.1.1"), 8)
	routes := BuildRoutes(res, 8, nil)
	var lan *RouteEntry
	for i := range routes {
		if routes[i].Prefix == netip.MustParsePrefix("10.0.1.0/25") {
			lan = &routes[i]
		}
	}
	if lan == nil {
		t.Fatalf("remote transit LAN 10.0.1.0/24 not installed; routes = %+v", routes)
	}
	if lan.Metric != 11 {
		t.Errorf("transit LAN metric = %d, want 11 (p2p 10 + transit link 1)", lan.Metric)
	}
	if lan.Type != RouteIntraArea {
		t.Errorf("transit LAN type = %v, want intra-area", lan.Type)
	}
	if len(lan.NextHops) != 1 || lan.NextHops[0].Addr != netip.MustParseAddr("10.0.0.2") {
		t.Errorf("transit LAN next-hops = %+v, want [10.0.0.2]", lan.NextHops)
	}
}

func TestOSPFNextHop(t *testing.T) {
	area := testArea()
	db := testSource(t, area,
		routerLSA(t, "1.1.1.1", transitLink(t, "10.0.0.1")),
		routerLSA(t, "2.2.2.2", transitLink(t, "10.0.0.2"), p2pLink(t, "3.3.3.3", "10.0.1.2", 4)),
		routerLSA(t, "3.3.3.3", p2pLink(t, "2.2.2.2", "10.0.1.3", 4), stubLink(t, "203.0.113.0", 2)),
		networkLSA(t, "10.0.0.254", "2.2.2.2", "255.255.255.0", "1.1.1.1", "2.2.2.2"),
	)
	res := Compute(BuildGraph(db, area), testRID(t, "1.1.1.1"), 8)
	b := res.Nodes[routerVertex(testRID(t, "2.2.2.2"))]
	if len(b.NextHops) != 1 || b.NextHops[0].Addr != netip.MustParseAddr("10.0.0.2") {
		t.Fatalf("transit next-hop = %+v, want B interface 10.0.0.2", b.NextHops)
	}
	c := res.Nodes[routerVertex(testRID(t, "3.3.3.3"))]
	if len(c.NextHops) != 1 || c.NextHops[0].Addr != netip.MustParseAddr("10.0.0.2") {
		t.Fatalf("inherited next-hop = %+v, want 10.0.0.2", c.NextHops)
	}
}

func TestOSPFSPFECMP(t *testing.T) {
	area := testArea()
	db := testSource(t, area,
		routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.0.1", 10), p2pLink(t, "3.3.3.3", "10.0.0.1", 10)),
		routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.0.2", 10), stubLink(t, "192.0.2.0", 5)),
		routerLSA(t, "3.3.3.3", p2pLink(t, "1.1.1.1", "10.0.0.3", 10), stubLink(t, "192.0.2.0", 5)),
	)
	res := Compute(BuildGraph(db, area), testRID(t, "1.1.1.1"), 8)
	routes := BuildRoutes(res, 8, nil)
	if len(routes) != 1 || len(routes[0].NextHops) != 2 {
		t.Fatalf("ECMP routes = %+v, want two next-hops", routes)
	}
	if routes[0].NextHops[0].Addr != netip.MustParseAddr("10.0.0.2") || routes[0].NextHops[1].Addr != netip.MustParseAddr("10.0.0.3") {
		t.Fatalf("ECMP next-hops = %+v", routes[0].NextHops)
	}
}

func TestOSPFStubAttach(t *testing.T) {
	area := testArea()
	res := Compute(BuildGraph(baseP2PSource(t, area), area), testRID(t, "1.1.1.1"), 8)
	routes := BuildRoutes(res, 8, staticInterfaceResolver{"10.0.0.2": "eth0"})
	if len(routes) != 1 {
		t.Fatalf("routes = %+v", routes)
	}
	if routes[0].Metric != 15 || routes[0].Type != RouteIntraArea || routes[0].NextHops[0].Interface != "eth0" {
		t.Fatalf("stub route = %+v", routes[0])
	}
}

type staticInterfaceResolver map[string]string

func (r staticInterfaceResolver) ResolveInterface(addr netip.Addr) (string, bool) {
	iface, ok := r[addr.String()]
	return iface, ok
}
