// Design: docs/architecture/isis/isis-10-auth.md -- IS-IS authentication backend (sign + verify).
//
// RFC: rfc/short/rfc5304.md -- IS-IS Cryptographic Authentication (HMAC-MD5, type 54)
// RFC: rfc/short/rfc5310.md -- IS-IS Generic Cryptographic Authentication (HMAC-SHA, type 3)
//
// The TLV 10 (Authentication) STRUCTURAL codec lives in tlv_auth.go (isis-2):
// it round-trips the auth-type octet plus an opaque value. The auth crypto
// backend isis-10 owns spans three files in this package (no behavior split):
//   - auth_types.go (this file): the algorithm/key types, typed errors, and the
//     pure per-algorithm helpers (auth-type byte, digest length, hash
//     constructor, Key-ID width, Apad fill, TLV-value framing, PDU class).
//   - auth_sign.go: SignPDU plus the encode/layout/digest/checksum machinery it
//     shares with verification.
//   - auth_verify.go: VerifyPDU plus the receive-side helpers.
//
// The backend operates on raw PDU bytes only and holds NO runtime state, so it
// is testable on raw bytes and free of any import of the config / circuit / lsdb
// layers (spec Key Design Decision: "verify/sign helpers in packet, key store in
// the component").
//
// The Authentication Data field pre-image before the digest depends on the
// algorithm family (the two RFCs disagree and an interop peer follows the RFC):
//   - HMAC-MD5 (RFC 5304 sec 2): the Authentication Value inside TLV 10 is ZEROED.
//   - HMAC-SHA family (RFC 5310 sec 3.3 / sec 3.5): the Authentication Data is
//     filled with Apad (0x878FE1F3 repeated), on BOTH sign and verify.
// LSPs additionally zero the Checksum and the Remaining Lifetime fields before
// the digest (RFC 5304 sec 2 / RFC 5310 sec 3.4-3.5), for every algorithm.
// IIH/CSNP/PSNP have no Checksum or Remaining Lifetime field, so only the
// Authentication Data pre-image is set. For LSPs the Fletcher checksum is computed
// AFTER the digest is in place (the signing order is build -> sign TLV 10 ->
// Fletcher checksum; on receive the checksum is accepted, then the three saved
// fields are set to their pre-image, the digest is recomputed, and the fields are
// restored).

package packet

import (
	"crypto/md5"  //nolint:gosec // G501: RFC 5304 mandates HMAC-MD5 (auth type 54) for IS-IS interop; HMAC construction, not raw MD5.
	"crypto/sha1" //nolint:gosec // G505: RFC 5310 lists HMAC-SHA-1 (auth type 3) for interop; HMAC construction, not raw SHA-1.
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"hash"
)

// AuthAlgorithm selects the digest algorithm carried in TLV 10. The values are
// internal identifiers; the on-wire authentication TYPE byte (1/3/54) is derived
// from the algorithm by authTypeFor, and for the generic-crypto family (type 3)
// the algorithm is NEVER sent on the wire (RFC 5310 sec 2: the algorithm is
// selected per-SA via the Key ID).
type AuthAlgorithm uint8

// Authentication algorithms (spec-isis-10). Cleartext is sanity-only; HMAC-MD5
// (RFC 5304) and the HMAC-SHA family (RFC 5310) provide integrity.
const (
	AuthAlgoNone       AuthAlgorithm = iota // no authentication configured
	AuthAlgoCleartext                       // ISO/IEC 10589 cleartext password (auth type 1)
	AuthAlgoHMACMD5                         // RFC 5304 HMAC-MD5 (auth type 54)
	AuthAlgoHMACSHA1                        // RFC 5310 HMAC-SHA-1 (auth type 3)
	AuthAlgoHMACSHA224                      // RFC 5310 HMAC-SHA-224 (auth type 3)
	AuthAlgoHMACSHA256                      // RFC 5310 HMAC-SHA-256 (auth type 3)
	AuthAlgoHMACSHA384                      // RFC 5310 HMAC-SHA-384 (auth type 3)
	AuthAlgoHMACSHA512                      // RFC 5310 HMAC-SHA-512 (auth type 3)
)

