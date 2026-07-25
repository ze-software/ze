// VALIDATES: AC-16 / A-9 -- base LFA selection is address-family neutral: it
// flows through the NextHopSource AF seam unchanged, so the OSPFv3 engine gets a
// base LFA backup next-hop (an IPv6 link-local) with no v6-specific LFA wiring.
// PREVENTS: LFA hardcoding IPv4 next-hops and skipping the v6 engine.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// v6NHSeam is a test NextHopSource that resolves each neighbor to an IPv6
// link-local, standing in for the OSPFv3 adjacency-table seam. The SPF metrics
// are AF-neutral; only the next-hop addresses differ by family.
type v6NHSeam struct{ addrs map[types.RouterID]netip.Addr }

func (s v6NHSeam) P2PNextHop(_ *Graph, neighbor, _ types.RouterID) (netip.Addr, bool) {
	a, ok := s.addrs[neighbor]
	return a, ok
}

func (s v6NHSeam) TransitNextHop(_ *Graph, router types.RouterID, _ types.LinkStateID) (netip.Addr, bool) {
	a, ok := s.addrs[router]
	return a, ok
}

func TestLFAv6NextHopSelection(t *testing.T) {
	area := testArea()
	g := BuildGraph(triangleAltSource(t), area)
	root := testRID(t, "1.1.1.1")
	seam := v6NHSeam{addrs: map[types.RouterID]netip.Addr{
		testRID(t, "2.2.2.2"): netip.MustParseAddr("fe80::2"),
		testRID(t, "3.3.3.3"): netip.MustParseAddr("fe80::3"),
	}}
	// Resolve the primary tree AND the LFA pass through the same v6 seam.
	res := computeWithNextHop(g, root, 8, seam)
	routes := BuildRoutes(res, 8, nil)
	attachAllBackups(routes, fastRerouteInput{
		root: root, maxPaths: 8, nh: seam,
		results: map[types.AreaID]*Result{area: res},
		graphs:  map[types.AreaID]*Graph{area: g},
		cfg:     FastRerouteConfig{Enabled: true, NodeProtection: true},
		// sr is nil: OSPFv3 SR carriage (RFC 8666) is out of scope, so v6 gets
		// base-LFA next-hop selection only.
	})
	r, ok := backupFor(routes, "192.0.2.0/24")
	if !ok || len(r.Backups) == 0 || !r.Backups[0].Valid() {
		t.Fatalf("no v6 base LFA backup attached: %+v", routes)
	}
	b := r.Backups[0]
	if !b.NextHop.Is6() || b.NextHop != netip.MustParseAddr("fe80::3") {
		t.Fatalf("v6 backup next-hop = %v, want fe80::3 (via the AF seam)", b.NextHop)
	}
	if len(b.RepairLabels) != 0 {
		t.Fatalf("v6 base LFA must carry no SR repair labels, got %v", b.RepairLabels)
	}
}
