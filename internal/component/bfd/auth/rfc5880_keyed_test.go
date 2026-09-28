// VALIDATES: RFC 5880 Section 6.7 authentication clause by clause: the
// Reserved byte on transmit, both SHA1 types as distinct enforced types, the
// Simple Password transmit and reception rules, the Keyed and Meticulous
// Keyed replay windows as a function of the RECEIVED Detect Mult, the digest
// span over the entire packet, and the bfd.AuthSeqKnown seeding for Keyed MD5.
// PREVENTS: a verifier whose window is a hard-coded constant, an MD5 or SHA1
// helper that returns a value both sides agree on without being the real
// digest, a digest over part of the packet, and a Simple Password verifier
// that lets a wrong Auth Type or Auth Len through.
//
// Method: every keyed expectation is computed here with crypto/md5 and
// crypto/sha1, independently of digestSigner, and every negative changes one
// field of an otherwise authentic packet, so the rule under test is the only
// one that can refuse it.
package auth

import (
	"bytes"
	"crypto/md5"  //nolint:gosec // RFC 5880 Section 6.7.3 names MD5
	"crypto/sha1" //nolint:gosec // RFC 5880 Section 6.7.4 names SHA1
	"encoding/binary"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// rfc5880KeyedTypes lists the four keyed Auth Types, MD5 before SHA1.
var rfc5880KeyedTypes = []uint8{
	packet.AuthTypeKeyedMD5,
	packet.AuthTypeMeticulousKeyedMD5,
	packet.AuthTypeKeyedSHA1,
	packet.AuthTypeMeticulousKeyedSHA1,
}

// rfc5880DetectMults are the received Detect Mult values the window tests
// drive: 1 and 5 bracket the 3 every other test uses, and 255 is the field
// maximum.
var rfc5880DetectMults = []uint8{1, 5, 255}

// rfc5880IsMD5 reports whether authType is one of the two MD5 types.
func rfc5880IsMD5(authType uint8) bool {
	if authType == packet.AuthTypeKeyedMD5 {
		return true
	}
	return authType == packet.AuthTypeMeticulousKeyedMD5
}

// rfc5880SectionLen returns the Auth Len RFC 5880 fixes for a keyed type:
// 24 for MD5 (Section 6.7.3) and 28 for SHA1 (Section 6.7.4).
func rfc5880SectionLen(authType uint8) int {
	if rfc5880IsMD5(authType) {
		return packet.AuthLenKeyedMD5
	}
	return packet.AuthLenKeyedSHA1
}

// rfc5880Sum computes the digest for authType with the standard library,
// never through md5Sum or sha1Sum.
func rfc5880Sum(authType uint8, data []byte) []byte {
	if rfc5880IsMD5(authType) {
		h := md5.Sum(data) //nolint:gosec // RFC 5880 Section 6.7.3 names MD5
		return h[:]
	}
	h := sha1.Sum(data) //nolint:gosec // RFC 5880 Section 6.7.4 names SHA1
	return h[:]
}

// rfc5880Control writes a Down-state mandatory section carrying detectMult
// into buf and returns the Control the verifier reads.
func rfc5880Control(buf []byte, sectionLen int, detectMult uint8) packet.Control {
	c := packet.Control{
		Version:               packet.Version,
		Diag:                  packet.DiagNone,
		State:                 packet.StateDown,
		Auth:                  true,
		DetectMult:            detectMult,
		Length:                uint8(packet.MandatoryLen + sectionLen),
		MyDiscriminator:       0x01020304,
		DesiredMinTxInterval:  1_000_000,
		RequiredMinRxInterval: 1_000_000,
	}
	c.WriteTo(buf, 0)
	return c
}

// rfc5880SignedMult signs a packet for cfg carrying seq and detectMult with
// the production signer.
func rfc5880SignedMult(t *testing.T, cfg Settings, seq uint32, detectMult uint8) ([]byte, packet.Control) {
	t.Helper()
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner(type %d): %v", cfg.Type, err)
	}
	buf := make([]byte, packet.MandatoryLen+signer.BodyLen())
	c := rfc5880Control(buf, signer.BodyLen(), detectMult)
	signer.Sign(buf, packet.MandatoryLen, seq)
	return buf, c
}

