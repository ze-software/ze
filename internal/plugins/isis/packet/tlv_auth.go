// Design: docs/architecture/wire/isis.md -- TLV 10 (Authentication) structural codec only
// RFC: rfc/short/rfc5304.md -- TLV 10 structure (sec 1): auth-type byte + value; HMAC-MD5 type 54
// RFC: rfc/short/rfc5310.md -- generic crypto / HMAC-SHA auth type 3
//
// This file is a STRUCTURAL codec only. The HMAC sign/verify logic, key store,
// and per-PDU enforcement live in spec-isis-10-auth. Here TLV 10 only needs to
// round-trip (auth-type byte + opaque value) and to have its position surfaced
// (AuthTLVIndex in tlv_opaque.go) so isis-10 can require it first (RFC 5304
// sec 1).

package packet

// IS-IS authentication type codes carried as the first value octet of TLV 10
// (RFC 5304 sec 1 / sec 4, RFC 5310). The codec treats everything after this
// octet as opaque.
const (
	AuthTypeReserved      = 0   // ISO/IEC 10589 (reserved)
	AuthTypeCleartext     = 1   // ISO/IEC 10589: cleartext password
	AuthTypeGenericCrypto = 3   // RFC 5310: generic cryptographic authentication (HMAC-SHA)
	AuthTypeHMACMD5       = 54  // RFC 5304 sec 2 (0x36)
	AuthTypeDomainPrivate = 255 // ISO/IEC 10589: routeing domain private method
)

// AuthTLV is the decoded TLV 10: the 1-octet authentication type followed by an
// opaque authentication value. The codec does not interpret the value (the
// digest / password / key-id structure is isis-10's concern); it only carries
// it so the TLV round-trips and isis-10 can verify/sign over the full PDU.
type AuthTLV struct {
	AuthType uint8
	Value    []byte // opaque; aliases the source buffer on decode
}

// decodeAuthTLV parses a TLV 10 value: the first octet is the authentication
// type, the remainder the opaque authentication value. A zero-length value is
// rejected (ErrLength): TLV 10 must carry at least the type octet (RFC 5304
// sec 1). The codec does not reject unknown auth types or malformed digest
// lengths -- that is enforcement (isis-10) -- but it does require the
// structural minimum so a truncated TLV is not silently accepted as valid
// (security review: "must not silently accept malformed auth TLV structure").
func decodeAuthTLV(value []byte) (AuthTLV, error) {
	if len(value) < 1 {
		return AuthTLV{}, ErrLength
	}
	return AuthTLV{
		AuthType: value[0],
		Value:    value[1:],
	}, nil
}

// valueLen returns the encoded TLV 10 value length (1 type octet + value).
func (t AuthTLV) valueLen() int { return 1 + len(t.Value) }

// writeAuthTLV emits TLV 10 (type+length+value) into buf at off, writing the
// auth-type octet then the opaque value. The caller positions this TLV first in
// the stream when present (RFC 5304 sec 1); the codec does not enforce
// ordering, only enables it. Buffer-first.
func writeAuthTLV(buf []byte, off int, t AuthTLV) int {
	vlen := t.valueLen()
	buf[off] = TLVAuthentication
	buf[off+1] = byte(vlen)
	off += TLVHeaderLen
	buf[off] = t.AuthType
	off++
	off += copy(buf[off:], t.Value)
	return off
}
