package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

type ospfRawSend struct {
	iface string
	dst   netip.Addr
	raw   []byte
}

type ospfRawTx struct{ sends []ospfRawSend }

func (r *ospfRawTx) Send(iface string, dst netip.Addr, payload []byte) error {
	raw := make([]byte, len(payload))
	copy(raw, payload)
	r.sends = append(r.sends, ospfRawSend{iface: iface, dst: dst, raw: raw})
	return nil
}

func v6BroadcastInterface(area types.AreaID, self types.RouterID) ospflsdb.InterfaceInfo {
	return ospflsdb.InterfaceInfo{
		Name:          "eth0",
		AreaID:        area,
		NetworkType:   ospflsdb.NetworkBroadcast,
		State:         ospflsdb.InterfaceStateDR,
		Priority:      7,
		Cost:          10,
		RouterID:      self,
		InterfaceID:   10,
		Options:       types.OptionE,
		DR:            self,
		IsV6:          true,
		IPv6LinkLocal: netip.MustParseAddr("fe80::1"),
		IPv6Prefixes: []netip.Prefix{
			netip.MustParsePrefix("2001:db8:1::/64"),
			netip.MustParsePrefix("fe80::/64"),
		},
		Neighbors: []ospflsdb.NeighborInfo{{RouterID: types.RouterID{172, 30, 0, 3}, Address: netip.MustParseAddr("fe80::3"), State: ospflsdb.NeighborStateFull}},
	}
}

func TestOSPFv6OriginateLinkLSA(t *testing.T) {
	e := newV6OriginEngine()
	area := types.BackboneArea
	self := types.RouterID{172, 30, 0, 2}
	iface := v6BroadcastInterface(area, self)
	tx := &ospfRawTx{}
	e.lsdb.SetTx(tx.Send)
	e.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
		other := iface
		other.Name = "eth1"
		other.InterfaceID = 11
		return []ospflsdb.InterfaceInfo{iface, other}
	})

	key, changed := e.v6OriginateLinkLSA(self, iface)
	if !changed {
		t.Fatal("v6OriginateLinkLSA did not originate")
	}
	lsa, ok := e.lsdb.LookupLinkLSA("eth0", key)
	if !ok {
		t.Fatal("self Link-LSA not installed in eth0 link store")
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	link, err := decoded.DecodeLink()
	if err != nil {
		t.Fatalf("DecodeLink: %v", err)
	}
	if link.RtrPriority != 7 || netip.AddrFrom16(link.LinkLocalAddr) != netip.MustParseAddr("fe80::1") {
		t.Fatalf("link scalars = %+v", link)
	}
	if len(link.Prefixes) != 1 {
		t.Fatalf("link prefixes = %d, want 1 global prefix", len(link.Prefixes))
	}
	pfx, ok := v6PrefixToNetip(link.Prefixes[0], afIPv6Unicast)
	if !ok || pfx != netip.MustParsePrefix("2001:db8:1::/64") {
		t.Fatalf("link prefix = %v ok=%v", pfx, ok)
	}
	if len(tx.sends) != 1 || tx.sends[0].iface != "eth0" {
		t.Fatalf("Link-LSA flood sends = %+v, want one send on eth0", tx.sends)
	}
	if _, ok := e.lsdb.LookupLinkLSA("eth1", key); ok {
		t.Fatal("self Link-LSA leaked into eth1 link store")
	}
}

func TestOSPFv6DRAggregatesLinkPrefixes(t *testing.T) {
	e := newV6OriginEngine()
	area := types.BackboneArea
	self := types.RouterID{172, 30, 0, 2}
	iface := v6BroadcastInterface(area, self)
	e.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{iface}
	})
	if _, changed := e.v6OriginateLinkLSA(self, iface); !changed {
		t.Fatal("self Link-LSA not originated")
	}
	neighborPrefix, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:2::/64"), 0)
	if !ok {
		t.Fatal("neighbor prefix conversion failed")
	}
	neighbor := v6NeighborLinkLSA(t, types.RouterID{172, 30, 0, 3}, 22, netip.MustParseAddr("fe80::3"), []ospfv3packet.Prefix{neighborPrefix})
	reason := e.lsdb.ReceiveUpdate(ospflsdb.ReceiveInput{
		Interface: "eth0",
		AreaID:    area,
		RouterID:  neighbor.Header.AdvertisingRouter,
		Src:       netip.MustParseAddr("fe80::3"),
		Update:    packet.LSUpdate{LSAs: []packet.LSA{neighbor}},
	})
	if reason != "" {
		t.Fatalf("neighbor Link-LSA receive failed: %s", reason)
	}
	if _, ok := e.lsdb.LookupLinkLSA("eth0", neighbor.Header.Key()); !ok {
		t.Fatal("neighbor Link-LSA was not installed through ReceiveUpdate")
	}

	key, changed := e.v6OriginateNetworkIntraAreaPrefix(area, self, iface)
	if !changed {
		t.Fatal("DR aggregation did not originate an Intra-Area-Prefix-LSA")
	}
	lsa, ok := e.lsdb.LookupLSA(area, key)
	if !ok {
		t.Fatal("Network-referencing Intra-Area-Prefix-LSA not installed")
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	iap, err := decoded.DecodeIntraAreaPrefix()
	if err != nil {
		t.Fatalf("DecodeIntraAreaPrefix: %v", err)
	}
	if iap.ReferencedLSType != ospfv3types.LSTypeNetwork || types.LinkStateID(iap.ReferencedLinkStateID) != v6SummaryLSID(iface.InterfaceID) {
		t.Fatalf("reference = type %#x lsid %v, want Network/%v", uint16(iap.ReferencedLSType), iap.ReferencedLinkStateID, v6SummaryLSID(iface.InterfaceID))
	}
	got := map[netip.Prefix]bool{}
	for _, p := range iap.Prefixes {
		pfx, ok := v6PrefixToNetip(p, afIPv6Unicast)
		if ok {
			got[pfx] = true
		}
	}
	if !got[netip.MustParsePrefix("2001:db8:1::/64")] || !got[netip.MustParsePrefix("2001:db8:2::/64")] || len(got) != 2 {
		t.Fatalf("aggregated prefixes = %v", got)
	}
}

func TestOSPFv2NoLinkLSA(t *testing.T) {
	now := time.Unix(0, 0)
	db := ospflsdb.New(func() time.Time { return now })
	self := types.RouterID{1, 1, 1, 1}
	db.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{{
			Name:        "eth0",
			AreaID:      types.BackboneArea,
			NetworkType: ospflsdb.NetworkPointToPoint,
			State:       "point-to-point",
			RouterID:    self,
			Address:     netip.MustParseAddr("192.0.2.1").As4(),
			NetworkMask: netip.MustParseAddr("255.255.255.0").As4(),
		}}
	})
	db.OriginateFromTopology(self, false)
	if links := db.Snapshot().Links; len(links) != 0 {
		t.Fatalf("OSPFv2 originated link-scope LSAs: %+v", links)
	}
}

func v6NeighborLinkLSA(t *testing.T, adv types.RouterID, ifid uint32, ll netip.Addr, prefixes []ospfv3packet.Prefix) packet.LSA {
	t.Helper()
	body := ospfv3packet.LinkLSA{RtrPriority: 1, Options: ospfv3types.OptV6 | ospfv3types.OptR, LinkLocalAddr: ll.As16(), Prefixes: prefixes}
	lsa := ospfv3packet.LSA{
		Header: v6OriginHeader(ospfv3types.LSTypeLink, ospfv3types.LinkStateID(v6SummaryLSID(ifid)), adv, types.InitialSequenceNumber, false),
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
