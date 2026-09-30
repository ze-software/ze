// Design: docs/architecture/isis/isis-5-adjacency.md -- the learned IPv4 next hop.
//
// VALIDATES: RFC 1195 sections 4.2 and 4.4 on the engine path a neighbor's Hello
// takes to the SPF next hop: a dispatched IIH -> the live adjacency table ->
// engineNextHopResolver. A neighbor that assigns many IPv4 addresses to one
// interface still forms an adjacency and yields a next hop taken from its own
// list, and a neighbor whose address is outside every subnet of the interface the
// adjacency runs on is forwarded to over that adjacency, marked on-link.
// PREVENTS: a next hop kept from an earlier address list, and a next hop chosen
// by subnet match (another interface whose subnet covers the neighbor's address)
// instead of by the adjacency.
package isis

import (
	"fmt"
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// onLinkBackend substitutes only the OS address source: isis-on-a holds
// 192.0.2.1/24 and isis-on-b holds 198.51.100.1/24.
type onLinkBackend struct{ iface.Backend }

func (*onLinkBackend) GetInterface(name string) (*iface.InterfaceInfo, error) {
	var addresses []iface.AddrInfo
	switch name {
	case "isis-on-a":
		addresses = []iface.AddrInfo{{Address: "192.0.2.1", PrefixLength: 24, Family: "ipv4"}}
	case "isis-on-b":
		addresses = []iface.AddrInfo{{Address: "198.51.100.1", PrefixLength: 24, Family: "ipv4"}}
	default:
		return nil, fmt.Errorf("unknown test interface %s", name)
	}
	return &iface.InterfaceInfo{Name: name, OsName: name, State: "up", MTU: 1500, Addresses: addresses}, nil
}

func (*onLinkBackend) Close() error { return nil }

var registerOnLinkBackend = sync.OnceValue(func() error {
	return iface.RegisterBackend("isis-on-link-test", func() (iface.Backend, error) {
		return &onLinkBackend{}, nil
	})
})

// useOnLinkBackend makes the test backend the active iface backend for the life
// of t and restores the previous one after.
func useOnLinkBackend(t *testing.T) {
	t.Helper()
	if err := registerOnLinkBackend(); err != nil {
		t.Fatal(err)
	}
	previous := iface.ActiveBackendName()
	if err := iface.LoadBackend("isis-on-link-test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := iface.CloseBackend(); err != nil {
			t.Error(err)
		}
		if previous != "" {
			if err := iface.LoadBackend(previous); err != nil {
				t.Error(err)
			}
		}
	})
}

const onLinkPeerConfig = `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{
"isis-on-a":{"level":"l1","hello-interval":"3600","circuit-type":"point-to-point"},
"isis-on-b":{"level":"l1","hello-interval":"3600","circuit-type":"point-to-point"}}}}}`

var onLinkPeer = types.SystemID{0, 0, 0, 0, 0, 2}

// onLinkPeerIIH encodes the IP-capable neighbor's P2P IIH whose TLV 132 lists
// addresses in order.
func onLinkPeerIIH(t *testing.T, addresses []netip.Addr) []byte {
	t.Helper()
	value := make([]byte, 0, 4*len(addresses))
	for _, a := range addresses {
		a4 := a.As4()
		value = append(value, a4[:]...)
	}
	h := packet.P2PHello{
		CircuitType:    packet.CircuitL1,
		SystemID:       onLinkPeer,
		HoldingTime:    30,
		LocalCircuitID: 7,
		TLVs: []packet.TLV{
			{Type: packet.TLVAreaAddresses, Value: []byte{3, 0x49, 0, 1}},
			{Type: packet.TLVProtocolsSupported, Value: []byte{packet.NLPIDIPv4}},
			{Type: packet.TLVIPInterfaceAddress, Value: value},
		},
	}
	buf := make([]byte, h.EncodedLen())
	return buf[:h.WriteTo(buf, 0)]
}

// sixtyThreeAddresses returns the TLV 132 maximum of 63 addresses, 203.0.<third>.1
// through 203.0.<third>.63.
func sixtyThreeAddresses(third byte) []netip.Addr {
	out := make([]netip.Addr, 0, 63)
	for i := range 63 {
		out = append(out, netip.AddrFrom4([4]byte{203, 0, third, byte(i + 1)}))
	}
	return out
}

func onLinkNextHop(t *testing.T, eng *engine) spf.NextHop {
	t.Helper()
	nh, ok := (*engineNextHopResolver)(eng).ResolveNextHop(spf.Level1, onLinkPeer)
	if !ok {
		t.Fatal("no IPv4 next hop for the Up neighbor")
	}
	return nh
}

