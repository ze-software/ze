package locrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// TestRaisedEbgpDistanceLetsOspfWin is the test AC-10 of
// spec-fixit-bgp-distance-declaration always named and never had.
//
// The spec's claim is the sentence an operator would use for the whole feature:
// raise the eBGP distance above OSPF's and the OSPF route is the one that wins.
// Until now that was INFERRED. TestBgpStampsTheDeclaredDistanceNotItsOwn
// (internal/component/bgp/plugins/rib/rib_bestchange_test.go) proves the
// declaration reaches the stamp, and stops there; it inserts no OSPF path and
// asserts only that the stamped value exceeds 110. Three independent closure
// gates in a row faulted the proof for stopping one layer above the behaviour,
// and this is that layer: selectBest is where the two protocols actually meet.
//
// PREVENTS: the declaration reaching the stamp correctly while cross-protocol
// selection ignores it, which is the exact defect the seam was built to fix and
// which every test written for that seam was structurally unable to see.
func TestRaisedEbgpDistanceLetsOspfWin(t *testing.T) {
	bgpID := redistevents.RegisterProtocol("test-bgp")
	ospfID := redistevents.RegisterProtocol("test-ospf")

	fam := family.IPv4Unicast
	prefix := netip.MustParsePrefix("10.0.0.0/24")

	// Default settings: eBGP at 20 beats OSPF at 110, so BGP wins.
	def := NewRIB()
	def.Insert(fam, prefix, Path{
		Source: bgpID, Instance: 1,
		NextHop:       netip.MustParseAddr("192.0.2.1"),
		AdminDistance: 20,
	})
	def.Insert(fam, prefix, Path{
		Source: ospfID, Instance: 2,
		NextHop:       netip.MustParseAddr("192.0.2.2"),
		AdminDistance: 110,
	})
	best, ok := def.Best(fam, prefix)
	if !ok {
		t.Fatal("no best path on the default settings")
	}
	if best.Source != bgpID {
		t.Fatalf("default settings: best is %v, want the eBGP path at 20", best.Source)
	}

	// The operator writes `rib { distance { ebgp 250 } }`. The declaration
	// reaches the BGP stamp through internal/core/rib/distance, so the eBGP
	// path arrives here carrying 250 instead of 20, and OSPF must now win.
	raised := NewRIB()
	raised.Insert(fam, prefix, Path{
		Source: bgpID, Instance: 1,
		NextHop:       netip.MustParseAddr("192.0.2.1"),
		AdminDistance: 250,
	})
	raised.Insert(fam, prefix, Path{
		Source: ospfID, Instance: 2,
		NextHop:       netip.MustParseAddr("192.0.2.2"),
		AdminDistance: 110,
	})
	best, ok = raised.Best(fam, prefix)
	if !ok {
		t.Fatal("no best path after the operator raised the eBGP distance")
	}
	if best.Source != ospfID {
		t.Fatalf("raised eBGP to 250: best is %v, want the OSPF path at 110", best.Source)
	}
	if best.NextHop != netip.MustParseAddr("192.0.2.2") {
		t.Errorf("next-hop = %v, want OSPF's 192.0.2.2", best.NextHop)
	}
}
