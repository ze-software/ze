// VALIDATES: RFC 5880 Section 6.7 authentication -- the Keyed and Meticulous
// Keyed MD5 / SHA1 section layout (type, length, key id, reserved byte,
// sequence number, digest), the digest coverage of the whole Control packet,
// and every receive-side discard rule the verifier enforces.
// PREVENTS: an auth section that leaks the shared secret onto the wire, a
// digest that covers only part of the packet, a verifier that accepts a
// replayed or wrong-key packet, and a replay floor that advances on a packet
// which failed its digest.
package auth

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// rfc5880Secret is the shared key used by every test below. It is longer than
// the MD5 key slot (16 bytes) and shorter than the SHA1 slot (20) so both
// truncation and zero-padding paths are exercised.
var rfc5880Secret = []byte("rfc5880-shared-key")

// rfc5880Signed returns a freshly signed packet for cfg carrying seq, along
// with the parsed Control the verifier expects.
func rfc5880Signed(t *testing.T, cfg Settings, seq uint32) ([]byte, packet.Control, Signer) {
	t.Helper()
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner(type %d): %v", cfg.Type, err)
	}
	buf := make([]byte, packet.MandatoryLen+signer.BodyLen())
	c := controlBytes(buf, signer.BodyLen())
	signer.Sign(buf, packet.MandatoryLen, seq)
	return buf, c, signer
}

// rfc5880Verifier builds a Verifier for cfg or fails the test.
func rfc5880Verifier(t *testing.T, cfg Settings) Verifier {
	t.Helper()
	v, err := NewVerifier(cfg)
	if err != nil {
		t.Fatalf("NewVerifier(type %d): %v", cfg.Type, err)
	}
	return v
}

// RFC requirement: RFC5880-6.7-1 positive -- an implementation that supports
// authentication supports BOTH SHA1 types. NewSigner and NewVerifier
// (internal/component/bfd/auth/signer.go:108,120) both switch on
// AuthTypeKeyedSHA1 and AuthTypeMeticulousKeyedSHA1, and a packet signed with
// either verifies end to end.
func TestRFC5880BothSHA1VariantsSupported(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeKeyedSHA1, packet.AuthTypeMeticulousKeyedSHA1} {
		cfg := Settings{Type: at, KeyID: 4, Secret: rfc5880Secret}
		buf, c, signer := rfc5880Signed(t, cfg, 21)
		if signer.AuthType() != at {
			t.Fatalf("signer AuthType = %d, want %d", signer.AuthType(), at)
		}
		var state SeqState
		if err := rfc5880Verifier(t, cfg).Verify(buf, c, &state); err != nil {
			t.Fatalf("SHA1 variant %d round trip: %v", at, err)
		}
	}
}

