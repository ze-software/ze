package engine

import (
	"testing"

	ikecrypto "codeberg.org/thomas-mangin/ze/internal/component/ike/crypto"
	"codeberg.org/thomas-mangin/ze/internal/component/ipsec"
)

func testSAWithKeys(t *testing.T) *SA {
	t.Helper()

	peer := testPeer()
	ikeGroup := testIKEGroup()

	sa, err := newInitiatorSA("test-peer", peer, ikeGroup, ipsec.ESPGroup{})
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}

	sa.RemoteNonce = make([]byte, 32)
	for i := range sa.RemoteNonce {
		sa.RemoteNonce[i] = byte(i)
	}
	sa.ResponderSPI = [8]byte{9, 8, 7, 6, 5, 4, 3, 2}
	sa.InitiatorSAInitMsg = make([]byte, 28)
	sa.ResponderSAInitMsg = make([]byte, 28)

	sa.Proposal = ikecrypto.IKEProposal{
		Encryption: ikecrypto.EncryptionTransform{ID: ikecrypto.ENCR_AES_CBC, KeyLength: 256},
		PRF:        ikecrypto.PRFTransform{ID: ikecrypto.PRF_HMAC_SHA2_256, KeyLength: 32, OutputLength: 32},
		Integrity:  ikecrypto.IntegrityTransform{ID: ikecrypto.AUTH_HMAC_SHA2_256_128, KeyLength: 32, TruncatedLength: 16},
		DHGroup:    ikecrypto.DHGroupTransform{ID: ikecrypto.DH_MODP_2048},
	}

	skeyseed, err := ikecrypto.DeriveSKEYSEED(
		sa.Proposal.PRF.ID,
		sa.LocalNonce, sa.RemoteNonce,
		make([]byte, 32), // dummy shared secret
	)
	if err != nil {
		t.Fatalf("DeriveSKEYSEED: %v", err)
	}

	skKeys, err := ikecrypto.DeriveSKKeys(
		sa.Proposal.PRF.ID, skeyseed,
		sa.LocalNonce, sa.RemoteNonce,
		sa.InitiatorSPI[:], sa.ResponderSPI[:],
		sa.Proposal.Encryption, sa.Proposal.Integrity,
	)
	if err != nil {
		t.Fatalf("DeriveSKKeys: %v", err)
	}
	sa.SKKeys = skKeys

	return sa
}

func TestAuthPSKCompute(t *testing.T) {
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = "test-secret-key"

	auth, err := computePSKAuth(sa)
	if err != nil {
		t.Fatalf("computePSKAuth: %v", err)
	}

	if auth.AuthMethod != 2 { // AuthMethodPSK
		t.Fatalf("expected auth method 2 (PSK), got %d", auth.AuthMethod)
	}
	if len(auth.AuthData) == 0 {
		t.Fatal("expected non-empty auth data")
	}
}

func TestAuthPSKVerify(t *testing.T) {
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = "test-secret-key"

	auth, err := computePSKAuth(sa)
	if err != nil {
		t.Fatalf("computePSKAuth: %v", err)
	}

	// Verify using the same signed octets the signer used (initiator perspective).
	signedOctets, err := computeSignedOctets(sa, sa.IsInitiator)
	if err != nil {
		t.Fatalf("computeSignedOctets: %v", err)
	}

	err = verifyPSKAuth(sa, auth.AuthData, signedOctets)
	if err != nil {
		t.Fatalf("verifyPSKAuth should succeed: %v", err)
	}
}

func TestAuthPSKVerifyFailsWithWrongKey(t *testing.T) {
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = "correct-key"

	auth, err := computePSKAuth(sa)
	if err != nil {
		t.Fatalf("computePSKAuth: %v", err)
	}

	// Verify with wrong key: recompute signed octets with the wrong PSK.
	sa.PeerCfg.Auth.PSK = "wrong-key"
	signedOctets, err := computeSignedOctets(sa, sa.IsInitiator)
	if err != nil {
		t.Fatalf("computeSignedOctets: %v", err)
	}

	err = verifyPSKAuth(sa, auth.AuthData, signedOctets)
	if err == nil {
		t.Fatal("verifyPSKAuth should fail with wrong key")
	}
}

func TestAuthPSKNoPSKConfigured(t *testing.T) {
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = ""

	_, err := computePSKAuth(sa)
	if err == nil {
		t.Fatal("computePSKAuth should fail with empty PSK")
	}
}

func TestContainsHashAlgo(t *testing.T) {
	algos := []uint16{2, 3, 4}
	if !containsHashAlgo(algos, 2) {
		t.Fatal("should contain 2")
	}
	if !containsHashAlgo(algos, 4) {
		t.Fatal("should contain 4")
	}
	if containsHashAlgo(algos, 1) {
		t.Fatal("should not contain 1")
	}
	if !containsHashAlgo(nil, 99) {
		t.Fatal("empty list should allow all")
	}
}