// rfc5880HandBuilt builds a keyed packet without digestSigner. It writes the
// section fields, places key in the Auth Key/Digest field padded with zero
// bytes, hashes data[:hashEnd] with the standard library, and stores the
// result in the field. hashEnd equal to the packet length is the compliant
// span; a shorter hashEnd is a peer that hashed less than the entire packet.
func rfc5880HandBuilt(authType, keyID uint8, key []byte, seq uint32, hashEnd int) ([]byte, packet.Control) {
	sectionLen := rfc5880SectionLen(authType)
	buf := make([]byte, packet.MandatoryLen+sectionLen)
	c := rfc5880Control(buf, sectionLen, 3)
	off := packet.MandatoryLen
	buf[off] = authType
	buf[off+1] = byte(sectionLen)
	buf[off+2] = keyID
	binary.BigEndian.PutUint32(buf[off+4:], seq)
	copy(buf[off+8:], key)
	digest := rfc5880Sum(authType, buf[:hashEnd])
	copy(buf[off+8:], digest)
	return buf, c
}

// RFC requirement: RFC5880-4.3-1 positive -- the Reserved byte of a Keyed MD5,
// Meticulous Keyed MD5, Keyed SHA1 and Meticulous Keyed SHA1 section is zero
// after Sign, although the buffer Sign writes into was filled with 0xFF, so
// the zero is written by the signer and not left over from the allocation.
func TestRFC5880ReservedByteZeroedOnTransmit(t *testing.T) {
	for _, at := range rfc5880KeyedTypes {
		signer, err := NewSigner(Settings{Type: at, KeyID: 6, Secret: rfc5880Secret})
		if err != nil {
			t.Fatalf("type %d: NewSigner: %v", at, err)
		}
		buf := bytes.Repeat([]byte{0xFF}, packet.MandatoryLen+signer.BodyLen())
		rfc5880Control(buf, signer.BodyLen(), 3)
		signer.Sign(buf, packet.MandatoryLen, 9)
		if got := buf[packet.MandatoryLen+3]; got != 0 {
			t.Fatalf("type %d: Reserved byte = %#x on transmit, want 0", at, got)
		}
	}
}

// RFC requirement: RFC5880-6.7-1 negative -- each SHA1 type is enforced as
// itself: for Keyed SHA1 and Meticulous Keyed SHA1 a packet whose hash byte is
// flipped is discarded with ErrDigestMismatch, a section of the other SHA1
// type is discarded, and a repeated Sequence Number is accepted by Keyed SHA1
// but discarded by Meticulous Keyed SHA1 with ErrSequenceOutsideWindow, so
// neither type is served by the other's code.
func TestRFC5880SHA1TypesEnforcedDistinctly(t *testing.T) {
	sha1Types := []uint8{packet.AuthTypeKeyedSHA1, packet.AuthTypeMeticulousKeyedSHA1}
	for i, at := range sha1Types {
		other := sha1Types[1-i]
		cfg := Settings{Type: at, KeyID: 4, Secret: rfc5880Secret}
		v := rfc5880Verifier(t, cfg)

		flipped, fc := rfc5880SignedMult(t, cfg, 21, 3)
		flipped[len(flipped)-1] ^= 0xFF
		var s1 SeqState
		if err := v.Verify(flipped, fc, &s1); !errors.Is(err, ErrDigestMismatch) {
			t.Fatalf("type %d: flipped hash: got %v, want ErrDigestMismatch", at, err)
		}

		foreign, oc := rfc5880SignedMult(t, Settings{Type: other, KeyID: 4, Secret: rfc5880Secret}, 21, 3)
		var s2 SeqState
		if err := v.Verify(foreign, oc, &s2); !errors.Is(err, ErrDigestMismatch) {
			t.Fatalf("type %d: section of type %d: got %v, want ErrDigestMismatch", at, other, err)
		}

		var s3 SeqState
		first, c1 := rfc5880SignedMult(t, cfg, 30, 3)
		if err := v.Verify(first, c1, &s3); err != nil {
			t.Fatalf("type %d: first packet: %v", at, err)
		}
		repeat, c2 := rfc5880SignedMult(t, cfg, 30, 3)
		err := v.Verify(repeat, c2, &s3)
		if at == packet.AuthTypeKeyedSHA1 && err != nil {
			t.Fatalf("Keyed SHA1 discarded a repeated sequence: %v", err)
		}
		if at == packet.AuthTypeMeticulousKeyedSHA1 && !errors.Is(err, ErrSequenceOutsideWindow) {
			t.Fatalf("Meticulous Keyed SHA1 repeated sequence: got %v, want ErrSequenceOutsideWindow", err)
		}
	}
}