// RFC requirement: RFC5880-6.7-1 negative -- support is an enumerated set, not
// a blanket accept: NewSigner and NewVerifier fall through to
// ErrUnsupportedType for the reserved type 0 and for any value RFC 5880
// Section 4.1 leaves undefined, so the SHA1 support above is a real switch arm
// rather than a catch-all.
func TestRFC5880UnsupportedAuthTypesRejected(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeReserved, 6, 200} {
		if _, err := NewSigner(Settings{Type: at, Secret: rfc5880Secret}); !errors.Is(err, ErrUnsupportedType) {
			t.Fatalf("NewSigner type %d: got %v, want ErrUnsupportedType", at, err)
		}
		if _, err := NewVerifier(Settings{Type: at, Secret: rfc5880Secret}); !errors.Is(err, ErrUnsupportedType) {
			t.Fatalf("NewVerifier type %d: got %v, want ErrUnsupportedType", at, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.3-1 positive -- the Keyed MD5 section carries
// Auth Type 2 (or 3 for the Meticulous variant) and Auth Len 24. Sign
// (internal/component/bfd/auth/sha1.go:80-81) writes s.authType and
// byte(s.bodyLen), and newMD5Signer (auth/md5.go) fixes bodyLen at
// packet.AuthLenKeyedMD5 == 24.
// RFC requirement: RFC5880-4.3-1 positive -- the Reserved byte of the section
// is set to zero on transmit: Sign hardcodes buf[off+3] = 0 (sha1.go:83).
func TestRFC5880KeyedMD5SectionHeader(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeKeyedMD5, packet.AuthTypeMeticulousKeyedMD5} {
		cfg := Settings{Type: at, KeyID: 6, Secret: rfc5880Secret}
		buf, _, signer := rfc5880Signed(t, cfg, 9)
		if signer.BodyLen() != packet.AuthLenKeyedMD5 {
			t.Fatalf("MD5 BodyLen = %d, want 24", signer.BodyLen())
		}
		off := packet.MandatoryLen
		if buf[off] != at {
			t.Fatalf("Auth Type byte = %d, want %d", buf[off], at)
		}
		if buf[off+1] != packet.AuthLenKeyedMD5 {
			t.Fatalf("Auth Len byte = %d, want 24", buf[off+1])
		}
		if buf[off+3] != 0 {
			t.Fatalf("Reserved byte = %d, want 0", buf[off+3])
		}
	}
}

// RFC requirement: RFC5880-6.7.3-1 negative -- a Keyed MD5 verifier rejects a
// section that does not carry type 2 and length 24. Verify
// (internal/component/bfd/auth/sha1.go:159-163) compares both bytes against
// the configured algorithm, so a SHA1-shaped section (type 4, len 28) offered
// to an MD5 session is discarded rather than reinterpreted.
func TestRFC5880KeyedMD5RejectsForeignSectionShape(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeKeyedMD5, KeyID: 6, Secret: rfc5880Secret}
	buf, c, _ := rfc5880Signed(t, cfg, 9)
	v := rfc5880Verifier(t, cfg)

	wrongType := bytes.Clone(buf)
	wrongType[packet.MandatoryLen] = packet.AuthTypeKeyedSHA1
	var s1 SeqState
	if err := v.Verify(wrongType, c, &s1); err == nil {
		t.Fatal("MD5 verifier accepted a section carrying Auth Type 4")
	}

	wrongLen := bytes.Clone(buf)
	wrongLen[packet.MandatoryLen+1] = packet.AuthLenKeyedSHA1
	var s2 SeqState
	if err := v.Verify(wrongLen, c, &s2); err == nil {
		t.Fatal("MD5 verifier accepted a section carrying Auth Len 28")
	}
}

// RFC requirement: RFC5880-6.7.4-1 positive -- the Keyed SHA1 section carries
// Auth Type 4 (or 5 for the Meticulous variant) and Auth Len 28. newSHA1Signer
// (internal/component/bfd/auth/sha1.go:193-195) fixes bodyLen at
// packet.AuthLenKeyedSHA1 == 28 and Sign (sha1.go:80-81) writes both bytes.
func TestRFC5880KeyedSHA1SectionHeader(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeKeyedSHA1, packet.AuthTypeMeticulousKeyedSHA1} {
		cfg := Settings{Type: at, KeyID: 11, Secret: rfc5880Secret}
		buf, _, signer := rfc5880Signed(t, cfg, 3)
		if signer.BodyLen() != packet.AuthLenKeyedSHA1 {
			t.Fatalf("SHA1 BodyLen = %d, want 28", signer.BodyLen())
		}
		off := packet.MandatoryLen
		if buf[off] != at {
			t.Fatalf("Auth Type byte = %d, want %d", buf[off], at)
		}
		if buf[off+1] != packet.AuthLenKeyedSHA1 {
			t.Fatalf("Auth Len byte = %d, want 28", buf[off+1])
		}
	}
}

// RFC requirement: RFC5880-6.7.4-1 negative -- a Keyed SHA1 verifier rejects a
// section whose type or length belongs to another algorithm (sha1.go:159-163),
// so the 4/28 pairing is enforced rather than assumed.
func TestRFC5880KeyedSHA1RejectsForeignSectionShape(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeKeyedSHA1, KeyID: 11, Secret: rfc5880Secret}
	buf, c, _ := rfc5880Signed(t, cfg, 3)
	v := rfc5880Verifier(t, cfg)

	wrongType := bytes.Clone(buf)
	wrongType[packet.MandatoryLen] = packet.AuthTypeKeyedMD5
	var s1 SeqState
	if err := v.Verify(wrongType, c, &s1); err == nil {
		t.Fatal("SHA1 verifier accepted a section carrying Auth Type 2")
	}

	wrongLen := bytes.Clone(buf)
	wrongLen[packet.MandatoryLen+1] = packet.AuthLenKeyedMD5
	var s2 SeqState
	if err := v.Verify(wrongLen, c, &s2); err == nil {
		t.Fatal("SHA1 verifier accepted a section carrying Auth Len 24")
	}
}

