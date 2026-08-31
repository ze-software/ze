// Design: rfc/short/rfc5880.md -- Simple Password authentication (Section 6.7.2)
// Related: sha1.go -- the keyed-digest signer and verifier this file sits beside
//
// Simple Password signer and verifier. RFC 5880 Section 4.2 lays the
// section out as:
//
//	0:    Auth Type (1, Simple Password)
//	1:    Auth Len (password length plus three, so 4 to 19)
//	2:    Auth Key ID
//	3..:  Password (1 to 16 bytes)
//
// There is no digest and no sequence number, so the password travels in
// clear and the section is identical in every packet. RFC 5880 Section
// 6.7.2 calls this "The most straightforward (and weakest) form of
// authentication": a peer that reads one packet can forge every packet
// after it, and nothing here detects a replay. Ze implements it because
// an implementation that offers the type owes the section's MUSTs, and
// it is offered only when the operator names it in the configuration.
//
// Safe for concurrent use: neither type is mutated after construction,
// and Verify holds no scratch state.
package auth

import (
	"crypto/subtle"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// simplePasswordLenValid reports whether octets is a password length the
// RFC allows.
//
// RFC 5880 Section 6.7.2 states the bound: "The password is a binary
// string, and MUST be 1 to 16 bytes in length".
func simplePasswordLenValid(octets int) bool {
	if octets < packet.SimplePasswordLenMin {
		return false
	}
	return octets <= packet.SimplePasswordLenMax
}

// simpleSigner writes the Simple Password authentication section on the
// transmit path. It carries no sequence number, so Sign ignores the seq
// argument the Signer interface passes.
type simpleSigner struct {
	keyID      uint8
	password   []byte
	sectionLen int
}

// newSimpleSigner builds the signer for cfg. The caller MUST have
// checked the password length with simplePasswordLenValid first, which
// is what bounds sectionLen to the 4 to 19 the RFC allows. The password
// is copied so a later change to cfg.Secret cannot change what goes on
// the wire.
func newSimpleSigner(cfg Settings) *simpleSigner {
	password := make([]byte, len(cfg.Secret))
	copy(password, cfg.Secret)
	sectionLen := packet.SimplePasswordHeaderLen + len(password)
	assertSimpleSectionLen(sectionLen)
	return &simpleSigner{
		keyID:      cfg.KeyID,
		password:   password,
		sectionLen: sectionLen,
	}
}

// assertSimpleSectionLen states the invariant NewSigner and NewVerifier
// establish: the section is inside the range RFC 5880 Section 6.7.2 calls
// "the proper length (4 to 19 bytes)".
//
// It is an assertion rather than an error return, because a caller that
// skipped simplePasswordLenValid is a Ze defect and no peer can reach it.
// The check is paired with that one deliberately: Sign narrows sectionLen
// to a byte, so an unvalidated construction would put a truncated Auth Len
// on the wire and every peer would discard the session in silence.
func assertSimpleSectionLen(sectionLen int) {
	if sectionLen < packet.AuthLenSimplePasswordMin {
		panic("BUG: bfd auth: Simple Password section below the 4-byte minimum")
	}
	if sectionLen > packet.AuthLenSimplePasswordMax {
		panic("BUG: bfd auth: Simple Password section above the 19-byte maximum")
	}
}

// AuthType reports the RFC 5880 Auth Type this signer emits.
func (s *simpleSigner) AuthType() uint8 { return packet.AuthTypeSimplePassword }

// BodyLen reports the total length of the authentication section, which
// is the password length plus the three header bytes.
func (s *simpleSigner) BodyLen() int { return s.sectionLen }

// Sign writes the full authentication section at buf[off:off+BodyLen()]
// and returns the byte count. The caller owns buf and MUST have sized it
// to at least off+BodyLen(). seq is unused: Simple Password carries no
// Sequence Number field.
//
// RFC 5880 Section 6.7.2 states what this writes: "The currently
// selected password and Key ID for the session MUST be stored in the
// Authentication Section of each outgoing BFD Control packet. The Auth
// Type field MUST be set to 1 (Simple Password). The Auth Len field
// MUST be set to the proper length (4 to 19 bytes)".
func (s *simpleSigner) Sign(buf []byte, off int, _ uint32) int {
	buf[off+0] = packet.AuthTypeSimplePassword
	buf[off+1] = byte(s.sectionLen)
	buf[off+2] = s.keyID
	copy(buf[off+packet.SimplePasswordHeaderLen:off+s.sectionLen], s.password)
	return s.sectionLen
}

// simpleVerifier mirrors simpleSigner on the receive side. It holds the
// one Password/Key ID pair the session is configured with, which is the
// set RFC 5880 Section 6.7.2 has the receiver match against.
type simpleVerifier struct {
	keyID      uint8
	password   []byte
	sectionLen int
}

// newSimpleVerifier builds the verifier for cfg under the same password
// length obligation as newSimpleSigner.
func newSimpleVerifier(cfg Settings) *simpleVerifier {
	password := make([]byte, len(cfg.Secret))
	copy(password, cfg.Secret)
	sectionLen := packet.SimplePasswordHeaderLen + len(password)
	assertSimpleSectionLen(sectionLen)
	return &simpleVerifier{
		keyID:      cfg.KeyID,
		password:   password,
		sectionLen: sectionLen,
	}
}

// AuthType reports the expected RFC 5880 Auth Type.
func (v *simpleVerifier) AuthType() uint8 { return packet.AuthTypeSimplePassword }

// Verify runs the RFC 5880 Section 6.7.2 reception rules in the order
// the RFC states them and returns nil only when every one passes. data
// comes off a socket from an unauthenticated peer, so each index below
// is bounded before it is taken and a malformed section returns an error
// rather than panicking.
//
// The SeqState argument is unused. Simple Password carries no Sequence
// Number, so there is no replay floor to check or advance -- which is
// the protection this type does not provide.
func (v *simpleVerifier) Verify(data []byte, c packet.Control, _ *SeqState) error {
	if len(data) < int(c.Length) {
		return ErrShortAuthBody
	}
	// RFC 5880 Section 6.8.6: "If the Length field is less than the
	// minimum correct value (24 if the A bit is clear, or 26 if the A
	// bit is set), the packet MUST be discarded." A Simple Password
	// section is at least four bytes, so this floor is stricter and it
	// is what makes the three header reads below in range.
	if int(c.Length) < packet.MandatoryLen+packet.AuthLenSimplePasswordMin {
		return ErrShortAuthBody
	}
	off := packet.MandatoryLen
	// RFC 5880 Section 6.7.2: "If the received BFD Control packet does
	// not contain an Authentication Section, or the Auth Type is not 1
	// (Simple Password), then the received packet MUST be discarded."
	if data[off+0] != packet.AuthTypeSimplePassword {
		return ErrPasswordMismatch
	}
	// RFC 5880 Section 6.7.2: "If the Auth Key ID field does not match
	// the ID of a configured password, the received packet MUST be
	// discarded."
	if data[off+2] != v.keyID {
		return ErrPasswordMismatch
	}
	// RFC 5880 Section 6.7.2: "If the Auth Len field is not equal to the
	// length of the password selected by the key ID, plus three, the
	// packet MUST be discarded."
	if int(data[off+1]) != v.sectionLen {
		return ErrPasswordMismatch
	}
	// The Control Length must cover the mandatory section and that
	// authentication section exactly. A packet that claims more carries
	// bytes the password was never chosen for, and accepting it would
	// let a peer append whatever it likes behind a matching password.
	if int(c.Length) != off+v.sectionLen {
		return ErrPasswordMismatch
	}
	// RFC 5880 Section 6.7.2: "If the Password field does not match the
	// password selected by the key ID, the packet MUST be discarded."
	received := data[off+packet.SimplePasswordHeaderLen : off+v.sectionLen]
	if subtle.ConstantTimeCompare(received, v.password) != 1 {
		return ErrPasswordMismatch
	}
	// RFC 5880 Section 6.7.2: "Otherwise, the packet MUST be accepted."
	return nil
}
