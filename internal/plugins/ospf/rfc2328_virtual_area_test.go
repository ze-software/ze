// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- receive-side Area ID check.
// Related: instance.go -- acceptsArea, the dispatcher's RFC 2328 Section 8.2 Area ID gate.
// Related: virtual_link.go -- virtualLinkTargetLocked, the case (2) virtual-link match.
//
// VALIDATES: RFC 2328 Section 8.2 case (2): an Area ID that does not match the receiving
// interface is accepted only when it indicates the backbone, the source Router ID is the other
// end of a configured virtual link, and the receiving interface attaches to that link's
// Transit area.
// PREVENTS: a backbone packet from any router, or on an interface outside the Transit area, or
// a non-backbone mismatched Area ID, passing the header check.
package ospf

import (
	"net/netip"
	"testing"

	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc2328VirtualAreaEngine builds an area border router with a transit interface eth0 in area
// 0.0.0.1, a backbone interface eth1, an interface eth2 in area 0.0.0.2, and a virtual link to
// 10.0.0.2 through Transit area 0.0.0.1 that SPF has resolved as reachable.
func rfc2328VirtualAreaEngine(t *testing.T) (*engine, *fakeBackend) {
	t.Helper()
	const j = `{"ospf":{"router-id":"10.0.0.1",
		"areas":{"area":{
			"0.0.0.0":{"area-id":"0.0.0.0"},
			"0.0.0.1":{"area-id":"0.0.0.1","virtual-link":{"10.0.0.2":{}}},
			"0.0.0.2":{"area-id":"0.0.0.2"}}},
		"interfaces":{"interface":{
			"eth0":{"area":"0.0.0.1","network-type":"point-to-point"},
			"eth1":{"area":"0.0.0.0","network-type":"point-to-point"},
			"eth2":{"area":"0.0.0.2","network-type":"point-to-point"}}}}}`
	eng, fb := vlEngine(t, j)
	eng.onVirtualLinksResolved([]ospfspf.VirtualNeighborResult{{
		TransitArea: vlArea(t, "0.0.0.1"), Neighbor: vlRID(t, "10.0.0.2"), Reachable: true, Cost: 10,
		Address:  netip.MustParseAddr("192.0.2.2"),
		NextHops: []ospfspf.NextHop{{Addr: netip.MustParseAddr("192.0.2.2"), Interface: "eth0"}},
	}})
	return eng, fb
}

// TestRFC2328VirtualLinkAreaIDAccepted: a backbone-Area packet from the virtual-link neighbor,
// arriving on the Transit-area interface, passes the Section 8.2 Area ID check.
func TestRFC2328VirtualLinkAreaIDAccepted(t *testing.T) {
	eng, fb := rfc2328VirtualAreaEngine(t)
	defer eng.shutdown()
	packet := transport.RawPacket{IfIndex: vlIfindex(t, fb, "eth0"), Src: netip.MustParseAddr("192.0.2.2")}

	// RFC requirement: RFC2328-8.2-2 positive -- case (2): the header Area ID is the backbone
	// (not eth0's area 0.0.0.1), the source Router ID 10.0.0.2 is the other end of the configured
	// virtual link, and eth0 attaches to that link's Transit area 0.0.0.1, so acceptsArea
	// accepts the packet.
	if !eng.acceptsArea(packet, Header{RouterID: vlRID(t, "10.0.0.2"), AreaID: types.BackboneArea}) {
		t.Fatal("a backbone packet from the virtual-link neighbor on the Transit-area interface was discarded")
	}
}

// TestRFC2328VirtualLinkAreaIDRefused: each clause of case (2) failing on its own, with case (1)
// failing too, discards the packet.
func TestRFC2328VirtualLinkAreaIDRefused(t *testing.T) {
	eng, fb := rfc2328VirtualAreaEngine(t)
	defer eng.shutdown()
	transit := transport.RawPacket{IfIndex: vlIfindex(t, fb, "eth0"), Src: netip.MustParseAddr("192.0.2.2")}
	other := transport.RawPacket{IfIndex: vlIfindex(t, fb, "eth2"), Src: netip.MustParseAddr("192.0.2.2")}
	peer := vlRID(t, "10.0.0.2")

	// RFC requirement: RFC2328-8.2-2 negative -- case (2) fails and case (1) fails, so the
	// packet is discarded: a header Area ID 0.0.0.2 that neither matches eth0's area nor
	// indicates the backbone; a backbone packet whose source Router ID 9.9.9.9 is not the other
	// end of a configured virtual link; and a backbone packet from the virtual-link neighbor on
	// eth2, which does not attach to the link's Transit area 0.0.0.1.
	if eng.acceptsArea(transit, Header{RouterID: peer, AreaID: vlArea(t, "0.0.0.2")}) {
		t.Fatal("a mismatched non-backbone Area ID was accepted")
	}
	if eng.acceptsArea(transit, Header{RouterID: vlRID(t, "9.9.9.9"), AreaID: types.BackboneArea}) {
		t.Fatal("a backbone packet from a router that is not a virtual-link endpoint was accepted")
	}
	if eng.acceptsArea(other, Header{RouterID: peer, AreaID: types.BackboneArea}) {
		t.Fatal("a virtual-link backbone packet on an interface outside the Transit area was accepted")
	}
}