// RFC requirement: RFC5880-6.7.3-2 positive -- the MD5 digest is calculated
// over the ENTIRE BFD Control packet. Sign
// (internal/component/bfd/auth/sha1.go:86) hashes buf[0:off+bodyLen], which
// spans the 24-byte mandatory section plus the whole auth section, and Verify
// (sha1.go:178-182) recomputes over the same span, so an unmodified packet
// verifies.
// RFC requirement: RFC5880-6.7.4-2 positive -- the SHA1 hash uses the same
// whole-packet span through the shared digestSigner/digestVerifier helpers.
func TestRFC5880DigestCoversWholePacket(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeKeyedMD5, packet.AuthTypeKeyedSHA1} {
		cfg := Settings{Type: at, KeyID: 2, Secret: rfc5880Secret}
		buf, c, _ := rfc5880Signed(t, cfg, 77)
		var state SeqState
		if err := rfc5880Verifier(t, cfg).Verify(buf, c, &state); err != nil {
			t.Fatalf("type %d: unmodified packet failed verification: %v", at, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.3-2 negative -- the coverage really is the
// whole packet: mutating a byte of the MANDATORY section (the Diagnostic /
// State byte, and the My Discriminator) after signing makes the recomputed
// digest differ and Verify (sha1.go:181-184) discards the packet. A digest
// covering only the auth section would accept these.
// RFC requirement: RFC5880-6.7.4-2 negative -- the same mutation is rejected
// by the SHA1 verifier through the same producer.
func TestRFC5880DigestRejectsMandatorySectionTamper(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeKeyedMD5, packet.AuthTypeKeyedSHA1} {
		cfg := Settings{Type: at, KeyID: 2, Secret: rfc5880Secret}
		buf, c, _ := rfc5880Signed(t, cfg, 77)
		v := rfc5880Verifier(t, cfg)

		for _, idx := range []int{1, 4, 20} {
			tampered := bytes.Clone(buf)
			tampered[idx] ^= 0xFF
			var state SeqState
			if err := v.Verify(tampered, c, &state); err == nil {
				t.Fatalf("type %d: verifier accepted a packet with mandatory byte %d flipped", at, idx)
			}
		}
	}
}

// RFC requirement: RFC5880-6.7.3-3 positive -- the secret key is never carried
// in the packet. Sign (internal/component/bfd/auth/sha1.go:85-87) copies the
// key into the digest slot only to compute the hash and then OVERWRITES that
// slot with the digest, so the emitted bytes contain no run of the key.
// RFC requirement: RFC5880-6.7.4-3 positive -- the SHA1 signer uses the same
// producer with a 20-byte key slot, and the emitted section likewise carries
// the digest rather than the key.
func TestRFC5880SecretKeyNotCarriedInPacket(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeKeyedMD5, packet.AuthTypeKeyedSHA1} {
		cfg := Settings{Type: at, KeyID: 2, Secret: rfc5880Secret}
		buf, _, signer := rfc5880Signed(t, cfg, 5)

		keySlot := signer.BodyLen() - 8
		want := make([]byte, keySlot)
		copy(want, rfc5880Secret)
		if bytes.Contains(buf, want) {
			t.Fatalf("type %d: the padded secret appears verbatim in the transmitted packet", at)
		}
		if bytes.Contains(buf, rfc5880Secret) {
			t.Fatalf("type %d: the secret appears verbatim in the transmitted packet", at)
		}
	}
}

// RFC requirement: RFC5880-6.7.2-3 positive -- the Auth Key ID field is set to
// the ID of the authentication key in use. Sign
// (internal/component/bfd/auth/sha1.go:82) writes s.keyID, which
// newDigestSigner (sha1.go:62) took from Settings.KeyID.
func TestRFC5880AuthKeyIDIsTheConfiguredKey(t *testing.T) {
	for _, id := range []uint8{0, 1, 200, 255} {
		cfg := Settings{Type: packet.AuthTypeKeyedSHA1, KeyID: id, Secret: rfc5880Secret}
		buf, c, _ := rfc5880Signed(t, cfg, 8)
		if got := buf[packet.MandatoryLen+2]; got != id {
			t.Fatalf("Auth Key ID field = %d, want the configured %d", got, id)
		}
		var state SeqState
		if err := rfc5880Verifier(t, cfg).Verify(buf, c, &state); err != nil {
			t.Fatalf("key id %d: verify: %v", id, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.2-3 negative -- the field is read back and
// checked rather than ignored: Verify (sha1.go:164-166) discards a packet
// whose Auth Key ID does not equal the configured key.
// RFC requirement: RFC5880-6.7.2-5 negative -- the same producer implements
// "if the Auth Key ID does not match any configured authentication key, the
// packet MUST be discarded"; ze configures exactly one key per session.
func TestRFC5880AuthKeyIDMismatchDiscarded(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeKeyedSHA1, KeyID: 3, Secret: rfc5880Secret}
	buf, c, _ := rfc5880Signed(t, cfg, 8)
	forged := bytes.Clone(buf)
	forged[packet.MandatoryLen+2] = 4 // no such key is configured

	var state SeqState
	if err := rfc5880Verifier(t, cfg).Verify(forged, c, &state); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("got %v, want ErrDigestMismatch for an unconfigured Auth Key ID", err)
	}
	if state.Initialized() {
		t.Fatal("replay floor advanced on a packet with an unconfigured key id")
	}
}

// RFC requirement: RFC5880-6.7.2-5 positive -- a packet whose Auth Key ID
// matches the configured authentication key passes the check at sha1.go:164
// and is accepted, so the discard above is key-scoped rather than blanket.
func TestRFC5880AuthKeyIDMatchAccepted(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeKeyedSHA1, KeyID: 3, Secret: rfc5880Secret}
	buf, c, _ := rfc5880Signed(t, cfg, 8)
	var state SeqState
	if err := rfc5880Verifier(t, cfg).Verify(buf, c, &state); err != nil {
		t.Fatalf("packet with the configured key id was discarded: %v", err)
	}
}

// RFC requirement: RFC5880-6.7.2-4 positive -- a packet whose Auth Type
// matches bfd.AuthType is accepted. Verify
// (internal/component/bfd/auth/sha1.go:159-161) compares the section's first
// byte against the verifier's configured type.
func TestRFC5880AuthTypeMatchAccepted(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeMeticulousKeyedSHA1, KeyID: 1, Secret: rfc5880Secret}
	buf, c, _ := rfc5880Signed(t, cfg, 12)
	var state SeqState
	if err := rfc5880Verifier(t, cfg).Verify(buf, c, &state); err != nil {
		t.Fatalf("matching Auth Type discarded: %v", err)
	}
}

// RFC requirement: RFC5880-6.7.2-4 negative -- a packet whose Auth Type does
// not match bfd.AuthType is discarded (sha1.go:159-161), so a peer cannot
// downgrade a Meticulous session to the non-meticulous variant by relabeling
// the section.
func TestRFC5880AuthTypeMismatchDiscarded(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeMeticulousKeyedSHA1, KeyID: 1, Secret: rfc5880Secret}
	buf, c, _ := rfc5880Signed(t, cfg, 12)
	forged := bytes.Clone(buf)
	forged[packet.MandatoryLen] = packet.AuthTypeKeyedSHA1

	var state SeqState
	if err := rfc5880Verifier(t, cfg).Verify(forged, c, &state); err == nil {
		t.Fatal("verifier accepted a section whose Auth Type differs from bfd.AuthType")
	}
}

