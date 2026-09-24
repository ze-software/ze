// Design: docs/architecture/ospf/ospf-8-spf-rib.md -- classless prefixes through SPF and Loc-RIB.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func classlessRouteComputer(t *testing.T) (*Computer, *ospflsdb.LSDB, *locrib.RIB) {
	t.Helper()
	stub := func(prefix string) packet.RouterLink {
		p := netip.MustParsePrefix(prefix)
		return packet.RouterLink{Type: packet.RouterLinkTypeStub, LinkID: types.LinkStateID(p.Addr().As4()), LinkData: maskFromBits(p.Bits()), Metric: 5}
	}
	area := testArea()
	db := testSource(t, area,
		routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.0.1", 10), p2pLink(t, "3.3.3.3", "10.0.1.1", 20), p2pLink(t, "4.4.4.4", "10.0.2.1", 30)),
		routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.0.2", 10), stub("192.0.0.0/16")),
		routerLSA(t, "3.3.3.3", p2pLink(t, "1.1.1.1", "10.0.1.3", 20), stub("192.0.2.0/24")),
		routerLSA(t, "4.4.4.4", p2pLink(t, "1.1.1.1", "10.0.2.4", 30), stub("192.0.2.128/25")),
	)
	loc := locrib.NewRIB()
	computer := NewComputer(Config{Source: db, Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area}, Installer: NewInstaller(loc)})
	t.Cleanup(computer.Stop)
	computer.Run()
	return computer, db, loc
}

// RFC requirement: RFC2328-4.4-2 positive -- SPF installs an overlapping supernet,
// class-C-sized network and subnet into the shared IP Loc-RIB with each advertised
// mask and distinct next hop intact; the address's historical class does not select a mask.
func TestRFC2328ClasslessPrefixesReachIPRIB(t *testing.T) {
	_, _, loc := classlessRouteComputer(t)
	for _, tc := range []struct{ prefix, nextHop string }{
		{"192.0.0.0/16", "10.0.0.2"},
		{"192.0.2.0/24", "10.0.1.3"},
		{"192.0.2.128/25", "10.0.2.4"},
	} {
		paths := lookupPaths(t, loc, netip.MustParsePrefix(tc.prefix))
		if len(paths) != 1 || paths[0].NextHop != netip.MustParseAddr(tc.nextHop) {
			t.Fatalf("%s IP route = %+v, want next hop %s", tc.prefix, paths, tc.nextHop)
		}
	}
}

// RFC requirement: RFC2328-4.4-2 negative -- removing the /24 route neither removes
// its covering /16 nor coalesces its still-reachable /25 into the withdrawn classful mask.
func TestRFC2328ClasslessWithdrawalPreservesOverlaps(t *testing.T) {
	computer, db, loc := classlessRouteComputer(t)
	peer := testRID(t, "3.3.3.3")
	if !db.Delete(testArea(), types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(peer), AdvertisingRouter: peer}) {
		t.Fatal("withdrawal Router-LSA missing")
	}
	computer.Run()
	if paths := lookupPaths(t, loc, netip.MustParsePrefix("192.0.2.0/24")); len(paths) != 0 {
		t.Fatalf("withdrawn /24 still installed: %+v", paths)
	}
	for _, tc := range []struct{ prefix, nextHop string }{
		{"192.0.0.0/16", "10.0.0.2"},
		{"192.0.2.128/25", "10.0.2.4"},
	} {
		paths := lookupPaths(t, loc, netip.MustParsePrefix(tc.prefix))
		if len(paths) != 1 || paths[0].NextHop != netip.MustParseAddr(tc.nextHop) {
			t.Fatalf("remaining %s IP route = %+v, want next hop %s", tc.prefix, paths, tc.nextHop)
		}
	}
}
