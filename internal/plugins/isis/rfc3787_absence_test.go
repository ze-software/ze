// Design: docs/architecture/isis/isis-9-spf-rib.md -- next-hop capability and terminal rejection.
// Related: rfc1195_neighbor_addresses_test.go -- the on-link backend and peer IIH helpers.
//
// VALIDATES: how Ze handles a neighbor's Hello that lacks what RFC 3787 sections
// 9 and 10 oblige an IP router to generate, on the engine path the Hello takes:
// a dispatched P2P IIH -> the live adjacency table -> engineNextHopResolver.
// A Hello with no Protocols Supported TLV (129), or one that omits the IP NLPID,
// comes from a router Ze must treat as not IP-capable, so IPv4 resolves to a
// terminal rejection (RFC 1195 section 4.5 discard). A Hello with no IP Interface
// Address TLV (132), or one with zero entries, leaves Ze without an address to
// forward to, so no IPv4 next hop is resolved at all.
// PREVENTS: an IPv4 next hop fabricated for a neighbor that never advertised IP,
// and a next hop invented for a neighbor that advertised no interface address.
package isis

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// absencePeerIIH encodes the neighbor's P2P IIH carrying exactly tlvs after the
// Area Addresses TLV.
func absencePeerIIH(t *testing.T, tlvs ...packet.TLV) []byte {
	t.Helper()
	h := packet.P2PHello{
		CircuitType:    packet.CircuitL1,
		SystemID:       onLinkPeer,
		HoldingTime:    30,
		LocalCircuitID: 7,
		TLVs:           append([]packet.TLV{{Type: packet.TLVAreaAddresses, Value: []byte{3, 0x49, 0, 1}}}, tlvs...),
	}
	buf := make([]byte, h.EncodedLen())
	return buf[:h.WriteTo(buf, 0)]
}

// absenceNeighborUp dispatches pdu on isis-on-a and requires the neighbor's
// adjacency to be Up: the absence handled below is about forwarding, not about
// adjacency formation, which RFC 1195 leaves independent of IP addressing.
func absenceNeighborUp(t *testing.T, pdu []byte) *engine {
	t.Helper()
	useOnLinkBackend(t)
	eng, _ := protocolEngine(t, onLinkPeerConfig)
	c := protocolLiveCircuit(t, eng, "isis-on-a")
	eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: pdu})
	rows := c.Table().Snapshot()
	if len(rows) != 1 || rows[0].State != "up" {
		t.Fatalf("neighbor did not form an Up adjacency: %+v", rows)
	}
	return eng
}

var absencePeerAddr = netip.MustParseAddr("192.0.2.2")

// RFC requirement: RFC3787-x-2 negative -- a neighbor's Hello that carries no
// Protocols Supported TLV (129), or a TLV 129 without the IP NLPID (0xCC),
// marks that neighbor not IP-capable: its Up adjacency resolves IPv4 to a
// terminal rejection (Unsupported, no address) even though its TLV 132 lists
// 192.0.2.2.
func TestRFC3787PeerWithoutIPProtocolsSupportedIsRejected(t *testing.T) {
	a4 := absencePeerAddr.As4()
	address := packet.TLV{Type: packet.TLVIPInterfaceAddress, Value: a4[:]}
	cases := []struct {
		name string
		pdu  func(t *testing.T) []byte
	}{
		{"no TLV 129", func(t *testing.T) []byte { return absencePeerIIH(t, address) }},
		{"TLV 129 CLNP only", func(t *testing.T) []byte {
			return absencePeerIIH(t, packet.TLV{Type: packet.TLVProtocolsSupported, Value: []byte{packet.NLPIDCLNP}}, address)
		}},
		{"TLV 129 IPv6 only", func(t *testing.T) []byte {
			return absencePeerIIH(t, packet.TLV{Type: packet.TLVProtocolsSupported, Value: []byte{packet.NLPIDIPv6}}, address)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eng := absenceNeighborUp(t, tc.pdu(t))
			nh, ok := (*engineNextHopResolver)(eng).ResolveNextHop(spf.Level1, onLinkPeer)
			if !ok {
				t.Fatal("no answer for an Up neighbor that is not IP-capable, want a terminal rejection")
			}
			if !nh.Unsupported {
				t.Fatalf("next hop %+v, want Unsupported (neighbor never advertised IP)", nh)
			}
			if nh.Addr.IsValid() {
				t.Fatalf("next hop address %v resolved for a neighbor that is not IP-capable", nh.Addr)
			}
		})
	}
}

// RFC requirement: RFC3787-10-1 negative -- a neighbor that advertises IP in
// TLV 129 but whose Hello carries no IP Interface Address TLV (132), or a TLV
// 132 with zero entries, forms an Up adjacency yet yields no IPv4 next hop: the
// resolver answers not-found rather than inventing an address or a rejection.
func TestRFC3787PeerWithoutIPInterfaceAddressHasNoNextHop(t *testing.T) {
	protocols := packet.TLV{Type: packet.TLVProtocolsSupported, Value: []byte{packet.NLPIDIPv4}}
	cases := []struct {
		name string
		pdu  func(t *testing.T) []byte
	}{
		{"no TLV 132", func(t *testing.T) []byte { return absencePeerIIH(t, protocols) }},
		{"TLV 132 with zero entries", func(t *testing.T) []byte {
			return absencePeerIIH(t, protocols, packet.TLV{Type: packet.TLVIPInterfaceAddress, Value: []byte{}})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eng := absenceNeighborUp(t, tc.pdu(t))
			nh, ok := (*engineNextHopResolver)(eng).ResolveNextHop(spf.Level1, onLinkPeer)
			if ok {
				t.Fatalf("next hop %+v resolved for a neighbor that advertised no interface address", nh)
			}
		})
	}
}
