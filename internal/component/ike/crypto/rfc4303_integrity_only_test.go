// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- ESP proposal negotiation
// RFC: rfc/short/rfc4303.md -- integrity-only ESP negotiable (Section 1)
package crypto

import (
	"errors"
	"testing"
)

// nullESP builds an ESP proposal of the null cipher beside one integrity transform.
func nullESP(integ IntegrityTransform) ESPProposal {
	return ESPProposal{Number: 1, Encryption: NewEncryptionTransform(ENCR_NULL, 0), Integrity: integ}
}

// VALIDATES: RFC 4303 Section 1, "Integrity-only ESP MUST be offered as a service
// selection option, e.g., it must be negotiable in SA management protocols". Method: for
// every integrity transform this build implements, the initiator's check of the
// accepted ESP proposal (NegotiateESP) is driven with ENCR_NULL beside that transform,
// against a local policy holding the same suite, so a matcher that admits the null
// cipher for one integrity algorithm only, or for none, goes red.
// PREVENTS: an ESP matcher that refuses a peer's integrity-only answer, which would
// leave integrity-only ESP configurable but never negotiated.
//
// RFC requirement: RFC4303-1-2 positive -- for every implemented integrity transform, NegotiateESP accepts an ENCR_NULL proposal carrying no Key Length attribute beside that transform against a local policy of the same suite, and returns the null cipher and the integrity transform unchanged.
func TestRFC4303IntegrityOnlyESPIsAccepted(t *testing.T) {
	names := SupportedESPIntegrityNames()
	if len(names) == 0 {
		t.Fatal("the integrity registry is empty, so nothing below would run")
	}
	for _, name := range names {
		integ, err := LookupESPIntegrity(name)
		if err != nil {
			t.Fatalf("LookupESPIntegrity(%s): %v", name, err)
		}
		offer := nullESP(integ)
		got, err := NegotiateESP([]ESPProposal{offer}, []ESPProposal{nullESP(integ)})
		if err != nil {
			t.Errorf("NegotiateESP(ENCR_NULL + %s) = %v, want acceptance", name, err)
			continue
		}
		if got.Encryption.ID != ENCR_NULL || got.Encryption.KeyLength != 0 {
			t.Errorf("NegotiateESP(ENCR_NULL + %s) chose cipher %+v, want ENCR_NULL with no key", name, got.Encryption)
		}
		if got.Integrity.ID != integ.ID {
			t.Errorf("NegotiateESP(ENCR_NULL + %s) chose integrity %d, want %d", name, got.Integrity.ID, integ.ID)
		}
	}
}

// VALIDATES: the limits of the null cipher Ze accepts for ESP. RFC 4303 Section 3.2:
// "both algorithms MUST NOT be simultaneously NULL", so ENCR_NULL beside AUTH_NONE is
// refused. The null cipher takes no key, so an offer carrying a Key Length attribute is
// refused. A peer answering ENCR_NULL to a local policy of AES-CBC answers a suite this
// side never sent. RFC 7296 Section 5 keeps ENCR_NULL out of the IKE SA, which
// TestRFC7296NegotiationRefusesNullIntegrityAndNullCipher proves.
// PREVENTS: the ESP admission of ENCR_NULL widening into ESP with no service, a keyed
// null cipher, or a suite the initiator did not offer.
//
// RFC requirement: RFC4303-1-2 negative -- the null cipher NegotiateESP admits is integrity-only ESP and nothing wider: it refuses ENCR_NULL beside AUTH_NONE with ErrProposalIncomplete, an ENCR_NULL offer carrying a Key Length attribute of 128 with ErrNoProposalChosen, and an ENCR_NULL answer against a local policy of AES-CBC with SHA-256 with ErrNoProposalChosen.
//
// RFC requirement: RFC4303-3.2-1 negative -- NegotiateESP refuses an ESP proposal pairing ENCR_NULL with AUTH_NONE, which selects neither confidentiality nor integrity, with ErrProposalIncomplete.
func TestIntegrityOnlyESPRefusals(t *testing.T) {
	sha256, err := LookupIntegrity(hashNameSHA256)
	if err != nil {
		t.Fatalf("LookupIntegrity: %v", err)
	}
	noIntegrity := nullESP(IntegrityTransform{ID: AUTH_NONE})
	if _, err := NegotiateESP([]ESPProposal{noIntegrity}, []ESPProposal{noIntegrity}); !errors.Is(err, ErrProposalIncomplete) {
		t.Errorf("NegotiateESP(ENCR_NULL + AUTH_NONE) = %v, want %v", err, ErrProposalIncomplete)
	}

	keyed := nullESP(sha256)
	keyed.Encryption.KeyLength = 128
	if _, err := NegotiateESP([]ESPProposal{keyed}, []ESPProposal{nullESP(sha256)}); !errors.Is(err, ErrNoProposalChosen) {
		t.Errorf("NegotiateESP(ENCR_NULL with Key Length 128) = %v, want %v", err, ErrNoProposalChosen)
	}

	aesPolicy := ESPProposal{Number: 1, Encryption: NewEncryptionTransform(ENCR_AES_CBC, 256), Integrity: sha256}
	if _, err := NegotiateESP([]ESPProposal{nullESP(sha256)}, []ESPProposal{aesPolicy}); !errors.Is(err, ErrNoProposalChosen) {
		t.Errorf("NegotiateESP(ENCR_NULL against AES-CBC policy) = %v, want %v", err, ErrNoProposalChosen)
	}
}
