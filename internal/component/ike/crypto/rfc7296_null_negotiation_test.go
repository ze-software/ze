// VALIDATES: RFC 7296 Section 5 at the negotiation itself: NegotiateIKE (responder) and
// VerifyAcceptedIKE (initiator) refuse an IKE proposal whose integrity is NONE beside a
// non-AEAD cipher, or whose cipher is ENCR_NULL, even when local policy names the same
// value.
// PREVENTS: an IKE SA negotiated without integrity protection or without encryption.
package crypto

import (
	"errors"
	"testing"
)

// encrNull is ENCR_NULL, Transform ID 11 of Transform Type 1 (RFC 7296 Section 3.3.2).
// Ze names no constant for it because it never negotiates it.
const encrNull EncryptionID = 11

// nullIKEProposals answers the two forbidden forms of an otherwise complete IKE proposal:
// AES-CBC with integrity NONE, and ENCR_NULL with HMAC-SHA2-256-128. Each one is used as
// both the peer's offer and local policy, so that nothing but the forbidden value can
// cause a refusal.
func nullIKEProposals() map[string]IKEProposal {
	integNone := ikePolicy()[0]
	integNone.Number = 1
	integNone.Integrity = IntegrityTransform{ID: AUTH_NONE}

	encNull := ikePolicy()[0]
	encNull.Number = 1
	encNull.Encryption = EncryptionTransform{ID: encrNull}
	return map[string]IKEProposal{
		"AUTH_NONE beside AES-CBC": integNone,
		"ENCR_NULL":                encNull,
	}
}

// RFC requirement: RFC7296-5-2 negative -- the peer's offer is refused on both sides of the
// negotiation: NegotiateIKE (ze responding) and VerifyAcceptedIKE (ze initiating and
// reading the accepted proposal) each refuse an IKE proposal carrying AUTH_NONE beside
// AES-CBC with ErrProposalIncomplete, and one carrying ENCR_NULL with
// ErrTransformUnspecified, with local policy holding the same values.
// RFC requirement: RFC7296-5-2 positive -- the same proposal with HMAC-SHA2-256-128 and
// AES-CBC is negotiated by both, so the refusal is caused by NONE or NULL alone.
//
// RFC 7296 Section 5: "implementations MUST NOT negotiate NONE as the IKE integrity
// protection algorithm or ENCR_NULL as the IKE encryption algorithm." An AEAD cipher
// carries its own integrity, so AUTH_NONE beside AES-GCM is not this case.
func TestRFC7296NegotiationRefusesNullIntegrityAndNullCipher(t *testing.T) {
	// The refusal is the one the null transform causes, not any error: AUTH_NONE beside
	// a non-AEAD cipher is an IKE proposal missing its mandatory integrity transform
	// (ikeProposalComplete), and ENCR_NULL is a cipher this implementation does not
	// specify for the IKE SA (acceptEncryption, specifiedEncryption).
	want := map[string]error{
		"AUTH_NONE beside AES-CBC": ErrProposalIncomplete,
		"ENCR_NULL":                ErrTransformUnspecified,
	}
	for name, p := range nullIKEProposals() {
		offer := []IKEProposal{p}
		policy := []IKEProposal{p}
		got, err := NegotiateIKE(offer, policy)
		if !errors.Is(err, want[name]) {
			t.Errorf("NegotiateIKE(%s) = %+v, %v; want %v", name, got, err, want[name])
		}
		got, err = VerifyAcceptedIKE(offer, policy)
		if !errors.Is(err, want[name]) {
			t.Errorf("VerifyAcceptedIKE(%s) = %+v, %v; want %v", name, got, err, want[name])
		}
	}

	offer := []IKEProposal{ikeOffer(1)}
	if _, err := NegotiateIKE(offer, ikePolicy()); err != nil {
		t.Errorf("NegotiateIKE(AES-CBC with HMAC-SHA2-256-128) = %v, want acceptance", err)
	}
	if _, err := VerifyAcceptedIKE(offer, ikePolicy()); err != nil {
		t.Errorf("VerifyAcceptedIKE(AES-CBC with HMAC-SHA2-256-128) = %v, want acceptance", err)
	}
}
