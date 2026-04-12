// Design: rfc/short/rfc5880.md -- BFD authentication (Section 6.7)
//
// Signer and Verifier interfaces for the BFD authentication section.
// A Signer attaches the auth payload to an outgoing packet; a Verifier
// checks the payload on an incoming packet. Both are keyed to a
// specific (key-id, secret) tuple plus an Auth Type.
//
// The split between Signer and Verifier (rather than a single
// "Authenticator" interface) lets test code drop in a pure-function
// signer or verifier without implementing the other half.
package auth

import (
	"errors"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// Errors surfaced by the auth package. Wrapping is done at the call
// site; the sentinel names match the RFC 5880 terminology where
// possible.
var (
	// ErrUnsupportedType is returned when an operator configures an
	// Auth Type the package does not implement. In ze this is
	// Simple Password (1) at the moment; adding future types means
	// adding a new constructor and a switch arm.
	ErrUnsupportedType = errors.New("bfd auth: unsupported auth type")

	// ErrKeyLengthInvalid is returned when the provided secret is
	// outside the range the selected Auth Type accepts.
	ErrKeyLengthInvalid = errors.New("bfd auth: invalid key length")

	// ErrDigestMismatch is returned by Verifier.Verify when the
	// computed digest differs from the one in the packet, or when
	// the received Key ID does not match the configured one.
	ErrDigestMismatch = errors.New("bfd auth: digest mismatch")

	// ErrSequenceRegress is returned when a received packet carries
	// a sequence number below the replay floor. RFC 5880 Section 6.8.1
	// ("RcvAuthSeq") defines the floor per the Meticulous vs
	// non-Meticulous rules.
	ErrSequenceRegress = errors.New("bfd auth: sequence regress")

	// ErrShortAuthBody is returned when the auth section body is
	// shorter than the fixed layout expected for the configured
	// Auth Type.
	ErrShortAuthBody = errors.New("bfd auth: short auth body")
)

// Settings is the operator-facing authentication configuration for one
// BFD session. Type + KeyID + Secret come from the YANG `auth` block;
// Meticulous is computed from the type enum string at parse time.
type Settings struct {
	Type       uint8
	KeyID      uint8
	Secret     []byte //nolint:gosec // Field is a BFD auth key; Settings is never serialized
	Meticulous bool
}

// Signer produces the authentication payload for an outgoing Control
// packet. Implementations are stateful across calls only through a
// monotonic sequence number (tracked by the caller and passed into
// Sign).
type Signer interface {
	// AuthType returns the RFC 5880 Auth Type this signer emits.
	AuthType() uint8

	// BodyLen returns the fixed length of the authentication
	// section payload (including the Auth Type + Auth Len
	// header). The caller uses this to size the outbound buffer
	// and set packet.Control.Length.
	BodyLen() int

	// Sign writes the Type/Len/KeyID/Seq/Digest block into buf at
	// offset off. The caller MUST have written the mandatory BFD
	// Control section into buf[0:packet.MandatoryLen] first so the
	// digest can cover it. seq is the outgoing sequence number;
	// the signer does not advance it (the caller owns the state).
	// Returns the number of bytes written (equal to BodyLen).
	Sign(buf []byte, off int, seq uint32) int
}

// Verifier checks the authentication payload on an incoming Control
// packet. Verify is called after packet.ParseControl has extracted the
// auth header; it receives the full packet bytes so the digest
// computation can run over the whole message with the digest field
// zeroed.
type Verifier interface {
	// AuthType returns the expected RFC 5880 Auth Type.
	AuthType() uint8

	// Verify checks that data[:c.Length] carries a valid auth
	// payload for the configured key. data MUST be at least
	// c.Length bytes long. The seqState argument tracks the
	// receiver-side replay floor (bfd.RcvAuthSeq) and, on success,
	// is advanced according to the Meticulous rules.
	//
	// Returns nil on success, ErrDigestMismatch on bad digest or
	// wrong key-id, ErrSequenceRegress on replay, and
	// ErrShortAuthBody on a truncated section.
	Verify(data []byte, c packet.Control, seqState *SeqState) error
}

// NewSigner returns a Signer for the given configuration or
// ErrUnsupportedType when the Auth Type is not implemented.
func NewSigner(cfg Settings) (Signer, error) {
	switch cfg.Type {
	case packet.AuthTypeKeyedSHA1, packet.AuthTypeMeticulousKeyedSHA1:
		return newSHA1Signer(cfg), nil
	case packet.AuthTypeKeyedMD5, packet.AuthTypeMeticulousKeyedMD5:
		return newMD5Signer(cfg), nil
	}
	return nil, ErrUnsupportedType
}

// NewVerifier returns a Verifier for the given configuration or
// ErrUnsupportedType when the Auth Type is not implemented.
func NewVerifier(cfg Settings) (Verifier, error) {
	switch cfg.Type {
	case packet.AuthTypeKeyedSHA1, packet.AuthTypeMeticulousKeyedSHA1:
		return newSHA1Verifier(cfg), nil
	case packet.AuthTypeKeyedMD5, packet.AuthTypeMeticulousKeyedMD5:
		return newMD5Verifier(cfg), nil
	}
	return nil, ErrUnsupportedType
}
