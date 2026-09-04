// VALIDATES: a proposal that carries a Transform Type the protocol does not use is
// unacceptable, and the proposals beside it are still processed. The check covers a type
// no registry assigns, and a type assigned to another protocol.
// PREVENTS: the silent drop. The wire-to-crypto converter switched on the five types it
// knew, and every other type fell through every case. A proposal ze was unable to read in
// full was then negotiated as though the peer had sent nothing unusual.
package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// tftUnassignedType is a Transform Type no protocol in RFC 7296 Section 3.3.3 uses. The
// table names types 1 to 5, so 6 is outside it.
const tftUnassignedType uint8 = 6

// tftIKEOffer builds the wire IKE proposals of the test policy, with one extra transform
// appended to the first proposal.
func tftIKEOffer(extra wire.Transform) []wire.Proposal {
	proposals := buildWireIKEProposals(testIKEGroup())
	proposals[0].Transforms = append(proposals[0].Transforms, extra)
	return proposals
}

// RFC requirement: RFC7296-3.3.6-4 negative -- "If the responder receives a proposal that contains
// a Transform Type it does not understand ... it MUST consider this proposal unacceptable"
// (RFC 7296 Section 3.3.6). A transform of an unassigned type makes the proposal
// unacceptable. An Extended Sequence Numbers transform does the same inside an IKE
// proposal, because RFC 7296 Section 3.3.3 gives that type to ESP and AH alone.
// RFC requirement: RFC7296-3.3.6-4 positive -- "however, other proposals in the same SA payload are
// processed as usual". A clean second proposal is accepted beside the refused one.
//
// RFC requirement: RFC7296-3.3.3-1 negative -- the table of RFC 7296 Section 3.3.3 bounds what each
// protocol understands, and the converter reads that table rather than a hand-written
// list of cases. A type outside a protocol's set never reaches negotiation unnoticed.
// RFC requirement: RFC7296-3.3.3-1 positive -- every type the table gives IKE is understood, so the
// proposal built from ENCR, PRF, INTEG and D-H negotiates.
func TestTftUnknownTransformTypeMakesProposalUnacceptable(t *testing.T) {
	local := buildIKEProposals(testIKEGroup())

	clean := wireProposalsToIKE(buildWireIKEProposals(testIKEGroup()))
	if _, err := crypto.NegotiateIKE(clean, local); err != nil {
		t.Fatalf("a proposal of understood types returned %v, want acceptance", err)
	}

	unknown := []wire.Transform{
		{Type: tftUnassignedType, ID: 1},
		{Type: wire.TransformTypeESN, ID: 0},
	}
	for _, extra := range unknown {
		offer := wireProposalsToIKE(tftIKEOffer(extra))
		_, err := crypto.NegotiateIKE(offer, local)
		if !errors.Is(err, crypto.ErrNoProposalChosen) {
			t.Errorf("an IKE proposal carrying transform type %d = %v, want a refusal", extra.Type, err)
		}
	}

	// Other proposals in the same SA payload are processed as usual. The second proposal
	// carries understood types alone, and it is the one accepted.
	mixed := tftIKEOffer(wire.Transform{Type: tftUnassignedType, ID: 1})
	second := buildWireIKEProposals(testIKEGroup())[0]
	second.Number = 2
	mixed = append(mixed, second)
	chosen, err := crypto.NegotiateIKE(wireProposalsToIKE(mixed), local)
	if err != nil {
		t.Fatalf("the clean second proposal returned %v, want acceptance", err)
	}
	if chosen.Number != 2 {
		t.Errorf("accepted proposal %d, want the clean second one", chosen.Number)
	}
}

// RFC requirement: RFC7296-3.3.6-4 negative -- the Child SA offer is held to the same rule. A
// pseudorandom function transform inside an ESP proposal is a type RFC 7296 Section 3.3.3
// gives to IKE alone, so the responder does not select that proposal.
// RFC requirement: RFC7296-3.3.6-4 positive -- the same offer without that transform is selected, so
// the refusal names the foreign type rather than the proposal as such.
func TestTftForeignTransformTypeRefusedInESPOffer(t *testing.T) {
	our := testESPGroup().Proposals[0]

	clean := &wire.PayloadSA{Proposals: buildWireESPProposals(testESPGroup(), 0x11223344, dhGroupNone)}
	if _, ok := matchOfferedESPProposal(clean, our, espDHMatch{}); !ok {
		t.Fatal("the ESP offer of understood types was not selected")
	}

	foreign := &wire.PayloadSA{Proposals: buildWireESPProposals(testESPGroup(), 0x11223344, dhGroupNone)}
	foreign.Proposals[0].Transforms = append(foreign.Proposals[0].Transforms,
		wire.Transform{Type: wire.TransformTypePRF, ID: uint16(crypto.PRF_HMAC_SHA2_256)})
	if _, ok := matchOfferedESPProposal(foreign, our, espDHMatch{}); ok {
		t.Error("an ESP proposal carrying a PRF transform was selected")
	}
}
