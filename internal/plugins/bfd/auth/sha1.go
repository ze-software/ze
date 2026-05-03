// Design: rfc/short/rfc5880.md -- Keyed / Meticulous Keyed SHA1 (Section 6.7.4)
//
// Keyed SHA1 and Meticulous Keyed SHA1 signer + verifier, plus the
// generic digestSigner / digestVerifier helpers both this file and
// md5.go build on. The body layout from RFC 5880 §6.7.3 and §6.7.4
// is identical across Keyed MD5, Meticulous Keyed MD5, Keyed SHA1
// and Meticulous Keyed SHA1 -- only the digest hash and its output
// size change.
//
// Layout of an auth section (24 bytes for MD5, 28 for SHA1):
//
//	0:  Auth Type
//	1:  Auth Len (equal to BodyLen, 24 or 28)
//	2:  Auth Key ID
//	3:  Reserved (0)
//	4..7: Sequence Number (big-endian)
//	8..:  Digest (16 bytes MD5 or 20 bytes SHA1)
//
// The digest is computed over the BFD Control packet up to and
// including the auth section with the digest bytes replaced by the
// secret key (zero-padded or truncated to the digest length). On
// verify, the same substitution is performed on a scratch copy of
// the packet before the hash is re-computed and compared in constant
// time.
package auth

