// VALIDATES: RFC 5880 Section 6.7.3 transmit and receipt rules for Keyed MD5
// and Meticulous Keyed MD5 that the summary once declared only through the
// Simple Password rows of Section 6.7.2: the Auth Key ID written on transmit,
// and the Auth Type, Auth Key ID and Auth Len discards on receipt. The
// Auth Type discard also covers Keyed SHA1 and Meticulous Keyed SHA1
// (Section 6.7.4) for the packet that carries no Authentication Section.
// PREVENTS: an MD5 signer that leaves a stale Key ID in a reused buffer, and
// an MD5 verifier that accepts a packet with no section, a foreign Auth Type,
// an unconfigured Key ID or an Auth Len other than 24.
//
// Method: every packet comes from the production signer, and every negative
// changes one field of an otherwise authentic packet, so the rule under test
// is the only one that can refuse it. No discarded packet may move the
// replay floor.
package auth

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// rfc5880MD5Types is both MD5 variants of RFC 5880 Section 6.7.3.
var rfc5880MD5Types = []uint8{packet.AuthTypeKeyedMD5, packet.AuthTypeMeticulousKeyedMD5}

// RFC requirement: RFC5880-6.7.3-15 positive -- for Keyed MD5 and Meticulous
// Keyed MD5, Sign writes the configured key ID (0, 1, 200 and 255) into the
// Auth Key ID field, and a verifier configured with that key accepts the
// packet.
func TestRFC5880MD5AuthKeyIDIsTheCurrentKey(t *testing.T) {
	for _, at := range rfc5880MD5Types {
		for _, id := range []uint8{0, 1, 200, 255} {
			cfg := Settings{Type: at, KeyID: id, Secret: rfc5880Secret}
			buf, c, _ := rfc5880Signed(t, cfg, 8)
			if got := buf[packet.MandatoryLen+2]; got != id {
				t.Fatalf("type %d: Auth Key ID field = %d, want the configured %d", at, got, id)
			}
			var state SeqState
			if err := rfc5880Verifier(t, cfg).Verify(buf, c, &state); err != nil {
				t.Fatalf("type %d, key id %d: verify: %v", at, id, err)
			}
		}
	}
}

