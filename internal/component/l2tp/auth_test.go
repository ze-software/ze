package l2tp

import (
	"bytes"
	"crypto/md5" //nolint:gosec // test mirrors RFC 2661 Section 4.2 MD5 computation.
	"testing"
)

// TestChallengeResponseKnown validates AC-16.
// VALIDATES: MD5(chap_id || secret || challenge) matches ChallengeResponse.
// PREVENTS: accidental reorder of (secret, challenge, chap_id) inputs.
func TestChallengeResponseKnown(t *testing.T) {
	secret := []byte("shared-secret-foo")
	challenge := []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
	}
	cases := []struct {
		name   string
		chapID byte
	}{
		{"SCCRP chapID=2", ChapIDSCCRP},
		{"SCCCN chapID=3", ChapIDSCCCN},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := append([]byte{tc.chapID}, secret...)
			input = append(input, challenge...)
			want := md5.Sum(input) //nolint:gosec // RFC 2661 Section 4.2 fixed algorithm.
			got := ChallengeResponse(tc.chapID, secret, challenge)
			if !bytes.Equal(got[:], want[:]) {
				t.Fatalf("response mismatch\n got  %x\n want %x", got, want)
			}
			if !VerifyChallengeResponse(tc.chapID, secret, challenge, got[:]) {
				t.Fatalf("VerifyChallengeResponse: expected true")
			}
		})
	}
}

// TestChallengeResponseCrossDirection validates that the same (secret,challenge)
// yields a different response for different chapID — the replay-protection
// guarantee RFC 2661 Section 4.2 relies on.
func TestChallengeResponseCrossDirection(t *testing.T) {
	secret := []byte("s")
	challenge := []byte{0x42}
	a := ChallengeResponse(ChapIDSCCRP, secret, challenge)
	b := ChallengeResponse(ChapIDSCCCN, secret, challenge)
	if a == b {
		t.Fatalf("SCCRP and SCCCN responses must differ")
	}
	if VerifyChallengeResponse(ChapIDSCCRP, secret, challenge, b[:]) {
		t.Fatalf("SCCRP should not accept SCCCN response (replay protection broken)")
	}
}

// TestVerifyChallengeResponseWrongLen validates len(got) != 16 rejection.
func TestVerifyChallengeResponseWrongLen(t *testing.T) {
	if VerifyChallengeResponse(2, []byte("s"), []byte("c"), []byte("short")) {
		t.Fatalf("expected reject")
	}
}

// TestChallengeResponseLongInput exercises the heap-fallback path of
// ChallengeResponse when secret+challenge exceed the stack buffer.
func TestChallengeResponseLongInput(t *testing.T) {
	secret := make([]byte, 200)
	for i := range secret {
		secret[i] = byte(i)
	}
	challenge := make([]byte, 200)
	got := ChallengeResponse(2, secret, challenge)
	input := append([]byte{2}, secret...)
	input = append(input, challenge...)
	want := md5.Sum(input) //nolint:gosec // RFC 2661 Section 4.2 fixed algorithm.
	if got != want {
		t.Fatalf("long-input mismatch")
	}
}
