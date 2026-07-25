// VALIDATES: an IPv4 point-to-multipoint neighbor and its /32 host route resolve their
// next-hop from the neighbor's advertised point-to-point interface address (RFC 2328
// sec 16.1), reusing the point-to-point next-hop path unchanged.
// PREVENTS: a PtMP route installing with a subnet-derived or missing next-hop.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
)

func TestOSPFPtMPNextHop(t *testing.T) {
	area := testArea()
	// A PtMP segment looks identical to point-to-point at the SPF layer: each router
	// advertises a Type-1 link per neighbor plus a /32 host route for its own address.
	hostRoute := packet.RouterLink{
		LinkID:   testLSID(t, "10.0.0.2"),
		LinkData: testIP(t, "255.255.255.255"),
		Type:     packet.RouterLinkTypeStub,
		Metric:   0, // RFC 2328 sec 12.4.1.4 host route cost 0
	}
	db := testSource(t, area,
		routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.0.1", 10)),
		routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.0.2", 10), hostRoute),
	)
	res := Compute(BuildGraph(db, area), testRID(t, "1.1.1.1"), 8)
	b := res.Nodes[routerVertex(testRID(t, "2.2.2.2"))]
	if b == nil || len(b.NextHops) != 1 || b.NextHops[0].Addr != netip.MustParseAddr("10.0.0.2") {
		t.Fatalf("PtMP neighbor next-hop = %+v, want 10.0.0.2 (its advertised p2p interface address)", b)
	}
	routes := BuildRoutes(res, 8, nil)
	var found bool
	for _, r := range routes {
		if r.Prefix != netip.MustParsePrefix("10.0.0.2/32") {
			continue
		}
		found = true
		if len(r.NextHops) != 1 || r.NextHops[0].Addr != netip.MustParseAddr("10.0.0.2") {
			t.Fatalf("PtMP /32 host route next-hops = %+v, want 10.0.0.2", r.NextHops)
		}
		if r.Metric != 10 {
			t.Fatalf("PtMP /32 host route metric = %d, want 10 (path cost + host cost 0)", r.Metric)
		}
	}
	if !found {
		t.Fatalf("PtMP /32 host route 10.0.0.2/32 not installed; routes=%+v", routes)
	}
}
