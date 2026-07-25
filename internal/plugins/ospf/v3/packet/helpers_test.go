// Design: plan/learned/969-ospfv3-2-wire.md -- shared OSPFv3 codec test fixtures.

package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func mustRouterID(t *testing.T, s string) types.RouterID {
	t.Helper()
	id, err := types.ParseRouterID(s)
	if err != nil {
		t.Fatalf("ParseRouterID(%q): %v", s, err)
	}
	return id
}

func mustAreaID(t *testing.T, s string) types.AreaID {
	t.Helper()
	id, err := types.ParseAreaID(s)
	if err != nil {
		t.Fatalf("ParseAreaID(%q): %v", s, err)
	}
	return id
}

func mustLSID(t *testing.T, s string) types.LinkStateID {
	t.Helper()
	id, err := types.ParseLinkStateID(s)
	if err != nil {
		t.Fatalf("ParseLinkStateID(%q): %v", s, err)
	}
	return id
}

func mustOptions(t *testing.T, v uint32) types.Options {
	t.Helper()
	return types.Options(v)
}

func mustPrefixLen(t *testing.T, bits uint8) types.PrefixLength {
	t.Helper()
	p, err := types.NewPrefixLength(bits)
	if err != nil {
		t.Fatalf("NewPrefixLength(%d): %v", bits, err)
	}
	return p
}

func sampleHeader(t *testing.T, pt PacketType) Header {
	t.Helper()
	return Header{
		Type:       pt,
		RouterID:   mustRouterID(t, "10.0.0.1"),
		AreaID:     mustAreaID(t, "0"),
		InstanceID: types.InstanceID(0),
	}
}

func sampleLSAHeader(t *testing.T, typ types.LSType, lsid string) LSAHeader {
	t.Helper()
	return LSAHeader{
		Age:               types.LSAge(10),
		Type:              typ,
		LinkStateID:       mustLSID(t, lsid),
		AdvertisingRouter: mustRouterID(t, "10.0.0.1"),
		Sequence:          types.InitialSequenceNumber,
	}
}

func encodePacket(t *testing.T, p Packet) []byte {
	t.Helper()
	buf := make([]byte, p.EncodedLen())
	n := (&p).WriteTo(buf, 0)
	if n != len(buf) {
		t.Fatalf("Packet.WriteTo wrote %d, want %d", n, len(buf))
	}
	return buf
}

func encodeLSA(t *testing.T, lsa LSA) []byte {
	t.Helper()
	buf := make([]byte, lsa.EncodedLen())
	n := (&lsa).WriteTo(buf, 0)
	if n != len(buf) {
		t.Fatalf("LSA.WriteTo wrote %d, want %d", n, len(buf))
	}
	return buf
}

// makePrefix builds a Prefix at the given length from a sample /64 address,
// padded to ByteLen with zero bits past the prefix length.
func makePrefix(t *testing.T, bits uint8, opts types.PrefixOptions, field16 uint16) Prefix {
	t.Helper()
	plen := mustPrefixLen(t, bits)
	addr := make([]byte, plen.ByteLen())
	// Fill the prefix bits from a deterministic 2001:db8:: pattern, then zero the
	// trailing bits past the prefix length so ValidatePadding accepts it.
	pattern := []byte{0x20, 0x01, 0x0d, 0xb8, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x11, 0x22, 0x33, 0x44}
	for i := 0; i < len(addr) && i < len(pattern); i++ {
		addr[i] = pattern[i]
	}
	zeroPadPast(addr, int(bits))
	return Prefix{Length: plen, Options: opts, Field16: field16, Address: addr}
}

// zeroPadPast zeroes every bit at index >= bits in b.
func zeroPadPast(b []byte, bits int) {
	for i := bits; i < len(b)*8; i++ {
		b[i/8] &^= 0x80 >> (uint(i) % 8)
	}
}

// sampleRouterLSA builds a Router-LSA with two link records for tests that need a
// concrete, encodable LSA (LSUpdate, LSAck, raw-bytes round-trip).
func sampleRouterLSA(t *testing.T) LSA {
	t.Helper()
	return LSA{
		Header: sampleLSAHeader(t, types.LSTypeRouter, "0.0.0.1"),
		Router: &RouterLSA{
			Flags:   RouterFlagB | RouterFlagE,
			Options: mustOptions(t, uint32(types.OptV6|types.OptR)),
			Links: []RouterLink{
				{Type: RouterLinkTypeP2P, Metric: 10, InterfaceID: types.InterfaceID(1), NeighborInterfaceID: types.InterfaceID(2), NeighborRouterID: mustRouterID(t, "10.0.0.2")},
				{Type: RouterLinkTypeTransit, Metric: 65535, InterfaceID: types.InterfaceID(3), NeighborInterfaceID: types.InterfaceID(4), NeighborRouterID: mustRouterID(t, "10.0.0.3")},
			},
		},
	}
}
