// Design: docs/architecture/ospf/ospf-2-wire.md -- checksum covered-range tests

// VALIDATES: RFC 2328 Section A.3.1: the OSPF packet checksum is the standard IP checksum
// of the whole packet starting with the header, excluding the 64-bit Authentication field,
// judged against the test's own RFC 1071 arithmetic rather than ze's PacketChecksum.
// PREVENTS: a generator and verifier that share one wrong covered range and so accept
// each other; the existing units only round-trip through PacketChecksum.
package packet

import "testing"

// simpleAuthWire encodes a Hello with AuType 1 and the given 8-octet password.
func simpleAuthWire(t *testing.T, password string) []byte {
	t.Helper()
	h := sampleHeader(t, PacketTypeHello)
	h.AuType = AuTypeSimple
	copy(h.Auth[:], password)
	hello := sampleHello(t)
	return encodePacket(t, Packet{Header: h, Hello: &hello})
}

// checksumOverWholePacket is the wrong answer: the complement of the sum over every
// octet with the checksum field cleared, the Authentication field included.
func checksumOverWholePacket(wire []byte) uint16 {
	cleared := append([]byte(nil), wire...)
	cleared[offChecksum] = 0
	cleared[offChecksum+1] = 0
	return ^rfc1071ReferenceFold(cleared)
}

// RFC requirement: RFC2328-A.3.1-2 positive -- with a non-zero simple password in the Authentication field, the Checksum WriteTo places equals the test's independent RFC 1071 checksum over every octet from the OSPF header on except the 64-bit Authentication field, differs from the checksum over the whole packet, does not change when only the password changes, and does change when a header octet (Router ID) changes.
func TestRFC2328PacketChecksumCoversHeaderExcludesAuth(t *testing.T) {
	// Goal: the covered range of the generator, judged independently.
	// Method: encode, read the field back, compare with rfc1071ReferenceChecksum.
	wire := simpleAuthWire(t, "password")
	got := readUint16(wire, offChecksum)
	if want := rfc1071ReferenceChecksum(wire); got != want {
		t.Fatalf("Checksum = %#04x, want %#04x (header and body, Authentication excluded)", got, want)
	}
	if whole := checksumOverWholePacket(wire); whole == got {
		t.Fatalf("setup: the password does not move the whole-packet sum (%#04x), the vector cannot tell the ranges apart", whole)
	}
	other := simpleAuthWire(t, "PASSWORD")
	if g := readUint16(other, offChecksum); g != got {
		t.Fatalf("Checksum with another password = %#04x, want the same %#04x: the Authentication field is covered", g, got)
	}
	h := sampleHeader(t, PacketTypeHello)
	h.AuType = AuTypeSimple
	copy(h.Auth[:], "password")
	h.RouterID[3] ^= 0x01
	hello := sampleHello(t)
	moved := encodePacket(t, Packet{Header: h, Hello: &hello})
	if g := readUint16(moved, offChecksum); g == got {
		t.Fatalf("Checksum unchanged (%#04x) after a Router ID change: the header is not covered", g)
	}
	if g, want := readUint16(moved, offChecksum), rfc1071ReferenceChecksum(moved); g != want {
		t.Fatalf("Checksum after the Router ID change = %#04x, want %#04x", g, want)
	}
}

// RFC requirement: RFC2328-A.3.1-2 negative -- VerifyPacketChecksum refuses a packet whose Checksum was computed over the wrong range: over the whole packet with the Authentication field included, and over the header alone (body excluded), each computed by the test's own RFC 1071 arithmetic; the correctly covered checksum is accepted as the control.
func TestRFC2328PacketChecksumWrongRangeRefused(t *testing.T) {
	// Goal: the verifier's covered range, judged independently.
	// Method: overwrite the Checksum field with an independently computed wrong-range value.
	wire := simpleAuthWire(t, "password")
	if !VerifyPacketChecksum(wire) {
		t.Fatalf("control: the correctly covered packet was refused")
	}
	headerOnly := func(w []byte) uint16 {
		cleared := append([]byte(nil), w[:CommonHeaderLen]...)
		cleared[offChecksum] = 0
		cleared[offChecksum+1] = 0
		return ^rfc1071ReferenceFold(cleared[:offAuth])
	}
	cases := []struct {
		name  string
		field uint16
	}{
		{"authentication included", checksumOverWholePacket(wire)},
		{"body excluded", headerOnly(wire)},
	}
	correct := readUint16(wire, offChecksum)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.field == correct {
				t.Fatalf("setup: the wrong-range checksum equals the correct %#04x", correct)
			}
			bad := append([]byte(nil), wire...)
			bad[offChecksum] = byte(tc.field >> 8)
			bad[offChecksum+1] = byte(tc.field)
			if VerifyPacketChecksum(bad) {
				t.Fatalf("VerifyPacketChecksum accepted the checksum %#04x computed with the %s", tc.field, tc.name)
			}
		})
	}
}
