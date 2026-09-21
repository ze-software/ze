// Design: docs/architecture/ospf/ospf-auth.md -- OSPFv2 cryptographic authentication.
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

// rfc7474KoCases lists the SHA algorithms with their block sizes. The 5-octet secret is
// shorter than every digest length L, so Ko is the secret zero-padded to L (RFC 5709
// §3.3) and then shorter than B, which is where the §6 padding applies.
var rfc7474KoCases = []struct {
	algo      string
	h         func() hash.Hash
	blockSize int
}{
	{algo: AuthHMACSHA256, h: sha256.New, blockSize: 64},
	{algo: AuthHMACSHA512, h: sha512.New, blockSize: 128},
}

// RFC requirement: RFC7474-6-2 positive -- the digest Sign writes equals an HMAC computed
// with Ko explicitly padded with zeros to the Ipad/Opad length B (64 for SHA-256, 128 for
// SHA-512) before the XOR, and Verify accepts it (hmacDigest, auth_verify.go, feeds the
// L-octet Ko to crypto/hmac; deriveKo zero-pads the short secret to L).
func TestRFC7474KoZeroPaddedToBlockSize(t *testing.T) {
	for _, tc := range rfc7474KoCases {
		t.Run(tc.algo, func(t *testing.T) {
			key := AuthKey{KeyID: 3, Algorithm: tc.algo, Secret: []byte("short")}
			wire := helloWire(t, AuTypeCryptographic)
			plen := len(wire)
			signed, err := Sign(wire, AuTypeCryptographic, key, 9, [4]byte{})
			require.NoError(t, err)

			l := authDigestLen(tc.algo)
			ko := deriveKo(key.Secret, l, tc.h)
			msg := append(append([]byte{}, signed[:plen]...), apad(l)...)
			want := hmacWithPad(tc.h, tc.blockSize, ko, msg, 0x00)
			if !bytes.Equal(signed[plen:], want) {
				t.Fatalf("wire digest %x != HMAC with zero-padded Ko %x", signed[plen:], want)
			}
			if _, ok := Verify(signed, AuTypeCryptographic, key, [4]byte{}); !ok {
				t.Fatalf("Verify rejected the zero-padded-Ko digest")
			}
		})
	}
}

// RFC requirement: RFC7474-6-2 negative -- a digest computed with Ko padded to the
// Ipad/Opad length with a non-zero octet (0xff) differs from the one Sign writes, and a
// packet carrying that digest is rejected by Verify (hmacDigest and Verify, auth_verify.go).
func TestRFC7474KoNonZeroPadRejected(t *testing.T) {
	for _, tc := range rfc7474KoCases {
		t.Run(tc.algo, func(t *testing.T) {
			key := AuthKey{KeyID: 3, Algorithm: tc.algo, Secret: []byte("short")}
			wire := helloWire(t, AuTypeCryptographic)
			plen := len(wire)
			signed, err := Sign(wire, AuTypeCryptographic, key, 9, [4]byte{})
			require.NoError(t, err)

			l := authDigestLen(tc.algo)
			ko := deriveKo(key.Secret, l, tc.h)
			msg := append(append([]byte{}, signed[:plen]...), apad(l)...)
			bad := hmacWithPad(tc.h, tc.blockSize, ko, msg, 0xff)
			if bytes.Equal(signed[plen:], bad) {
				t.Fatalf("wire digest equals the HMAC with a 0xff-padded Ko: the pad is not zero")
			}
			forged := append(append([]byte{}, signed[:plen]...), bad...)
			if _, ok := Verify(forged, AuTypeCryptographic, key, [4]byte{}); ok {
				t.Fatalf("Verify accepted a digest built from a 0xff-padded Ko")
			}
		})
	}
}
