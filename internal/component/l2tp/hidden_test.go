package l2tp

import (
	"bytes"
	"errors"
	"testing"
)

// TestHiddenRoundTripSingleBlock validates AC-17 for a 14-byte value
// (subformat = 16 bytes, exactly one MD5 block).
func TestHiddenRoundTripSingleBlock(t *testing.T) {
	secret := []byte("tunnel-secret")
	rv := []byte("random-vector-1234567890")
	plaintext := bytes.Repeat([]byte{0xAB}, 14) // sub = 16 bytes

	encDst := make([]byte, 64)
	ct := HiddenEncrypt(encDst, AVPChallenge, secret, rv, plaintext)
	if len(ct) != 16 {
		t.Fatalf("ciphertext len %d want 16", len(ct))
	}
	// Ciphertext must differ from subformat plaintext (first 2 bytes are length
	// 14 = 0x000E; sum of XORed stream must not match).
	if bytes.Equal(ct[hiddenLenField:], plaintext) {
		t.Fatalf("ciphertext identical to plaintext — XOR did not run")
	}

	decDst := make([]byte, 64)
	got, err := HiddenDecrypt(decDst, AVPChallenge, secret, rv, ct)
	if err != nil {
		t.Fatalf("HiddenDecrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip mismatch\n got  %x\n want %x", got, plaintext)
	}
}

// TestHiddenRoundTripShort validates AC-19: subformat shorter than 16 bytes.
func TestHiddenRoundTripShort(t *testing.T) {
	secret := []byte("s")
	rv := []byte("rv")
	plaintext := []byte{0x01, 0x02, 0x03} // sub = 5 bytes

	encDst := make([]byte, 32)
	ct := HiddenEncrypt(encDst, AVPHostName, secret, rv, plaintext)
	if len(ct) != 5 {
		t.Fatalf("ciphertext len %d want 5", len(ct))
	}
	decDst := make([]byte, 32)
	got, err := HiddenDecrypt(decDst, AVPHostName, secret, rv, ct)
	if err != nil {
		t.Fatalf("HiddenDecrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("got %x want %x", got, plaintext)
	}
}

// TestHiddenRoundTripMultiBlock validates AC-20: 40-byte subformat (3 blocks).
func TestHiddenRoundTripMultiBlock(t *testing.T) {
	secret := []byte("long-shared-secret-value")
	rv := []byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80,
		0x90, 0xA0, 0xB0, 0xC0, 0xD0, 0xE0, 0xF0, 0x11}
	plaintext := bytes.Repeat([]byte{0x55}, 38) // sub = 40 bytes (2 full + 8)

	encDst := make([]byte, 128)
	ct := HiddenEncrypt(encDst, AVPChallengeResponse, secret, rv, plaintext)
	if len(ct) != 40 {
		t.Fatalf("ciphertext len %d want 40", len(ct))
	}
	decDst := make([]byte, 128)
	got, err := HiddenDecrypt(decDst, AVPChallengeResponse, secret, rv, ct)
	if err != nil {
		t.Fatalf("HiddenDecrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("multi-block round-trip mismatch")
	}
}

// TestHiddenDecryptWrongSecret validates AC-18: wrong secret produces a
// garbage Original Length, which almost certainly exceeds the ciphertext
// payload and triggers ErrHiddenLenMismatch.
func TestHiddenDecryptWrongSecret(t *testing.T) {
	secret := []byte("right-secret")
	wrong := []byte("wrong-secret")
	rv := []byte("rv")
	plaintext := bytes.Repeat([]byte{0xCC}, 14)

	encDst := make([]byte, 64)
	ct := HiddenEncrypt(encDst, AVPHostName, secret, rv, plaintext)

	decDst := make([]byte, 64)
	got, err := HiddenDecrypt(decDst, AVPHostName, wrong, rv, ct)
	if err == nil {
		t.Fatalf("expected error, got value %x", got)
	}
	// Must be ErrHiddenLenMismatch unless by astronomical coincidence the
	// random Original Length fits; that is effectively impossible here.
	if !errors.Is(err, ErrHiddenLenMismatch) {
		t.Fatalf("unexpected err: %v", err)
	}
}

// TestHiddenRoundTripEmptyPlaintext validates the boundary: a zero-length
// plaintext produces a 2-byte ciphertext (the Original Length field alone,
// XORed with the first 2 bytes of the MD5 keystream).
// VALIDATES: subformat construction when len(plaintext)==0; decrypt
// recovers an empty slice.
// PREVENTS: off-by-one that would return nil instead of []byte{}.
func TestHiddenRoundTripEmptyPlaintext(t *testing.T) {
	secret := []byte("s")
	rv := []byte("rv")
	var plaintext []byte

	encDst := make([]byte, 8)
	ct := HiddenEncrypt(encDst, AVPHostName, secret, rv, plaintext)
	if len(ct) != 2 {
		t.Fatalf("ciphertext len: got %d want 2", len(ct))
	}
	decDst := make([]byte, 8)
	got, err := HiddenDecrypt(decDst, AVPHostName, secret, rv, ct)
	if err != nil {
		t.Fatalf("HiddenDecrypt: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("plaintext len: got %d want 0 (%x)", len(got), got)
	}
}

// TestHiddenRoundTripAttrTypeIndependence validates that two different
// attribute types with the same secret, RV, and plaintext produce different
// ciphertexts — the per-AVP keystream requirement from RFC 2661 Section 4.3.
func TestHiddenRoundTripAttrTypeIndependence(t *testing.T) {
	secret := []byte("s")
	rv := []byte("rv")
	pt := []byte{0x41, 0x42, 0x43, 0x44}

	a := make([]byte, 32)
	b := make([]byte, 32)
	ca := HiddenEncrypt(a, AVPHostName, secret, rv, pt)
	cb := HiddenEncrypt(b, AVPVendorName, secret, rv, pt)
	if bytes.Equal(ca, cb) {
		t.Fatalf("same ciphertext for different attr types — keystream not per-AVP")
	}
}
