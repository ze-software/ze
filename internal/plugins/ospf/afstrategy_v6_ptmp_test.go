// VALIDATES: an OSPFv3 point-to-multipoint interface's address-free Type-1 Router-LSA
// link translates into the shared SPF graph exactly like point-to-point, and the v6
// next-hop resolves to the neighbor's link-local from the adjacency (RFC 5340 sec 3.8.1),
// reusing v6NextHop.P2PNextHop unchanged.
// PREVENTS: a broadcast-only assumption in v6RouterLinks/BuildGraph dropping a PtMP link,
// or the PtMP next-hop resolving to something other than the neighbor link-local.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

type nopNbrSender struct{}

func (nopNbrSender) SendPacket(string, netip.Addr, []byte) error { return nil }

func TestOSPFv3PtMPNextHop(t *testing.T) {
	self := types.RouterID{1, 1, 1, 1}
	neighborID := types.RouterID{2, 2, 2, 2}
	area := types.BackboneArea

	// A PtMP interface originates one address-free Type-1 link per Full neighbor -- the
	// same wire as point-to-point. Confirm BuildGraph translates it into a p2p edge keyed
	// by the neighbor Router ID (which the shared next-hop resolver consumes).
	rlsa := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age: 1, Type: ospfv3types.LSTypeRouter,
			LinkStateID: ospfv3types.LinkStateID{}, AdvertisingRouter: ospfv3types.RouterID(self),
			Sequence: ospfv3types.InitialSequenceNumber,
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
	src := fakeV6Source{
		headers: []packet.LSAHeader{hdr},
		lsas:    map[types.LSAKey]packet.LSA{hdr.Key(): {Header: hdr, RawBytes: raw}},
	}
	g := v6Strategy{}.BuildGraph(src, area)
	rv := g.Routers[self]
	if rv == nil || len(rv.Links) != 1 || rv.Links[0].Type != packet.RouterLinkTypeP2P {
		t.Fatalf("PtMP p2p link did not translate into the SPF graph: %+v", rv)
	}
	if rv.Links[0].LinkID != types.LinkStateID(neighborID) {
		t.Fatalf("graph link id = %v, want neighbor router id 2.2.2.2", rv.Links[0].LinkID)
	}

	// The next-hop resolves to the neighbor's link-local from the adjacency table.
	ll := netip.MustParseAddr("fe80::2")
	tbl := neighbor.NewTable(neighbor.NopMetrics())
	tbl.SetSender(nopNbrSender{})
	tbl.ConfigureInterface(neighbor.InterfaceConfig{
		Name: "eth0", AreaID: area, RouterID: self, NetworkType: neighbor.NetworkPointToMultipoint,
		InterfaceMTU: 1500, DeadInterval: 40,
	})
	now := time.Unix(1, 0)
	if reason := tbl.Hello(neighbor.HelloInput{
		InterfaceName: "eth0", AreaID: area, LocalRouterID: self, NeighborID: neighborID,
		Address: ll, TwoWay: true, NetworkType: neighbor.NetworkPointToMultipoint,
		DeadInterval: 40, InterfaceMTU: 1500, Now: now,
	}); reason != "" {
		t.Fatalf("Hello: %s", reason)
	}
	// Drive the DD exchange to Exchange so AddressOf exposes the neighbor's link-local.
	if reason := tbl.HandleDBDesc("eth0", neighborID, packet.DBDesc{
		InterfaceMTU: 1500, Options: types.OptionE,
		Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 7,
	}); reason != "" {
		t.Fatalf("HandleDBDesc: %s", reason)
	}
	nh := v6NextHop{neighbors: tbl}
	got, ok := nh.P2PNextHop(g, neighborID, self)
	if !ok || got != ll {
		t.Fatalf("v6 PtMP next-hop = %v ok=%v, want neighbor link-local %v", got, ok, ll)
	}
}
