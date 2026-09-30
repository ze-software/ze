// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- proposal selection
// Related: initiator.go -- appendIKECombinations reads a peer's IKE offer
// Related: responder.go -- espProposalMatches reads a peer's ESP offer
//
// VALIDATES: RFC 7296 Section 3.3.6, "if the responder receives a transform that it does
// not understand, or one that contains a Transform Attribute it does not understand, it
// MUST consider this transform unacceptable; other transforms with the same Transform
// Type are processed as usual". Each clause is driven through the two readers the
// responder runs on a peer's SA payload: wireProposalsToIKE then crypto.NegotiateIKE
// (IKE_SA_INIT, IKE SA rekey), and matchOfferedESPProposal (Child SA).
//
// PREVENTS: a transform carrying an attribute Ze does not understand being accepted as
// if the attribute were absent. The wire parser keeps such an attribute precisely so
// that negotiation can refuse the transform.
package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// utUnknownAttr is a Transform Attribute type RFC 7296 Section 3.3.5 does not define.
// Key Length (14) is the only attribute type the RFC defines.
const utUnknownAttr uint16 = 99

// utUnknownEncrID is an encryption Transform ID with no specification in Ze.
const utUnknownEncrID uint16 = 1024

// utWithUnknownAttr returns a copy of t that also carries an attribute Ze does not
// understand. The copy owns its attribute slice, so the source transform is unchanged.
func utWithUnknownAttr(t wire.Transform) wire.Transform {
	attrs := make([]wire.TransformAttr, 0, len(t.Attrs)+1)
	attrs = append(attrs, t.Attrs...)
	attrs = append(attrs, wire.TransformAttr{Type: utUnknownAttr, Value: 1})
	t.Attrs = attrs
	return t
}

// utIKEOffer builds the wire IKE offer of the test policy, and REPLACES the transforms of
// one type in the first proposal with those given, in the order given.
func utIKEOffer(transformType uint8, replacement ...wire.Transform) []wire.Proposal {
	proposals := buildWireIKEProposals(testIKEGroup())
	kept := make([]wire.Transform, 0, len(proposals[0].Transforms)+len(replacement))
	for _, t := range proposals[0].Transforms {
		if t.Type != transformType {
			kept = append(kept, t)
		}
	}
	kept = append(kept, replacement...)
	proposals[0].Transforms = kept
	return proposals
}

// utPolicyTransform returns the transform of one type the test policy offers.
func utPolicyTransform(t *testing.T, transformType uint8) wire.Transform {
	t.Helper()
	for _, tr := range buildWireIKEProposals(testIKEGroup())[0].Transforms {
		if tr.Type == transformType {
			return tr
		}
	}
	t.Fatalf("the test policy offers no transform of type %d", transformType)
	return wire.Transform{}
}

// utESPOffer is one ESP proposal that carries exactly the transforms given.
func utESPOffer(transforms ...wire.Transform) *wire.PayloadSA {
	return &wire.PayloadSA{Proposals: []wire.Proposal{{
		Number:     1,
		ProtocolID: wire.ProtocolESP,
		Transforms: transforms,
	}}}
}

// utESPPolicy is the configured ESP proposal the ESP cases match against, with the
// ENCR and INTEG transforms it resolves to.
func utESPPolicy(t *testing.T) (ipsec.ESPProposal, wire.Transform, wire.Transform) {
	t.Helper()
	our := ipsec.ESPProposal{Number: 1, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256}
	enc := lookupEncryption(our.Encryption)
	integ := lookupIntegrity(our.Hash)
	if enc.ID == 0 || integ.ID == 0 {
		t.Fatal("the configured ESP proposal resolves to no algorithm")
	}
	return our, espEncTransform(uint16(enc.ID), enc.KeyLength), wire.Transform{Type: wire.TransformTypeINTG, ID: uint16(integ.ID)}
}