// RFC requirement: RFC5880-6.7.2-6 positive -- a section whose Auth Len equals
// the expected fixed length for the configured type is accepted. Verify
// (internal/component/bfd/auth/sha1.go:146-163) checks the total Control
// Length against MandatoryLen+bodyLen AND the section's own Auth Len byte
// against bodyLen.
func TestRFC5880AuthLenExpectedAccepted(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeKeyedSHA1, KeyID: 1, Secret: rfc5880Secret}
	buf, c, _ := rfc5880Signed(t, cfg, 30)
	if c.Length != packet.MandatoryLen+packet.AuthLenKeyedSHA1 {
		t.Fatalf("Control Length = %d, want %d", c.Length, packet.MandatoryLen+packet.AuthLenKeyedSHA1)
	}
	var state SeqState
	if err := rfc5880Verifier(t, cfg).Verify(buf, c, &state); err != nil {
		t.Fatalf("expected Auth Len discarded: %v", err)
	}
}

// RFC requirement: RFC5880-6.7.2-6 negative -- an Auth Len that does not match
// the expected length is discarded, both when the section byte is forged
// (sha1.go:162-163) and when the total Control Length is short or long
// (sha1.go:147-157). This is what keeps a forged length from driving an
// over-read of the digest slot.
func TestRFC5880AuthLenMismatchDiscarded(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeKeyedSHA1, KeyID: 1, Secret: rfc5880Secret}
	buf, c, _ := rfc5880Signed(t, cfg, 30)
	v := rfc5880Verifier(t, cfg)

	forgedByte := bytes.Clone(buf)
	forgedByte[packet.MandatoryLen+1] = packet.AuthLenKeyedSHA1 - 1
	var s1 SeqState
	if err := v.Verify(forgedByte, c, &s1); err == nil {
		t.Fatal("verifier accepted a forged Auth Len byte")
	}

	short := c
	short.Length = packet.MandatoryLen + packet.AuthLenKeyedSHA1 - 1
	var s2 SeqState
	if err := v.Verify(buf, short, &s2); !errors.Is(err, ErrShortAuthBody) {
		t.Fatalf("short Control Length: got %v, want ErrShortAuthBody", err)
	}

	long := c
	long.Length = packet.MandatoryLen + packet.AuthLenKeyedSHA1 + 1
	longBuf := make([]byte, len(buf)+1)
	copy(longBuf, buf)
	var s3 SeqState
	if err := v.Verify(longBuf, long, &s3); err == nil {
		t.Fatal("verifier accepted a Control Length longer than the fixed auth section")
	}
}

// RFC requirement: RFC5880-6.7.3-11 positive -- a packet whose digest matches
// the locally computed value is accepted. Verify
// (internal/component/bfd/auth/sha1.go:181-184) compares the two in constant
// time and returns nil on equality.
func TestRFC5880DigestMatchAccepted(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeKeyedSHA1, KeyID: 1, Secret: rfc5880Secret}
	buf, c, _ := rfc5880Signed(t, cfg, 44)
	var state SeqState
	if err := rfc5880Verifier(t, cfg).Verify(buf, c, &state); err != nil {
		t.Fatalf("matching digest discarded: %v", err)
	}
}

