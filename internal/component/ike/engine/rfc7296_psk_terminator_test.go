package engine

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// psktermSecret is a 64-octet ASCII secret, the length RFC 7296 Section 2.15 says the
// management interface MUST accept.
const psktermSecret = "correct horse battery staple correct horse battery staple 1234567"

// psktermAuth computes RFC 7296 Section 2.15's AUTH = prf(prf(Shared Secret,
// "Key Pad for IKEv2"), <SignedOctets>) with the standard library's HMAC-SHA2-256,
// independently of Ze's PRF, over exactly the secret octets it is given.
func psktermAuth(secret, signedOctets []byte) []byte {
	pad := hmac.New(sha256.New, secret)
	pad.Write([]byte("Key Pad for IKEv2"))
	key := pad.Sum(nil)
	auth := hmac.New(sha256.New, key)
	auth.Write(signedOctets)
	return auth.Sum(nil)
}

// psktermSA returns the keyed test SA with the PSK mode and the operator secret, and
// the octets it signs as the initiator.
func psktermSA(t *testing.T) (*SA, []byte) {
	t.Helper()
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = psktermSecret
	signedOctets, err := computeSignedOctets(sa, sa.IsInitiator)
	if err != nil {
		t.Fatalf("computeSignedOctets: %v", err)
	}
	return sa, signedOctets
}

// VALIDATES: the configured pre-shared secret is used as the Shared Secret exactly as
// the operator wrote it, with no NUL terminator added, on the sending and the
// receiving side.
//
// METHOD: the AUTH computePSKAuth sends and the AUTH verifyPSKAuth accepts are compared
// with an independent HMAC-SHA2-256 computation over the 64 secret octets alone, and
// with the same computation over the secret plus one 0x00 octet.
//
// RFC requirement: RFC7296-2.15-2 positive -- computePSKAuth sends the AUTH computed over the 64 configured octets with no terminator, which differs from the AUTH over those octets plus 0x00, and verifyPSKAuth accepts the AUTH a peer computed over the same 64 octets.
func TestRFC7296PSKIsUsedWithoutANullTerminator(t *testing.T) {
	sa, signedOctets := psktermSA(t)
	exact := psktermAuth([]byte(psktermSecret), signedOctets)
	terminated := psktermAuth([]byte(psktermSecret+"\x00"), signedOctets)

	sent, err := computePSKAuth(sa)
	if err != nil {
		t.Fatalf("computePSKAuth: %v", err)
	}
	if !bytes.Equal(sent.AuthData, exact) {
		t.Fatalf("sent AUTH %x, want %x, the AUTH over the configured octets alone", sent.AuthData, exact)
	}
	if bytes.Equal(sent.AuthData, terminated) {
		t.Fatal("sent AUTH equals the AUTH over the secret plus a NUL terminator")
	}

	if err := verifyPSKAuth(sa, exact, signedOctets); err != nil {
		t.Fatalf("an AUTH over the configured octets alone was refused: %v", err)
	}
}

// VALIDATES: a peer that adds a NUL terminator to the shared secret computes an AUTH
// Ze refuses, so Ze's receiving side does not add one either.
//
// METHOD: an AUTH computed independently over the secret plus one 0x00 octet is
// offered to verifyPSKAuth.
//
// RFC requirement: RFC7296-2.15-2 negative -- verifyPSKAuth refuses, as an authentication failure, the AUTH a peer computed over the 64 configured octets plus a 0x00 terminator.
func TestRFC7296PSKWithANullTerminatorAddedIsRefused(t *testing.T) {
	sa, signedOctets := psktermSA(t)
	terminated := psktermAuth([]byte(psktermSecret+"\x00"), signedOctets)

	err := verifyPSKAuth(sa, terminated, signedOctets)
	if !errors.Is(err, errAuthFailed) {
		t.Fatalf("an AUTH over the secret plus a NUL terminator: got %v, want %v", err, errAuthFailed)
	}
}
