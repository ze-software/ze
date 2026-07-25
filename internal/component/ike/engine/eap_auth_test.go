// Design: plan/learned/744-ipsec-9-ikev2-eap-nat.md -- AUTH from MSK test

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
)

func TestAuthFromMSK(t *testing.T) {
	var msk [64]byte
	for i := range msk {
		msk[i] = byte(i)
	}
	signedOctets := []byte("test signed octets data for AUTH verification")

	auth, err := ComputeAuthFromMSK(crypto.PRF_HMAC_SHA2_256, msk, signedOctets)
	if err != nil {
		t.Fatalf("ComputeAuthFromMSK: %v", err)
	}
	if len(auth) != 32 {
		t.Fatalf("auth length: got %d, want 32 (SHA-256 PRF output)", len(auth))
	}

	// Same inputs produce same output.
	auth2, err := ComputeAuthFromMSK(crypto.PRF_HMAC_SHA2_256, msk, signedOctets)
	if err != nil {
		t.Fatal(err)
	}
	for i := range auth {
		if auth[i] != auth2[i] {
			t.Fatal("determinism: same inputs produced different AUTH")
		}
	}

	// Verify succeeds.
	if err := VerifyAuthFromMSK(crypto.PRF_HMAC_SHA2_256, msk, signedOctets, auth); err != nil {
		t.Fatalf("VerifyAuthFromMSK: %v", err)
	}

	// Different MSK fails.
	var badMSK [64]byte
	badMSK[0] = 0xFF
	if err := VerifyAuthFromMSK(crypto.PRF_HMAC_SHA2_256, badMSK, signedOctets, auth); err == nil {
		t.Fatal("expected verification failure with wrong MSK")
	}

	// Different signed octets fails.
	if err := VerifyAuthFromMSK(crypto.PRF_HMAC_SHA2_256, msk, []byte("different"), auth); err == nil {
		t.Fatal("expected verification failure with wrong signed octets")
	}
}

func TestAuthFromMSKDifferentPRF(t *testing.T) {
	var msk [64]byte
	for i := range msk {
		msk[i] = byte(i + 100)
	}
	signedOctets := []byte("test data")

	auth256, err := ComputeAuthFromMSK(crypto.PRF_HMAC_SHA2_256, msk, signedOctets)
	if err != nil {
		t.Fatal(err)
	}

	auth512, err := ComputeAuthFromMSK(crypto.PRF_HMAC_SHA2_512, msk, signedOctets)
	if err != nil {
		t.Fatal(err)
	}

	if len(auth256) == len(auth512) {
		same := true
		for i := range auth256 {
			if auth256[i] != auth512[i] {
				same = false
				break
			}
		}
		if same {
			t.Fatal("different PRFs produced same AUTH")
		}
	}
}
