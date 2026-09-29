// VALIDATES: the RFC 5880 Section 6.7 authentication fields that no digest
// can confound. A Simple Password receiver refuses a packet whose Key ID or
// password is not the selected pair (Section 6.7.2), and a Keyed MD5 or Keyed
// SHA1 receiver refuses a packet whose Auth Type or Auth Len carries the other
// algorithm's value even when the digest was honestly computed over that
// altered section (Sections 6.7.3 and 6.7.4).
// PREVENTS: a verifier that authenticates the password or digest alone and
// lets the Key ID, Auth Type or Auth Len through unchecked.
//
// Method: every negative is an otherwise authentic packet. The Simple
// Password section has no digest, so a changed Key ID or password byte is the
// only difference. The keyed packets are built by hand and hashed with
// crypto/md5 or crypto/sha1 AFTER the field is altered, so the digest step
// accepts them and only the field check can refuse them.
package auth

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// rfc5880SelectedPassword is the password every Simple Password case selects.
var rfc5880SelectedPassword = []byte("selected-pw")

// rfc5880KeyedAltered builds a keyed packet for a verifier of authType whose
// section carries wireType and wireLen in place of the values RFC 5880 fixes
// for authType. The section length, the Control Length and the digest
// algorithm follow authType, and the digest is computed with the standard
// library over the packet as altered, with key in the digest field.
func rfc5880KeyedAltered(authType, wireType, wireLen, keyID uint8, key []byte) ([]byte, packet.Control) {
	sectionLen := rfc5880SectionLen(authType)
	buf := make([]byte, packet.MandatoryLen+sectionLen)
	c := rfc5880Control(buf, sectionLen, 3)
	off := packet.MandatoryLen
	buf[off] = wireType
	buf[off+1] = wireLen
	buf[off+2] = keyID
	binary.BigEndian.PutUint32(buf[off+4:], 7)
	copy(buf[off+8:], key)
	digest := rfc5880Sum(authType, buf)
	copy(buf[off+8:], digest)
	return buf, c
}

// rfc5880OtherAlgorithm returns the Auth Type and Auth Len of the other
// digest algorithm with the same replay mode: Keyed MD5 pairs with Keyed
// SHA1, and Meticulous Keyed MD5 with Meticulous Keyed SHA1.
func rfc5880OtherAlgorithm(authType uint8) (otherType, otherLen uint8) {
	switch authType {
	case packet.AuthTypeKeyedMD5:
		return packet.AuthTypeKeyedSHA1, packet.AuthLenKeyedSHA1
	case packet.AuthTypeMeticulousKeyedMD5:
		return packet.AuthTypeMeticulousKeyedSHA1, packet.AuthLenKeyedSHA1
	case packet.AuthTypeKeyedSHA1:
		return packet.AuthTypeKeyedMD5, packet.AuthLenKeyedMD5
	}
	return packet.AuthTypeMeticulousKeyedMD5, packet.AuthLenKeyedMD5
}

