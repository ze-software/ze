// Design: docs/architecture/isis/isis-5-adjacency.md -- received protocol capabilities.
// Goal: follow wire Hello capabilities through the live table into route resolution.
// Method: dispatch peer IIHs, retaining ISO adjacency while changing the supported
// network protocols; inspect the next-hop contract consumed by the SPF installer.

package isis

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

func protocolPeerIIH(t *testing.T, nlpids []byte) []byte {
	t.Helper()
	v6 := netip.MustParseAddr("fe80::2").As16()
	h := packet.P2PHello{
		CircuitType:    packet.CircuitL1,
		SystemID:       types.SystemID{0, 0, 0, 0, 0, 2},
		HoldingTime:    30,
		LocalCircuitID: 7,
		TLVs: []packet.TLV{
			{Type: packet.TLVAreaAddresses, Value: []byte{3, 0x49, 0, 1}},
			{Type: packet.TLVIPInterfaceAddress, Value: []byte{198, 51, 100, 2}},
			{Type: packet.TLVIPv6InterfaceAddress, Value: v6[:]},
		},
	}
	if nlpids != nil {
		h.TLVs = append(h.TLVs, packet.TLV{Type: packet.TLVProtocolsSupported, Value: nlpids})
	}
	buf := make([]byte, h.EncodedLen())
	return buf[:h.WriteTo(buf, 0)]
}

// RFC requirement: RFC1195-4.5-1 positive -- a dispatched OSI-only Hello keeps
// its ISO adjacency but resolves to an explicit unsupported IPv4 next hop rather
// than a usable IP address. The route-installation consequence is covered separately.
func TestRFC1195OSINeighborIsTerminalIPv4NextHop(t *testing.T) {
	eng, _ := protocolEngine(t, protocolP2PConfig)
	c := protocolLiveCircuit(t, eng, "eth0")
	eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: protocolPeerIIH(t, nil)})
	nh, ok := (*engineNextHopResolver)(eng).ResolveNextHop(spf.Level1, types.SystemID{0, 0, 0, 0, 0, 2})
	if !ok || !nh.Unsupported || nh.Addr.IsValid() {
		t.Fatalf("OSI-only adjacency was not terminal: next-hop=%+v present=%v", nh, ok)
	}
	rows := c.Table().Snapshot()
	if len(rows) != 1 || rows[0].State != "up" {
		t.Fatalf("IP capability changed ISO topology: %+v", rows)
	}
}

// RFC requirement: RFC1195-4.5-1 negative -- capable-to-OSI-only-to-capable
// changes on the same live adjacency cannot preserve a stale forwarding next hop
// or a stale rejection; both IPv4 and IPv6 resolution follow the received NLPIDs.
func TestRFC1195NeighborProtocolTransitions(t *testing.T) {
	eng, _ := protocolEngine(t, protocolP2PConfig)
	c := protocolLiveCircuit(t, eng, "eth0")
	peer := types.SystemID{0, 0, 0, 0, 0, 2}
	for _, step := range []struct {
		name        string
		nlpids      []byte
		unsupported bool
	}{
		{"capable", []byte{packet.NLPIDIPv4, packet.NLPIDIPv6}, false},
		{"OSI-only", nil, true},
		{"capable-again", []byte{packet.NLPIDIPv4, packet.NLPIDIPv6}, false},
	} {
		eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: protocolPeerIIH(t, step.nlpids)})
		v4, ok4 := (*engineNextHopResolver)(eng).ResolveNextHop(spf.Level1, peer)
		v6, ok6 := (*engineNextHopResolverV6)(eng).ResolveNextHopV6(spf.Level1, peer)
		if !ok4 || !ok6 {
			t.Fatalf("%s lost explicit route outcome: IPv4=%v IPv6=%v", step.name, ok4, ok6)
		}
		if v4.Unsupported != step.unsupported || v6.Unsupported != step.unsupported {
			t.Fatalf("%s retained old protocol state: IPv4=%+v IPv6=%+v", step.name, v4, v6)
		}
		if !step.unsupported {
			if v4.Addr != netip.MustParseAddr("198.51.100.2") || v6.Addr != netip.MustParseAddr("fe80::2") {
				t.Fatalf("%s lost supported addresses: IPv4=%+v IPv6=%+v", step.name, v4, v6)
			}
			if v4.Interface != "eth0" || !v4.OnLink || v6.Interface != "eth0" {
				t.Fatalf("%s lost physical adjacency egress: IPv4=%+v IPv6=%+v", step.name, v4, v6)
			}
		}
	}
}
