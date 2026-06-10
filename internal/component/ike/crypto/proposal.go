// Design: plan/learned/739-ipsec-6-ikev2-crypto.md -- IKE/ESP proposal negotiation

package crypto

import "errors"

var ErrNoProposalChosen = errors.New("NO_PROPOSAL_CHOSEN")

// IKEProposal represents a single IKE SA crypto proposal.
type IKEProposal struct {
	Number     uint16
	Encryption EncryptionTransform
	PRF        PRFTransform
	Integrity  IntegrityTransform
	DHGroup    DHGroupTransform
}

// ESPProposal represents a single ESP/Child SA crypto proposal.
type ESPProposal struct {
	Number     uint16
	Encryption EncryptionTransform
	Integrity  IntegrityTransform
}

// NegotiateIKE selects the first remote proposal acceptable to local policy.
// RFC 7296 Section 2.7: responder picks exactly one proposal.
func NegotiateIKE(remote, local []IKEProposal) (IKEProposal, error) {
	for _, r := range remote {
		for _, l := range local {
			if r.Encryption.ID == l.Encryption.ID &&
				r.Encryption.KeyLength == l.Encryption.KeyLength &&
				r.PRF.ID == l.PRF.ID &&
				r.Integrity.ID == l.Integrity.ID &&
				r.DHGroup.ID == l.DHGroup.ID {
				chosen := l
				chosen.Number = r.Number
				return chosen, nil
			}
		}
	}
	return IKEProposal{}, ErrNoProposalChosen
}

// NegotiateESP selects the first remote ESP proposal acceptable to local policy.
func NegotiateESP(remote, local []ESPProposal) (ESPProposal, error) {
	for _, r := range remote {
		for _, l := range local {
			if r.Encryption.ID == l.Encryption.ID &&
				r.Encryption.KeyLength == l.Encryption.KeyLength &&
				r.Integrity.ID == l.Integrity.ID {
				return r, nil
			}
		}
	}
	return ESPProposal{}, ErrNoProposalChosen
}