import (
	"crypto/sha1" //nolint:gosec // RFC 5880 §6.7.4 mandates SHA1; not used for general cryptographic integrity
	"crypto/subtle"
	"encoding/binary"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// digestFunc abstracts over crypto/sha1.Sum and crypto/md5.Sum. The
// return is a fresh slice so callers do not need to know the fixed
// array size of the specific algorithm.
type digestFunc func(data []byte) []byte

// digestSigner is the shared Signer implementation for the four
// Keyed / Meticulous Keyed variants. bodyLen and digest vary per
// algorithm; the auth-section layout is fixed.
type digestSigner struct {
	authType uint8
	keyID    uint8
	bodyLen  int
	key      []byte
	digest   digestFunc
}

// newDigestSigner builds a signer from the configuration and the
// algorithm-specific parameters. The secret is copied into a
// bodyLen-sized scratch so sign time never touches the caller's
// original byte slice.
func newDigestSigner(cfg Settings, bodyLen int, digest digestFunc) *digestSigner {
	keySize := bodyLen - 8 // body = Type+Len+KeyID+Rsvd+Seq(4) + Digest(keySize)
	key := make([]byte, keySize)
	copy(key, cfg.Secret)
	return &digestSigner{
		authType: cfg.Type,
		keyID:    cfg.KeyID,
		bodyLen:  bodyLen,
		key:      key,
		digest:   digest,
	}
}

// AuthType reports the RFC 5880 Auth Type this signer emits.
func (s *digestSigner) AuthType() uint8 { return s.authType }

// BodyLen reports the total length of the auth section.
func (s *digestSigner) BodyLen() int { return s.bodyLen }

// Sign writes the full auth section at buf[off:off+s.bodyLen] and
// computes the digest over buf[0:off+s.bodyLen] with the digest
// bytes pre-filled with s.key (RFC 5880 §6.7.3 / §6.7.4). The
// caller MUST have written the mandatory BFD Control section into
// buf[0:packet.MandatoryLen] first so the hash covers it.
func (s *digestSigner) Sign(buf []byte, off int, seq uint32) int {
	buf[off+0] = s.authType
	buf[off+1] = byte(s.bodyLen)
	buf[off+2] = s.keyID
	buf[off+3] = 0
	binary.BigEndian.PutUint32(buf[off+4:], seq)
	copy(buf[off+8:off+s.bodyLen], s.key)
	h := s.digest(buf[0 : off+s.bodyLen])
	copy(buf[off+8:off+s.bodyLen], h)
	return s.bodyLen
}

// digestVerifier mirrors digestSigner on the receive side.
//
// BFD runs one goroutine per session, so Verify is never called
// concurrently on a given verifier. That lets us park the two per-call
// scratch buffers (received digest + packet copy with key substituted)
// directly on the struct and reuse them across every Verify invocation
// -- sub-second BFD timers made these two makes the dominant allocation
// source on the receive path.
type digestVerifier struct {
	authType   uint8
	keyID      uint8
	bodyLen    int
	key        []byte
	digest     digestFunc
	meticulous bool

	// received holds the digest extracted from the incoming packet.
	// Length is fixed by the algorithm (16 for MD5, 20 for SHA1 =
	// bodyLen-8) and is allocated once by newDigestVerifier.
	received []byte

	// scratch is reused for the digest-over-packet computation. It is
	// sized at construction to the largest BFD Control + auth section
	// combination (MandatoryLen + bodyLen) which bounds the packet
	// length the verifier ever inspects.
	scratch []byte
}

// newDigestVerifier builds a verifier with the same parameters as
// newDigestSigner plus the replay-protection mode. Pre-allocates the
// two scratch buffers Verify needs so the receive hot path never calls
// `make`.
func newDigestVerifier(cfg Settings, bodyLen int, digest digestFunc, meticulous bool) *digestVerifier {
	keySize := bodyLen - 8
	key := make([]byte, keySize)
	copy(key, cfg.Secret)
	return &digestVerifier{
		authType:   cfg.Type,
		keyID:      cfg.KeyID,
		bodyLen:    bodyLen,
		key:        key,
		digest:     digest,
		meticulous: meticulous,
		received:   make([]byte, keySize),
		scratch:    make([]byte, packet.MandatoryLen+bodyLen),
	}
}

// AuthType reports the expected RFC 5880 Auth Type.
func (v *digestVerifier) AuthType() uint8 { return v.authType }

// Verify runs the RFC 5880 §6.8.6 reception check: length, key-id,
// replay-protection, constant-time digest compare. data MUST span at
// least c.Length bytes; shorter input returns ErrShortAuthBody.
func (v *digestVerifier) Verify(data []byte, c packet.Control, seqState *SeqState) error {
	expectedLen := packet.MandatoryLen + v.bodyLen
	if int(c.Length) < expectedLen {
		return ErrShortAuthBody
	}
	if len(data) < int(c.Length) {
		return ErrShortAuthBody
	}
	// RFC 5880 Section 6.8.7: transmitted Length is the mandatory
	// control section plus the fixed authentication section length.
	if int(c.Length) != expectedLen {
		return ErrDigestMismatch
	}
	off := packet.MandatoryLen
	if data[off+0] != v.authType {
		return ErrDigestMismatch
	}
	if int(data[off+1]) != v.bodyLen {
		return ErrDigestMismatch
	}
	if data[off+2] != v.keyID {
		return ErrDigestMismatch
	}
	seq := binary.BigEndian.Uint32(data[off+4:])
	if err := seqState.Check(seq, v.meticulous); err != nil {
		return err
	}
	// Reuse the pre-allocated per-verifier scratch. v.received is sized
	// bodyLen-8 (the digest length, fixed by algorithm) and v.scratch is
	// sized packet.MandatoryLen+bodyLen (the largest packet this
	// verifier ever inspects). c.Length is bounded by those at parse
	// time, so the copies below stay within cap.
	copy(v.received, data[off+8:off+v.bodyLen])
	scratch := v.scratch[:c.Length]
	copy(scratch, data[:c.Length])
	copy(scratch[off+8:off+v.bodyLen], v.key)
	h := v.digest(scratch)
	if subtle.ConstantTimeCompare(h, v.received) != 1 {
		return ErrDigestMismatch
	}
	seqState.Advance(seq, v.meticulous)
	return nil
}

// sha1Sum is a digestFunc adapter over stdlib sha1.Sum.
func sha1Sum(b []byte) []byte { h := sha1.Sum(b); return h[:] } //nolint:gosec // see file-level comment

// newSHA1Signer builds a digestSigner configured for Keyed SHA1.
func newSHA1Signer(cfg Settings) *digestSigner {
	return newDigestSigner(cfg, packet.AuthLenKeyedSHA1, sha1Sum)
}

// newSHA1Verifier builds a digestVerifier configured for Keyed SHA1.
func newSHA1Verifier(cfg Settings) *digestVerifier {
	meticulous := cfg.Type == packet.AuthTypeMeticulousKeyedSHA1 || cfg.Meticulous
	return newDigestVerifier(cfg, packet.AuthLenKeyedSHA1, sha1Sum, meticulous)
}
