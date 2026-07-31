package crypto

import "testing"

// aeadIKEProposal builds a complete IKE proposal that offers the given AEAD cipher.
// The integrity transform is NONE. RFC 7296 Section 3.3 makes that the correct value
// for an AEAD cipher.
func aeadIKEProposal(id EncryptionID) *IKEProposal {
	return &IKEProposal{
		Number:     1,
		Encryption: EncryptionTransform{ID: id, KeyLength: 256, IsAEAD: true},
		PRF:        PRFTransform{ID: PRF_HMAC_SHA2_256},
		Integrity:  IntegrityTransform{ID: AUTH_NONE},
		DHGroup:    DHGroupTransform{ID: DH_ECP_256},
	}
}

// VALIDATES: every cipher the single AEAD predicate names is accepted by
// ikeProposalComplete when the proposal carries INTEG NONE. The predicate and its
// consumer read the same list.
// PREVENTS: the divergence a second copy creates. ikeProposalComplete once asked a
// private isAEAD that compared against ENCR_AES_GCM_16 alone, so an AEAD cipher added
// to the one documented list was rejected with ErrProposalIncomplete. The registry
// agreement test cannot see this: both of its sides are fed by NewEncryptionTransform.
func TestIKEProposalCompleteAcceptsEveryAEAD(t *testing.T) {
	if len(aeadSaltBytes) == 0 {
		t.Fatal("aeadSaltBytes is empty, so this test proves nothing")
	}
	for id := range aeadSaltBytes {
		if err := ikeProposalComplete(aeadIKEProposal(id)); err != nil {
			t.Errorf("ikeProposalComplete(%s, INTEG NONE) = %v, want nil for an AEAD cipher",
				id, err)
		}
	}
}

// VALIDATES: a non-AEAD cipher offered with INTEG NONE is still refused.
// PREVENTS: a predicate that answers true for every cipher, which would accept an
// AES-CBC proposal carrying no integrity transform at all.
func TestIKEProposalCompleteRefusesNonAEADWithoutIntegrity(t *testing.T) {
	p := aeadIKEProposal(ENCR_AES_CBC)
	p.Encryption.IsAEAD = false
	if err := ikeProposalComplete(p); err == nil {
		t.Error("ikeProposalComplete(aes-cbc, INTEG NONE) = nil, want ErrProposalIncomplete")
	}
}

// VALIDATES: the ESP key-material length of every AEAD cipher carries that cipher's
// own salt, read from the registry rather than from one number shared by all of them.
// PREVENTS: an AEAD added to the registry whose KEYMAT is sized with another cipher's
// salt, which shortens the key and moves the second direction's offset.
func TestEncKeyMaterialLenSaltsEveryAEAD(t *testing.T) {
	for id, salt := range aeadSaltBytes {
		enc := EncryptionTransform{ID: id, KeyLength: 256}
		if got, want := encKeyMaterialLen(enc), 32+salt; got != want {
			t.Errorf("encKeyMaterialLen(%s, 256 bits) = %d, want %d (32 octet key plus %d octet salt)",
				id, got, want, salt)
		}
	}
}

// VALIDATES: AES-GCM takes a four octet salt. RFC 4106 Section 8.1: "The size of the
// KEYMAT for the AES-GCM-ESP MUST be four octets longer than is needed for the
// associated AES key."
// PREVENTS: a registry edit that changes the one salt length Ze has a citation for.
// The per-algorithm test above reads its expected value from the same map, so it
// cannot pin any number on its own.
func TestAESGCMSaltIsFourOctets(t *testing.T) {
	salt, ok := aeadSaltBytes[ENCR_AES_GCM_16]
	if !ok {
		t.Fatal("ENCR_AES_GCM_16 is absent from aeadSaltBytes, so it is not AEAD")
	}
	if salt != 4 {
		t.Errorf("AES-GCM salt = %d octets, want 4 (RFC 4106 Section 8.1)", salt)
	}
}
