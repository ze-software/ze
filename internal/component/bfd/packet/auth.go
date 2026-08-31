// Design: rfc/short/rfc5880.md -- Authentication Section (Section 4.2-4.4)
// Related: control.go -- 24-byte mandatory section codec
//
// Authentication section parser. The Auth Type and Auth Len fields are
// always parsed; the variable-length type-specific body is left as a byte
// slice for the caller to validate.
//
// This file parses the section header only. The type-specific check --
// the keyed digest, or the Simple Password comparison -- runs in
// internal/component/bfd/auth against the session keys.
package packet

import "errors"

// Auth Type values from RFC 5880 Section 4.1.
const (
	AuthTypeReserved            uint8 = 0
	AuthTypeSimplePassword      uint8 = 1
	AuthTypeKeyedMD5            uint8 = 2
	AuthTypeMeticulousKeyedMD5  uint8 = 3
	AuthTypeKeyedSHA1           uint8 = 4
	AuthTypeMeticulousKeyedSHA1 uint8 = 5
)

// Length constants for the authentication section. The keyed variants
// have one fixed section length each. Simple Password sizes its section
// from the password it carries, so it has a range and a header length
// instead (RFC 5880 Section 4.2: "For Simple Password authentication,
// the length is equal to the password length plus three").
const (
	AuthHeaderLen            = 2  // Auth Type + Auth Len bytes
	AuthLenKeyedMD5          = 24 // Type+Len+KeyID+Reserved+Seq(4)+Digest(16)
	AuthLenKeyedSHA1         = 28 // Type+Len+KeyID+Reserved+Seq(4)+Digest(20)
	AuthLenSimplePasswordMin = 4  // Type+Len+KeyID+Password(>=1)
	AuthLenSimplePasswordMax = 19 // Type+Len+KeyID+Password(<=16)

	// SimplePasswordHeaderLen is the Type+Len+KeyID prefix that sits in
	// front of the password bytes.
	SimplePasswordHeaderLen = 3

	// SimplePasswordLenMin and SimplePasswordLenMax bound the password
	// itself. RFC 5880 Section 4.2 states it: "The password is a binary
	// string, and MUST be from 1 to 16 bytes in length".
	SimplePasswordLenMin = 1
	SimplePasswordLenMax = 16
)

// AuthHeader is the parsed two-byte authentication-section header. The
// type-specific body is exposed as a byte slice; callers verify it against
// their session keys.
type AuthHeader struct {
	Type uint8
	Len  uint8
	Body []byte // Length = Len - AuthHeaderLen, aliasing the input buffer.
}

// Errors returned by ParseAuth.
var (
	ErrAuthShort       = errors.New("bfd: authentication section shorter than 2 bytes")
	ErrAuthLenInvalid  = errors.New("bfd: authentication length below header size")
	ErrAuthLenOverflow = errors.New("bfd: authentication length exceeds available data")
)

// ParseAuth parses the authentication section beginning at the start of
// data. data MUST point at the first byte of the auth section -- typically
// the byte after the mandatory section, i.e. data[MandatoryLen:].
//
// ParseAuth returns the parsed header (with Body aliasing the input buffer)
// and an error. It does NOT validate the authentication digest, the
// password, the key ID, or the sequence number; those checks belong to the
// session's Verifier in internal/component/bfd/auth.
//
// ParseAuth never allocates.
func ParseAuth(data []byte) (AuthHeader, error) {
	if len(data) < AuthHeaderLen {
		return AuthHeader{}, ErrAuthShort
	}
	h := AuthHeader{
		Type: data[0],
		Len:  data[1],
	}
	if h.Len < AuthHeaderLen {
		return h, ErrAuthLenInvalid
	}
	if int(h.Len) > len(data) {
		return h, ErrAuthLenOverflow
	}
	h.Body = data[AuthHeaderLen:h.Len]
	return h, nil
}