// RFC requirement: RFC5880-6.7.3-11 negative -- a packet whose digest does not
// match the computed value is discarded, whether the digest bytes were flipped
// or the packet was signed with a different secret (sha1.go:181-184). The
// replay floor is left untouched so a forged packet cannot poison
// bfd.RcvAuthSeq.
func TestRFC5880DigestMismatchDiscarded(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeKeyedSHA1, KeyID: 1, Secret: rfc5880Secret}
	buf, c, _ := rfc5880Signed(t, cfg, 44)
	v := rfc5880Verifier(t, cfg)

	flipped := bytes.Clone(buf)
	flipped[len(flipped)-1] ^= 0xFF
	var s1 SeqState
	if err := v.Verify(flipped, c, &s1); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("flipped digest: got %v, want ErrDigestMismatch", err)
	}
	if s1.Initialized() {
		t.Fatal("replay floor advanced on a packet with a bad digest")
	}

	wrongKey := Settings{Type: packet.AuthTypeKeyedSHA1, KeyID: 1, Secret: []byte("a-different-key")}
	forged, fc, _ := rfc5880Signed(t, wrongKey, 44)
	var s2 SeqState
	if err := v.Verify(forged, fc, &s2); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("wrong-key digest: got %v, want ErrDigestMismatch", err)
	}
}

// RFC requirement: RFC5880-6.8.1-12 positive -- bfd.AuthSeqKnown is
// initialized to zero. The zero value of SeqState
// (internal/component/bfd/auth/meticulous.go:28-34) has initialized false,
// which Initialized() reports, and Check (meticulous.go:43-45) therefore
// accepts any first sequence number.
func TestRFC5880AuthSeqKnownStartsZero(t *testing.T) {
	var state SeqState
	if state.Initialized() {
		t.Fatal("bfd.AuthSeqKnown is set on a fresh session")
	}
	if state.Last() != 0 {
		t.Fatalf("bfd.RcvAuthSeq = %d on a fresh session, want 0", state.Last())
	}
	if err := state.Check(0xFFFF0000, true); err != nil {
		t.Fatalf("first sequence rejected while bfd.AuthSeqKnown is 0: %v", err)
	}
}

// RFC requirement: RFC5880-6.8.1-12 negative -- the zero is an initial value
// rather than a constant: Advance (meticulous.go:63-67) sets initialized on
// the first accepted packet, so bfd.AuthSeqKnown does become 1.
func TestRFC5880AuthSeqKnownSetAfterFirstPacket(t *testing.T) {
	var state SeqState
	state.Advance(9000, true)
	if !state.Initialized() {
		t.Fatal("bfd.AuthSeqKnown still 0 after accepting a packet")
	}
}

// RFC requirement: RFC5880-6.7.3-12 positive -- when bfd.AuthSeqKnown is 0 it
// is set to 1 and bfd.RcvAuthSeq is set to the received Sequence Number.
// Verify (internal/component/bfd/auth/sha1.go:185) calls Advance, which on an
// uninitialized state stores seq and sets initialized (meticulous.go:63-67).
func TestRFC5880FirstAuthenticatedPacketSeedsReplayFloor(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeKeyedSHA1, KeyID: 1, Secret: rfc5880Secret}
	const seq uint32 = 0xDEAD00
	buf, c, _ := rfc5880Signed(t, cfg, seq)

	var state SeqState
	if state.Initialized() {
		t.Fatal("precondition: bfd.AuthSeqKnown must start at 0")
	}
	if err := rfc5880Verifier(t, cfg).Verify(buf, c, &state); err != nil {
		t.Fatalf("first authenticated packet: %v", err)
	}
	if !state.Initialized() {
		t.Fatal("bfd.AuthSeqKnown not set to 1 after the first accepted packet")
	}
	if state.Last() != seq {
		t.Fatalf("bfd.RcvAuthSeq = %#x, want the received %#x", state.Last(), seq)
	}
}

// RFC requirement: RFC5880-6.7.3-12 negative -- the seeding happens only for a
// packet that actually passed authentication: Verify returns before reaching
// Advance when the digest fails (sha1.go:181-184), so a forged first packet
// leaves bfd.AuthSeqKnown at 0 instead of pinning the floor to an
// attacker-chosen sequence.
func TestRFC5880ForgedFirstPacketDoesNotSeedReplayFloor(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeKeyedSHA1, KeyID: 1, Secret: rfc5880Secret}
	buf, c, _ := rfc5880Signed(t, cfg, 0x7000)
	forged := bytes.Clone(buf)
	forged[len(forged)-2] ^= 0xFF

	var state SeqState
	if err := rfc5880Verifier(t, cfg).Verify(forged, c, &state); err == nil {
		t.Fatal("forged packet accepted")
	}
	if state.Initialized() {
		t.Fatal("bfd.AuthSeqKnown set by a packet that failed authentication")
	}
	if state.Last() != 0 {
		t.Fatalf("bfd.RcvAuthSeq = %#x after a forged packet, want 0", state.Last())
	}
}