// RFC requirement: RFC1195-4.2-1 positive -- a neighbor whose P2P IIH lists 63
// IPv4 addresses on one interface (the TLV 132 maximum) forms an Up adjacency,
// and the engine resolves it to an on-link IPv4 next hop taken from that list
// (its first address) on the interface the Hello arrived on.
func TestRFC1195MultiAddressNeighborForwarding(t *testing.T) {
	useOnLinkBackend(t)
	eng, _ := protocolEngine(t, onLinkPeerConfig)
	c := protocolLiveCircuit(t, eng, "isis-on-a")
	addresses := sixtyThreeAddresses(113)
	eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: onLinkPeerIIH(t, addresses)})

	rows := c.Table().Snapshot()
	if len(rows) != 1 || rows[0].State != "up" {
		t.Fatalf("63-address neighbor did not form an Up adjacency: %+v", rows)
	}
	nh := onLinkNextHop(t, eng)
	if nh.Addr != addresses[0] || nh.Interface != "isis-on-a" || !nh.OnLink {
		t.Fatalf("next hop %+v, want %v on-link via isis-on-a", nh, addresses[0])
	}
}

// RFC requirement: RFC1195-4.2-1 negative -- when the same neighbor re-sends its
// IIH with a different 63-address list, the next hop follows the new list: no
// address of the earlier list is kept as the next hop.
func TestRFC1195MultiAddressNeighborRenumbered(t *testing.T) {
	useOnLinkBackend(t)
	eng, _ := protocolEngine(t, onLinkPeerConfig)
	c := protocolLiveCircuit(t, eng, "isis-on-a")
	before := sixtyThreeAddresses(113)
	after := sixtyThreeAddresses(114)
	eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: onLinkPeerIIH(t, before)})
	eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: onLinkPeerIIH(t, after)})

	nh := onLinkNextHop(t, eng)
	for _, old := range before {
		if nh.Addr == old {
			t.Fatalf("next hop %v kept from the neighbor's earlier address list", nh.Addr)
		}
	}
	if nh.Addr != after[0] {
		t.Fatalf("next hop %v, want %v from the current list", nh.Addr, after[0])
	}
}

// RFC requirement: RFC1195-4.4-1 positive -- the adjacency runs on isis-on-a
// (192.0.2.1/24) and the neighbor's only address, 198.51.100.2, is on a
// different logical subnet: the engine still resolves it as the next hop via
// isis-on-a, marked OnLink so it is forwarded over the adjacency rather than
// looked up recursively.
func TestRFC1195OffSubnetNeighborIsOnLinkNextHop(t *testing.T) {
	useOnLinkBackend(t)
	eng, _ := protocolEngine(t, onLinkPeerConfig)
	c := protocolLiveCircuit(t, eng, "isis-on-a")
	peer := netip.MustParseAddr("198.51.100.2")
	local := interfaceIPv4(InterfaceConfig{Name: "isis-on-a"})
	if local != netip.MustParseAddr("192.0.2.1") || netip.PrefixFrom(local, 24).Contains(peer) {
		t.Fatalf("setup: isis-on-a address %v must be valid and off the neighbor's subnet", local)
	}
	eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: onLinkPeerIIH(t, []netip.Addr{peer})})

	nh := onLinkNextHop(t, eng)
	if nh.Addr != peer || nh.Interface != "isis-on-a" || !nh.OnLink {
		t.Fatalf("next hop %+v, want %v on-link via isis-on-a", nh, peer)
	}
}

// RFC requirement: RFC1195-4.4-1 negative -- isis-on-b holds 198.51.100.1/24,
// the subnet that covers the neighbor's address, but carries no adjacency to
// it: the next hop is never placed on isis-on-b by subnet match, it stays on
// isis-on-a where the adjacency is.
func TestRFC1195OffSubnetNeighborNotResolvedBySubnet(t *testing.T) {
	useOnLinkBackend(t)
	eng, _ := protocolEngine(t, onLinkPeerConfig)
	a := protocolLiveCircuit(t, eng, "isis-on-a")
	peer := netip.MustParseAddr("198.51.100.2")
	covering := interfaceIPv4(InterfaceConfig{Name: "isis-on-b"})
	if !netip.PrefixFrom(covering, 24).Contains(peer) {
		t.Fatalf("setup: isis-on-b address %v must cover %v", covering, peer)
	}
	eng.dispatch.dispatch(transport.RawFrame{IfIndex: a.IfIndex(), PDU: onLinkPeerIIH(t, []netip.Addr{peer})})

	nh := onLinkNextHop(t, eng)
	if nh.Interface == "isis-on-b" {
		t.Fatalf("next hop %+v placed on the subnet-matching interface without an adjacency", nh)
	}
	if nh.Interface != "isis-on-a" || !nh.OnLink {
		t.Fatalf("next hop %+v, want on-link via isis-on-a", nh)
	}
}
