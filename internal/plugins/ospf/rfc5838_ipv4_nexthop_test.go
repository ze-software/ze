// Design: docs/architecture/ospf/ospf-ext-15-multi-af.md -- one engine per address family
// RFC: rfc/short/rfc5838.md (Section 2.5 IPv4 next hops from the Link-LSA)
// Related: origination_v6_link.go -- v6OriginateLinkLSA; afstrategy_v6.go -- v6NextHop
package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// VALIDATES: an IPv4 address family's Link-LSA carries the link's IPv4 address in
// the first 32 bits of its "link local address" field with the other bits zero,
// and its IPv4 on-link prefix; an IPv6 family keeps its IPv6 link-local.
// Method: originate the Link-LSA of one interface from an IPv4-unicast engine and
// from an IPv6-unicast engine, and decode what each installed.
// PREVENTS: an IPv4-over-OSPFv3 router advertising an IPv6 link-local, which no
// IPv4 next hop can be computed from (two Ze routers installed no IPv4 route).
func TestRFC5838IPv4LinkLSACarriesTheLinkIPv4Address(t *testing.T) {
	area := types.BackboneArea
	self := types.RouterID{172, 30, 0, 2}
	iface := v6BroadcastInterface(area, self)
	iface.Address = [4]byte{172, 30, 0, 2}
	iface.NetworkMask = [4]byte{255, 255, 255, 0}

	// RFC requirement: RFC5838-2.5-1 positive -- the IPv4 AF's Link-LSA carries the
	// link's IPv4 address in the leading 32 bits, the remaining bits zero.
	e := newV6OriginEngine()
	e.af = afIPv4Unicast
	link := originatedLinkBody(t, e, self, iface)
	want := [16]byte{172, 30, 0, 2}
	if link.LinkLocalAddr != want {
		t.Fatalf("IPv4 AF Link-LSA link local address = % x, want % x", link.LinkLocalAddr, want)
	}
	if len(link.Prefixes) != 1 {
		t.Fatalf("IPv4 AF Link-LSA prefixes = %d, want the one IPv4 on-link prefix", len(link.Prefixes))
	}
	if pfx, ok := v6PrefixToNetip(link.Prefixes[0], afIPv4Unicast); !ok || pfx != netip.MustParsePrefix("172.30.0.0/24") {
		t.Fatalf("IPv4 AF Link-LSA prefix = %v ok=%v, want 172.30.0.0/24", pfx, ok)
	}

	// The IPv6 family is unchanged: its link-local stays in the field.
	e6 := newV6OriginEngine()
	e6.af = afIPv6Unicast
	link6 := originatedLinkBody(t, e6, self, iface)
	if netip.AddrFrom16(link6.LinkLocalAddr) != netip.MustParseAddr("fe80::1") {
		t.Fatalf("IPv6 AF Link-LSA link local address = %v, want fe80::1", netip.AddrFrom16(link6.LinkLocalAddr))
	}
}

func originatedLinkBody(t *testing.T, e *engine, self types.RouterID, iface ospflsdb.InterfaceInfo) ospfv3packet.LinkLSA {
	t.Helper()
	e.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo { return []ospflsdb.InterfaceInfo{iface} })
	key, changed := e.v6OriginateLinkLSA(self, iface)
	if !changed {
		t.Fatal("v6OriginateLinkLSA did not originate")
	}
	lsa, ok := e.lsdb.LookupLinkLSA(iface.Name, key)
	if !ok {
		t.Fatal("self Link-LSA not installed")
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	link, err := decoded.DecodeLink()
	if err != nil {
		t.Fatalf("DecodeLink: %v", err)
	}
	return link
}

// VALIDATES: an IPv4 address family computes its next hop from the IPv4 address in
// the neighbor's Link-LSA, never from the neighbor's IPv6 link-local.
// Method: a Full point-to-point neighbor whose Hello came from fe80::2 and whose
// Link-LSA carries 172.30.0.3; resolve the SPF next hop with and without that
// Link-LSA in the link-scope LSDB.
// PREVENTS: an IPv4 route over OSPFv3 installed through an IPv6 link-local, or
// installed at all with no IPv4 next hop the neighbor advertised.
func TestRFC5838IPv4NextHopFromNeighborLinkLSA(t *testing.T) {
	self := types.RouterID{1, 1, 1, 1}
	neighborID := types.RouterID{2, 2, 2, 2}
	area := types.BackboneArea
	g, link := v6P2PGraph(t, self, neighborID, area)
	tbl := fullV6Neighbor(t, self, neighborID, area)

	db := ospflsdb.New(func() time.Time { return time.Unix(0, 0) })
	nh := v6NextHop{neighbors: tbl, lsdb: db, af: afIPv4Unicast}
	// RFC requirement: RFC5838-2.5-1 negative -- with no Link-LSA from the neighbor
	// there is no IPv4 next hop, and the IPv6 link-local is never used instead.
	if got, ok := nh.P2PNextHop(g, neighborID, self, link); ok {
		t.Fatalf("IPv4 next hop without the neighbor's Link-LSA = %v, want none", got)
	}

	db.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{{
			Name: "eth0", AreaID: area, NetworkType: types.NetworkPointToPoint, State: "point-to-point",
			RouterID: self, InterfaceID: 7, IsV6: true,
			Neighbors: []ospflsdb.NeighborInfo{{RouterID: neighborID, Address: netip.MustParseAddr("fe80::2"), State: ospflsdb.NeighborStateFull, InterfaceID: 11}},
		}}
	})
	neighborLink := ipv4AFLinkLSA(t, neighborID, 11, [4]byte{172, 30, 0, 3})
	if reason := db.ReceiveUpdate(ospflsdb.ReceiveInput{
		Interface: "eth0", AreaID: area, RouterID: neighborID, Src: netip.MustParseAddr("fe80::2"),
		Update: packet.LSUpdate{LSAs: []packet.LSA{neighborLink}},
	}); reason != "" {
		t.Fatalf("neighbor Link-LSA receive: %s", reason)
	}
	// RFC requirement: RFC5838-2.5-1 positive -- the IPv4 next hop is the address the
	// neighbor's Link-LSA carries in its leading 32 bits.
	got, ok := nh.P2PNextHop(g, neighborID, self, link)
	if !ok || got.Addr != netip.MustParseAddr("172.30.0.3") || got.Interface != "eth0" {
		t.Fatalf("IPv4 next hop = %v ok=%v, want 172.30.0.3 on eth0", got, ok)
	}
}

