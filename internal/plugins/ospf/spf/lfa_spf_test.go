// VALIDATES: RFC 5286 Section 3 -- per-neighbor SPF (A-1: Compute re-rooted at a
// neighbor yields correct D_opt(N,*)), and the per-primary backup keying (AC-10:
// each primary next-hop of an ECMP prefix carries its own backup, parallel to
// NextHops, never folded into the ECMP set).
// PREVENTS: a bespoke reverse-SPF (A-1) and a route-scalar backup that loses
// per-primary protection.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// attachTopo builds the backbone-area graph rooted at S (1.1.1.1), runs SPF +
// BuildRoutes, then attaches LFA/TI-LFA backups, mirroring Computer.Run's
// fast-reroute pass on one area.
func attachTopo(t *testing.T, db Source, cfg FastRerouteConfig, sr SRResolver) []RouteEntry {
	t.Helper()
	area := testArea()
	root := testRID(t, "1.1.1.1")
	g := BuildGraph(db, area)
	res := Compute(g, root, 8)
	routes := BuildRoutes(res, 8, nil)
	attachAllBackups(routes, fastRerouteInput{
		root:     root,
		maxPaths: 8,
		nh:       v4NextHop{},
		results:  map[types.AreaID]*Result{area: res},
		graphs:   map[types.AreaID]*Graph{area: g},
		cfg:      cfg,
		sr:       sr,
	})
	return routes
}

func backupFor(routes []RouteEntry, prefix string) (RouteEntry, bool) {
	p := netip.MustParsePrefix(prefix)
	for _, r := range routes {
		if r.Prefix == p {
			return r, true
		}
	}
	return RouteEntry{}, false
}

// triangleAltSource: S(1.1.1.1)-E(2.2.2.2)-N(3.3.3.3) triangle, all links cost 10,
// with a stub 192.0.2.0/24 (cost 1) behind E. N is a link-protecting LFA for the
// primary S->E->stub.
func triangleAltSource(t *testing.T) Source {
	t.Helper()
	return testSource(t, testArea(),
		routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.12.1", 10), p2pLink(t, "3.3.3.3", "10.0.13.1", 10)),
		routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.12.2", 10), p2pLink(t, "3.3.3.3", "10.0.23.2", 10), stubLink(t, "192.0.2.0", 1)),
		routerLSA(t, "3.3.3.3", p2pLink(t, "1.1.1.1", "10.0.13.3", 10), p2pLink(t, "2.2.2.2", "10.0.23.3", 10)),
	)
}

func TestPerNeighborSPFDistances(t *testing.T) {
	// A-1: Compute re-rooted at neighbor N=3.3.3.3 yields D_opt(N,*) directly from
	// Nodes[*].Metric, with no new graph code.
	area := testArea()
	g := BuildGraph(triangleAltSource(t), area)
	sptN := Compute(g, testRID(t, "3.3.3.3"), 8)
	if got := vertexDist(sptN, routerVertex(testRID(t, "1.1.1.1"))); got != 10 {
		t.Fatalf("D_opt(N,S) = %d, want 10", got)
	}
	if got := vertexDist(sptN, routerVertex(testRID(t, "2.2.2.2"))); got != 10 {
		t.Fatalf("D_opt(N,E) = %d, want 10", got)
	}
	// And the base LFA attaches N (10.0.13.3) as the link-protecting backup for the
	// prefix behind E.
	routes := attachTopo(t, triangleAltSource(t), FastRerouteConfig{Enabled: true, NodeProtection: true}, nil)
	r, ok := backupFor(routes, "192.0.2.0/24")
	if !ok {
		t.Fatalf("prefix 192.0.2.0/24 not in route set")
	}
	if len(r.Backups) != 1 || !r.Backups[0].Valid() {
		t.Fatalf("no backup attached: %+v", r.Backups)
	}
	if r.Backups[0].NextHop != netip.MustParseAddr("10.0.13.3") {
		t.Fatalf("backup = %v, want 10.0.13.3 (via N)", r.Backups[0].NextHop)
	}
	if !r.Backups[0].LinkProtect {
		t.Fatalf("backup class = %s, want link-protecting", r.Backups[0].Class())
	}
}

// ecmpBackupSource: S has two equal-cost paths to D (via E1 and E2), plus a
// non-primary neighbor N reachable at cost 10 whose direct link to D (cost 20)
// makes it a loop-free alternate but never a primary.
func ecmpBackupSource(t *testing.T) Source {
	t.Helper()
	return testSource(t, testArea(),
		routerLSA(t, "1.1.1.1",
			p2pLink(t, "2.2.2.2", "10.0.12.1", 10),
			p2pLink(t, "3.3.3.3", "10.0.13.1", 10),
			p2pLink(t, "5.5.5.5", "10.0.15.1", 10)),
		routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.12.2", 10), p2pLink(t, "4.4.4.4", "10.0.24.2", 5)),
		routerLSA(t, "3.3.3.3", p2pLink(t, "1.1.1.1", "10.0.13.3", 10), p2pLink(t, "4.4.4.4", "10.0.34.3", 5)),
		routerLSA(t, "5.5.5.5", p2pLink(t, "1.1.1.1", "10.0.15.5", 10), p2pLink(t, "4.4.4.4", "10.0.45.5", 20)),
		routerLSA(t, "4.4.4.4",
			p2pLink(t, "2.2.2.2", "10.0.24.4", 5),
			p2pLink(t, "3.3.3.3", "10.0.34.4", 5),
			p2pLink(t, "5.5.5.5", "10.0.45.4", 20),
			stubLink(t, "198.51.100.0", 1)),
	)
}

func TestBackupPerPrimaryNextHop(t *testing.T) {
	// AC-10: the prefix behind D has two ECMP primaries (via E1, E2). Each primary
	// next-hop carries its own backup in a parallel slice; the backups are not
	// merged into the primary ECMP next-hop set (which stays length 2).
	routes := attachTopo(t, ecmpBackupSource(t), FastRerouteConfig{Enabled: true, NodeProtection: true}, nil)
	r, ok := backupFor(routes, "198.51.100.0/24")
	if !ok {
		t.Fatalf("prefix not in route set: %+v", routes)
	}
	if len(r.NextHops) != 2 {
		t.Fatalf("primary next-hops = %d, want 2 ECMP", len(r.NextHops))
	}
	if len(r.Backups) != len(r.NextHops) {
		t.Fatalf("backups len %d != next-hops len %d (must be per-primary parallel)", len(r.Backups), len(r.NextHops))
	}
	for i, b := range r.Backups {
		if !b.Valid() {
			t.Fatalf("primary %d (%v) has no backup", i, r.NextHops[i].Addr)
		}
	}
}
