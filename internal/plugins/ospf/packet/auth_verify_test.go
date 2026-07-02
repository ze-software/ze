// VALIDATES: spec-ospf-12 RFC 2328 App D / RFC 5709 / RFC 7474 -- sign/verify round-trip
// for AuType 1 (Simple), AuType 2 (Keyed-MD5 + HMAC-SHA-1/256/384/512), and AuType 3
// (extended 64-bit sequence); the AuType 2/3 8-byte field layout; the appended digest
// excluded from Packet Length; a zeroed checksum; a wrong key rejected; constant-time
// compare.
// PREVENTS: regressions where a digest is mis-framed, the wrong bytes are covered, the
// checksum is backfilled, the sequence number is lost, or a forged packet verifies.
package packet

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helloWire builds a minimal encoded Hello with the given AuType, ready for signing.
func helloWire(t *testing.T, au AuType) []byte {
	t.Helper()
	p := Packet{
		Header: Header{Type: PacketTypeHello, AuType: au},
		Hello:  &Hello{NetworkMask: [4]byte{255, 255, 255, 0}, HelloInterval: 10, DeadInterval: 40, Priority: 1},
	}
	buf := make([]byte, p.EncodedLen())
	n := p.WriteTo(buf, 0)
	return buf[:n]
}

func TestOSPFAuthSignVerifySimple(t *testing.T) {
	key := AuthKey{KeyID: 1, Algorithm: AuthSimple, Secret: []byte("s3cr3t")}
	wire := helloWire(t, AuTypeSimple)
	signed, err := Sign(wire, AuTypeSimple, key, 0, [4]byte{})
	require.NoError(t, err)
	// The 8-byte auth field carries the password (zero-padded), the checksum is normal.
	assert.Equal(t, append([]byte("s3cr3t"), 0, 0), signed[offAuth:offAuth+AuthFieldLen])
	require.True(t, VerifyPacketChecksum(signed), "AuType 1 keeps a normal checksum")

	seq, ok := Verify(signed, AuTypeSimple, key, [4]byte{})
	assert.True(t, ok)
	assert.Zero(t, seq)
	_, bad := Verify(signed, AuTypeSimple, AuthKey{Algorithm: AuthSimple, Secret: []byte("wrong")}, [4]byte{})
	assert.False(t, bad, "wrong password rejected")
}

func TestOSPFAuthSignVerifyCrypto(t *testing.T) {
	for _, algo := range []string{AuthMD5, AuthHMACSHA1, AuthHMACSHA256, AuthHMACSHA384, AuthHMACSHA512} {
		t.Run(algo, func(t *testing.T) {
			key := AuthKey{KeyID: 7, Algorithm: algo, Secret: []byte("correct horse battery staple")}
			wire := helloWire(t, AuTypeCryptographic)
			plen := len(wire)
			signed, err := Sign(wire, AuTypeCryptographic, key, 42, [4]byte{})
			require.NoError(t, err)

			// 8-byte field: Reserved=0, Key ID=7, Auth Data Length=L, Crypto Seq=42.
			af := signed[offAuth : offAuth+AuthFieldLen]
			assert.Equal(t, []byte{0, 0}, af[0:2], "reserved")
			assert.Equal(t, byte(7), af[2], "key id")
			assert.Equal(t, byte(authDigestLen(algo)), af[3], "auth data length = digest length")
			assert.Equal(t, uint32(42), readUint32(af, 4), "crypto sequence")
			// Checksum is zero (trap #10), Packet Length excludes the appended digest.
			assert.Zero(t, readUint16(signed, offChecksum), "checksum zero for crypto auth")
			assert.Equal(t, plen, int(readUint16(signed, offLength)), "Packet Length excludes the digest")
			assert.Len(t, signed, plen+authDigestLen(algo), "digest appended after the body")

			seq, ok := Verify(signed, AuTypeCryptographic, key, [4]byte{})
			assert.True(t, ok, "correct key verifies")
			assert.Equal(t, uint64(42), seq)

			_, bad := Verify(signed, AuTypeCryptographic, AuthKey{KeyID: 7, Algorithm: algo, Secret: []byte("wrong key")}, [4]byte{})
			assert.False(t, bad, "wrong key rejected")

			// A flipped digest byte must fail (constant-time compare still rejects).
			tampered := bytes.Clone(signed)
			tampered[len(tampered)-1] ^= 0xff
			_, t2 := Verify(tampered, AuTypeCryptographic, key, [4]byte{})
			assert.False(t, t2, "tampered digest rejected")
		})
	}
}