// RFC requirement: RFC5880-6.7.3-9 positive -- for the Keyed (non-meticulous)
// variants a Sequence Number at or above bfd.RcvAuthSeq is accepted. Check
// (internal/component/bfd/auth/meticulous.go:53-56) rejects only seq < last,
// so an equal or greater sequence passes and advances the floor.
func TestRFC5880KeyedSequenceAtOrAboveFloorAccepted(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeKeyedSHA1, KeyID: 1, Secret: rfc5880Secret}
	v := rfc5880Verifier(t, cfg)
	var state SeqState

	for _, seq := range []uint32{100, 100, 101, 500} {
		buf, c, _ := rfc5880Signed(t, cfg, seq)
		if err := v.Verify(buf, c, &state); err != nil {
			t.Fatalf("sequence %d rejected with floor %d: %v", seq, state.Last(), err)
		}
	}
	if state.Last() != 500 {
		t.Fatalf("bfd.RcvAuthSeq = %d, want 500", state.Last())
	}
}

// RFC requirement: RFC5880-6.7.3-9 negative -- a Sequence Number below
// bfd.RcvAuthSeq is discarded with ErrSequenceRegress
// (internal/component/bfd/auth/meticulous.go:53-55), which is the replay
// protection: a captured older packet cannot be re-injected.
func TestRFC5880KeyedSequenceBelowFloorDiscarded(t *testing.T) {
	cfg := Settings{Type: packet.AuthTypeKeyedSHA1, KeyID: 1, Secret: rfc5880Secret}
	v := rfc5880Verifier(t, cfg)
	var state SeqState

	buf, c, _ := rfc5880Signed(t, cfg, 400)
	if err := v.Verify(buf, c, &state); err != nil {
		t.Fatalf("floor packet: %v", err)
	}
	old, oc, _ := rfc5880Signed(t, cfg, 399)
	if err := v.Verify(old, oc, &state); !errors.Is(err, ErrSequenceRegress) {
		t.Fatalf("got %v, want ErrSequenceRegress for a sequence below the floor", err)
	}
	if state.Last() != 400 {
		t.Fatalf("bfd.RcvAuthSeq moved to %d on a rejected packet", state.Last())
	}
}

// RFC requirement: RFC5880-6.7.3-4 negative -- for Meticulous Keyed MD5 the
// per-packet increment of bfd.XmitAuthSeq is mandatory: a transmitter that
// re-used the previous sequence is rejected by the peer, because Check
// (internal/component/bfd/auth/meticulous.go:47-51) requires strictly greater
// for the meticulous variants.
// RFC requirement: RFC5880-6.7.4-4 negative -- the same strict rule applies to
// Meticulous Keyed SHA1 through the same producer.
func TestRFC5880MeticulousRejectsUnincrementedSequence(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeMeticulousKeyedMD5, packet.AuthTypeMeticulousKeyedSHA1} {
		cfg := Settings{Type: at, KeyID: 1, Secret: rfc5880Secret}
		v := rfc5880Verifier(t, cfg)
		var state SeqState

		buf, c, _ := rfc5880Signed(t, cfg, 50)
		if err := v.Verify(buf, c, &state); err != nil {
			t.Fatalf("type %d: first packet: %v", at, err)
		}
		again, ac, _ := rfc5880Signed(t, cfg, 50)
		if err := v.Verify(again, ac, &state); !errors.Is(err, ErrSequenceRegress) {
			t.Fatalf("type %d: a repeated sequence was accepted (%v)", at, err)
		}
		next, nc, _ := rfc5880Signed(t, cfg, 51)
		if err := v.Verify(next, nc, &state); err != nil {
			t.Fatalf("type %d: incremented sequence rejected: %v", at, err)
		}
	}
}

// rfc5880Password is the Simple Password used by the tests below. It is eight
// bytes, which sits between the one-byte minimum and the sixteen-byte maximum
// so a length change in either direction is visible in the Auth Len field.
var rfc5880Password = []byte("passw0rd")

// rfc5880SimpleSigned returns a freshly signed Simple Password packet for the
// given key id and password, along with the parsed Control the verifier reads.
func rfc5880SimpleSigned(t *testing.T, keyID uint8, password []byte) ([]byte, packet.Control) {
	t.Helper()
	cfg := Settings{Type: packet.AuthTypeSimplePassword, KeyID: keyID, Secret: password}
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner(simple password, %d-byte password): %v", len(password), err)
	}
	buf := make([]byte, packet.MandatoryLen+signer.BodyLen())
	c := controlBytes(buf, signer.BodyLen())
	signer.Sign(buf, packet.MandatoryLen, 0)
	return buf, c
}

