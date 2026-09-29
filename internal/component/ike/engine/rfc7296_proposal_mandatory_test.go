package engine

import (
	"errors"
	"testing"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// propmandNegotiate runs the IKE_SA_INIT responder's selection over one peer proposal:
// the same wireProposalsToIKE and crypto.NegotiateIKE calls handleSAInitRequest
// (responder.go) makes, against the local proposals of group.
func propmandNegotiate(peer []wire.Transform, group ipsec.IKEGroup) (ikecrypto.IKEProposal, error) {
	remote := []wire.Proposal{{Number: 1, ProtocolID: wire.ProtocolIKE, Transforms: peer}}
	return ikecrypto.NegotiateIKE(wireProposalsToIKE(remote), buildIKEProposals(group))
}

// propmandWithout returns the transforms of full minus every transform of one type.
func propmandWithout(full []wire.Transform, transformType uint8) []wire.Transform {
	var kept []wire.Transform
	for _, tr := range full {
		if tr.Type != transformType {
			kept = append(kept, tr)
		}
	}
	return kept
}

// propmandAEADGroup is testIKEGroup with an AEAD cipher, whose IKE proposal carries
// no INTEG transform.
func propmandAEADGroup() ipsec.IKEGroup {
	group := testIKEGroup()
	group.Proposals[0].Encryption = ipsec.EncryptionAES256GCM
	return group
}

// VALIDATES: the responder accepts a peer IKE proposal that carries every mandatory
// Transform Type, and an AEAD proposal that omits INTEG, the type the IKE line marks
// INTEG* (optional with an AEAD cipher).
//
// METHOD: ze's own wire proposals are the peer's offer, run through the responder's
// selection.
//
// RFC requirement: RFC7296-3.3.2-1 positive -- a proposal with ENCR, PRF, INTEG and D-H is chosen with the offered ENCR, INTEG and D-H values, and an AES-GCM proposal with ENCR, PRF and D-H and no INTEG is chosen with integrity NONE.
func TestRFC7296PeerIKEProposalWithTheMandatoryTypesIsAccepted(t *testing.T) {
	group := testIKEGroup()
	full := buildWireIKEProposals(group)[0].Transforms
	chosen, err := propmandNegotiate(full, group)
	if err != nil {
		t.Fatalf("a complete IKE proposal was refused: %v", err)
	}
	if chosen.Encryption.ID == 0 || chosen.Integrity.ID == ikecrypto.AUTH_NONE || chosen.DHGroup.ID != 14 {
		t.Fatalf("chosen = ENCR %d, INTEG %d, D-H %d; want the offered ENCR, a real INTEG and group 14",
			chosen.Encryption.ID, chosen.Integrity.ID, chosen.DHGroup.ID)
	}

	aead := propmandAEADGroup()
	aeadOffer := buildWireIKEProposals(aead)[0].Transforms
	if countTransform(wire.Proposal{Transforms: aeadOffer}, wire.TransformTypeINTG) != 0 {
		t.Fatal("the AEAD proposal carries an INTEG transform, so it cannot show INTEG is optional")
	}
	chosen, err = propmandNegotiate(aeadOffer, aead)
	if err != nil {
		t.Fatalf("an AEAD IKE proposal without INTEG was refused: %v", err)
	}
	if !chosen.Encryption.ID.IsAEAD() || chosen.Integrity.ID != ikecrypto.AUTH_NONE {
		t.Fatalf("AEAD chosen = ENCR %d, INTEG %d; want an AEAD cipher and integrity NONE",
			chosen.Encryption.ID, chosen.Integrity.ID)
	}
}

// VALIDATES: the responder refuses a peer IKE proposal missing ENCR, INTEG (with a
// non-AEAD cipher) or D-H, each a mandatory type on the IKE line of Section 3.3.3.
//
// METHOD: one mandatory transform at a time is removed from ze's complete wire
// proposal, and the rest is run through the responder's selection. The reason is the
// one ikeProposalComplete (crypto/proposal.go) names, so a refusal for any other cause
// does not pass.
//
// RFC requirement: RFC7296-3.3.2-1 negative -- a proposal missing ENCR or INTEG is refused as incomplete, and one missing D-H is refused because its group is NONE; nothing is chosen in each case.
func TestRFC7296PeerIKEProposalMissingAMandatoryTypeIsRefused(t *testing.T) {
	group := testIKEGroup()
	full := buildWireIKEProposals(group)[0].Transforms
	cases := []struct {
		name          string
		transformType uint8
		reason        error
	}{
		{"ENCR", wire.TransformTypeENCR, ikecrypto.ErrProposalIncomplete},
		{"INTEG", wire.TransformTypeINTG, ikecrypto.ErrProposalIncomplete},
		{"D-H", wire.TransformTypeDH, ikecrypto.ErrDHGroupNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			offer := propmandWithout(full, tc.transformType)
			if len(offer) != len(full)-1 {
				t.Fatalf("removing %s left %d of %d transforms, want one fewer", tc.name, len(offer), len(full))
			}
			chosen, err := propmandNegotiate(offer, group)
			if !errors.Is(err, tc.reason) {
				t.Fatalf("a proposal without %s: err = %v, want %v", tc.name, err, tc.reason)
			}
			if chosen != (ikecrypto.IKEProposal{}) {
				t.Fatalf("a proposal without %s chose %+v", tc.name, chosen)
			}
		})
	}
}