// TestAuthUnaffectedByInstanceSplit proves AC-11 / A-6 / R-3 (RFC 6549): narrowing the
// on-wire AuType to the low octet (offset 15) leaves cryptographic authentication correct.
// At Instance ID 0 the header offset 14 stays zero (AuType still read from offset 15) and
// the AuType-2 digest verifies; the digest covers offset 14, so a non-zero Instance ID is
// bound into the digest (a flip of offset 14 fails verification), exactly as a peer at the
// same Instance ID would compute it.
func TestAuthUnaffectedByInstanceSplit(t *testing.T) {
	key := AuthKey{KeyID: 7, Algorithm: AuthHMACSHA256, Secret: []byte("instance-split-auth-key")}

	// Instance ID 0: today's bytes. Sign, then confirm offset 14 is zero, offset 15 is the
	// AuType, and the digest verifies (bit-for-bit compatible auth path).
	wire0 := helloWire(t, AuTypeCryptographic)
	require.Equal(t, byte(0), wire0[offInstanceID], "Instance ID 0: offset 14 zero before signing")
	require.Equal(t, byte(AuTypeCryptographic), wire0[offAuType], "AuType in the low octet (offset 15)")
	signed0, err := Sign(wire0, AuTypeCryptographic, key, 11, [4]byte{})
	require.NoError(t, err)
	require.Equal(t, byte(0), signed0[offInstanceID], "signing must not disturb the Instance ID octet")
	_, ok := Verify(signed0, AuTypeCryptographic, key, [4]byte{})
	assert.True(t, ok, "AuType 2 at Instance ID 0 verifies unchanged")

	// The digest covers offset 14, so flipping the Instance ID octet on the wire breaks it.
	tampered := bytes.Clone(signed0)
	tampered[offInstanceID] = 0x05
	_, bad := Verify(tampered, AuTypeCryptographic, key, [4]byte{})
	assert.False(t, bad, "a mutated Instance ID octet fails the digest (Instance ID is authenticated)")

	// A packet built at a non-zero Instance ID signs and verifies internally-consistently.
	p := Packet{
		Header: Header{Type: PacketTypeHello, InstanceID: 0x05, AuType: AuTypeCryptographic},
		Hello:  &Hello{NetworkMask: [4]byte{255, 255, 255, 0}, HelloInterval: 10, DeadInterval: 40, Priority: 1},
	}
	wire5 := make([]byte, p.EncodedLen())
	p.WriteTo(wire5, 0)
	require.Equal(t, byte(0x05), wire5[offInstanceID], "non-zero Instance ID stamped at offset 14")
	signed5, err := Sign(wire5, AuTypeCryptographic, key, 12, [4]byte{})
	require.NoError(t, err)
	require.Equal(t, byte(0x05), signed5[offInstanceID], "signing preserves the non-zero Instance ID")
	_, ok5 := Verify(signed5, AuTypeCryptographic, key, [4]byte{})
	assert.True(t, ok5, "AuType 2 at a non-zero Instance ID verifies")
}

func TestApadSrcBoundaries(t *testing.T) {
	src := [4]byte{10, 20, 30, 40}
	// l >= 4: the first four octets are the source address, the remainder keeps the fill.
	got := apadSrc(20, src)
	require.Len(t, got, 20)
	assert.Equal(t, src[:], got[:4], "first four octets are the IPv4 source address")
	assert.Equal(t, apad(20)[4:], got[4:], "octets past the source address keep the 0x878FE1F3 fill")
	// l == 0 (degenerate, unreachable for real digests): no copy, no panic, empty result.
	assert.Empty(t, apadSrc(0, src))
}

func TestOSPFAuthType3SequenceTrailer(t *testing.T) {
	key := AuthKey{KeyID: 0x01020304, Algorithm: AuthHMACSHA256, Secret: []byte("extended-seq-key")}
	src := [4]byte{192, 0, 2, 1}
	wire := helloWire(t, AuTypeCryptographicESN)
	plen := len(wire)
	const seq = uint64(0x0000000500000009) // boot count 5, counter 9
	signed, err := Sign(wire, AuTypeCryptographicESN, key, seq, src)
	require.NoError(t, err)

	// 8-byte field: 24-bit reserved 0, Auth Data Length (= 8 + digest), 32-bit Key ID.
	af := signed[offAuth : offAuth+AuthFieldLen]
	assert.Equal(t, []byte{0, 0, 0}, af[0:3], "24-bit reserved")
	assert.Equal(t, byte(8+authDigestLen(AuthHMACSHA256)), af[3], "auth data length includes the 8 sequence octets")
	assert.Equal(t, uint32(0x01020304), readUint32(af, 4), "32-bit key id in the former sequence position")
	// Wire: packet || 64-bit seq || digest; Packet Length excludes both.
	assert.Equal(t, plen, int(readUint16(signed, offLength)))
	assert.Len(t, signed, plen+8+authDigestLen(AuthHMACSHA256))
	assert.Equal(t, seq, readUint64(signed[plen:], 0), "64-bit sequence appended high-word first")

	got, ok := Verify(signed, AuTypeCryptographicESN, key, src)
	assert.True(t, ok)
	assert.Equal(t, seq, got, "verify returns the 64-bit sequence")
}

