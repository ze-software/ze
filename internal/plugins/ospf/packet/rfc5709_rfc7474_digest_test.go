// Design: docs/architecture/ospf/ospf-12-auth.md -- OSPFv2 cryptographic authentication.
// Related: auth_verify.go -- Sign and Verify, the producers.
//
// VALIDATES: RFC 5709 sec 3.1, the Authentication Data Length is the hash output length in
// octets: the literal 20, 32, 48 and 64 for HMAC-SHA-1/256/384/512, and a packet whose
// length field names another length is refused even when its digest is valid. RFC 7474
// sec 2, the 64-bit sequence number after the packet is covered by the digest: the AuType 3
// digest Sign writes equals an independent HMAC over packet || sequence || Apad, and a
// packet whose sequence octets were changed after signing is refused.
// PREVENTS: a length table that drifts from the hash (the older units compare the field
// with the same table), a receiver that ignores the length field, and a digest that leaves
// the sequence out on both sides (the round trip would still verify).
package packet

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

// RFC requirement: RFC5709-3.1-2 positive -- Sign (AuType 2) sets the Authentication Data
// Length to the hash output length in octets, the literal 20, 32, 48 and 64 for
// HMAC-SHA-1, -256, -384 and -512, and appends exactly that many digest octets, equal to an
// independently computed RFC 5709 digest.
func TestRFC5709AuthDataLengthIsHashOutputLength(t *testing.T) {
	// Goal: the length field against the hash, not against the production table. Method:
	// sign one Hello under each algorithm, read af[3], the trailer length and the trailer.
	cases := []struct {
		algorithm string
		octets    int
	}{
		{algorithm: AuthHMACSHA1, octets: 20},
		{algorithm: AuthHMACSHA256, octets: 32},
		{algorithm: AuthHMACSHA384, octets: 48},
		{algorithm: AuthHMACSHA512, octets: 64},
	}
	for _, tc := range cases {
		t.Run(tc.algorithm, func(t *testing.T) {
			key := AuthKey{KeyID: 7, Algorithm: tc.algorithm, Secret: []byte("rfc5709-length")}
			wire := helloWire(t, AuTypeCryptographic)
			plen := len(wire)
			signed, err := Sign(wire, AuTypeCryptographic, key, 1, [4]byte{})
			if err != nil {
				t.Fatalf("Sign: %v", err)
			}
			if got := int(signed[offAuth+3]); got != tc.octets {
				t.Fatalf("Authentication Data Length = %d, want %d", got, tc.octets)
			}
			if got := len(signed) - plen; got != tc.octets {
				t.Fatalf("trailer is %d octets, want %d", got, tc.octets)
			}
			want := rfc5709ReferenceDigest(t, tc.algorithm, key.Secret, signed[:plen])
			if !bytes.Equal(signed[plen:], want) {
				t.Fatalf("trailer % x, want the independent digest % x", signed[plen:], want)
			}
		})
	}
}

// rfc5709PacketWithLengthField builds an AuType 2 HMAC-SHA-1 Hello whose Authentication
// Data Length field holds lengthField, followed by a digest computed independently over
// that very header, so only the length field can make it fail.
func rfc5709PacketWithLengthField(t *testing.T, key AuthKey, lengthField byte) []byte {
	t.Helper()
	wire := helloWire(t, AuTypeCryptographic)
	af := wire[offAuth : offAuth+AuthFieldLen]
	af[0], af[1] = 0, 0
	af[2] = byte(key.KeyID)
	af[3] = lengthField
	writeUint32(wire, offAuth+4, 1)
	return append(wire, rfc5709ReferenceDigest(t, AuthHMACSHA1, key.Secret, wire)...)
}

// RFC requirement: RFC5709-3.1-2 negative -- a received HMAC-SHA-1 packet whose
// Authentication Data Length says 32 (not the 20-octet SHA-1 output) is refused by Verify,
// although its 20-octet digest is valid over the packet as sent; the same packet with the
// field at 20 verifies.
func TestRFC5709WrongAuthDataLengthRejected(t *testing.T) {
	// Goal: the length field is checked on receive. Method: a digest that is valid over a
	// header carrying the wrong length, and a control with the right length.
	key := AuthKey{KeyID: 7, Algorithm: AuthHMACSHA1, Secret: []byte("rfc5709-length")}
	if _, ok := Verify(rfc5709PacketWithLengthField(t, key, 20), AuTypeCryptographic, key, [4]byte{}); !ok {
		t.Fatal("control: the packet with Authentication Data Length 20 does not verify")
	}
	if _, ok := Verify(rfc5709PacketWithLengthField(t, key, 32), AuTypeCryptographic, key, [4]byte{}); ok {
		t.Fatal("a SHA-1 packet whose Authentication Data Length is 32 verified")
	}
}