// RFC requirement: RFC5880-6.7.2-8 positive -- the transmitted section carries
// Auth Type 1 and an Auth Len equal to the password length plus three, which
// is the "proper length (4 to 19 bytes)" the RFC names. simpleSigner.Sign
// (internal/component/bfd/auth/simple.go) writes both bytes and
// newSimpleSigner derives sectionLen from the configured password.
// RFC requirement: RFC5880-4.2-1 positive -- the one-byte and sixteen-byte
// passwords are both configured and both encoded, so the whole permitted range
// reaches the wire.
func TestRFC5880SimplePasswordSectionHeader(t *testing.T) {
	for _, octets := range []int{1, 8, 16} {
		password := bytes.Repeat([]byte{'p'}, octets)
		buf, _ := rfc5880SimpleSigned(t, 3, password)
		off := packet.MandatoryLen
		if buf[off] != packet.AuthTypeSimplePassword {
			t.Fatalf("%d-byte password: Auth Type = %d, want 1", octets, buf[off])
		}
		wantLen := packet.SimplePasswordHeaderLen + octets
		if int(buf[off+1]) != wantLen {
			t.Fatalf("%d-byte password: Auth Len = %d, want %d", octets, buf[off+1], wantLen)
		}
		if wantLen < packet.AuthLenSimplePasswordMin || wantLen > packet.AuthLenSimplePasswordMax {
			t.Fatalf("%d-byte password: Auth Len %d outside the 4 to 19 the RFC allows", octets, wantLen)
		}
	}
}

