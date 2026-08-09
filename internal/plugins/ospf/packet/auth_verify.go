// Design: docs/architecture/ospf/ospf-12-auth.md -- OSPFv2 authentication sign/verify.
// RFC: rfc/short/rfc2328.md (App D Keyed-MD5), rfc/short/rfc5709.md (HMAC-SHA + Apad),
// rfc/short/rfc7474.md (AuType 3 extended 64-bit sequence).
//
// The OSPF common-header codec (ospf-2) already zeroes the Checksum and excludes the
// 8-byte auth field from the checksum for AuType 2/3, and the Packet Length covers only
// header+body. This file is the cryptographic backend: it sets the 8-byte auth field,
// computes the digest over the (zero-checksum) packet, and appends the trailer.

package packet

import (
	"crypto/hmac"
	"crypto/md5"  //nolint:gosec // G501: Keyed-MD5 is RFC 2328 App D wire auth, not a security primitive choice
	"crypto/sha1" //nolint:gosec // G505: HMAC-SHA-1 is an RFC 5709 wire algorithm selected by config
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"hash"
)

// Authentication algorithm identifiers (match the ze-ospf-conf.yang algorithm enum).
const (
	AuthSimple     = "simple"
	AuthMD5        = "md5"
	AuthHMACSHA1   = "hmac-sha-1"
	AuthHMACSHA256 = "hmac-sha-256"
	AuthHMACSHA384 = "hmac-sha-384"
	AuthHMACSHA512 = "hmac-sha-512"
)

// apadSeed is the RFC 5709 §3.3 Apad value 0x878FE1F3, repeated L/4 times to length L.
var apadSeed = [4]byte{0x87, 0x8F, 0xE1, 0xF3}

// ospfv2CryptoProtocolID is the RFC 7474 §6 two-octet OSPFv2 Cryptographic Protocol ID
// (IANA "Authentication Cryptographic Protocol ID" = 1), appended to the key for AuType
// 3 to block cross-protocol replay.
var ospfv2CryptoProtocolID = [2]byte{0x00, 0x01}

// AuthKey is a resolved signing/verifying key.
type AuthKey struct {
	KeyID     uint32
	Algorithm string
	Secret    []byte //nolint:gosec // G117: decoded key material held in memory only, never serialized
}

// authDigestLen returns the digest length L (octets) for a cryptographic algorithm, or
// 0 for an unknown/simple algorithm.
func authDigestLen(algo string) int {
	switch algo {
	case AuthMD5:
		return 16
	case AuthHMACSHA1:
		return 20
	case AuthHMACSHA256:
		return 32
	case AuthHMACSHA384:
		return 48
	case AuthHMACSHA512:
		return 64
	}
	return 0
}

func authHash(algo string) func() hash.Hash {
	switch algo {
	case AuthHMACSHA1:
		return sha1.New
	case AuthHMACSHA256:
		return sha256.New
	case AuthHMACSHA384:
		return sha512.New384
	case AuthHMACSHA512:
		return sha512.New
	}
	return nil
}

// apad returns the L-octet RFC 5709 Apad fill.
func apad(l int) []byte {
	out := make([]byte, l)
	for i := range out {
		out[i] = apadSeed[i%4]
	}
	return out
}

// apadSrc returns the L-octet Apad with its first four octets overwritten by the IPv4
// source address (the remainder keeps the 0x878FE1F3 fill). It is used only for AuType 3
// (RFC 7474); AuType 2 keeps the plain RFC 5709 Apad.
//
// RFC 7474 Section 5: "OSPF routers sending OSPF packets must initialize the first 4
// octets of Apad to the value of the IP source address that would be used when sending
// the OSPFv2 packet" (and receivers use the incoming packet's source address), so a
// spoofed source address yields a digest mismatch.
func apadSrc(l int, src [4]byte) []byte {
	out := apad(l)
	if l >= 4 {
		copy(out[:4], src[:])
	}
	return out
}