// RFC requirement: RFC5880-6.7.2-3 negative -- no Simple Password session is
// built without a password to store: NewSigner refuses a nil and an empty
// password with ErrKeyLengthInvalid.
func TestRFC5880SimplePasswordSignerNeedsPassword(t *testing.T) {
	for _, password := range [][]byte{nil, {}} {
		cfg := Settings{Type: packet.AuthTypeSimplePassword, KeyID: 42, Secret: password}
		if _, err := NewSigner(cfg); !errors.Is(err, ErrKeyLengthInvalid) {
			t.Fatalf("NewSigner with a %d-byte password: got %v, want ErrKeyLengthInvalid", len(password), err)
		}
	}
}

// RFC requirement: RFC5880-6.7.2-4 positive -- a packet that contains an
// Authentication Section (A bit set, Length above 24) whose Auth Type is 1 is
// accepted by simpleVerifier.Verify, for 1, 8 and 16 byte passwords.
func TestRFC5880SimplePasswordSectionOfTypeOneAccepted(t *testing.T) {
	for _, octets := range []int{1, 8, 16} {
		password := bytes.Repeat([]byte{'s'}, octets)
		buf, c := rfc5880SimpleSigned(t, 5, password)
		if !c.Auth || int(c.Length) <= packet.MandatoryLen || buf[packet.MandatoryLen] != packet.AuthTypeSimplePassword {
			t.Fatalf("%d-byte password: precondition: a section of Auth Type 1 must be present", octets)
		}
		v := rfc5880Verifier(t, Settings{Type: packet.AuthTypeSimplePassword, KeyID: 5, Secret: password})
		if err := v.Verify(buf, c, nil); err != nil {
			t.Fatalf("%d-byte password: section of Auth Type 1 discarded: %v", octets, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.2-4 negative -- simpleVerifier.Verify discards
// a packet that carries no Authentication Section (A bit clear, Length 24)
// with ErrShortAuthBody, and discards a section whose Auth Type is 0, 2, 3,
// 4, 5, 6 or 255, with the Key ID, Auth Len and password still correct, with
// ErrPasswordMismatch.
func TestRFC5880SimplePasswordMissingSectionOrOtherTypeDiscarded(t *testing.T) {
	v := rfc5880Verifier(t, Settings{Type: packet.AuthTypeSimplePassword, KeyID: 5, Secret: rfc5880Password})

	bare := make([]byte, packet.MandatoryLen)
	c := packet.Control{
		Version:               packet.Version,
		State:                 packet.StateDown,
		DetectMult:            3,
		Length:                packet.MandatoryLen,
		MyDiscriminator:       0x01020304,
		DesiredMinTxInterval:  1_000_000,
		RequiredMinRxInterval: 1_000_000,
	}
	c.WriteTo(bare, 0)
	if err := v.Verify(bare, c, nil); !errors.Is(err, ErrShortAuthBody) {
		t.Fatalf("packet without an Authentication Section: got %v, want ErrShortAuthBody", err)
	}

	buf, sc := rfc5880SimpleSigned(t, 5, rfc5880Password)
	for _, at := range []uint8{0, 2, 3, 4, 5, 6, 255} {
		other := bytes.Clone(buf)
		other[packet.MandatoryLen] = at
		if err := v.Verify(other, sc, nil); !errors.Is(err, ErrPasswordMismatch) {
			t.Fatalf("Auth Type %d offered to a Simple Password session: got %v, want ErrPasswordMismatch", at, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.2-6 positive -- a section whose Auth Len equals
// the configured password length plus three is accepted by
// simpleVerifier.Verify, for 1, 8 and 16 byte passwords.
func TestRFC5880SimplePasswordAuthLenPlusThreeAccepted(t *testing.T) {
	for _, octets := range []int{1, 8, 16} {
		password := bytes.Repeat([]byte{'l'}, octets)
		buf, c := rfc5880SimpleSigned(t, 2, password)
		if got := int(buf[packet.MandatoryLen+1]); got != octets+3 {
			t.Fatalf("%d-byte password: precondition: Auth Len = %d, want %d", octets, got, octets+3)
		}
		v := rfc5880Verifier(t, Settings{Type: packet.AuthTypeSimplePassword, KeyID: 2, Secret: password})
		if err := v.Verify(buf, c, nil); err != nil {
			t.Fatalf("%d-byte password: Auth Len %d discarded: %v", octets, octets+3, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.2-6 negative -- a section whose Auth Len is not
// the configured password length plus three (0, plus two, plus four, 255) is
// discarded by simpleVerifier.Verify with ErrPasswordMismatch, for 1, 8 and 16
// byte passwords, while the Auth Type, Key ID, Control Length and password
// stay correct.
func TestRFC5880SimplePasswordAuthLenNotPlusThreeDiscarded(t *testing.T) {
	for _, octets := range []int{1, 8, 16} {
		password := bytes.Repeat([]byte{'l'}, octets)
		buf, c := rfc5880SimpleSigned(t, 2, password)
		v := rfc5880Verifier(t, Settings{Type: packet.AuthTypeSimplePassword, KeyID: 2, Secret: password})
		for _, authLen := range []int{0, octets + 2, octets + 4, 255} {
			forged := bytes.Clone(buf)
			forged[packet.MandatoryLen+1] = byte(authLen)
			if err := v.Verify(forged, c, nil); !errors.Is(err, ErrPasswordMismatch) {
				t.Fatalf("%d-byte password: Auth Len %d: got %v, want ErrPasswordMismatch", octets, authLen, err)
			}
		}
	}
}

// RFC requirement: RFC5880-6.7.2-8 negative -- no Simple Password section
// outside 4 to 19 bytes is built: NewSigner refuses the 0-byte and the
// 17-byte password, whose sections would carry Auth Len 3 and 20, with
// ErrKeyLengthInvalid.
func TestRFC5880SimplePasswordSectionOutsideProperLengthNotBuilt(t *testing.T) {
	for _, octets := range []int{0, packet.SimplePasswordLenMax + 1} {
		cfg := Settings{Type: packet.AuthTypeSimplePassword, KeyID: 3, Secret: bytes.Repeat([]byte{'p'}, octets)}
		if _, err := NewSigner(cfg); !errors.Is(err, ErrKeyLengthInvalid) {
			t.Fatalf("NewSigner with a %d-byte password (Auth Len %d): got %v, want ErrKeyLengthInvalid",
				octets, octets+packet.SimplePasswordHeaderLen, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.3-1 negative -- no Keyed MD5 or Meticulous
// Keyed MD5 section other than the 24-byte one is built: NewSigner refuses a
// key of 17, 20 and 32 bytes, which does not fit the 16-byte Auth Key/Digest
// field of a 24-byte section, with ErrKeyLengthInvalid.
func TestRFC5880KeyedMD5OversizedKeyNotSigned(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeKeyedMD5, packet.AuthTypeMeticulousKeyedMD5} {
		for _, octets := range []int{packet.KeyedMD5KeyLenMax + 1, 20, 32} {
			cfg := Settings{Type: at, KeyID: 6, Secret: bytes.Repeat([]byte{'k'}, octets)}
			if _, err := NewSigner(cfg); !errors.Is(err, ErrKeyLengthInvalid) {
				t.Fatalf("type %d: NewSigner with a %d-byte key: got %v, want ErrKeyLengthInvalid", at, octets, err)
			}
		}
	}
}

// RFC requirement: RFC5880-6.7.4-1 negative -- no Keyed SHA1 or Meticulous
// Keyed SHA1 section other than the 28-byte one is built: NewSigner refuses a
// key of 21 and 32 bytes, which does not fit the 20-byte Auth Key/Hash field
// of a 28-byte section, with ErrKeyLengthInvalid.
func TestRFC5880KeyedSHA1OversizedKeyNotSigned(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeKeyedSHA1, packet.AuthTypeMeticulousKeyedSHA1} {
		for _, octets := range []int{packet.KeyedSHA1KeyLenMax + 1, 32} {
			cfg := Settings{Type: at, KeyID: 11, Secret: bytes.Repeat([]byte{'k'}, octets)}
			if _, err := NewSigner(cfg); !errors.Is(err, ErrKeyLengthInvalid) {
				t.Fatalf("type %d: NewSigner with a %d-byte key: got %v, want ErrKeyLengthInvalid", at, octets, err)
			}
		}
	}
}

// RFC requirement: RFC5880-6.7.3-9 positive -- for Keyed MD5 (and Keyed SHA1)
// a Sequence Number equal to bfd.RcvAuthSeq and one exactly 3 * Detect Mult
// ahead of it are accepted, with Detect Mult read from the received packet
// and driven at 1, 5 and 255; across the 32-bit wrap, 3 * Detect Mult ahead
// of 0xFFFFFFFE is accepted too. Each accepted packet moves bfd.RcvAuthSeq to
// its sequence.
func TestRFC5880KeyedWindowFollowsReceivedDetectMult(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeKeyedMD5, packet.AuthTypeKeyedSHA1} {
		cfg := Settings{Type: at, KeyID: 1, Secret: rfc5880Secret}
		v := rfc5880Verifier(t, cfg)
		for _, mult := range rfc5880DetectMults {
			span := 3 * uint32(mult)
			var state SeqState
			for _, seq := range []uint32{1000, 1000, 1000 + span} {
				buf, c := rfc5880SignedMult(t, cfg, seq, mult)
				if err := v.Verify(buf, c, &state); err != nil {
					t.Fatalf("type %d, Detect Mult %d: sequence %d with bfd.RcvAuthSeq %d: %v", at, mult, seq, state.Last(), err)
				}
			}
			if state.Last() != 1000+span {
				t.Fatalf("type %d, Detect Mult %d: bfd.RcvAuthSeq = %d, want %d", at, mult, state.Last(), 1000+span)
			}

			var wrap SeqState
			const floor uint32 = 0xFFFFFFFE
			for _, seq := range []uint32{floor, floor + span} {
				buf, c := rfc5880SignedMult(t, cfg, seq, mult)
				if err := v.Verify(buf, c, &wrap); err != nil {
					t.Fatalf("type %d, Detect Mult %d: sequence %#x after %#x: %v", at, mult, seq, floor, err)
				}
			}
			if wrap.Last() != floor+span {
				t.Fatalf("type %d, Detect Mult %d: bfd.RcvAuthSeq = %#x, want %#x", at, mult, wrap.Last(), floor+span)
			}
		}
	}
}

// RFC requirement: RFC5880-6.7.3-9 negative -- for Keyed MD5 (and Keyed SHA1)
// a Sequence Number one below bfd.RcvAuthSeq, one past bfd.RcvAuthSeq + 3 *
// Detect Mult, and one half the circular space away is discarded with
// ErrSequenceOutsideWindow, with Detect Mult read from the received packet and
// driven at 1, 5 and 255; 0xFFFFFFFF behind a floor of 2 across the wrap is
// discarded too. No discarded packet moves bfd.RcvAuthSeq.
func TestRFC5880KeyedOutsideWindowDiscarded(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeKeyedMD5, packet.AuthTypeKeyedSHA1} {
		cfg := Settings{Type: at, KeyID: 1, Secret: rfc5880Secret}
		v := rfc5880Verifier(t, cfg)
		for _, mult := range rfc5880DetectMults {
			span := 3 * uint32(mult)
			rfc5880RequireOutside(t, v, cfg, mult, 1000, []uint32{999, 1000 + span + 1, 1000 + 0x80000000})
			rfc5880RequireOutside(t, v, cfg, mult, 2, []uint32{0xFFFFFFFF})
		}
	}
}

// rfc5880RequireOutside seeds a fresh bfd.RcvAuthSeq at floor with an
// authentic packet, then requires every sequence in outside to be discarded
// with ErrSequenceOutsideWindow and the floor to stay put.
func rfc5880RequireOutside(t *testing.T, v Verifier, cfg Settings, mult uint8, floor uint32, outside []uint32) {
	t.Helper()
	var state SeqState
	buf, c := rfc5880SignedMult(t, cfg, floor, mult)
	if err := v.Verify(buf, c, &state); err != nil {
		t.Fatalf("type %d, Detect Mult %d: floor packet %d: %v", cfg.Type, mult, floor, err)
	}
	for _, seq := range outside {
		out, oc := rfc5880SignedMult(t, cfg, seq, mult)
		if err := v.Verify(out, oc, &state); !errors.Is(err, ErrSequenceOutsideWindow) {
			t.Fatalf("type %d, Detect Mult %d: sequence %#x with bfd.RcvAuthSeq %#x: got %v, want ErrSequenceOutsideWindow",
				cfg.Type, mult, seq, floor, err)
		}
	}
	if state.Last() != floor {
		t.Fatalf("type %d, Detect Mult %d: bfd.RcvAuthSeq moved to %#x on discarded packets, want %#x", cfg.Type, mult, state.Last(), floor)
	}
}

// RFC requirement: RFC5880-6.7.3-10 positive -- for Meticulous Keyed MD5 (and
// Meticulous Keyed SHA1) bfd.RcvAuthSeq+1 and bfd.RcvAuthSeq + 3 * Detect Mult
// are accepted, with Detect Mult read from the received packet and driven at
// 1, 5 and 255; across the 32-bit wrap, 0 after 0xFFFFFFFF and 3 * Detect
// Mult ahead of 0xFFFFFFFE are accepted too.
func TestRFC5880MeticulousWindowFollowsReceivedDetectMult(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeMeticulousKeyedMD5, packet.AuthTypeMeticulousKeyedSHA1} {
		cfg := Settings{Type: at, KeyID: 1, Secret: rfc5880Secret}
		v := rfc5880Verifier(t, cfg)
		for _, mult := range rfc5880DetectMults {
			span := 3 * uint32(mult)
			runs := [][]uint32{
				{1000, 1001, 1001 + span},
				{0xFFFFFFFF, 0},
				{0xFFFFFFFE, 0xFFFFFFFE + span},
			}
			for _, run := range runs {
				var state SeqState
				for _, seq := range run {
					buf, c := rfc5880SignedMult(t, cfg, seq, mult)
					if err := v.Verify(buf, c, &state); err != nil {
						t.Fatalf("type %d, Detect Mult %d: sequence %#x with bfd.RcvAuthSeq %#x: %v", at, mult, seq, state.Last(), err)
					}
				}
				if want := run[len(run)-1]; state.Last() != want {
					t.Fatalf("type %d, Detect Mult %d: bfd.RcvAuthSeq = %#x, want %#x", at, mult, state.Last(), want)
				}
			}
		}
	}
}

// RFC requirement: RFC5880-6.7.3-10 negative -- for Meticulous Keyed MD5 (and
// Meticulous Keyed SHA1) a Sequence Number equal to bfd.RcvAuthSeq, one below
// it, and one past bfd.RcvAuthSeq + 3 * Detect Mult is discarded with
// ErrSequenceOutsideWindow, with Detect Mult read from the received packet and
// driven at 1, 5 and 255; 0xFFFFFFFF behind a floor of 2 across the wrap is
// discarded too. No discarded packet moves bfd.RcvAuthSeq.
func TestRFC5880MeticulousOutsideWindowDiscarded(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeMeticulousKeyedMD5, packet.AuthTypeMeticulousKeyedSHA1} {
		cfg := Settings{Type: at, KeyID: 1, Secret: rfc5880Secret}
		v := rfc5880Verifier(t, cfg)
		for _, mult := range rfc5880DetectMults {
			span := 3 * uint32(mult)
			rfc5880RequireOutside(t, v, cfg, mult, 1000, []uint32{1000, 999, 1000 + span + 1})
			rfc5880RequireOutside(t, v, cfg, mult, 2, []uint32{0xFFFFFFFF})
		}
	}
}

// RFC requirement: RFC5880-6.7.3-11 positive -- for Keyed MD5 and Meticulous
// Keyed MD5 a packet built here, with crypto/md5 over the entire packet and
// the key in the Auth Key/Digest field, is accepted by the verifier, and
// bfd.RcvAuthSeq then holds its sequence.
func TestRFC5880KeyedMD5IndependentDigestAccepted(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeKeyedMD5, packet.AuthTypeMeticulousKeyedMD5} {
		v := rfc5880Verifier(t, Settings{Type: at, KeyID: 7, Secret: rfc5880Secret})
		buf, c := rfc5880HandBuilt(at, 7, rfc5880Secret, 44, packet.MandatoryLen+packet.AuthLenKeyedMD5)
		var state SeqState
		if err := v.Verify(buf, c, &state); err != nil {
			t.Fatalf("type %d: packet carrying the MD5 digest of the entire packet discarded: %v", at, err)
		}
		if state.Last() != 44 {
			t.Fatalf("type %d: bfd.RcvAuthSeq = %d, want 44", at, state.Last())
		}
	}
}

// RFC requirement: RFC5880-6.7.3-11 negative -- for Keyed MD5 and Meticulous
// Keyed MD5, once an authentic packet set bfd.RcvAuthSeq to 40, a packet whose
// digest byte is flipped and a packet whose MD5 digest was computed here with
// a different key are each discarded with ErrDigestMismatch, and neither moves
// bfd.RcvAuthSeq.
func TestRFC5880KeyedMD5DigestMismatchDiscarded(t *testing.T) {
	const packetLen = packet.MandatoryLen + packet.AuthLenKeyedMD5
	for _, at := range []uint8{packet.AuthTypeKeyedMD5, packet.AuthTypeMeticulousKeyedMD5} {
		v := rfc5880Verifier(t, Settings{Type: at, KeyID: 7, Secret: rfc5880Secret})
		var state SeqState
		floor, fc := rfc5880HandBuilt(at, 7, rfc5880Secret, 40, packetLen)
		if err := v.Verify(floor, fc, &state); err != nil {
			t.Fatalf("type %d: authentic packet setting the floor: %v", at, err)
		}

		flipped, c1 := rfc5880HandBuilt(at, 7, rfc5880Secret, 41, packetLen)
		flipped[packetLen-1] ^= 0xFF
		if err := v.Verify(flipped, c1, &state); !errors.Is(err, ErrDigestMismatch) {
			t.Fatalf("type %d: flipped digest: got %v, want ErrDigestMismatch", at, err)
		}

		otherKey, c2 := rfc5880HandBuilt(at, 7, []byte("another-md5-key"), 42, packetLen)
		if err := v.Verify(otherKey, c2, &state); !errors.Is(err, ErrDigestMismatch) {
			t.Fatalf("type %d: digest under another key: got %v, want ErrDigestMismatch", at, err)
		}
		if state.Last() != 40 {
			t.Fatalf("type %d: bfd.RcvAuthSeq moved to %d on discarded packets, want 40", at, state.Last())
		}
	}
}

// RFC requirement: RFC5880-6.7.3-2 positive -- for Keyed MD5 and Meticulous
// Keyed MD5 the digest Sign transmits equals crypto/md5 computed here over
// the entire packet, from byte 0 to the Control Length, with the key padded
// into the Auth Key/Digest field; a packet built that way is accepted.
// RFC requirement: RFC5880-6.7.4-2 positive -- for Keyed SHA1 and Meticulous
// Keyed SHA1 the hash Sign transmits equals crypto/sha1 computed here over the
// entire packet the same way, and a packet built that way is accepted.
func TestRFC5880DigestSpansEntirePacket(t *testing.T) {
	for _, at := range rfc5880KeyedTypes {
		cfg := Settings{Type: at, KeyID: 2, Secret: rfc5880Secret}
		buf, c := rfc5880SignedMult(t, cfg, 77, 3)
		field := packet.MandatoryLen + 8
		slot := rfc5880SectionLen(at) - 8

		keyed := bytes.Clone(buf[:c.Length])
		padded := make([]byte, slot)
		copy(padded, rfc5880Secret)
		copy(keyed[field:field+slot], padded)
		if want := rfc5880Sum(at, keyed); !bytes.Equal(buf[field:field+slot], want) {
			t.Fatalf("type %d: transmitted digest %x, want %x over the entire packet", at, buf[field:field+slot], want)
		}

		built, bc := rfc5880HandBuilt(at, 2, rfc5880Secret, 77, int(c.Length))
		var state SeqState
		if err := rfc5880Verifier(t, cfg).Verify(built, bc, &state); err != nil {
			t.Fatalf("type %d: packet hashed over its entire length discarded: %v", at, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.3-2 negative -- for Keyed MD5 and Meticulous
// Keyed MD5 a packet whose digest was computed over less than the entire
// packet (the mandatory section only, up to the Sequence Number, up to the
// Auth Key/Digest field, all but the last byte) is discarded with
// ErrDigestMismatch, and so is a signed packet with any one byte flipped among
// the 24 mandatory bytes, the Reserved byte and the four Sequence Number bytes.
// RFC requirement: RFC5880-6.7.4-2 negative -- the same partial hashes and the
// same single-byte changes are discarded for Keyed SHA1 and Meticulous Keyed
// SHA1.
func TestRFC5880DigestOverPartialPacketDiscarded(t *testing.T) {
	off := packet.MandatoryLen
	for _, at := range rfc5880KeyedTypes {
		cfg := Settings{Type: at, KeyID: 2, Secret: rfc5880Secret}
		v := rfc5880Verifier(t, cfg)
		packetLen := off + rfc5880SectionLen(at)

		for _, hashEnd := range []int{off, off + 4, off + 8, packetLen - 1} {
			partial, pc := rfc5880HandBuilt(at, 2, rfc5880Secret, 77, hashEnd)
			var state SeqState
			if err := v.Verify(partial, pc, &state); !errors.Is(err, ErrDigestMismatch) {
				t.Fatalf("type %d: digest over the first %d of %d bytes: got %v, want ErrDigestMismatch", at, hashEnd, packetLen, err)
			}
		}

		buf, c := rfc5880SignedMult(t, cfg, 77, 3)
		for idx := range off + 8 {
			if idx >= off && idx < off+3 {
				continue // Auth Type, Auth Len and Key ID: refused by their own checks first.
			}
			tampered := bytes.Clone(buf)
			tampered[idx] ^= 0xFF
			var state SeqState
			if err := v.Verify(tampered, c, &state); !errors.Is(err, ErrDigestMismatch) {
				t.Fatalf("type %d: byte %d flipped after signing: got %v, want ErrDigestMismatch", at, idx, err)
			}
		}
	}
}

// RFC requirement: RFC5880-6.7.3-12 positive -- for Keyed MD5 and Meticulous
// Keyed MD5, while bfd.AuthSeqKnown is 0 the first packet sets it to 1 and
// sets bfd.RcvAuthSeq to the received Sequence Number, whether the digest then
// matches or not (the step precedes the digest comparison, owner decision
// D-14, 2026-09-27).
func TestRFC5880KeyedMD5FirstPacketSeedsReplayFloor(t *testing.T) {
	const packetLen = packet.MandatoryLen + packet.AuthLenKeyedMD5
	for _, at := range []uint8{packet.AuthTypeKeyedMD5, packet.AuthTypeMeticulousKeyedMD5} {
		v := rfc5880Verifier(t, Settings{Type: at, KeyID: 1, Secret: rfc5880Secret})

		authentic, ac := rfc5880HandBuilt(at, 1, rfc5880Secret, 0xDEAD00, packetLen)
		var s1 SeqState
		if err := v.Verify(authentic, ac, &s1); err != nil {
			t.Fatalf("type %d: authentic first packet: %v", at, err)
		}
		if !s1.Initialized() || s1.Last() != 0xDEAD00 {
			t.Fatalf("type %d: after an authentic first packet bfd.AuthSeqKnown %v, bfd.RcvAuthSeq %#x, want true and 0xdead00",
				at, s1.Initialized(), s1.Last())
		}

		forged, fc := rfc5880HandBuilt(at, 1, rfc5880Secret, 0x7000, packetLen)
		forged[packetLen-1] ^= 0xFF
		var s2 SeqState
		if err := v.Verify(forged, fc, &s2); !errors.Is(err, ErrDigestMismatch) {
			t.Fatalf("type %d: forged first packet: got %v, want ErrDigestMismatch", at, err)
		}
		if !s2.Initialized() || s2.Last() != 0x7000 {
			t.Fatalf("type %d: after a forged first packet bfd.AuthSeqKnown %v, bfd.RcvAuthSeq %#x, want true and 0x7000",
				at, s2.Initialized(), s2.Last())
		}
	}
}

// RFC requirement: RFC5880-6.7.3-12 negative -- for Keyed MD5 and Meticulous
// Keyed MD5 the seeding runs only while bfd.AuthSeqKnown is 0: once an
// authentic packet set bfd.RcvAuthSeq to 100, a forged packet inside the
// window is discarded with ErrDigestMismatch and a packet half the circular
// space away is discarded with ErrSequenceOutsideWindow, and bfd.RcvAuthSeq
// stays 100 after both.
func TestRFC5880KeyedMD5KnownFloorNotReseeded(t *testing.T) {
	const packetLen = packet.MandatoryLen + packet.AuthLenKeyedMD5
	for _, at := range []uint8{packet.AuthTypeKeyedMD5, packet.AuthTypeMeticulousKeyedMD5} {
		v := rfc5880Verifier(t, Settings{Type: at, KeyID: 1, Secret: rfc5880Secret})
		var state SeqState
		first, c0 := rfc5880HandBuilt(at, 1, rfc5880Secret, 100, packetLen)
		if err := v.Verify(first, c0, &state); err != nil {
			t.Fatalf("type %d: authentic first packet: %v", at, err)
		}

		forged, c1 := rfc5880HandBuilt(at, 1, rfc5880Secret, 105, packetLen)
		forged[packetLen-1] ^= 0xFF
		if err := v.Verify(forged, c1, &state); !errors.Is(err, ErrDigestMismatch) {
			t.Fatalf("type %d: forged in-window packet: got %v, want ErrDigestMismatch", at, err)
		}
		far, c2 := rfc5880HandBuilt(at, 1, rfc5880Secret, 100+0x80000000, packetLen)
		if err := v.Verify(far, c2, &state); !errors.Is(err, ErrSequenceOutsideWindow) {
			t.Fatalf("type %d: packet half the space away: got %v, want ErrSequenceOutsideWindow", at, err)
		}
		if state.Last() != 100 {
			t.Fatalf("type %d: bfd.RcvAuthSeq = %d with bfd.AuthSeqKnown 1, want 100", at, state.Last())
		}
	}
}
