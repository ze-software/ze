// Design: plan/spec-isis-12-ipv6.md -- IIH TLV 232 (IPv6 link-local) origination.
//
// VALIDATES: a dual-stack circuit advertises NLPID 0x8E in TLV 129 and carries
// the IPv6 LINK-LOCAL address in the IIH TLV 232 (RFC 5308 sec 3: a Hello carries
// ONLY link-local addresses, AC-4); an IPv4-only circuit emits neither; a circuit
// with IPv6 enabled but no link-local address omits TLV 232.

package circuit

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
)

// dualStackLAN returns a LAN circuit with IPv6 advertised and a link-local addr.
func dualStackLAN(t *testing.T, s Sender) *Circuit {
	t.Helper()
	c := lanCircuit(t, s)
	c.advertiseIPv6 = true
	c.ipv6LinkLocal = netip.MustParseAddr("fe80::1")
	return c
}

// TestISISIIHTLV232LinkLocal: a dual-stack IIH advertises NLPID 0x8E and carries
// ONLY the link-local address in TLV 232 (RFC 5308 sec 3).
//
// RFC requirement: RFC5308-3-1 positive -- the dual-stack Hello TLV 232 carries exactly one address and it is link-local.
// RFC requirement: RFC5308-4-1 positive -- the dual-stack Hello lists NLPID 0x8E in the Protocols Supported TLV (129).
func TestISISIIHTLV232LinkLocal(t *testing.T) {
	s := &fakeSender{mtu: 1500}
	c := dualStackLAN(t, s)
	if err := c.SendHello(); err != nil {
		t.Fatal(err)
	}
	p := decodeSent(t, s)
	if p.LANHello == nil {
		t.Fatalf("expected a LAN IIH, got %+v", p)
	}

	// TLV 129 lists both NLPIDs.
	var nlpids []uint8
	for _, tl := range p.LANHello.TLVs {
		if tl.Type == packet.TLVProtocolsSupported {
			nlpids = packet.DecodeProtocolsSupportedTLV(tl.Value).NLPIDs
		}
	}
	if len(nlpids) != 2 || nlpids[0] != packet.NLPIDIPv4 || nlpids[1] != packet.NLPIDIPv6 {
		t.Errorf("TLV 129 NLPIDs = %v, want [0xCC 0x8E]", nlpids)
	}

	// TLV 232 carries ONLY the link-local address (RFC 5308 sec 3).
	if !hasTLV(p.LANHello.TLVs, packet.TLVIPv6InterfaceAddress) {
		t.Fatal("dual-stack IIH missing TLV 232 (IPv6 link-local)")
	}
	for _, tl := range p.LANHello.TLVs {
		if tl.Type != packet.TLVIPv6InterfaceAddress {
			continue
		}
		dec, err := packet.DecodeIPv6InterfaceAddrTLV(tl.Value)
		if err != nil {
			t.Fatalf("decode TLV 232: %v", err)
		}
		if len(dec.Addresses) != 1 {
			t.Fatalf("TLV 232 has %d addresses, want 1", len(dec.Addresses))
		}
		if !dec.Addresses[0].IsLinkLocalUnicast() {
			t.Errorf("RFC 5308 sec 3 violation: IIH TLV 232 address %v is not link-local", dec.Addresses[0])
		}
	}
}

// TestISISIIHNoTLV232WhenIPv4Only: an IPv4-only circuit emits neither NLPID 0x8E
// nor TLV 232 (AC-7: IPv6 disabled originates nothing IPv6).
//
// RFC requirement: RFC5308-4-1 negative -- an IPv4-only Hello omits NLPID 0x8E from the Protocols Supported TLV (129).
func TestISISIIHNoTLV232WhenIPv4Only(t *testing.T) {
	s := &fakeSender{mtu: 1500}
	c := lanCircuit(t, s) // IPv6 not advertised
	if err := c.SendHello(); err != nil {
		t.Fatal(err)
	}
	p := decodeSent(t, s)
	if hasTLV(p.LANHello.TLVs, packet.TLVIPv6InterfaceAddress) {
		t.Error("IPv4-only IIH must not carry TLV 232")
	}
	for _, tl := range p.LANHello.TLVs {
		if tl.Type == packet.TLVProtocolsSupported {
			for _, n := range packet.DecodeProtocolsSupportedTLV(tl.Value).NLPIDs {
				if n == packet.NLPIDIPv6 {
					t.Error("IPv4-only IIH must not advertise NLPID 0x8E")
				}
			}
		}
	}
}

// TestISISIIHTLV232OmittedNoLinkLocal: IPv6 enabled but no link-local address ->
// TLV 232 omitted (the codec never emits an empty/invalid address).
//
// RFC requirement: RFC5308-3-1 negative -- with IPv6 enabled but no link-local address the Hello omits TLV 232 (never emits a non-link-local or empty address).
func TestISISIIHTLV232OmittedNoLinkLocal(t *testing.T) {
	s := &fakeSender{mtu: 1500}
	c := lanCircuit(t, s)
	c.advertiseIPv6 = true // NLPID 0x8E, but no link-local address set
	if err := c.SendHello(); err != nil {
		t.Fatal(err)
	}
	p := decodeSent(t, s)
	if hasTLV(p.LANHello.TLVs, packet.TLVIPv6InterfaceAddress) {
		t.Error("TLV 232 must be omitted when no link-local address is configured")
	}
}
