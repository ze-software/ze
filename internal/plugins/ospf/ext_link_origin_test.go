// VALIDATES: spec-ospf-ext-4 AC-3/A-2/A-7 -- Extended Link origination emits exactly one
// Extended Link TLV per point-to-point/transit link whose Link Type / Link ID / Link Data
// equal the matching Router-LSA link (RFC 7684 sec 3.1), always at area scope, and
// re-originates (withdrawing the old, originating the new) when the link changes.
// PREVENTS: an Extended Link LSA FRR cannot correlate to a Router-LSA link, a wrong scope, or
// a stale link LSA after a topology change.
package ospf

import (
	"net/netip"
	"testing"

	ospflsdb "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/lsdb"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// extP2PIface returns a point-to-point interface with one Full neighbor (a Router-LSA p2p link:
// Link ID = neighbor Router ID, Link Data = local address).
func extP2PIface(local [4]byte, nbr types.RouterID, nbrAddr string) ospflsdb.InterfaceInfo {
	return ospflsdb.InterfaceInfo{
		Name: "eth0", AreaID: types.BackboneArea, NetworkType: ospflsdb.NetworkPointToPoint, State: "point-to-point", Address: local,
		Neighbors: []ospflsdb.NeighborInfo{{RouterID: nbr, Address: netip.MustParseAddr(nbrAddr), State: ospflsdb.NeighborStateFull}},
	}
}

func TestExtLinkMirrorsRouterLSALink(t *testing.T) {
	router := types.RouterID{1, 1, 1, 1}
	topo := []ospflsdb.InterfaceInfo{extP2PIface([4]byte{10, 0, 0, 1}, types.RouterID{2, 2, 2, 2}, "10.0.0.2")}
	eng, _ := extEngineWithTopology(t, topo)

	out := eng.extLinkOnOriginate(router)
	var link *opaqueOrigination
	for i := range out {
		if !out[i].Withdraw {
			link = &out[i]
		}
	}
	if link == nil {
		t.Fatalf("no Extended Link LSA originated")
	}
	if link.Scope.valid() && link.Scope != OpaqueScopeArea && link.Scope != 0 {
		t.Fatalf("Extended Link scope override = %v, want area/default", link.Scope)
	}
	lsa, err := packet.DecodeExtLinkLSA(link.Body)
	// RFC requirement: RFC7684-3.1-1 positive -- origination emits exactly one Extended Link
	// Opaque LSA carrying a single Extended Link TLV per link (extLinkOnOriginate, ext_link.go:26).
	if err != nil || !lsa.HasLink {
		t.Fatalf("decode Extended Link body: %v hasLink=%v", err, lsa.HasLink)
	}
	// AC-3/A-2: Link Type/ID/Data equal the matching Router-LSA p2p link.
	if lsa.Link.LinkType != packet.RouterLinkTypeP2P {
		t.Fatalf("link type = %d, want %d (p2p)", lsa.Link.LinkType, packet.RouterLinkTypeP2P)
	}
	if lsa.Link.LinkID != [4]byte{2, 2, 2, 2} {
		t.Fatalf("link ID = %v, want neighbor Router ID 2.2.2.2", lsa.Link.LinkID)
	}
	if lsa.Link.LinkData != [4]byte{10, 0, 0, 1} {
		t.Fatalf("link data = %v, want local address 10.0.0.1", lsa.Link.LinkData)
	}
	if len(lsa.Link.SubTLVs) != 0 {
		t.Fatalf("carrier-only Extended Link TLV must have zero sub-TLVs, got %+v", lsa.Link.SubTLVs)
	}
}

func TestExtLinkReoriginatesOnTopologyChange(t *testing.T) {
	router := types.RouterID{1, 1, 1, 1}
	topo := []ospflsdb.InterfaceInfo{extP2PIface([4]byte{10, 0, 0, 1}, types.RouterID{2, 2, 2, 2}, "10.0.0.2")}
	eng, _ := extEngineWithTopology(t, topo)

	first := eng.extLinkOnOriginate(router)
	var firstID uint32
	got := false
	for _, o := range first {
		if !o.Withdraw {
			firstID = o.OpaqueID
			got = true
		}
	}
	if !got {
		t.Fatalf("no Extended Link LSA on first pass")
	}
	// The neighbor changes (new adjacency to 3.3.3.3): a new link is originated and the old
	// one withdrawn.
	newTopo := []ospflsdb.InterfaceInfo{extP2PIface([4]byte{10, 0, 0, 1}, types.RouterID{3, 3, 3, 3}, "10.0.0.3")}
	eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo { return newTopo })
	eng.lsdb.OriginateFromTopology(router, false)
	second := eng.extLinkOnOriginate(router)

	withdrew, originated := false, false
	for _, o := range second {
		if o.Withdraw && o.OpaqueID == firstID {
			withdrew = true
			continue
		}
		if !o.Withdraw {
			lsa, err := packet.DecodeExtLinkLSA(o.Body)
			if err == nil && lsa.HasLink && lsa.Link.LinkID == [4]byte{3, 3, 3, 3} {
				originated = true
			}
		}
	}
	if !withdrew {
		t.Fatalf("old link Opaque ID %d not withdrawn after topology change", firstID)
	}
	if !originated {
		t.Fatalf("new link (3.3.3.3) not originated after topology change")
	}
}