// Errors returned by the auth backend. They are typed sentinels so callers can
// match without parsing strings; verify failures never echo key material or the
// received digest (security review: no leakage).
var (
	// ErrAuthMissing reports that authentication is configured but the received
	// PDU carries no TLV 10 (downgrade resistance, RFC 5304 sec 2 / spec R-6).
	ErrAuthMissing = errors.New("isis auth: authentication TLV (10) missing")
	// ErrAuthNotFirst reports that TLV 10 is present but is not the first TLV
	// (RFC 5304 sec 1; AC-8).
	ErrAuthNotFirst = errors.New("isis auth: authentication TLV (10) not first")
	// ErrAuthTypeMismatch reports that the received auth type byte does not match
	// any candidate key's algorithm.
	ErrAuthTypeMismatch = errors.New("isis auth: authentication type mismatch")
	// ErrAuthMismatch reports that no candidate key produced a matching digest
	// (the PDU is rejected; the caller increments ze_isis_auth_failures_total).
	ErrAuthMismatch = errors.New("isis auth: authentication value mismatch")
	// ErrAuthUnsupported reports an algorithm this build cannot sign/verify.
	ErrAuthUnsupported = errors.New("isis auth: unsupported authentication algorithm")
	// ErrAuthMalformed reports a PDU that cannot be decoded for signing/verifying.
	ErrAuthMalformed = errors.New("isis auth: malformed PDU")
	// ErrAuthPurgeExtraTLV reports an authenticated purge that carries a TLV other
	// than the authentication TLV (RFC 5304 sec 2: MUST NOT accept such purges).
	ErrAuthPurgeExtraTLV = errors.New("isis auth: purge carries non-authentication TLV")
)

// Key is one authentication key used to sign or verify. Secret is the raw key
// material (the $9$-decoded plaintext supplied by the component key store, held
// only in memory); the auth backend never logs it. KeyID is carried on the wire
// only for the generic-crypto family (type 3, RFC 5310); it is ignored for
// cleartext and HMAC-MD5.
type Key struct {
	Algorithm AuthAlgorithm
	Secret    []byte //nolint:gosec // G117: field name describes key material, not a literal secret; never logged.
	KeyID     uint16
}

// apadPattern is the 4-octet Apad value the RFC 5310 generic-crypto family fills
// into the Authentication Data field before hashing (RFC 5310 sec 3.3: "Apad ...
// the hex value 0x878FE1F3 repeated (L/4) times"). It is NOT used for HMAC-MD5,
// whose pre-image zeroes the value (RFC 5304 sec 2).
var apadPattern = [4]byte{0x87, 0x8F, 0xE1, 0xF3}

// fillApad writes the Apad pattern (0x87 0x8F 0xE1 0xF3, repeated) across
// buf[start:end]. RFC 5310 sec 3.3 step 1 (sign) and sec 3.5 (verify) BOTH fill
// the Authentication Data field with Apad before computing the HMAC digest, so a
// generic-crypto digest is taken over Apad-filled bytes, not zeroed bytes. The
// digest length for every RFC 5310 algorithm is a multiple of 4 (16/20/28/32/48/64
// after Key ID is excluded), so the pattern tiles exactly; the modulo indexing is
// defensive against any partial region.
func fillApad(buf []byte, start, end int) {
	for i := start; i < end; i++ {
		buf[i] = apadPattern[(i-start)%4]
	}
}

// authTypeFor maps an algorithm to its on-wire TLV 10 authentication type byte
// (RFC 5304 sec 2: HMAC-MD5 = 54; RFC 5310 sec 3.1: generic crypto = 3;
// ISO/IEC 10589: cleartext = 1).
func authTypeFor(a AuthAlgorithm) (uint8, bool) {
	switch a {
	case AuthAlgoCleartext:
		return AuthTypeCleartext, true
	case AuthAlgoHMACMD5:
		return AuthTypeHMACMD5, true
	case AuthAlgoHMACSHA1, AuthAlgoHMACSHA224, AuthAlgoHMACSHA256, AuthAlgoHMACSHA384, AuthAlgoHMACSHA512:
		return AuthTypeGenericCrypto, true
	default:
		return 0, false
	}
}

