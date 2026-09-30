// VALIDATES: RFC 5880 Section 6.7.4 "If the Auth Len field is not equal to
// 28, the packet MUST be discarded." for Keyed SHA1 and Meticulous Keyed SHA1,
// with the Auth Len byte check isolated from the digest check.
// PREVENTS: a SHA1 verifier that trusts the section's own Auth Len byte, which
// the older TestRFC5880AuthLenMismatchDiscarded cannot catch: its forged byte
// sits inside the hashed span, so the digest refuses the packet whether or not
// the Auth Len check exists.
//
// Method: each packet is built by hand and hashed over its forged Auth Len
// byte with the correct key, so it is authentic in every field except Auth
// Len. Only the Auth Len check can refuse it, and a refusal that came later
// than the replay step would have seeded bfd.RcvAuthSeq.
package auth

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// rfc5880SHA1Types is both SHA1 variants of RFC 5880 Section 6.7.4.
var rfc5880SHA1Types = []uint8{packet.AuthTypeKeyedSHA1, packet.AuthTypeMeticulousKeyedSHA1}

// rfc5880SHA1WithAuthLen returns a SHA1 packet whose Auth Len byte is authLen
// while the section and the Control Length keep the 28-byte shape, and whose
// digest is computed with the key over that byte, so the digest verifies.
func rfc5880SHA1WithAuthLen(authType, authLen uint8, seq uint32) ([]byte, packet.Control) {
	buf, c := rfc5880HandBuilt(authType, 1, rfc5880Secret, seq, 0)
	off := packet.MandatoryLen
	buf[off+1] = authLen
	copy(buf[off+8:], make([]byte, packet.AuthLenKeyedSHA1-8))
	copy(buf[off+8:], rfc5880Secret)
	copy(buf[off+8:], rfc5880Sum(authType, buf))
	return buf, c
}

// RFC requirement: RFC5880-6.7.4-16 positive -- the hand-built packet with
// Auth Len 28 is accepted by a Keyed SHA1 and a Meticulous Keyed SHA1
// verifier, which shows the builder below produces an authentic packet, so
// the negative's refusal comes from the Auth Len byte alone.
func TestRFC5880SHA1AuthLen28HandBuiltAccepted(t *testing.T) {
	for _, at := range rfc5880SHA1Types {
		buf, c := rfc5880SHA1WithAuthLen(at, packet.AuthLenKeyedSHA1, 30)
		var state SeqState
		v := rfc5880Verifier(t, Settings{Type: at, KeyID: 1, Secret: rfc5880Secret})
		if err := v.Verify(buf, c, &state); err != nil {
			t.Fatalf("type %d: authentic Auth Len 28 packet discarded: %v", at, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.4-16 negative -- a Keyed SHA1 or Meticulous
// Keyed SHA1 packet whose digest is correct for its bytes but whose Auth Len
// byte is 0, 20, 24, 27, 29 or 255 is discarded with ErrDigestMismatch, and the
// discard leaves bfd.RcvAuthSeq unseeded, so the packet was refused before
// the replay step and not by the digest comparison.
func TestRFC5880SHA1AuthLenByteDiscardedWithAuthenticDigest(t *testing.T) {
	for _, at := range rfc5880SHA1Types {
		v := rfc5880Verifier(t, Settings{Type: at, KeyID: 1, Secret: rfc5880Secret})
		for _, authLen := range []uint8{0, 20, 24, 27, 29, 255} {
			buf, c := rfc5880SHA1WithAuthLen(at, authLen, 30)
			var state SeqState
			if err := v.Verify(buf, c, &state); !errors.Is(err, ErrDigestMismatch) {
				t.Fatalf("type %d: Auth Len %d with an authentic digest: got %v, want ErrDigestMismatch", at, authLen, err)
			}
			if state.Initialized() {
				t.Fatalf("type %d: Auth Len %d seeded bfd.RcvAuthSeq to %d", at, authLen, state.Last())
			}
		}
	}
}
