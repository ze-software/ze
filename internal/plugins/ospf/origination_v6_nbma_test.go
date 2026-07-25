// VALIDATES: OSPFv3 NBMA + point-to-multipoint origination. PtMP emits one address-free
// Type-1 Router-LSA link per Full neighbor and a /128 LA-bit host route in the
// Intra-Area-Prefix-LSA, and no Network-LSA; NBMA emits a transit link and (as DR) a
// 0x2002 Network-LSA; every NBMA + PtMP interface originates its Link-LSA.
// PREVENTS: a v6 PtMP interface leaking a subnet prefix or a Network-LSA, mis-encoding
// the /128 host route, or an NBMA/PtMP interface never originating its Link-LSA.
package ospf

import (
	"net/netip"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func v6PtMPInterface(area types.AreaID, self, neighbor types.RouterID) ospflsdb.InterfaceInfo {
	return ospflsdb.InterfaceInfo{
		Name:          "eth0",
		AreaID:        area,
		NetworkType:   ospflsdb.NetworkPointToMultipoint,
		State:         "point-to-point",
		Cost:          10,
		RouterID:      self,
		InterfaceID:   7,
		IsV6:          true,
		IPv6LinkLocal: netip.MustParseAddr("fe80::2"),
		IPv6Prefixes:  []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")},
		IPv6Addresses: []netip.Addr{netip.MustParseAddr("2001:db8:1::2")},
		Neighbors: []ospflsdb.NeighborInfo{{
			RouterID:    neighbor,
			Address:     netip.MustParseAddr("fe80::1"),
			State:       ospflsdb.NeighborStateFull,
			InterfaceID: 11,
		}},
	}
}

func v6NBMAInterface(area types.AreaID, self types.RouterID) ospflsdb.InterfaceInfo {
	iface := v6BroadcastInterface(area, self)
	iface.NetworkType = ospflsdb.NetworkNBMA
	return iface
}

func TestOSPFv3PtMPRouterLSALinks(t *testing.T) {
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	neighbor := types.RouterID{172, 30, 0, 1}
	area := types.BackboneArea
	opts := ospfv3types.OptV6 | ospfv3types.OptR
	h, ok := e.v6OriginateRouter(area, router, opts, []ospflsdb.InterfaceInfo{v6PtMPInterface(area, router, neighbor)}, false, false, false, false)
	if !ok {
		t.Fatalf("v6OriginateRouter returned false")
	}
	lsa, _ := e.lsdb.LookupLSA(area, h.Key())
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	body, err := decoded.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if len(body.Links) != 1 {
		t.Fatalf("PtMP links = %d, want 1 (per Full neighbor)", len(body.Links))
	}
	l := body.Links[0]
	if l.Type != ospfv3packet.RouterLinkTypeP2P {
		t.Fatalf("link type = %d, want p2p (PtMP is address-free point-to-point)", l.Type)
	}
	if l.NeighborRouterID != ospfv3types.RouterID(neighbor) || l.NeighborInterfaceID != 11 || l.InterfaceID != 7 {
		t.Fatalf("link = %+v, want neighbor rid/ifid 11 + our ifid 7", l)
	}
}

func TestOSPFv3PtMPHostRoute(t *testing.T) {
	router := types.RouterID{172, 30, 0, 2}
	prefixes := v6InterfacePrefixes([]ospflsdb.InterfaceInfo{v6PtMPInterface(types.BackboneArea, router, types.RouterID{172, 30, 0, 1})})
	if len(prefixes) != 1 {
		t.Fatalf("PtMP prefixes = %d, want 1 (/128 host route only, no subnet)", len(prefixes))
	}
	p := prefixes[0]
	if p.Length != ospfv3types.MaxPrefixLength {
		t.Fatalf("prefix length = %d, want 128", p.Length)
	}
	if !p.Options.Has(ospfv3types.OptPrefixLA) {
		t.Fatalf("prefix options = %#x, want LA-bit set", uint8(p.Options))
	}
	pfx, ok := v6PrefixToNetip(p, afIPv6Unicast)
	if !ok || pfx != netip.MustParsePrefix("2001:db8:1::2/128") {
		t.Fatalf("host route = %v ok=%v, want 2001:db8:1::2/128", pfx, ok)
	}
}

func TestOSPFv3PtMPNoNetworkLSA(t *testing.T) {
	router := types.RouterID{172, 30, 0, 2}
	iface := v6PtMPInterface(types.BackboneArea, router, types.RouterID{172, 30, 0, 1})
	if v6OriginatesNetworkLSA(iface, router) {
		t.Fatalf("PtMP interface must not originate a Network-LSA")
	}
}

func TestOSPFv3NBMANetworkLSA(t *testing.T) {
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	area := types.BackboneArea
	iface := v6NBMAInterface(area, router)
	if !v6OriginatesNetworkLSA(iface, router) {
		t.Fatalf("NBMA DR must originate a Network-LSA")
	}
	key, ok := e.v6OriginateNetwork(area, router, ospfv3types.OptV6|ospfv3types.OptR, iface)
	if !ok {
		t.Fatalf("v6OriginateNetwork returned false")
	}
	lsa, found := e.lsdb.LookupLSA(area, key)
	if !found {
		t.Fatalf("NBMA Network-LSA not installed")
	}
	if !ospfv3packet.VerifyLSAChecksum(lsa.RawBytes) {
		t.Fatalf("NBMA Network-LSA checksum invalid")
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	if _, err := decoded.DecodeNetwork(); err != nil {
		t.Fatalf("DecodeNetwork: %v", err)
	}
}

func TestOSPFv3NBMALinkLSA(t *testing.T) {
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	iface := v6NBMAInterface(types.BackboneArea, router)
	e.lsdb.SetTx((&ospfRawTx{}).Send)
	e.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo { return []ospflsdb.InterfaceInfo{iface} })
	if !v6ShouldOriginateLinkLSA(iface) {
		t.Fatalf("NBMA interface must originate a Link-LSA")
	}
	key, changed := e.v6OriginateLinkLSA(router, iface)
	if !changed {
		t.Fatalf("v6OriginateLinkLSA did not originate")
	}
	if _, ok := e.lsdb.LookupLinkLSA("eth0", key); !ok {
		t.Fatalf("NBMA Link-LSA not installed")
	}
}

func TestOSPFv3PtMPLinkLSA(t *testing.T) {
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	iface := v6PtMPInterface(types.BackboneArea, router, types.RouterID{172, 30, 0, 1})
	e.lsdb.SetTx((&ospfRawTx{}).Send)
	e.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo { return []ospflsdb.InterfaceInfo{iface} })
	if !v6ShouldOriginateLinkLSA(iface) {
		t.Fatalf("PtMP interface must originate a Link-LSA")
	}
	key, changed := e.v6OriginateLinkLSA(router, iface)
	if !changed {
		t.Fatalf("v6OriginateLinkLSA did not originate")
	}
	if _, ok := e.lsdb.LookupLinkLSA("eth0", key); !ok {
		t.Fatalf("PtMP Link-LSA not installed")
	}
}