func v6P2PGraph(t *testing.T, self, neighborID types.RouterID, area types.AreaID) (*ospfspf.Graph, packet.RouterLink) {
	t.Helper()
	rlsa := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age: 1, Type: ospfv3types.LSTypeRouter,
			AdvertisingRouter: ospfv3types.RouterID(self), Sequence: ospfv3types.InitialSequenceNumber,
		},
		Router: &ospfv3packet.RouterLSA{
			Options: ospfv3types.OptV6 | ospfv3types.OptR,
			Links: []ospfv3packet.RouterLink{{
				Type: ospfv3packet.RouterLinkTypeP2P, Metric: 10,
				InterfaceID: 7, NeighborInterfaceID: 11, NeighborRouterID: ospfv3types.RouterID(neighborID),
			}},
		},
	}
	raw := make([]byte, (&rlsa).EncodedLen())
	(&rlsa).WriteTo(raw, 0)
	hdr := v6LSAHeaderToNeutral(rlsa.Header)
	g := v6Strategy{}.BuildGraph(fakeV6Source{
		headers: []packet.LSAHeader{hdr},
		lsas:    map[types.LSAKey]packet.LSA{hdr.Key(): {Header: hdr, RawBytes: raw}},
	}, area)
	rv := g.Routers[self]
	if rv == nil || len(rv.Links) != 1 {
		t.Fatalf("p2p link did not translate into the SPF graph: %+v", rv)
	}
	return g, rv.Links[0]
}

func fullV6Neighbor(t *testing.T, self, neighborID types.RouterID, area types.AreaID) *neighbor.Table {
	t.Helper()
	tbl := neighbor.NewTable(neighbor.NopMetrics())
	tbl.SetSender(nopNbrSender{})
	tbl.ConfigureInterface(neighbor.InterfaceConfig{
		Name: "eth0", AreaID: area, RouterID: self, NetworkType: types.NetworkPointToPoint,
		InterfaceID: 7, InterfaceMTU: 1500, DeadInterval: 40,
	})
	if reason := tbl.Hello(neighbor.HelloInput{
		InterfaceName: "eth0", AreaID: area, LocalRouterID: self, NeighborID: neighborID,
		Address: netip.MustParseAddr("fe80::2"), InterfaceID: 11, TwoWay: true, NetworkType: types.NetworkPointToPoint,
		DeadInterval: 40, InterfaceMTU: 1500, Now: time.Unix(1, 0),
	}); reason != "" {
		t.Fatalf("Hello: %s", reason)
	}
	for _, dd := range []packet.DBDesc{
		{InterfaceMTU: 1500, Options: types.OptionE, Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 7},
		{InterfaceMTU: 1500, Options: types.OptionE, Flags: packet.DDFlagMaster, DDSequence: 8},
	} {
		if reason := tbl.HandleDBDesc("eth0", neighborID, dd); reason != "" {
			t.Fatalf("HandleDBDesc: %s", reason)
		}
	}
	return tbl
}

// ipv4AFLinkLSA builds a neighbor's IPv4-AF Link-LSA: its IPv4 address in the
// leading 32 bits of the link local address field (RFC 5838 Section 2.5).
func ipv4AFLinkLSA(t *testing.T, adv types.RouterID, ifaceID uint32, ipv4 [4]byte) packet.LSA {
	t.Helper()
	var ll [16]byte
	copy(ll[:], ipv4[:])
	body := ospfv3packet.LinkLSA{RtrPriority: 1, Options: ospfv3types.OptV6 | ospfv3types.OptR, LinkLocalAddr: ll}
	lsa := ospfv3packet.LSA{
		Header: v6OriginHeader(ospfv3types.LSTypeLink, ospfv3types.LinkStateID(v6SummaryLSID(ifaceID)), adv, types.InitialSequenceNumber, false),
		Link:   &body,
	}
	raw := make([]byte, lsa.EncodedLen())
	lsa.WriteTo(raw, 0)
	decoded, err := ospfv3packet.DecodeLSA(raw)
	if err != nil {
		t.Fatalf("DecodeLSA(link): %v", err)
	}
	return packet.LSA{Header: v6LSAHeaderToNeutral(decoded.Header), Body: decoded.Body, RawBytes: raw}
}
