package auth

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// controlBytes writes a minimal BFD Control packet into buf and
// returns the parsed Control for the verifier.
func controlBytes(buf []byte, authLen int) packet.Control {
	c := packet.Control{
		Version:               packet.Version,
		Diag:                  packet.DiagNone,
		State:                 packet.StateDown,
		Auth:                  true,
		DetectMult:            3,
		Length:                uint8(packet.MandatoryLen + authLen),
		MyDiscriminator:       0x01020304,
		YourDiscriminator:     0,
		DesiredMinTxInterval:  1_000_000,
		RequiredMinRxInterval: 1_000_000,
	}
	c.WriteTo(buf, 0)
	return c
}

// VALIDATES: Keyed SHA1 Sign followed by Verify with the same key
// returns no error and advances the replay state.
// PREVENTS: regression where the digest placement / substitution /
// endian encoding of the sequence number drifts from RFC 5880 §6.7.4.
func TestSignerVerifier_SHA1_RoundTrip(t *testing.T) {
	cfg := Settings{
		Type:   packet.AuthTypeKeyedSHA1,
		KeyID:  7,
		Secret: []byte("SHARED-SECRET-123"),
	}
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	verifier, err := NewVerifier(cfg)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	buf := make([]byte, packet.MandatoryLen+signer.BodyLen())
	c := controlBytes(buf, signer.BodyLen())
	signer.Sign(buf, packet.MandatoryLen, 42)

	var state SeqState
	if err := verifier.Verify(buf, c, &state); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if state.Last() != 42 {
		t.Fatalf("SeqState.Last = %d, want 42", state.Last())
	}
}

// VALIDATES: Verifier rejects packets signed with the wrong secret.
// PREVENTS: false accept when an attacker's key matches the bytes.
func TestVerifier_SHA1_Mismatch(t *testing.T) {
	signer, _ := NewSigner(Settings{
		Type:   packet.AuthTypeKeyedSHA1,
		KeyID:  1,
		Secret: []byte("correct"),
	})
	verifier, _ := NewVerifier(Settings{
		Type:   packet.AuthTypeKeyedSHA1,
		KeyID:  1,
		Secret: []byte("wrong"),
	})

	buf := make([]byte, packet.MandatoryLen+signer.BodyLen())
	c := controlBytes(buf, signer.BodyLen())
	signer.Sign(buf, packet.MandatoryLen, 1)

	var state SeqState
	err := verifier.Verify(buf, c, &state)
	if err == nil {
		t.Fatal("Verify with wrong key returned nil error")
	}
}

// VALIDATES: Keyed MD5 round-trip works via the generic digest helper.
// PREVENTS: the two algorithm variants diverging in body layout.
func TestSignerVerifier_MD5_RoundTrip(t *testing.T) {
	cfg := Settings{
		Type:   packet.AuthTypeKeyedMD5,
		KeyID:  3,
		Secret: []byte("md5-secret"),
	}
	signer, _ := NewSigner(cfg)
	verifier, _ := NewVerifier(cfg)

	buf := make([]byte, packet.MandatoryLen+signer.BodyLen())
	c := controlBytes(buf, signer.BodyLen())
	signer.Sign(buf, packet.MandatoryLen, 5)

	var state SeqState
	if err := verifier.Verify(buf, c, &state); err != nil {
		t.Fatalf("MD5 Verify: %v", err)
	}
}

// VALIDATES: Meticulous variants reject replays of a previously-
// accepted sequence number.
// PREVENTS: drift from RFC 5880 §6.7.3 (Meticulous = strict increase).
func TestVerifier_MeticulousReplay(t *testing.T) {
	cfg := Settings{
		Type:   packet.AuthTypeMeticulousKeyedSHA1,
		KeyID:  1,
		Secret: []byte("k"),
	}
	signer, _ := NewSigner(cfg)
	verifier, _ := NewVerifier(cfg)

	buf := make([]byte, packet.MandatoryLen+signer.BodyLen())
	c := controlBytes(buf, signer.BodyLen())

	var state SeqState
	signer.Sign(buf, packet.MandatoryLen, 10)
	if err := verifier.Verify(buf, c, &state); err != nil {
		t.Fatalf("first accept: %v", err)
	}

	// Replay the same seq: must be rejected.
	signer.Sign(buf, packet.MandatoryLen, 10)
	if err := verifier.Verify(buf, c, &state); err == nil {
		t.Fatal("meticulous accepted duplicate sequence")
	}

	// Accept strictly greater.
	signer.Sign(buf, packet.MandatoryLen, 11)
	if err := verifier.Verify(buf, c, &state); err != nil {
		t.Fatalf("strictly-greater reject: %v", err)
	}
}

// VALIDATES: non-meticulous Keyed SHA1 accepts an equal sequence
// number (RFC 5880 §6.7.3 relaxation).
// PREVENTS: mis-tightening the non-meticulous check.
func TestVerifier_NonMeticulousEqual(t *testing.T) {
	cfg := Settings{
		Type:   packet.AuthTypeKeyedSHA1,
		KeyID:  1,
		Secret: []byte("k"),
	}
	signer, _ := NewSigner(cfg)
	verifier, _ := NewVerifier(cfg)

	buf := make([]byte, packet.MandatoryLen+signer.BodyLen())
	c := controlBytes(buf, signer.BodyLen())

	var state SeqState
	signer.Sign(buf, packet.MandatoryLen, 5)
	if err := verifier.Verify(buf, c, &state); err != nil {
		t.Fatalf("first: %v", err)
	}
	signer.Sign(buf, packet.MandatoryLen, 5)
	if err := verifier.Verify(buf, c, &state); err != nil {
		t.Fatalf("equal replay rejected by non-meticulous: %v", err)
	}
}

// VALIDATES: NewSigner / NewVerifier reject Auth Type 1 (Simple
// Password) so the package surface cannot be used to send it.
// PREVENTS: accidental enablement of an insecure type via
// configuration drift.
func TestUnsupportedTypes(t *testing.T) {
	cases := []uint8{
		packet.AuthTypeReserved,
		packet.AuthTypeSimplePassword,
		99,
	}
	for _, at := range cases {
		if _, err := NewSigner(Settings{Type: at}); err == nil {
			t.Errorf("NewSigner type %d: want error, got nil", at)
		}
		if _, err := NewVerifier(Settings{Type: at}); err == nil {
			t.Errorf("NewVerifier type %d: want error, got nil", at)
		}
	}
}
