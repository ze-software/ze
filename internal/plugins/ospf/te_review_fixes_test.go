// VALIDATES: TE origination review fixes -- a multi-area TE router originates a Router-Address
// (Instance 0) into EVERY area with a TE link (RFC 3630 sec 2.4.1), self-originated TE links
// enter the router's own TED, and the TED Router-Address table is bounded.
// PREVENTS: a Router-Address advertised into only the lowest-numbered area, the local router's
// own TE links missing from `show ospf te-database` / a future rsvpte CSPF, and an unbounded
// Router-Address table.
package ospf

import (
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestTEMultiAreaRouterAddress pins fix D: two TE links in different areas yield a
// Router-Address (Instance 0) in each area, not only the lowest-numbered.
func TestTEMultiAreaRouterAddress(t *testing.T) {
	const cfg = `{"ospf":{"router-id":"1.1.1.1","router-address":"9.9.9.9","opaque":true,
	  "areas":{"area":{"0":{"area-id":"0"},"1":{"area-id":"0.0.0.1"}}},
	  "interfaces":{"interface":{
	    "eth0":{"name":"eth0","area":"0","network-type":"point-to-point","traffic-engineering":{"enable":true}},
	    "eth1":{"name":"eth1","area":"0.0.0.1","network-type":"point-to-point","traffic-engineering":{"enable":true}}}}}}`
	eng := teEngineWithTopology(t, cfg, []ospflsdb.InterfaceInfo{
		p2pTopo("eth0", [4]byte{10, 0, 0, 1}, types.RouterID{2, 2, 2, 2}, "10.0.0.2"),
		p2pTopo("eth1", [4]byte{10, 0, 1, 1}, types.RouterID{3, 3, 3, 3}, "10.0.1.3"),
	})

	raAreas := map[types.AreaID]bool{}
	for _, o := range eng.teOriginateType1(types.RouterID{1, 1, 1, 1}) {
		if o.Withdraw {
			continue
		}
		if decodeOrigTELSA(t, o).IsRouterAddress {
			if o.OpaqueID != 0 {
				t.Fatalf("Router-Address must use Instance 0, got %d", o.OpaqueID)
			}
			raAreas[o.Area] = true
		}
	}
	if !raAreas[types.BackboneArea] || !raAreas[(types.AreaID{0, 0, 0, 1})] {
		t.Fatalf("Router-Address not originated into every TE area: %v", raAreas)
	}
	if len(raAreas) != 2 {
		t.Fatalf("expected a Router-Address in exactly the 2 TE areas, got %d: %v", len(raAreas), raAreas)
	}
}

// TestTEOriginateInstallsSelfTED pins fix G: the router's own configured TE link and its
// Router-Address are present (and usable) in its own TED, feeding `show ospf te-database` and
// a future rsvpte CSPF, since self-originated LSAs bypass the reception path.
func TestTEOriginateInstallsSelfTED(t *testing.T) {
	eng := teEngineWithTopology(t, teOrigCfg,
		[]ospflsdb.InterfaceInfo{p2pTopo("eth0", [4]byte{10, 0, 0, 1}, types.RouterID{2, 2, 2, 2}, "10.0.0.2")})
	self := types.RouterID{1, 1, 1, 1}
	eng.teOriginateType1(self)

	snap := eng.ted.Snapshot()
	foundLink := false
	for i := range snap.Links {
		l := &snap.Links[i]
		if l.AdvertisingRouter == self && l.Link.HasLinkID && l.Link.LinkID == [4]byte{2, 2, 2, 2} {
			if !l.Usable {
				t.Fatalf("self TE link must be usable")
			}
			foundLink = true
		}
	}
	if !foundLink {
		t.Fatalf("self TE link absent from own TED: %+v", snap.Links)
	}
	foundRA := false
	for _, ra := range snap.RouterAddresses {
		if ra.Router == self && ra.Address == [4]byte{9, 9, 9, 9} {
			foundRA = true
		}
	}
	if !foundRA {
		t.Fatalf("self Router-Address absent from own TED: %+v", snap.RouterAddresses)
	}

	// The show handler renders the same TED, so the local link surfaces in `show ospf te-database`.
	if rows := eng.teDatabaseSnapshot(); len(rows) != 1 {
		t.Fatalf("te-database snapshot wrapping = %d, want 1", len(rows))
	}
}

// TestTEDRouterAddressBounded pins fix F: the TED Router-Address table is bounded like the
// link table, evicting the oldest instance rather than growing without limit.
func TestTEDRouterAddressBounded(t *testing.T) {
	eng, _ := newRedistEngine(t, teCfgJSON)
	eng.ted.setMax(4)
	ra := packet.TELSA{IsRouterAddress: true, RouterAddress: [4]byte{9, 9, 9, 9}}
	for i := range 20 {
		adv := types.RouterID{byte(i), 0, 0, 1}
		eng.ted.applyLSA(adv, types.BackboneArea, OpaqueScopeArea, packet.TEOpaqueType, 0, ra, true)
	}
	if n := len(eng.ted.routerRA); n > 4 {
		t.Fatalf("TED Router-Address table grew to %d, must be bounded at 4", n)
	}
}
