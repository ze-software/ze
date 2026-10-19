package lsdb

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// unnumberedTopo is one point-to-point interface with a Full neighbor. The
// unnumbered case borrows 10.255.0.1 as a /32 on ifIndex 7 (gap #55).
func unnumberedTopo(unnumbered bool) OriginInput {
	info := InterfaceInfo{
		Name:        "eth0",
		AreaID:      area("0.0.0.0"),
		AreaType:    types.AreaTypeNormal,
		NetworkType: types.NetworkPointToPoint,
		State:       "point-to-point",
		Address:     ip4("10.255.0.1"),
		NetworkMask: ip4("255.255.255.255"),
		Cost:        10,
		RouterID:    rid("1.1.1.1"),
		Options:     types.OptionE,
		Neighbors:   []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: naddr4("10.255.0.2"), State: NeighborStateFull}},
		Unnumbered:  unnumbered,
		IfIndex:     7,
	}
	return OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, Interfaces: []InterfaceInfo{info}}
}

// TestUnnumberedP2PLinkDataIsIfIndex proves the Router-LSA of an unnumbered
// point-to-point interface carries the MIB-II ifIndex as the Type 1 Link Data
// and no stub link, while the same interface numbered keeps its address and
// its stub (RFC 2328 Section 12.4.1.1, owner decision for gap #55).
func TestUnnumberedP2PLinkDataIsIfIndex(t *testing.T) {
	p2p := routerLinksByType(t, unnumberedTopo(true), packet.RouterLinkTypeP2P)
	if len(p2p) != 1 {
		t.Fatalf("unnumbered: %d point-to-point links, want 1", len(p2p))
	}
	if p2p[0].LinkData != [4]byte{0, 0, 0, 7} {
		t.Fatalf("unnumbered: Link Data %v, want ifIndex 0.0.0.7", p2p[0].LinkData)
	}
	if stubs := routerLinksByType(t, unnumberedTopo(true), packet.RouterLinkTypeStub); len(stubs) != 0 {
		t.Fatalf("unnumbered: %d stub links %v, want none", len(stubs), stubs)
	}

	numbered := routerLinksByType(t, unnumberedTopo(false), packet.RouterLinkTypeP2P)
	if len(numbered) != 1 || numbered[0].LinkData != ip4("10.255.0.1") {
		t.Fatalf("numbered: point-to-point links %v, want Link Data 10.255.0.1", numbered)
	}
	if stubs := routerLinksByType(t, unnumberedTopo(false), packet.RouterLinkTypeStub); len(stubs) != 1 {
		t.Fatalf("numbered: %d stub links, want 1", len(stubs))
	}
}
