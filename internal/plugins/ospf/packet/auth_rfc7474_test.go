// Design: docs/architecture/ospf/ospf-12-auth.md -- OSPFv2 cryptographic authentication.
// RFC: rfc/short/rfc7474.md -- Section 6 (Ko zero-padded to the Ipad/Opad length).
package packet

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"hash"
	"testing"

	"github.com/stretchr/testify/require"
)

// hmacWithPad computes HMAC(Ko, msg) by hand: Ko is extended to the hash block size B
// with the pad octet before the XOR with Ipad (0x36) and Opad (0x5c). A zero pad is the
// RFC 7474 §6 construction; any other pad octet is the violating one.
func hmacWithPad(h func() hash.Hash, blockSize int, ko, msg []byte, pad byte) []byte {
	kb := make([]byte, blockSize)
	copy(kb, ko)
	for i := len(ko); i < blockSize; i++ {
		kb[i] = pad
	}
	ipad := make([]byte, blockSize)
	opad := make([]byte, blockSize)
	for i := range kb {
		ipad[i] = kb[i] ^ 0x36
		opad[i] = kb[i] ^ 0x5c
	}
	inner := h()
	inner.Write(ipad)
	inner.Write(msg)
	outer := h()
	outer.Write(opad)
	outer.Write(inner.Sum(nil))
	return outer.Sum(nil)
}

// rfc7474KoCases covers both SHA block sizes. The short secret plus the protocol
// ID is zero-padded to L independently of deriveKo, then the reference hash
// explicitly pads Ko to B.
var rfc7474KoCases = []struct {
	algo      string
	h         func() hash.Hash
	blockSize int
}{
	{algo: AuthHMACSHA256, h: sha256.New, blockSize: 64},
	{algo: AuthHMACSHA512, h: sha512.New, blockSize: 128},
}

// MUTATION: Pad Ko with nonzero bytes to the hash block size in hmacDigest.
// RFC requirement: RFC7474-6-2 positive -- AuType-3 Sign matches an independently constructed HMAC with Ko zero-padded to B before the Ipad/Opad XOR, and Verify accepts that digest for SHA-256 and SHA-512.
func TestRFC7474KoZeroPaddedToBlockSize(t *testing.T) {
	for _, tc := range rfc7474KoCases {
		t.Run(tc.algo, func(t *testing.T) {
			key := AuthKey{KeyID: 3, Algorithm: tc.algo, Secret: []byte("short")}
			src := [4]byte{192, 0, 2, 1}
			wire := helloWire(t, AuTypeCryptographicESN)
			plen := len(wire)
			signed, err := Sign(wire, AuTypeCryptographicESN, key, 9, src)
			require.NoError(t, err)

			l := tc.h().Size()
			ko := make([]byte, l)
			copy(ko, []byte{'s', 'h', 'o', 'r', 't', 0, 1})
			msg := append(bytes.Clone(signed[:plen+8]), bytes.Repeat([]byte{0x87, 0x8f, 0xe1, 0xf3}, l/4)...)
			copy(msg[plen+8:plen+12], src[:])
			want := hmacWithPad(tc.h, tc.blockSize, ko, msg, 0x00)
			if !bytes.Equal(signed[plen+8:], want) {
				t.Fatalf("wire digest %x != HMAC with zero-padded Ko %x", signed[plen+8:], want)
			}
			if _, ok := Verify(signed, AuTypeCryptographicESN, key, src); !ok {
				t.Fatalf("Verify rejected the zero-padded-Ko digest")
			}
		})
	}
}

// MUTATION: Pad Ko with 0xff bytes to the hash block size in hmacDigest.
// RFC requirement: RFC7474-6-2 negative -- AuType-3 Verify rejects SHA-256 and SHA-512 digests whose Ko uses 0xff padding to B instead of zero padding before the Ipad/Opad XOR.
func TestRFC7474KoNonZeroPadRejected(t *testing.T) {
	for _, tc := range rfc7474KoCases {
		t.Run(tc.algo, func(t *testing.T) {
			key := AuthKey{KeyID: 3, Algorithm: tc.algo, Secret: []byte("short")}
			src := [4]byte{192, 0, 2, 1}
			wire := helloWire(t, AuTypeCryptographicESN)
			plen := len(wire)
			signed, err := Sign(wire, AuTypeCryptographicESN, key, 9, src)
			require.NoError(t, err)

			l := tc.h().Size()
			ko := make([]byte, l)
			copy(ko, []byte{'s', 'h', 'o', 'r', 't', 0, 1})
			msg := append(bytes.Clone(signed[:plen+8]), bytes.Repeat([]byte{0x87, 0x8f, 0xe1, 0xf3}, l/4)...)
			copy(msg[plen+8:plen+12], src[:])
			bad := hmacWithPad(tc.h, tc.blockSize, ko, msg, 0xff)
			if bytes.Equal(signed[plen+8:], bad) {
				t.Fatalf("wire digest equals the HMAC with a 0xff-padded Ko: the pad is not zero")
			}
			forged := append(bytes.Clone(signed[:plen+8]), bad...)
			if _, ok := Verify(forged, AuTypeCryptographicESN, key, src); ok {
				t.Fatalf("Verify accepted a digest built from a 0xff-padded Ko")
			}
		})
	}
}
