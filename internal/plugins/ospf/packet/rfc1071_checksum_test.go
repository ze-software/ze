// Design: docs/architecture/ospf/ospf-2-wire.md -- checksum covered-range tests

package packet

import "testing"

// rfc1071ReferenceFold is the test's own RFC 1071 arithmetic, written apart
// from types.InternetChecksumPair so the production generator and verifier are
// judged against an independent answer. It sums the 16-bit big-endian words of
// each segment (every OSPFv2 segment is even-length) into a 64-bit accumulator
// and folds until no carry is left.
func rfc1071ReferenceFold(segments ...[]byte) uint16 {
	var sum uint64
	for _, segment := range segments {
		for i := 0; i+1 < len(segment); i += 2 {
			sum += uint64(segment[i])<<8 + uint64(segment[i+1])
		}
	}
	for sum > 0xffff {
		sum = sum>>16 + sum&0xffff
	}
	return uint16(sum)
}

// rfc1071ReferenceChecksum answers the checksum an OSPFv2 packet must carry:
// the field cleared, the words summed over every octet except the 64-bit
// Authentication field (RFC 2328 A.3.1), and the sum complemented.
func rfc1071ReferenceChecksum(wire []byte) uint16 {
	cleared := append([]byte(nil), wire...)
	cleared[offChecksum] = 0
	cleared[offChecksum+1] = 0
	return ^rfc1071ReferenceFold(cleared[:offAuth], cleared[offAuth+AuthFieldLen:])
}

// samplePacketWire encodes a Hello packet into a buffer prefilled with 0xA5,
// with checksum preset in the header, and returns the wire bytes.
func samplePacketWire(t *testing.T, checksum uint16) []byte {
	t.Helper()
	hello := sampleHello(t)
	header := sampleHeader(t, PacketTypeHello)
	header.Checksum = checksum
	p := Packet{Header: header, Hello: &hello}
	buf := make([]byte, p.EncodedLen())
	for i := range buf {
		buf[i] = 0xa5
	}
	if n := (&p).WriteTo(buf, 0); n != len(buf) {
		t.Fatalf("Packet.WriteTo wrote %d, want %d", n, len(buf))
	}
	return buf
}

// VALIDATES: the generate procedure, step by step, against an independent
// answer. The packet's header arrives with a stale Checksum 0xBEEF and the
// output buffer holds 0xA5 garbage, so an encoder that summed before clearing
// the field would fold 0xBEEF (or 0xA5A5) into the sum. The field ze writes
// must equal the test's own RFC 1071 checksum (field cleared, sum, complement)
// and must not depend on what the field held before.
//
// RFC requirement: RFC1071-1-1 positive -- with a stale 0xBEEF in the header's Checksum and 0xA5 garbage in the buffer, WriteTo clears the field, sums, and places the complement: the field equals the test's independent RFC 1071 checksum and the same packet encoded from a zero Checksum, and the whole covered packet then folds to 0xFFFF under the test's arithmetic.
// RFC requirement: RFC1071-1-3 positive -- the 16-bit one's complement sum over the covered octets is computed and its complement is placed in the packet's checksum field by WriteTo: the field read back from the wire equals the complement of the test's independent sum.
func TestRFC1071PacketChecksumClearedSummedAndComplemented(t *testing.T) {
	stale := samplePacketWire(t, 0xbeef)
	fresh := samplePacketWire(t, 0)
	got := readUint16(stale, offChecksum)
	if want := rfc1071ReferenceChecksum(stale); got != want {
		t.Fatalf("checksum field = %#04x, want %#04x: the stale field or buffer garbage entered the sum", got, want)
	}
	if other := readUint16(fresh, offChecksum); got != other {
		t.Fatalf("checksum with a stale header field = %#04x, from a zero field = %#04x: the field was not cleared", got, other)
	}
	if fold := rfc1071ReferenceFold(stale[:offAuth], stale[offAuth+AuthFieldLen:]); fold != 0xffff {
		t.Fatalf("covered packet folds to %#04x with ze's checksum in place, want 0xffff", fold)
	}
}

// VALIDATES: the receive check sums the same octets INCLUDING the checksum
// field and succeeds on all ones. The checksum placed in the field is the
// test's own (rfc1071ReferenceChecksum), not ze's generator's, so the verifier
// is judged against independent arithmetic.
//
// RFC requirement: RFC1071-1-5 positive -- VerifyPacketChecksum accepts a packet whose checksum field holds the test's independently computed checksum, whose covered words (checksum field included) fold to 0xFFFF under the test's arithmetic.
func TestRFC1071VerifyAcceptsAllOnesSum(t *testing.T) {
	wire := samplePacketWire(t, 0)
	want := rfc1071ReferenceChecksum(wire)
	writeUint16(wire, offChecksum, want)
	if fold := rfc1071ReferenceFold(wire[:offAuth], wire[offAuth+AuthFieldLen:]); fold != 0xffff {
		t.Fatalf("setup: covered packet folds to %#04x, want 0xffff", fold)
	}
	if !VerifyPacketChecksum(wire) {
		t.Fatalf("VerifyPacketChecksum refused a packet whose sum including the checksum field is all ones: % x", wire)
	}
}

// VALIDATES: a sum that is not all ones fails the check. Each case starts from
// a packet that verifies and changes one thing: the checksum field alone (so a
// verifier that left the field out of the sum, or compared it with nothing,
// is caught), one covered body octet, and the field cleared to zero.
//
// RFC requirement: RFC1071-1-5 negative -- VerifyPacketChecksum refuses a packet whose covered words, checksum field included, no longer fold to 0xFFFF: the checksum field off by one, a flipped body octet, and a zeroed checksum field are each refused.
func TestRFC1071VerifyRefusesSumNotAllOnes(t *testing.T) {
	base := samplePacketWire(t, 0)
	good := rfc1071ReferenceChecksum(base)
	writeUint16(base, offChecksum, good)
	if !VerifyPacketChecksum(base) {
		t.Fatalf("setup: VerifyPacketChecksum refused the valid base packet")
	}
	cases := []struct {
		name   string
		mutate func(wire []byte)
	}{
		{"checksum field off by one", func(wire []byte) { writeUint16(wire, offChecksum, good+1) }},
		{"body octet flipped", func(wire []byte) { wire[len(wire)-1] ^= 0xff }},
		{"checksum field zeroed", func(wire []byte) { writeUint16(wire, offChecksum, 0) }},
	}
	for _, tc := range cases {
		wire := append([]byte(nil), base...)
		tc.mutate(wire)
		if fold := rfc1071ReferenceFold(wire[:offAuth], wire[offAuth+AuthFieldLen:]); fold == 0xffff {
			t.Fatalf("%s: setup still folds to 0xffff", tc.name)
		}
		if VerifyPacketChecksum(wire) {
			t.Errorf("%s: VerifyPacketChecksum accepted a packet whose sum is not all ones", tc.name)
		}
	}
}