// digestLen returns the on-wire authentication-value length (octets) for an
// HMAC algorithm. Cleartext has no fixed length (the value is the password).
func digestLen(a AuthAlgorithm) int {
	switch a {
	case AuthAlgoHMACMD5:
		return md5.Size // 16 (RFC 5304 sec 2)
	case AuthAlgoHMACSHA1:
		return sha1.Size // 20
	case AuthAlgoHMACSHA224:
		return sha256.Size224 // 28
	case AuthAlgoHMACSHA256:
		return sha256.Size // 32
	case AuthAlgoHMACSHA384:
		return sha512.Size384 // 48
	case AuthAlgoHMACSHA512:
		return sha512.Size // 64
	default:
		return 0
	}
}

// newHash returns a fresh hash.Hash constructor for an HMAC-SHA family algorithm
// (RFC 5310 sec 1: algorithm agility over the SHA family), or nil for a
// non-HMAC-SHA algorithm.
func newHash(a AuthAlgorithm) func() hash.Hash {
	switch a {
	case AuthAlgoHMACMD5:
		return md5.New //nolint:gosec // RFC 5304 HMAC-MD5
	case AuthAlgoHMACSHA1:
		return sha1.New //nolint:gosec // RFC 5310 HMAC-SHA-1
	case AuthAlgoHMACSHA224:
		return sha256.New224
	case AuthAlgoHMACSHA256:
		return sha256.New
	case AuthAlgoHMACSHA384:
		return sha512.New384
	case AuthAlgoHMACSHA512:
		return sha512.New
	default:
		return nil
	}
}

// keyIDOctets returns the number of Key-ID octets the algorithm carries in the
// TLV 10 value AFTER the auth-type byte. The generic-crypto family (type 3,
// RFC 5310 sec 3.1) carries a 2-octet Key ID before the authentication data;
// cleartext and HMAC-MD5 carry none.
func keyIDOctets(a AuthAlgorithm) int {
	if t, ok := authTypeFor(a); ok && t == AuthTypeGenericCrypto {
		return 2
	}
	return 0
}

// authValue builds the TLV 10 value bytes (the part AFTER the auth-type octet)
// for a key whose authentication value is provided in valueOrDigest. For the
// generic-crypto family it prepends the 2-octet Key ID (RFC 5310 sec 3.1). The
// auth-type octet itself is written by writeAuthTLV.
func authValue(key Key, valueOrDigest []byte) []byte {
	if keyIDOctets(key.Algorithm) == 2 {
		out := make([]byte, 0, 2+len(valueOrDigest))
		out = append(out, byte(key.KeyID>>8), byte(key.KeyID))
		out = append(out, valueOrDigest...)
		return out
	}
	return valueOrDigest
}

// placeholderValue returns a zero-filled TLV 10 value of the correct length for
// a key, used so the PDU is encoded at its FINAL size before the digest is
// computed (the digest covers the PDU with the Authentication Value zeroed,
// RFC 5304 sec 2; encoding at the final size keeps every other field at its
// final offset). For cleartext there is no digest: the value is the password.
func placeholderValue(key Key) ([]byte, error) {
	t, ok := authTypeFor(key.Algorithm)
	if !ok {
		return nil, ErrAuthUnsupported
	}
	if t == AuthTypeCleartext {
		// Cleartext carries the password directly (sanity only, not security).
		return append([]byte(nil), key.Secret...), nil
	}
	n := digestLen(key.Algorithm)
	if n == 0 {
		return nil, ErrAuthUnsupported
	}
	return authValue(key, make([]byte, n)), nil
}

// pduClass distinguishes the field-zeroing rules and the position of TLV 10
// within an encoded PDU.
type pduClass uint8

const (
	classHello pduClass = iota // IIH (LAN/P2P): zero only the Authentication Value
	classLSP                   // LSP: also zero Checksum + Remaining Lifetime, then Fletcher
	classSNP                   // CSNP/PSNP: zero only the Authentication Value
)

// classOf returns the PDU class for a PDU type, or ok=false for an unknown type.
func classOf(pt PDUType) (pduClass, bool) {
	switch pt {
	case PDUTypeL1LANHello, PDUTypeL2LANHello, PDUTypeP2PHello:
		return classHello, true
	case PDUTypeL1LSP, PDUTypeL2LSP:
		return classLSP, true
	case PDUTypeL1CSNP, PDUTypeL2CSNP, PDUTypeL1PSNP, PDUTypeL2PSNP:
		return classSNP, true
	default:
		return 0, false
	}
}