// RFC requirement: RFC5880-6.7.2-8 negative -- the pairing is enforced on
// reception rather than assumed: simpleVerifier.Verify compares the Auth Type
// byte against 1 and the Auth Len byte against the password length plus three,
// so a section carrying a keyed-MD5 shape, or a length one byte off in either
// direction, is discarded.
func TestRFC5880SimplePasswordRejectsForeignSectionShape(t *testing.T) {
	buf, c := rfc5880SimpleSigned(t, 3, rfc5880Password)
	v := rfc5880Verifier(t, Settings{
		Type:   packet.AuthTypeSimplePassword,
		KeyID:  3,
		Secret: rfc5880Password,
	})
	off := packet.MandatoryLen

	wrongType := bytes.Clone(buf)
	wrongType[off] = packet.AuthTypeKeyedMD5
	if err := v.Verify(wrongType, c, nil); !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf("Auth Type 2 offered to a Simple Password session: got %v", err)
	}

	for _, delta := range []int{-1, +1} {
		wrongLen := bytes.Clone(buf)
		wrongLen[off+1] = byte(int(wrongLen[off+1]) + delta)
		if err := v.Verify(wrongLen, c, nil); !errors.Is(err, ErrPasswordMismatch) {
			t.Fatalf("Auth Len off by %d accepted: got %v", delta, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.2-3 positive -- "The currently selected password
// and Key ID for the session MUST be stored in the Authentication Section of
// each outgoing BFD Control packet." simpleSigner.Sign writes the configured
// Key ID at offset 2 and copies the password from offset 3, and it writes the
// same section on every packet, so the pair is present on each transmission.
func TestRFC5880SimplePasswordSectionCarriesPasswordAndKeyID(t *testing.T) {
	const keyID = 42
	first, _ := rfc5880SimpleSigned(t, keyID, rfc5880Password)
	second, _ := rfc5880SimpleSigned(t, keyID, rfc5880Password)
	off := packet.MandatoryLen

	if first[off+2] != keyID {
		t.Fatalf("Auth Key ID = %d, want %d", first[off+2], keyID)
	}
	carried := first[off+packet.SimplePasswordHeaderLen : off+packet.SimplePasswordHeaderLen+len(rfc5880Password)]
	if !bytes.Equal(carried, rfc5880Password) {
		t.Fatalf("password on the wire = %q, want %q", carried, rfc5880Password)
	}
	if !bytes.Equal(first[off:], second[off:]) {
		t.Fatal("the authentication section differs between two packets of the same session")
	}
}

// RFC requirement: RFC5880-6.7.2-7 positive -- "The receiving system accepts the
// packet if the Password and Key ID matches one of the Password/ID pairs
// configured in that system." simpleVerifier.Verify returns nil once the type,
// key id, length and password all match the configured pair.
func TestRFC5880SimplePasswordMatchAccepted(t *testing.T) {
	for _, octets := range []int{1, 8, 16} {
		password := bytes.Repeat([]byte{'q'}, octets)
		buf, c := rfc5880SimpleSigned(t, 9, password)
		v := rfc5880Verifier(t, Settings{
			Type:   packet.AuthTypeSimplePassword,
			KeyID:  9,
			Secret: password,
		})
		if err := v.Verify(buf, c, nil); err != nil {
			t.Fatalf("%d-byte password round trip: %v", octets, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.2-7 negative -- "If the Password field does not
// match the password selected by the key ID, the packet MUST be discarded."
// The constant-time compare in simpleVerifier.Verify rejects a password that
// differs in one byte, one that differs in length, and the empty tail a
// truncated section leaves behind.
func TestRFC5880SimplePasswordMismatchDiscarded(t *testing.T) {
	v := rfc5880Verifier(t, Settings{
		Type:   packet.AuthTypeSimplePassword,
		KeyID:  9,
		Secret: rfc5880Password,
	})

	oneByteOff := bytes.Clone(rfc5880Password)
	oneByteOff[len(oneByteOff)-1] ^= 0x01
	buf, c := rfc5880SimpleSigned(t, 9, oneByteOff)
	if err := v.Verify(buf, c, nil); !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf("a password differing in one byte was accepted: %v", err)
	}

	shorter, sc := rfc5880SimpleSigned(t, 9, rfc5880Password[:len(rfc5880Password)-1])
	if err := v.Verify(shorter, sc, nil); !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf("a shorter password was accepted: %v", err)
	}

	longer := append(bytes.Clone(rfc5880Password), 'x')
	lbuf, lc := rfc5880SimpleSigned(t, 9, longer)
	if err := v.Verify(lbuf, lc, nil); !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf("a longer password was accepted: %v", err)
	}
}

// RFC requirement: RFC5880-6.7.2-5 negative -- "If the Auth Key ID field does
// not match the ID of a configured password, the received packet MUST be
// discarded." simpleVerifier.Verify compares the byte at offset 2 against the
// configured key id before it looks at the password, so a packet carrying the
// right password under the wrong key id is still discarded.
func TestRFC5880SimplePasswordWrongKeyIDDiscarded(t *testing.T) {
	buf, c := rfc5880SimpleSigned(t, 7, rfc5880Password)
	v := rfc5880Verifier(t, Settings{
		Type:   packet.AuthTypeSimplePassword,
		KeyID:  8,
		Secret: rfc5880Password,
	})
	if err := v.Verify(buf, c, nil); !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf("key id 7 accepted by a session configured for key id 8: %v", err)
	}
}

// RFC requirement: RFC5880-6.7.2-5 positive -- the same producer accepts the
// packet once the key id does match, so the check above is a comparison and
// not a blanket refusal.
func TestRFC5880SimplePasswordMatchingKeyIDAccepted(t *testing.T) {
	buf, c := rfc5880SimpleSigned(t, 7, rfc5880Password)
	v := rfc5880Verifier(t, Settings{
		Type:   packet.AuthTypeSimplePassword,
		KeyID:  7,
		Secret: rfc5880Password,
	})
	if err := v.Verify(buf, c, nil); err != nil {
		t.Fatalf("matching key id rejected: %v", err)
	}
}

// RFC requirement: RFC5880-4.2-1 negative -- "The password is a binary string,
// and MUST be from 1 to 16 bytes in length." NewSigner and NewVerifier refuse a
// zero-byte and a seventeen-byte password with ErrKeyLengthInvalid, so no
// session can be built that would encode an Auth Len outside 4 to 19.
func TestRFC5880SimplePasswordLengthOutOfRangeRefused(t *testing.T) {
	for _, octets := range []int{0, packet.SimplePasswordLenMax + 1, 32, 255} {
		cfg := Settings{
			Type:   packet.AuthTypeSimplePassword,
			KeyID:  1,
			Secret: bytes.Repeat([]byte{'p'}, octets),
		}
		if _, err := NewSigner(cfg); !errors.Is(err, ErrKeyLengthInvalid) {
			t.Fatalf("NewSigner with a %d-byte password: got %v, want ErrKeyLengthInvalid", octets, err)
		}
		if _, err := NewVerifier(cfg); !errors.Is(err, ErrKeyLengthInvalid) {
			t.Fatalf("NewVerifier with a %d-byte password: got %v, want ErrKeyLengthInvalid", octets, err)
		}
	}
}

// VALIDATES: the Simple Password verifier treats a truncated or over-long
// section as an operating error and returns, because the bytes arrive from an
// unauthenticated peer.
// PREVENTS: an index out of range on the receive path when a peer sends a
// Control Length that does not describe the datagram it sent.
func TestRFC5880SimplePasswordMalformedSectionRejected(t *testing.T) {
	buf, c := rfc5880SimpleSigned(t, 9, rfc5880Password)
	v := rfc5880Verifier(t, Settings{
		Type:   packet.AuthTypeSimplePassword,
		KeyID:  9,
		Secret: rfc5880Password,
	})

	for cut := range buf {
		if err := v.Verify(buf[:cut], c, nil); err == nil {
			t.Fatalf("a %d-byte packet claiming Length %d was accepted", cut, c.Length)
		}
	}

	trailing := append(bytes.Clone(buf), 0xFF, 0xFF)
	over := c
	over.Length = uint8(len(trailing))
	if err := v.Verify(trailing, over, nil); !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf("trailing bytes behind a matching password were accepted: %v", err)
	}
}
