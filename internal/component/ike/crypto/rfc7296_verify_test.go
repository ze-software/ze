// VALIDATES: the initiator's check of an accepted offer against the offers it sent. The
// check refuses an encryption key length this side never sent. It refuses a length above
// the configured one and a length below it.
// PREVENTS: one acceptance rule for both roles. A responder can accept a key that supplies
// greater security. An initiator cannot. The peer returns the attribute unchanged, so an
// unsent key length names a suite this side never offered.
package crypto

import (
	"errors"
	"testing"
)

// verSentIKE is the single IKE proposal this side sends. It names AES-CBC with a 128-bit
// key, which is what an operator configuration of aes128 produces.
func verSentIKE() []IKEProposal { return ikePolicy() }

// verAnswer builds the accepted IKE offer a responder returns, with one key length.
func verAnswer(keyLen uint16) IKEProposal {
	p := ikeOffer(1)
	p.Encryption = EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: keyLen}
	return p
}

// verSentESP is the single ESP proposal this side sends.
func verSentESP() []ESPProposal {
	return []ESPProposal{{
		Number:     1,
		Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128},
		Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128},
	}}
}

// RFC requirement: RFC7296-3.3.6-3 negative -- RFC 7296 Section 3.3.6: "The initiator of an
// exchange MUST check that the accepted offer is consistent with one of its proposals".
// It MUST stop the exchange when the offer does not agree.
// An answer of 256 bits against a sent 128 is refused.
// An answer of 64 bits is refused too. RFC 7296 Section 3.3.5 states that the attribute
// returns unchanged, so an initiator that accepts several key lengths sends one transform
// for each. A longer key is a suite this side never sent.
// RFC requirement: RFC7296-3.3.6-3 positive -- the key length this side did send is accepted. The
// refusal therefore names the unsent attribute rather than blocks every answer. A second
// proposal that names the longer key makes the same 256-bit answer consistent again.
func TestVerInitiatorRefusesUnsentIKEKeyLength(t *testing.T) {
	sent := verSentIKE()

	if _, err := VerifyAcceptedIKE([]IKEProposal{verAnswer(256)}, sent); !errors.Is(err, ErrNoProposalChosen) {
		t.Errorf("VerifyAcceptedIKE(256-bit answer to a 128-bit offer) = %v, want ErrNoProposalChosen", err)
	}
	if _, err := VerifyAcceptedIKE([]IKEProposal{verAnswer(64)}, sent); !errors.Is(err, ErrNoProposalChosen) {
		t.Errorf("VerifyAcceptedIKE(64-bit answer to a 128-bit offer) = %v, want ErrNoProposalChosen", err)
	}

	chosen, err := VerifyAcceptedIKE([]IKEProposal{verAnswer(128)}, sent)
	if err != nil {
		t.Fatalf("VerifyAcceptedIKE(the key length we sent) = %v, want acceptance", err)
	}
	if chosen.Encryption.KeyLength != 128 {
		t.Errorf("accepted key length = %d, want the 128 we sent", chosen.Encryption.KeyLength)
	}

	widened := append(verSentIKE(), IKEProposal{
		Number:     2,
		Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 256},
		PRF:        PRFTransform{ID: PRF_HMAC_SHA2_256},
		Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128},
		DHGroup:    DHGroupTransform{ID: DH_MODP_2048},
	})
	if _, err := VerifyAcceptedIKE([]IKEProposal{verAnswer(256)}, widened); err != nil {
		t.Errorf("VerifyAcceptedIKE(256-bit answer we did send) = %v, want acceptance", err)
	}
}

// RFC requirement: RFC7296-3.3.6-3 negative -- the same rule covers the Child SA answer. NegotiateESP
// serves the initiator's check alone, so it refuses an ESP key length this side never sent.
// RFC requirement: RFC7296-3.3.6-3 positive -- the ESP key length this side sent is accepted, and the
// accepted proposal carries the peer's own attributes unmodified.
func TestVerInitiatorRefusesUnsentESPKeyLength(t *testing.T) {
	sent := verSentESP()

	longer := sent[0]
	longer.Encryption.KeyLength = 256
	if _, err := NegotiateESP([]ESPProposal{longer}, sent); !errors.Is(err, ErrNoProposalChosen) {
		t.Errorf("NegotiateESP(256-bit answer to a 128-bit offer) = %v, want ErrNoProposalChosen", err)
	}

	chosen, err := NegotiateESP([]ESPProposal{sent[0]}, sent)
	if err != nil {
		t.Fatalf("NegotiateESP(the key length we sent) = %v, want acceptance", err)
	}
	if chosen.Encryption.KeyLength != 128 {
		t.Errorf("accepted ESP key length = %d, want the 128 we sent", chosen.Encryption.KeyLength)
	}
}

// VALIDATES: the responder keeps the Section 3.3.5 allowance, and it reports every use of
// it. PolicyKeyLength holds the configured length when a longer key is accepted, and zero
// when the accepted length is the configured one.
// PREVENTS: a silent upgrade. An operator who configures aes128 and gets AES-256 has no
// record of the change without this field.
func TestVerResponderReportsKeyLengthUpgrade(t *testing.T) {
	upgraded, err := NegotiateIKE([]IKEProposal{verAnswer(256)}, verSentIKE())
	if err != nil {
		t.Fatalf("NegotiateIKE(256-bit offer against a 128-bit policy) = %v, want acceptance", err)
	}
	if upgraded.Encryption.KeyLength != 256 {
		t.Errorf("accepted key length = %d, want the offered 256", upgraded.Encryption.KeyLength)
	}
	if upgraded.PolicyKeyLength != 128 {
		t.Errorf("PolicyKeyLength = %d, want the configured 128", upgraded.PolicyKeyLength)
	}

	same, err := NegotiateIKE([]IKEProposal{verAnswer(128)}, verSentIKE())
	if err != nil {
		t.Fatalf("NegotiateIKE(128-bit offer against a 128-bit policy) = %v, want acceptance", err)
	}
	if same.PolicyKeyLength != 0 {
		t.Errorf("PolicyKeyLength = %d, want 0 when the accepted key is the configured one", same.PolicyKeyLength)
	}
}