// RFC requirement: RFC5880-6.7.3-15 negative -- no receiver can know the
// sender's current key, so the input is forced toward a violation instead:
// the section of a reused buffer is filled with 0xFF, a stale Key ID among
// it, before Sign runs. For both MD5 variants the field afterwards holds the
// configured key ID, not the stale byte.
func TestRFC5880MD5AuthKeyIDOverwritesStaleBuffer(t *testing.T) {
	for _, at := range rfc5880MD5Types {
		cfg := Settings{Type: at, KeyID: 7, Secret: rfc5880Secret}
		signer, err := NewSigner(cfg)
		if err != nil {
			t.Fatalf("type %d: NewSigner: %v", at, err)
		}
		buf := make([]byte, packet.MandatoryLen+signer.BodyLen())
		c := rfc5880Control(buf, signer.BodyLen(), 3)
		for i := packet.MandatoryLen; i < len(buf); i++ {
			buf[i] = 0xFF
		}
		signer.Sign(buf, packet.MandatoryLen, 8)
		if got := buf[packet.MandatoryLen+2]; got != 7 {
			t.Fatalf("type %d: Auth Key ID field = %#x after Sign over a stale buffer, want 7", at, got)
		}
		var state SeqState
		if err := rfc5880Verifier(t, cfg).Verify(buf, c, &state); err != nil {
			t.Fatalf("type %d: verify: %v", at, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.3-16 positive -- a packet that carries an
// Authentication Section whose Auth Type is the configured one (2 for Keyed
// MD5, 3 for Meticulous Keyed MD5) is accepted.
func TestRFC5880MD5AuthTypeCorrectAccepted(t *testing.T) {
	for _, at := range rfc5880MD5Types {
		cfg := Settings{Type: at, KeyID: 1, Secret: rfc5880Secret}
		buf, c, _ := rfc5880Signed(t, cfg, 12)
		if buf[packet.MandatoryLen] != at {
			t.Fatalf("type %d: Auth Type byte = %d", at, buf[packet.MandatoryLen])
		}
		var state SeqState
		if err := rfc5880Verifier(t, cfg).Verify(buf, c, &state); err != nil {
			t.Fatalf("type %d: correct Auth Type discarded: %v", at, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.3-16 negative -- for both MD5 variants, a
// packet with no Authentication Section (A bit clear, Length 24) is discarded
// with ErrShortAuthBody, and a section whose Auth Type is 0, 1, 4, 5, 6, 255
// or the other MD5 variant is discarded with ErrDigestMismatch. No discarded
// packet sets the replay floor.
// RFC requirement: RFC5880-6.7.4-14 negative -- the same two discards for
// Keyed SHA1 (4) and Meticulous Keyed SHA1 (5): no section, or an Auth Type
// of 0, 1, 2, 3, 6, 255 or the other SHA1 variant.
func TestRFC5880KeyedMissingSectionOrWrongTypeDiscarded(t *testing.T) {
	for _, at := range rfc5880KeyedTypes {
		cfg := Settings{Type: at, KeyID: 1, Secret: rfc5880Secret}
		v := rfc5880Verifier(t, cfg)

		bare := make([]byte, packet.MandatoryLen)
		bc := packet.Control{
			Version:               packet.Version,
			State:                 packet.StateDown,
			DetectMult:            3,
			Length:                packet.MandatoryLen,
			MyDiscriminator:       0x01020304,
			DesiredMinTxInterval:  1_000_000,
			RequiredMinRxInterval: 1_000_000,
		}
		bc.WriteTo(bare, 0)
		var s0 SeqState
		if err := v.Verify(bare, bc, &s0); !errors.Is(err, ErrShortAuthBody) {
			t.Fatalf("type %d: packet without an Authentication Section: got %v, want ErrShortAuthBody", at, err)
		}
		if s0.Initialized() {
			t.Fatalf("type %d: replay floor set by a packet without an Authentication Section", at)
		}

		buf, c, _ := rfc5880Signed(t, cfg, 12)
		for other := range 256 {
			wrong := uint8(other)
			if wrong == at {
				continue
			}
			if !rfc5880SampledAuthType(wrong) {
				continue
			}
			forged := bytes.Clone(buf)
			forged[packet.MandatoryLen] = wrong
			var state SeqState
			if err := v.Verify(forged, c, &state); !errors.Is(err, ErrDigestMismatch) {
				t.Fatalf("type %d session offered Auth Type %d: got %v, want ErrDigestMismatch", at, wrong, err)
			}
			if state.Initialized() {
				t.Fatalf("type %d session: replay floor set by Auth Type %d", at, wrong)
			}
		}
	}
}

// rfc5880SampledAuthType reports whether the wrong-type sweep offers authType:
// every defined type, the first undefined one, and the top of the byte.
func rfc5880SampledAuthType(authType uint8) bool {
	switch authType {
	case 0, 1, 2, 3, 4, 5, 6, 255:
		return true
	}
	return false
}

// RFC requirement: RFC5880-6.7.3-17 positive -- for both MD5 variants a
// packet whose Auth Key ID matches the configured authentication key is
// accepted.
func TestRFC5880MD5ConfiguredKeyIDAccepted(t *testing.T) {
	for _, at := range rfc5880MD5Types {
		cfg := Settings{Type: at, KeyID: 3, Secret: rfc5880Secret}
		buf, c, _ := rfc5880Signed(t, cfg, 8)
		var state SeqState
		if err := rfc5880Verifier(t, cfg).Verify(buf, c, &state); err != nil {
			t.Fatalf("type %d: packet with the configured key id discarded: %v", at, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.3-17 negative -- for both MD5 variants a
// packet whose Auth Key ID is 2, 4 or 255 while key 3 is configured is
// discarded with ErrDigestMismatch, and the replay floor stays unset.
func TestRFC5880MD5UnconfiguredKeyIDDiscarded(t *testing.T) {
	for _, at := range rfc5880MD5Types {
		cfg := Settings{Type: at, KeyID: 3, Secret: rfc5880Secret}
		buf, c, _ := rfc5880Signed(t, cfg, 8)
		v := rfc5880Verifier(t, cfg)
		for _, id := range []uint8{2, 4, 255} {
			forged := bytes.Clone(buf)
			forged[packet.MandatoryLen+2] = id
			var state SeqState
			if err := v.Verify(forged, c, &state); !errors.Is(err, ErrDigestMismatch) {
				t.Fatalf("type %d: Auth Key ID %d: got %v, want ErrDigestMismatch", at, id, err)
			}
			if state.Initialized() {
				t.Fatalf("type %d: replay floor set by unconfigured Auth Key ID %d", at, id)
			}
		}
	}
}

// RFC requirement: RFC5880-6.7.3-18 positive -- for both MD5 variants a
// section whose Auth Len is 24, in a packet whose Length is 24 + 24, is
// accepted.
func TestRFC5880MD5AuthLen24Accepted(t *testing.T) {
	for _, at := range rfc5880MD5Types {
		cfg := Settings{Type: at, KeyID: 1, Secret: rfc5880Secret}
		buf, c, _ := rfc5880Signed(t, cfg, 30)
		if buf[packet.MandatoryLen+1] != 24 {
			t.Fatalf("type %d: Auth Len byte = %d, want 24", at, buf[packet.MandatoryLen+1])
		}
		if c.Length != packet.MandatoryLen+24 {
			t.Fatalf("type %d: Control Length = %d, want %d", at, c.Length, packet.MandatoryLen+24)
		}
		var state SeqState
		if err := rfc5880Verifier(t, cfg).Verify(buf, c, &state); err != nil {
			t.Fatalf("type %d: Auth Len 24 discarded: %v", at, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.3-18 negative -- for both MD5 variants an Auth
// Len byte of 0, 23, 25 or 28 is discarded, and so is a Control Length one
// short of 24 + 24 (ErrShortAuthBody) or one past it. No discarded packet
// sets the replay floor.
func TestRFC5880MD5AuthLenNot24Discarded(t *testing.T) {
	for _, at := range rfc5880MD5Types {
		cfg := Settings{Type: at, KeyID: 1, Secret: rfc5880Secret}
		buf, c, _ := rfc5880Signed(t, cfg, 30)
		v := rfc5880Verifier(t, cfg)
		for _, authLen := range []uint8{0, 23, 25, 28} {
			forged := bytes.Clone(buf)
			forged[packet.MandatoryLen+1] = authLen
			var state SeqState
			if err := v.Verify(forged, c, &state); !errors.Is(err, ErrDigestMismatch) {
				t.Fatalf("type %d: Auth Len %d: got %v, want ErrDigestMismatch", at, authLen, err)
			}
			if state.Initialized() {
				t.Fatalf("type %d: replay floor set by Auth Len %d", at, authLen)
			}
		}

		short := c
		short.Length = packet.MandatoryLen + 23
		var s1 SeqState
		if err := v.Verify(buf, short, &s1); !errors.Is(err, ErrShortAuthBody) {
			t.Fatalf("type %d: Control Length 47: got %v, want ErrShortAuthBody", at, err)
		}

		long := c
		long.Length = packet.MandatoryLen + 25
		longBuf := make([]byte, len(buf)+1)
		copy(longBuf, buf)
		var s2 SeqState
		if err := v.Verify(longBuf, long, &s2); err == nil {
			t.Fatalf("type %d: Control Length 49 accepted", at)
		}
		if s1.Initialized() || s2.Initialized() {
			t.Fatalf("type %d: replay floor set by a packet of the wrong length", at)
		}
	}
}