// deriveKo reduces secret to the L-octet working key Ko (RFC 5709 §3.3 step 1): K when
// already L octets, H(K) when longer, K zero-padded on the right when shorter.
func deriveKo(secret []byte, l int, h func() hash.Hash) []byte {
	ko := make([]byte, l)
	switch {
	case len(secret) > l:
		hh := h()
		hh.Write(secret)
		copy(ko, hh.Sum(nil))
	default:
		copy(ko, secret)
	}
	return ko
}

// hmacDigest computes the RFC 5709 HMAC-SHA digest over msg.
func hmacDigest(algo string, secret, msg []byte) []byte {
	h := authHash(algo)
	ko := deriveKo(secret, authDigestLen(algo), h)
	mac := hmac.New(h, ko)
	mac.Write(msg)
	return mac.Sum(nil)
}

// md5KeyedDigest computes the RFC 2328 App D keyed-MD5 digest MD5(pkt || key16), the
// trailer key region being the 16-byte secret (truncated/zero-padded).
func md5KeyedDigest(secret, pkt []byte) []byte {
	var key16 [16]byte
	copy(key16[:], secret)
	d := md5.New() //nolint:gosec // G401: RFC 2328 App D keyed-MD5 is a fixed wire algorithm, not a security choice
	d.Write(pkt)
	d.Write(key16[:])
	return d.Sum(nil)
}

// cryptoDigest builds the AuType 2 / AuType 3 digest over the message (the trailer is
// already appended to msg as the key16 for MD5 or Apad for HMAC).
func cryptoDigest(key AuthKey, pkt, trailer []byte) []byte {
	if key.Algorithm == AuthMD5 {
		return md5KeyedDigest(key.Secret, pkt)
	}
	msg := make([]byte, 0, len(pkt)+len(trailer))
	msg = append(msg, pkt...)
	msg = append(msg, trailer...)
	return hmacDigest(key.Algorithm, key.Secret, msg)
}

// Sign authenticates an encoded OSPF packet under auType with key and returns the wire
// bytes. pkt is the encoded packet (header+body); for AuType 2/3 the encoder has already
// zeroed the Checksum. seq is the cryptographic sequence number (32-bit for AuType 2,
// 64-bit for AuType 3). src is the IPv4 source address the packet is sent from; it is
// folded into the Apad of AuType 3 digests per RFC 7474 §5 and ignored for AuType 0/1/2.
// pkt may be mutated in place (the auth field is written).
func Sign(pkt []byte, auType AuType, key AuthKey, seq uint64, src [4]byte) ([]byte, error) {
	if len(pkt) < CommonHeaderLen {
		return nil, ErrShortBuffer
	}
	switch auType {
	case AuTypeNull:
		return pkt, nil
	case AuTypeSimple:
		var pw [AuthFieldLen]byte
		copy(pw[:], key.Secret)
		copy(pkt[offAuth:offAuth+AuthFieldLen], pw[:])
		return pkt, nil
	case AuTypeCryptographic:
		l := authDigestLen(key.Algorithm)
		if l == 0 {
			return nil, ErrLength
		}
		af := pkt[offAuth : offAuth+AuthFieldLen]
		af[0], af[1] = 0, 0
		af[2] = byte(key.KeyID)
		af[3] = byte(l)
		writeUint32(pkt, offAuth+4, uint32(seq))
		digest := cryptoDigest(key, pkt, apad(l))
		return append(pkt, digest...), nil
	case AuTypeCryptographicESN:
		l := authDigestLen(key.Algorithm)
		if l == 0 {
			return nil, ErrLength
		}
		af := pkt[offAuth : offAuth+AuthFieldLen]
		af[0], af[1], af[2] = 0, 0, 0
		af[3] = byte(8 + l)
		writeUint32(pkt, offAuth+4, key.KeyID)
		var seqb [8]byte
		writeUint64(seqb[:], 0, seq)
		// RFC 7474 §5: the 64-bit sequence is covered by the digest (before the Apad
		// trailer) and the IP source address is bound into the Apad; §6: the protocol ID
		// is appended to the key.
		esn := AuthKey{Algorithm: key.Algorithm, Secret: append(append([]byte{}, key.Secret...), ospfv2CryptoProtocolID[:]...)}
		digest := cryptoDigest(esn, append(append([]byte{}, pkt...), seqb[:]...), apadSrc(l, src))
		out := make([]byte, 0, len(pkt)+8+l)
		out = append(out, pkt...)
		out = append(out, seqb[:]...)
		out = append(out, digest...)
		return out, nil
	}
	return nil, ErrUnknownType
}

