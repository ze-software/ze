// Related: initiator.go -- buildWireIKEProposals, espProposalToWire
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// aeadIKEGroup offers one AES-GCM proposal, which is what makes every
// encryption algorithm in the proposal an authenticated encryption algorithm.
// Hash is still set, so a builder that ignores the AEAD property has an
// integrity transform available to emit and the test can tell the two apart.
func aeadIKEGroup() ipsec.IKEGroup {
	return ipsec.IKEGroup{
		Name: "test-ike-aead",
		Proposals: []ipsec.IKEProposal{{
			Number:     1,
			Encryption: ipsec.EncryptionAES256GCM,
			Hash:       ipsec.HashSHA256,
			DHGroup:    14,
		}},
	}
}

func integrityTransformCount(p wire.Proposal) int {
	count := 0
	for _, transform := range p.Transforms {
		if transform.Type == wire.TransformTypeINTG {
			count++
		}
	}
	return count
}

// RFC requirement: RFC5282-8-2 positive -- an IKE proposal whose only encryption
// algorithm is an AEAD cipher goes on the wire carrying NO Transform Type 3, so
// the integrity transform is omitted rather than sent as AUTH_NONE.
//
// RFC 5282 Section 8: "This document further updates [RFC4306] to require that
// if all of the encryption algorithms in any proposal are authenticated
// encryption algorithms, then the proposal MUST NOT propose any integrity
// transforms."
//
// AUTH_NONE is still a proposed integrity transform, which is the reading this
// asserts: the sentence forbids proposing one, not proposing a particular value.
// espProposalToWire has always omitted it for an AEAD ESP proposal, so before
// this test ze answered the same obligation two different ways on two rails.
func TestRFC5282AEADIKEProposalCarriesNoIntegrityTransform(t *testing.T) {
	proposals := buildWireIKEProposals(aeadIKEGroup())
	if len(proposals) != 1 {
		t.Fatalf("buildWireIKEProposals returned %d proposals, want 1", len(proposals))
	}
	if got := integrityTransformCount(proposals[0]); got != 0 {
		t.Fatalf("AEAD proposal carries %d Transform Type 3, want 0: RFC 5282 Section 8 forbids proposing any integrity transform when every encryption algorithm is AEAD", got)
	}
	// The proposal must still be a complete offer. Dropping Type 3 must not have
	// dropped anything else, or this test would pass against a builder that
	// emitted nothing at all.
	want := map[uint8]bool{wire.TransformTypeENCR: false, wire.TransformTypePRF: false, wire.TransformTypeDH: false}
	for _, transform := range proposals[0].Transforms {
		if _, ok := want[transform.Type]; ok {
			want[transform.Type] = true
		}
	}
	for transformType, present := range want {
		if !present {
			t.Fatalf("AEAD proposal lost Transform Type %d; the omission must be the integrity transform alone", transformType)
		}
	}
}

// RFC requirement: RFC5282-8-2 negative -- a proposal whose encryption algorithm
// is NOT an AEAD cipher still carries exactly one Transform Type 3, so the
// omission above is conditional on the AEAD property rather than unconditional.
//
// Without this polarity a builder that never emitted an integrity transform
// would satisfy the positive case while breaking every non-AEAD proposal ze
// sends, which RFC 7296 Section 3.3.3 requires to carry one.
func TestRFC5282NonAEADIKEProposalKeepsItsIntegrityTransform(t *testing.T) {
	proposals := buildWireIKEProposals(testIKEGroup())
	if len(proposals) != 1 {
		t.Fatalf("buildWireIKEProposals returned %d proposals, want 1", len(proposals))
	}
	if got := integrityTransformCount(proposals[0]); got != 1 {
		t.Fatalf("AES-CBC proposal carries %d Transform Type 3, want exactly 1", got)
	}
}