// RFC requirement: RFC5880-6.7.2-3 negative -- simpleVerifier.Verify, holding
// Key ID 5 and a selected password, discards with ErrPasswordMismatch a Simple
// Password packet whose Key ID is 4, 6 or 255 with the password unchanged, and
// one whose password differs from the selected one in its first, a middle or
// its last byte with Key ID 5 unchanged; the same verifier accepts the packet
// that carries the selected pair.
func TestRFC5880SimplePasswordOtherThanSelectedPairDiscarded(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeSimplePassword, KeyID: 5, Secret: rfc5880SelectedPassword}
	v := rfc5880Verifier(t, cfg)

	selected, c := rfc5880SimpleSigned(t, 5, rfc5880SelectedPassword)
	if err := v.Verify(selected, c, nil); err != nil {
		t.Fatalf("packet carrying the selected pair discarded: %v", err)
	}

	for _, keyID := range []uint8{4, 6, 255} {
		buf, other := rfc5880SimpleSigned(t, keyID, rfc5880SelectedPassword)
		if err := v.Verify(buf, other, nil); !errors.Is(err, ErrPasswordMismatch) {
			t.Fatalf("Key ID %d with the selected password: got %v, want ErrPasswordMismatch", keyID, err)
		}
	}

	last := len(rfc5880SelectedPassword) - 1
	for _, at := range []int{0, last / 2, last} {
		password := append([]byte(nil), rfc5880SelectedPassword...)
		password[at] ^= 0x01
		buf, other := rfc5880SimpleSigned(t, 5, password)
		if err := v.Verify(buf, other, nil); !errors.Is(err, ErrPasswordMismatch) {
			t.Fatalf("password byte %d changed, Key ID 5: got %v, want ErrPasswordMismatch", at, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.3-1 negative -- a Keyed MD5 and a Meticulous
// Keyed MD5 verifier discard with ErrDigestMismatch a packet whose MD5 digest
// was computed over the section with Auth Type set to the SHA1 value (4 or 5)
// and Auth Len 24, and one computed over Auth Type 2 or 3 with Auth Len set to
// 28; the packet built the same way with Auth Type 2 or 3 and Auth Len 24 is
// accepted.
func TestRFC5880KeyedMD5OtherAlgorithmFieldsDiscarded(t *testing.T) {
	rfc5880RequireOtherAlgorithmFieldsDiscarded(t, packet.AuthTypeKeyedMD5)
	rfc5880RequireOtherAlgorithmFieldsDiscarded(t, packet.AuthTypeMeticulousKeyedMD5)
}

// RFC requirement: RFC5880-6.7.4-1 negative -- a Keyed SHA1 and a Meticulous
// Keyed SHA1 verifier discard with ErrDigestMismatch a packet whose SHA1 hash
// was computed over the section with Auth Type set to the MD5 value (2 or 3)
// and Auth Len 28, and one computed over Auth Type 4 or 5 with Auth Len set to
// 24; the packet built the same way with Auth Type 4 or 5 and Auth Len 28 is
// accepted.
func TestRFC5880KeyedSHA1OtherAlgorithmFieldsDiscarded(t *testing.T) {
	rfc5880RequireOtherAlgorithmFieldsDiscarded(t, packet.AuthTypeKeyedSHA1)
	rfc5880RequireOtherAlgorithmFieldsDiscarded(t, packet.AuthTypeMeticulousKeyedSHA1)
}

// rfc5880RequireOtherAlgorithmFieldsDiscarded runs one keyed type through the
// accepted control and the two altered packets. Each packet uses a fresh
// replay state, so the sequence window cannot be what refuses it.
func rfc5880RequireOtherAlgorithmFieldsDiscarded(t *testing.T, authType uint8) {
	t.Helper()
	key := []byte("rfc5880-field-key")[:packet.AuthLenKeyedMD5-8]
	cfg := Settings{Type: authType, KeyID: 3, Secret: key}
	v := rfc5880Verifier(t, cfg)
	ownLen := uint8(rfc5880SectionLen(authType))
	otherType, otherLen := rfc5880OtherAlgorithm(authType)

	buf, c := rfc5880KeyedAltered(authType, authType, ownLen, 3, key)
	if err := v.Verify(buf, c, &SeqState{}); err != nil {
		t.Fatalf("type %d: packet with Auth Type %d and Auth Len %d discarded: %v", authType, authType, ownLen, err)
	}

	buf, c = rfc5880KeyedAltered(authType, otherType, ownLen, 3, key)
	if err := v.Verify(buf, c, &SeqState{}); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("type %d verifier, Auth Type %d hashed in: got %v, want ErrDigestMismatch", authType, otherType, err)
	}

	buf, c = rfc5880KeyedAltered(authType, authType, otherLen, 3, key)
	if err := v.Verify(buf, c, &SeqState{}); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("type %d verifier, Auth Len %d hashed in: got %v, want ErrDigestMismatch", authType, otherLen, err)
	}
}