// Verify checks an OSPF packet's authentication under auType with key and returns the
// cryptographic sequence number plus whether it verified. src is the IPv4 source address
// of the received packet (the IP header source); it is folded into the AuType 3 Apad per
// RFC 7474 §5 so a spoofed source yields a mismatch, and ignored for AuType 0/1/2. All
// digest and password comparisons are constant-time.
func Verify(wire []byte, auType AuType, key AuthKey, src [4]byte) (uint64, bool) {
	if len(wire) < CommonHeaderLen {
		return 0, false
	}
	switch auType {
	case AuTypeNull:
		return 0, true
	case AuTypeSimple:
		var pw [AuthFieldLen]byte
		copy(pw[:], key.Secret)
		return 0, subtle.ConstantTimeCompare(wire[offAuth:offAuth+AuthFieldLen], pw[:]) == 1
	case AuTypeCryptographic:
		l := authDigestLen(key.Algorithm)
		if l == 0 {
			return 0, false
		}
		af := wire[offAuth : offAuth+AuthFieldLen]
		plen := int(readUint16(wire, offLength))
		// RFC 5709 §3.2: the authentication trailer is exactly Auth Data Len octets
		// immediately after the OSPF packet; reject any extra trailing bytes (== not >=).
		if int(af[3]) != l || plen < CommonHeaderLen || plen+l != len(wire) {
			return 0, false
		}
		// RFC 5709 §3.2 / RFC 2328 App D: the key is selected implicitly by the packet
		// Key ID; a digest computed under any other key id MUST NOT verify. The AuType 2
		// Key ID field is a single octet.
		if uint32(af[2]) != key.KeyID {
			return 0, false
		}
		seq := uint64(readUint32(af, 4))
		expect := cryptoDigest(key, wire[:plen], apad(l))
		return seq, subtle.ConstantTimeCompare(expect, wire[plen:plen+l]) == 1
	case AuTypeCryptographicESN:
		l := authDigestLen(key.Algorithm)
		if l == 0 {
			return 0, false
		}
		af := wire[offAuth : offAuth+AuthFieldLen]
		plen := int(readUint16(wire, offLength))
		// RFC 7474 §3: the trailer is exactly the 8-octet extended sequence followed by
		// Auth Data Len's digest; reject any extra trailing bytes (== not >=).
		if int(af[3]) != 8+l || plen < CommonHeaderLen || plen+8+l != len(wire) {
			return 0, false
		}
		// RFC 7474 §3: the 32-bit Key ID occupies the four octets after the reserved and
		// length fields; select the key implicitly by it so a foreign key id never verifies.
		if readUint32(af, 4) != key.KeyID {
			return 0, false
		}
		seq := readUint64(wire, plen)
		esn := AuthKey{Algorithm: key.Algorithm, Secret: append(append([]byte{}, key.Secret...), ospfv2CryptoProtocolID[:]...)}
		expect := cryptoDigest(esn, wire[:plen+8], apadSrc(l, src))
		return seq, subtle.ConstantTimeCompare(expect, wire[plen+8:plen+8+l]) == 1
	}
	return 0, false
}