// RFC requirement: RFC7296-3.3.6-5 positive -- "other transforms with the same Transform
// Type are processed as usual": an encryption transform Ze does not understand (an
// unknown Transform ID) and one carrying a Transform Attribute Ze does not understand
// each sit beside a sibling of the same type that Ze runs, in both orders; the IKE offer
// is accepted with the understood sibling and the ESP offer matches.
func TestRFC7296UnacceptableTransformLeavesSiblingsOnOffer(t *testing.T) {
	local := buildIKEProposals(testIKEGroup())
	encr := utPolicyTransform(t, wire.TransformTypeENCR)
	unknownID := wire.Transform{Type: wire.TransformTypeENCR, ID: utUnknownEncrID}

	ikeCases := map[string][][]wire.Transform{
		"unknown transform id": {{unknownID, encr}, {encr, unknownID}},
		"unknown attribute":    {{utWithUnknownAttr(encr), encr}, {encr, utWithUnknownAttr(encr)}},
	}
	for name, orders := range ikeCases {
		for i, order := range orders {
			chosen, err := crypto.NegotiateIKE(wireProposalsToIKE(utIKEOffer(wire.TransformTypeENCR, order...)), local)
			if err != nil {
				t.Errorf("IKE %s, order %d: refused with %v, want the understood sibling accepted", name, i, err)
				continue
			}
			if uint16(chosen.Encryption.ID) != encr.ID {
				t.Errorf("IKE %s, order %d: selected encryption %d, want %d", name, i, chosen.Encryption.ID, encr.ID)
			}
		}
	}

	our, espEncr, espInteg := utESPPolicy(t)
	espUnknownID := wire.Transform{Type: wire.TransformTypeENCR, ID: utUnknownEncrID}
	espCases := map[string][][]wire.Transform{
		"unknown transform id": {{espUnknownID, espEncr, espInteg}, {espEncr, espUnknownID, espInteg}},
		"unknown attribute":    {{utWithUnknownAttr(espEncr), espEncr, espInteg}, {espEncr, utWithUnknownAttr(espEncr), espInteg}},
		"unknown integrity attribute": {
			{espEncr, utWithUnknownAttr(espInteg), espInteg},
			{espEncr, espInteg, utWithUnknownAttr(espInteg)},
		},
	}
	for name, orders := range espCases {
		for i, order := range orders {
			if _, ok := matchOfferedESPProposal(utESPOffer(order...), our, espDHMatch{Unbound: true}); !ok {
				t.Errorf("ESP %s, order %d: refused, want the understood sibling matched", name, i)
			}
		}
	}
}

// RFC requirement: RFC7296-3.3.6-5 negative -- "it MUST consider this transform
// unacceptable": a transform Ze does not understand (an unknown encryption Transform
// ID), and each IKE transform (ENCR, PRF, INTEG, D-H) and each ESP transform (ENCR,
// INTEG, ESN) of the configured suite carrying a Transform Attribute Ze does not
// understand, offered with no sibling of its type, leaves the proposal refused
// (NO_PROPOSAL_CHOSEN for IKE, no ESP match). The same offers without the attribute are
// accepted, so the attribute alone is what the refusal answers.
func TestRFC7296UnacceptableTransformAloneRefusesTheProposal(t *testing.T) {
	local := buildIKEProposals(testIKEGroup())

	unknownID := wire.Transform{Type: wire.TransformTypeENCR, ID: utUnknownEncrID}
	if _, err := crypto.NegotiateIKE(wireProposalsToIKE(utIKEOffer(wire.TransformTypeENCR, unknownID)), local); !errors.Is(err, crypto.ErrNoProposalChosen) {
		t.Errorf("IKE offer whose only ENCR is unknown id %d = %v, want ErrNoProposalChosen", utUnknownEncrID, err)
	}

	for _, transformType := range []uint8{wire.TransformTypeENCR, wire.TransformTypePRF, wire.TransformTypeINTG, wire.TransformTypeDH} {
		understood := utPolicyTransform(t, transformType)
		if _, err := crypto.NegotiateIKE(wireProposalsToIKE(utIKEOffer(transformType, understood)), local); err != nil {
			t.Fatalf("IKE control, type %d without the attribute: %v, want accepted", transformType, err)
		}
		offer := utIKEOffer(transformType, utWithUnknownAttr(understood))
		if _, err := crypto.NegotiateIKE(wireProposalsToIKE(offer), local); !errors.Is(err, crypto.ErrNoProposalChosen) {
			t.Errorf("IKE offer whose only type-%d transform carries attribute %d = %v, want ErrNoProposalChosen", transformType, utUnknownAttr, err)
		}
	}

	our, espEncr, espInteg := utESPPolicy(t)
	esn := wire.Transform{Type: wire.TransformTypeESN, ID: 0}
	if _, ok := matchOfferedESPProposal(utESPOffer(espEncr, espInteg, esn), our, espDHMatch{Unbound: true}); !ok {
		t.Fatal("ESP control without the attribute was refused")
	}
	espCases := map[string][]wire.Transform{
		"unknown transform id": {{Type: wire.TransformTypeENCR, ID: utUnknownEncrID}, espInteg, esn},
		"ENCR with attribute":  {utWithUnknownAttr(espEncr), espInteg, esn},
		"INTEG with attribute": {espEncr, utWithUnknownAttr(espInteg), esn},
		"ESN with attribute":   {espEncr, espInteg, utWithUnknownAttr(esn)},
	}
	for name, transforms := range espCases {
		if _, ok := matchOfferedESPProposal(utESPOffer(transforms...), our, espDHMatch{Unbound: true}); ok {
			t.Errorf("ESP %s: matched, want the proposal refused", name)
		}
	}
}