// rfc7474ReferenceDigest is the RFC 7474 AuType 3 HMAC-SHA-256 digest built without the
// production helpers: key || Protocol ID 00 01 (sec 6), message packet || 64-bit sequence
// || Apad, Apad being the source address then 0x878FE1F3 to 32 octets (sec 5).
func rfc7474ReferenceDigest(secret, pkt []byte, seq [8]byte, src [4]byte) []byte {
	ko := make([]byte, sha256.Size)
	copy(ko, append(bytes.Clone(secret), 0x00, 0x01))
	msg := append(bytes.Clone(pkt), seq[:]...)
	msg = append(msg, src[:]...)
	msg = append(msg, bytes.Repeat([]byte{0x87, 0x8f, 0xe1, 0xf3}, (sha256.Size-4)/4)...)
	return hmacWithPad(sha256.New, sha256.BlockSize, ko, msg, 0)
}

// RFC requirement: RFC7474-2-1 positive -- the AuType 3 packet Sign writes carries the
// 64-bit sequence in the 8 octets after the OSPF packet, and its digest equals an
// independent HMAC-SHA-256 over packet || those 8 octets || Apad: the sequence is included
// in the digest calculation.
// RFC requirement: RFC7474-3-1 positive -- the same packet's authentication field is the
// literal 00 00 00 28 01 02 03 04: the 24-bit zero field, Auth Data Length 40 (8 + 32), and
// the 32-bit Key ID 0x01020304 in the former sequence position; the OSPF Packet Length
// excludes the 8 sequence octets that follow the packet, and the digest covers them.
func TestRFC7474SequenceIncludedInDigest(t *testing.T) {
	// Goal: digest coverage, which a round trip cannot show. Method: sign, then compare the
	// trailer with a digest built independently over the packet and the sequence.
	key := AuthKey{KeyID: 0x01020304, Algorithm: AuthHMACSHA256, Secret: []byte("extended-seq-key")}
	src := [4]byte{192, 0, 2, 1}
	wire := helloWire(t, AuTypeCryptographicESN)
	plen := len(wire)
	seq := [8]byte{0x00, 0x00, 0x00, 0x05, 0x00, 0x00, 0x00, 0x09}
	signed, err := Sign(wire, AuTypeCryptographicESN, key, 0x0000000500000009, src)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(signed) != plen+8+sha256.Size {
		t.Fatalf("signed packet is %d octets, want %d", len(signed), plen+8+sha256.Size)
	}
	// Authentication field: 24-bit zero, Auth Data Length 8 + 32, 32-bit Key ID where the
	// 32-bit sequence used to be; the OSPF Packet Length still excludes the 8 octets.
	if want := []byte{0x00, 0x00, 0x00, 0x28, 0x01, 0x02, 0x03, 0x04}; !bytes.Equal(signed[offAuth:offAuth+AuthFieldLen], want) {
		t.Fatalf("authentication field % x, want % x", signed[offAuth:offAuth+AuthFieldLen], want)
	}
	if got := int(readUint16(signed, offLength)); got != plen {
		t.Fatalf("OSPF Packet Length = %d, want %d (the sequence is outside it)", got, plen)
	}
	if !bytes.Equal(signed[plen:plen+8], seq[:]) {
		t.Fatalf("octets after the packet % x, want the sequence % x", signed[plen:plen+8], seq)
	}
	want := rfc7474ReferenceDigest(key.Secret, signed[:plen], seq, src)
	if !bytes.Equal(signed[plen+8:], want) {
		t.Fatalf("digest % x, want % x (packet || sequence || Apad)", signed[plen+8:], want)
	}
}

// RFC requirement: RFC7474-2-1 negative -- a received AuType 3 packet whose 8 sequence
// octets were changed after signing (digest and packet untouched) is refused by Verify,
// because the sequence is part of the digest; the unchanged packet verifies.
// RFC requirement: RFC7474-3-1 negative -- the 64-bit sequence after the packet is
// protected by the digest: the same changed-sequence packet is refused.
func TestRFC7474SequenceChangeBreaksDigest(t *testing.T) {
	// Goal: the receiver hashes the sequence too. Method: flip one sequence octet only.
	key := AuthKey{KeyID: 0x01020304, Algorithm: AuthHMACSHA256, Secret: []byte("extended-seq-key")}
	src := [4]byte{192, 0, 2, 1}
	wire := helloWire(t, AuTypeCryptographicESN)
	plen := len(wire)
	signed, err := Sign(wire, AuTypeCryptographicESN, key, 0x0000000500000009, src)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, ok := Verify(signed, AuTypeCryptographicESN, key, src); !ok {
		t.Fatal("control: the unchanged packet does not verify")
	}
	changed := bytes.Clone(signed)
	changed[plen+7] ^= 0x01
	if _, ok := Verify(changed, AuTypeCryptographicESN, key, src); ok {
		t.Fatal("a packet whose sequence octets changed after signing verified")
	}
}
