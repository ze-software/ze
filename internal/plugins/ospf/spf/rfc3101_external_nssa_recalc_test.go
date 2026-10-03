// VALIDATES: the production Computer recalculates Type-7 external routes from
// the current per-area ASBR graph, including withdrawal after reachability loss.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func nssaCalculationComputer(t *testing.T) (*Computer, *ospflsdb.LSDB, packet.LSA) {
	t.Helper()
	area := types.AreaID{0, 0, 0, 7}
	root := testRID(t, "1.1.1.1")
	peer := routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.7.2", 10))
	peer.Router.Flags = packet.RouterFlagE
	db := testSource(t, area,
		routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.7.1", 10)), peer)
	external := type7LSA(t, "203.0.113.0", "255.255.255.0", "2.2.2.2", 5, false)
	external.External.ExternalType2 = false
	external.External.ForwardingAddr = [4]byte{}
	if !db.Install(area, external) {
		t.Fatal("failed to install initial Type-7")
	}
	backbonePeer := routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.0.2", 1))
	backbonePeer.Router.Flags = packet.RouterFlagE
	for _, lsa := range []packet.LSA{
		routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.0.1", 1)), backbonePeer,
	} {
		if !db.Install(types.BackboneArea, lsa) {
			t.Fatal("failed to install cheaper backbone ASBR path")
		}
	}
	computer := NewComputer(Config{Source: db, Root: root, Areas: []types.AreaID{types.BackboneArea, area}, AreaConfigs: []AreaConfig{{AreaID: types.BackboneArea, AreaType: types.AreaTypeNormal}, {AreaID: area, AreaType: types.AreaTypeNSSA}}, Installer: NewInstaller(locrib.NewRIB())})
	t.Cleanup(computer.Stop)
	return computer, db, external
}

// RFC requirement: RFC3101-2.5-5 positive -- Computer.Run performs the NSSA external calculation, then updates the installed external when only its Type-7 metric changes.
// RFC requirement: RFC3101-2.5-2 positive -- the complete SPF pipeline preserves the NSSA ASBR entry even when the same ASBR is cheaper over the backbone.
// MUTATION: omit NSSAAreas from the external stage, or retain the preceding external route instead of recalculating it.
func TestNSSAExternalCalculationRunsOnType7Change(t *testing.T) {
	computer, db, external := nssaCalculationComputer(t)
	prefix := netip.MustParsePrefix("203.0.113.0/24")
	first := computer.Run()
	if len(first.Added) != 1 || first.Added[0].Prefix != prefix || first.Added[0].Metric != 15 {
		t.Fatalf("initial NSSA route delta = %+v, want cost 15", first)
	}
	external.Header.Sequence = external.Header.Sequence.Next()
	external.External.Metric = 9
	area := types.AreaID{0, 0, 0, 7}
	if !db.Install(area, external) {
		t.Fatal("failed to install changed Type-7")
	}
	computer.TriggerArea(area)
	updated := computer.Run()
	if len(updated.Changed) != 1 || updated.Changed[0].Prefix != prefix || updated.Changed[0].Metric != 19 {
		t.Fatalf("changed NSSA route delta = %+v, want cost 19", updated)
	}
}

// RFC requirement: RFC3101-2.5-5 negative -- an external recalculation withdraws the installed Type-7 when the originating NSSA loses its ASBR, despite the LSA remaining present.
// RFC requirement: RFC3101-2.5-2 negative -- a surviving backbone path to the same ASBR cannot retain its Type-7 after its NSSA path disappears.
// MUTATION: reuse cached ASBR reachability from the preceding external calculation.
func TestNSSAExternalCalculationDropsLostASBR(t *testing.T) {
	computer, db, _ := nssaCalculationComputer(t)
	first := computer.Run()
	if len(first.Added) != 1 {
		t.Fatalf("initial external delta = %+v", first)
	}
	area := types.AreaID{0, 0, 0, 7}
	peer := routerLSA(t, "2.2.2.2")
	peer.Router.Flags = packet.RouterFlagE
	peer.Header.Sequence = peer.Header.Sequence.Next()
	if !db.Install(area, peer) {
		t.Fatal("failed to install disconnected ASBR")
	}
	computer.TriggerArea(area)
	removed := computer.Run()
	if len(removed.Removed) != 1 || removed.Removed[0] != netip.MustParsePrefix("203.0.113.0/24") {
		t.Fatalf("lost ASBR delta = %+v, want external withdrawal", removed)
	}
}
