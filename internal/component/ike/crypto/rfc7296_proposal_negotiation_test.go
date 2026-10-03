package crypto

import (
	"errors"
	"testing"
)

// RFC requirement: RFC7296-2.7-1 positive -- NegotiateIKE (proposal.go:27) selects exactly
// one proposal whose ENCR/PRF/INTEG/DH transforms all match a local proposal, returning that
// single combination (the responder picks exactly one transform of each type).
func TestProposalNegotiationFirstMatch(t *testing.T) {
	remote := []IKEProposal{
		{
			Number:     1,
			Encryption: EncryptionTransform{ID: ENCR_AES_GCM_16, KeyLength: 256, IsAEAD: true},
			PRF:        PRFTransform{ID: PRF_HMAC_SHA2_256},
			Integrity:  IntegrityTransform{ID: AUTH_NONE},
			DHGroup:    DHGroupTransform{ID: DH_ECP_256},
		},
		{
			Number:     2,
			Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128},
			PRF:        PRFTransform{ID: PRF_HMAC_SHA2_256},
			Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128},
			DHGroup:    DHGroupTransform{ID: DH_MODP_2048},
		},
		{
			Number:     3,
			Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 256},
			PRF:        PRFTransform{ID: PRF_HMAC_SHA2_384},
			Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_384_192},
			DHGroup:    DHGroupTransform{ID: DH_ECP_384},
		},
	}

	local := []IKEProposal{
		{
			Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128},
			PRF:        PRFTransform{ID: PRF_HMAC_SHA2_256},
			Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128},
			DHGroup:    DHGroupTransform{ID: DH_MODP_2048},
		},
		{
			Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 256},
			PRF:        PRFTransform{ID: PRF_HMAC_SHA2_384},
			Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_384_192},
			DHGroup:    DHGroupTransform{ID: DH_ECP_384},
		},
	}

	chosen, err := NegotiateIKE(remote, local)
	if err != nil {
		t.Fatalf("NegotiateIKE: %v", err)
	}
	if chosen.Number != 2 {
		t.Errorf("chose proposal %d, want 2 (first match)", chosen.Number)
	}
	if chosen.Encryption.ID != ENCR_AES_CBC || chosen.Encryption.KeyLength != 128 {
		t.Errorf("wrong encryption: ID=%d KeyLength=%d", chosen.Encryption.ID, chosen.Encryption.KeyLength)
	}
}

// RFC requirement: RFC7296-2.7-1 negative -- when no proposal offers a transform combination
// the local policy accepts, NegotiateIKE (proposal.go:41) returns ErrNoProposalChosen so the
// responder rejects all with NO_PROPOSAL_CHOSEN rather than accepting a partial match.
func TestProposalNegotiationNoMatch(t *testing.T) {
	remote := []IKEProposal{
		{
			Number:     1,
			Encryption: EncryptionTransform{ID: ENCR_AES_GCM_16, KeyLength: 256, IsAEAD: true},
			PRF:        PRFTransform{ID: PRF_HMAC_SHA2_512},
			Integrity:  IntegrityTransform{ID: AUTH_NONE},
			DHGroup:    DHGroupTransform{ID: DH_ECP_384},
		},
	}

	local := []IKEProposal{
		{
			Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128},
			PRF:        PRFTransform{ID: PRF_HMAC_SHA2_256},
			Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128},
			DHGroup:    DHGroupTransform{ID: DH_MODP_2048},
		},
	}

	_, err := NegotiateIKE(remote, local)
	if !errors.Is(err, ErrNoProposalChosen) {
		t.Errorf("NegotiateIKE = %v, want ErrNoProposalChosen", err)
	}
}

func TestProposalNegotiationESP(t *testing.T) {
	remote := []ESPProposal{
		{
			Number:     1,
			Encryption: EncryptionTransform{ID: ENCR_AES_GCM_16, KeyLength: 128, IsAEAD: true},
			Integrity:  IntegrityTransform{ID: AUTH_NONE},
		},
		{
			Number:     2,
			Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 256},
			Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128},
		},
	}

	local := []ESPProposal{
		{
			Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 256},
			Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128},
		},
	}

	chosen, err := NegotiateESP(remote, local)
	if err != nil {
		t.Fatalf("NegotiateESP: %v", err)
	}
	if chosen.Number != 2 {
		t.Errorf("chose proposal %d, want 2", chosen.Number)
	}
}

func TestProposalNegotiationESPNoMatch(t *testing.T) {
	remote := []ESPProposal{
		{
			Number:     1,
			Encryption: EncryptionTransform{ID: ENCR_AES_GCM_16, KeyLength: 128, IsAEAD: true},
			Integrity:  IntegrityTransform{ID: AUTH_NONE},
		},
	}

	local := []ESPProposal{
		{
			Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128},
			Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128},
		},
	}

	_, err := NegotiateESP(remote, local)
	if !errors.Is(err, ErrNoProposalChosen) {
		t.Errorf("NegotiateESP = %v, want ErrNoProposalChosen", err)
	}
}

func TestProposalNegotiationEmptyRemote(t *testing.T) {
	local := []IKEProposal{
		{
			Encryption: EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128},
			PRF:        PRFTransform{ID: PRF_HMAC_SHA2_256},
			Integrity:  IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128},
			DHGroup:    DHGroupTransform{ID: DH_MODP_2048},
		},
	}

	_, err := NegotiateIKE(nil, local)
	if !errors.Is(err, ErrNoProposalChosen) {
		t.Errorf("NegotiateIKE(empty remote) = %v, want ErrNoProposalChosen", err)
	}
}