// TestOSPFAuthType3SourceBinding proves the RFC 7474 §5 requirement that the IPv4 source
// address is folded into the AuType 3 Apad: a packet signed for one source address fails
// to verify when presented with a different source (the spoof-detection property), and
// verifies only when the source matches. AuType 2 (RFC 5709) is unaffected by src.
func TestOSPFAuthType3SourceBinding(t *testing.T) {
	key := AuthKey{KeyID: 9, Algorithm: AuthHMACSHA256, Secret: []byte("rfc7474-apad-binds-source")}
	const seq = uint64(0x0000000100000007)
	signSrc := [4]byte{198, 51, 100, 7}
	spoofSrc := [4]byte{203, 0, 113, 9}

	signed, err := Sign(helloWire(t, AuTypeCryptographicESN), AuTypeCryptographicESN, key, seq, signSrc)
	require.NoError(t, err)

	_, spoofed := Verify(signed, AuTypeCryptographicESN, key, spoofSrc)
	assert.False(t, spoofed, "a different (spoofed) source address fails the AuType 3 digest")

	got, matched := Verify(signed, AuTypeCryptographicESN, key, signSrc)
	assert.True(t, matched, "the matching source address verifies")
	assert.Equal(t, seq, got)

	// AuType 2 keeps the plain RFC 5709 Apad: src must NOT affect its digest.
	signedA2, err := Sign(helloWire(t, AuTypeCryptographic), AuTypeCryptographic, key, 3, signSrc)
	require.NoError(t, err)
	require.NotNil(t, signedA2)
	_, a2ok := Verify(signedA2, AuTypeCryptographic, key, spoofSrc)
	assert.True(t, a2ok, "AuType 2 ignores the source address (RFC 5709 Apad unchanged)")
}

// TestOSPFAuthCryptoRejectsExtraTrailerBytes proves AC-1 (RFC 5709 §3.2 / RFC 7474 §3):
// the authentication trailer is exactly Auth Data Len octets (plus the 8-octet sequence
// for AuType 3); a packet with any extra unauthenticated bytes after the digest is
// rejected rather than silently accepted.
func TestOSPFAuthCryptoRejectsExtraTrailerBytes(t *testing.T) {
	key := AuthKey{KeyID: 7, Algorithm: AuthHMACSHA256, Secret: []byte("trailer-length-key")}
	for _, au := range []AuType{AuTypeCryptographic, AuTypeCryptographicESN} {
		signed, err := Sign(helloWire(t, au), au, key, 5, [4]byte{})
		require.NoError(t, err)
		_, ok := Verify(signed, au, key, [4]byte{})
		require.True(t, ok, "exact-length packet verifies")

		padded := append(bytes.Clone(signed), 0x00)
		_, bad := Verify(padded, au, key, [4]byte{})
		assert.False(t, bad, "extra byte after the digest trailer is rejected for AuType %d", au)
	}
}

// TestOSPFAuthCryptoRejectsKeyIDMismatch proves AC-2 (RFC 5709 §3.2 / RFC 2328 App D):
// the key is selected implicitly by the packet Key ID. Both keys share the same secret,
// so the digest itself matches; only the Key ID differs. Without Key-ID selection the
// wrong key would verify, so this guards the receive-side key-id binding.
func TestOSPFAuthCryptoRejectsKeyIDMismatch(t *testing.T) {
	secret := []byte("same-secret-different-key-id")

	a2 := helloWire(t, AuTypeCryptographic)
	signedA2, err := Sign(a2, AuTypeCryptographic, AuthKey{KeyID: 7, Algorithm: AuthHMACSHA256, Secret: secret}, 1, [4]byte{})
	require.NoError(t, err)
	_, mismatch2 := Verify(signedA2, AuTypeCryptographic, AuthKey{KeyID: 8, Algorithm: AuthHMACSHA256, Secret: secret}, [4]byte{})
	assert.False(t, mismatch2, "AuType 2: a non-matching Key ID must not verify even with the right secret")
	_, match2 := Verify(signedA2, AuTypeCryptographic, AuthKey{KeyID: 7, Algorithm: AuthHMACSHA256, Secret: secret}, [4]byte{})
	assert.True(t, match2, "AuType 2: matching Key ID verifies")

	a3 := helloWire(t, AuTypeCryptographicESN)
	signedA3, err := Sign(a3, AuTypeCryptographicESN, AuthKey{KeyID: 0x11223344, Algorithm: AuthHMACSHA256, Secret: secret}, 0x0000000100000001, [4]byte{})
	require.NoError(t, err)
	_, mismatch3 := Verify(signedA3, AuTypeCryptographicESN, AuthKey{KeyID: 0x55667788, Algorithm: AuthHMACSHA256, Secret: secret}, [4]byte{})
	assert.False(t, mismatch3, "AuType 3: a non-matching 32-bit Key ID must not verify even with the right secret")
	_, match3 := Verify(signedA3, AuTypeCryptographicESN, AuthKey{KeyID: 0x11223344, Algorithm: AuthHMACSHA256, Secret: secret}, [4]byte{})
	assert.True(t, match3, "AuType 3: matching Key ID verifies")
}
