// VALIDATES: a wire proposal that carries SEVERAL transforms of one Transform Type
// offers every one of them. The order is the sender's preference. Negotiation CAN select
// any of them. The check covers D-H, PRF, integrity and encryption.
//
// PREVENTS: the collapse. wireProposalsToIKE assigned each transform into a scalar field.
// N transforms of one type became the LAST one read. A peer that offered [group 14, group
// NONE] was read as offering NONE alone. Ze refused it with NO_PROPOSAL_CHOSEN, although
// group 14 was on offer and configured. strongSwan offers alternatives this way.
package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// altDHNone is Transform ID 0 of Transform Type 4. RFC 7296 Section 3.3.2 names it NONE.
// RFC 7296 Section 3.3.6 refuses it for an IKE SA.
const altDHNone uint16 = 0

// altDHOffer builds the wire IKE offer of the test policy. It REPLACES the single D-H
// transform of the first proposal with the groups named, in the order named.
func altDHOffer(groups ...uint16) []wire.Proposal {
	proposals := buildWireIKEProposals(testIKEGroup())
	kept := make([]wire.Transform, 0, len(proposals[0].Transforms)+len(groups))
	for _, t := range proposals[0].Transforms {
		if t.Type != wire.TransformTypeDH {
			kept = append(kept, t)
		}
	}
	for _, g := range groups {
		kept = append(kept, wire.Transform{Type: wire.TransformTypeDH, ID: g})
	}
	proposals[0].Transforms = kept
	return proposals
}

// altTwoGroupPolicy is a local policy of two proposals. They differ only in their D-H
// group, so a peer that offers both groups has a real choice to express.
func altTwoGroupPolicy() ipsec.IKEGroup {
	group := testIKEGroup()
	second := group.Proposals[0]
	second.Number = 2
	second.DHGroup = ipsec.DHGroup(crypto.DH_ECP_256)
	group.Proposals = append(group.Proposals, second)
	return group
}

// RFC requirement: RFC7296-3.3.6-5 positive -- "other transforms with the same Transform
// Type are processed as usual" (RFC 7296 Section 3.3.6). A transform the responder cannot
// use makes THAT TRANSFORM unusable. It leaves the siblings of the same type on offer.
//
// RFC 7296 Section 3.3 states the encoding this relies on. Several transforms of one type
// inside one proposal are alternatives. The sender lists the most preferred one first.
//
// This is the receive half of that obligation. The engine's wire-to-crypto reader is the
// only layer that CAN see two transforms of one type, so the proof belongs here. The
// crypto layer's own tagged pair proves the per-PROPOSAL half alone. crypto.IKEProposal
// holds ONE transform per type, so a test written there cannot express this case.
//
// WHAT WOULD FALSIFY THIS: a reader that keeps one transform per type, whichever one it
// keeps. Both orders are asserted below. Keeping the first or the last both go red.
func TestAltDHAlternativesAreBothOffered(t *testing.T) {
	local := buildIKEProposals(testIKEGroup())

	// The peer offers a group Ze runs beside one it refuses. The usable group is still
	// on offer in either order, and it is the one selected.
	for _, order := range [][]uint16{
		{uint16(crypto.DH_MODP_2048), altDHNone},
		{altDHNone, uint16(crypto.DH_MODP_2048)},
	} {
		offer := wireProposalsToIKE(altDHOffer(order...))
		if len(offer) != 2 {
			t.Fatalf("offer %v expanded to %d parameter sets, want 2; the assertions below would not be about alternatives", order, len(offer))
		}
		chosen, err := crypto.NegotiateIKE(offer, local)
		if err != nil {
			t.Fatalf("offer %v was refused with %v, want group 14 selected; it was on offer and is configured", order, err)
		}
		if chosen.DHGroup.ID != crypto.DH_MODP_2048 {
			t.Errorf("offer %v selected group %d, want %d", order, chosen.DHGroup.ID, crypto.DH_MODP_2048)
		}
	}
}

// RFC requirement: RFC7296-3.3.6-5 negative -- the discriminator. It rests on a property
// the code HAS and not on a guard that is absent. Reading every transform of a type ADDS
// candidates. It relaxes no comparison.
//
// A proposal whose D-H transforms are ALL unusable is therefore still refused. The
// acceptance above is a decision about what the peer offered. It is not a reader that
// stopped refusing.
//
// WHAT WOULD FALSIFY THIS: an expansion that reads a missing or unusable transform as a
// wildcard. A fallback to local policy, when no offered value matches, does the same.
func TestAltUnusableAlternativesAreStillRefused(t *testing.T) {
	local := buildIKEProposals(testIKEGroup())

	// NONE alone. RFC 7296 Section 3.3.6 refuses it for an IKE SA.
	if _, err := crypto.NegotiateIKE(wireProposalsToIKE(altDHOffer(altDHNone)), local); !errors.Is(err, crypto.ErrDHGroupNone) {
		t.Errorf("an offer of D-H NONE alone = %v, want ErrDHGroupNone", err)
	}

	// Two groups, both real, neither specified by this implementation. Two unusable
	// alternatives are not more permitted than one.
	unsupported := altDHOffer(2, 5)
	offer := wireProposalsToIKE(unsupported)
	if len(offer) != 2 {
		t.Fatalf("expanded to %d parameter sets, want 2", len(offer))
	}
	if _, err := crypto.NegotiateIKE(offer, local); err == nil {
		t.Error("an offer of groups 2 and 5 was accepted; neither is specified by this implementation")
	}
}

// The sender's order is its preference (RFC 7296 Section 3.3). A peer that offers two
// groups Ze runs therefore has its FIRST choice selected. This also pins determinism. The
// same offer against the same policy selects the same group every time, and a reversed
// offer reverses the outcome.
//
// Untagged deliberately. No extracted requirement makes a RESPONDER honor the sender's
// preference among transforms of one type. Section 3.3 states the ordering as the
// sender's obligation. This asserts the behavior the fix chose, so that a later change to
// the expansion order is visible rather than silent.
func TestAltPeerPreferenceOrderDecidesSelection(t *testing.T) {
	local := buildIKEProposals(altTwoGroupPolicy())

	cases := []struct {
		offer []uint16
		want  crypto.DHGroupID
	}{
		{[]uint16{uint16(crypto.DH_MODP_2048), uint16(crypto.DH_ECP_256)}, crypto.DH_MODP_2048},
		{[]uint16{uint16(crypto.DH_ECP_256), uint16(crypto.DH_MODP_2048)}, crypto.DH_ECP_256},
	}
	for _, tc := range cases {
		for range 2 { // same input twice: selection must not vary between calls
			chosen, err := crypto.NegotiateIKE(wireProposalsToIKE(altDHOffer(tc.offer...)), local)
			if err != nil {
				t.Fatalf("offer %v was refused with %v; both groups are configured", tc.offer, err)
			}
			if chosen.DHGroup.ID != tc.want {
				t.Errorf("offer %v selected group %d, want %d (the peer's first choice)", tc.offer, chosen.DHGroup.ID, tc.want)
			}
		}
	}
}

// Every one of the four Transform Types IKE uses is expanded, and not D-H alone. The
// reader is asserted directly here. Negotiation stops at the first match, so it cannot
// show what the whole offer contained.
//
// Untagged deliberately. This is the structural half of the fix that
// TestAltDHAlternativesAreBothOffered proves behaviorally for one type.
func TestAltEveryTransformTypeExpandsInPeerOrder(t *testing.T) {
	offer := []wire.Proposal{{
		Number:     1,
		ProtocolID: wire.ProtocolIKE,
		Transforms: []wire.Transform{
			{Type: wire.TransformTypeENCR, ID: uint16(crypto.ENCR_AES_CBC), Attrs: []wire.TransformAttr{{Type: wire.AttrTypeKeyLength, Value: 256}}},
			{Type: wire.TransformTypeENCR, ID: uint16(crypto.ENCR_AES_CBC), Attrs: []wire.TransformAttr{{Type: wire.AttrTypeKeyLength, Value: 128}}},
			{Type: wire.TransformTypePRF, ID: uint16(crypto.PRF_HMAC_SHA2_256)},
			{Type: wire.TransformTypePRF, ID: uint16(crypto.PRF_HMAC_SHA2_384)},
			{Type: wire.TransformTypeINTG, ID: uint16(crypto.AUTH_HMAC_SHA2_256_128)},
			{Type: wire.TransformTypeINTG, ID: uint16(crypto.AUTH_HMAC_SHA2_384_192)},
			{Type: wire.TransformTypeDH, ID: uint16(crypto.DH_MODP_2048)},
			{Type: wire.TransformTypeDH, ID: uint16(crypto.DH_ECP_256)},
		},
	}}

	got := wireProposalsToIKE(offer)
	if len(got) != 16 {
		t.Fatalf("2x2x2x2 alternatives expanded to %d parameter sets, want 16", len(got))
	}

	// The first set emitted is the peer's first choice of every type.
	first := got[0]
	if first.Encryption.KeyLength != 256 {
		t.Errorf("first set encryption key length = %d, want 256", first.Encryption.KeyLength)
	}
	if first.PRF.ID != crypto.PRF_HMAC_SHA2_256 {
		t.Errorf("first set PRF = %d, want %d", first.PRF.ID, crypto.PRF_HMAC_SHA2_256)
	}
	if first.Integrity.ID != crypto.AUTH_HMAC_SHA2_256_128 {
		t.Errorf("first set integrity = %d, want %d", first.Integrity.ID, crypto.AUTH_HMAC_SHA2_256_128)
	}
	if first.DHGroup.ID != crypto.DH_MODP_2048 {
		t.Errorf("first set D-H = %d, want %d", first.DHGroup.ID, crypto.DH_MODP_2048)
	}

	// Every set carries the proposal number of the offer it came from. An accepted set
	// therefore still names the proposal the peer sent (RFC 7296 Section 3.3.1).
	for i, p := range got {
		if p.Number != 1 {
			t.Errorf("set %d carries proposal number %d, want 1", i, p.Number)
		}
	}

	// Each type's alternatives are all present, so the expansion dropped none of them.
	seen := map[crypto.DHGroupID]bool{}
	for _, p := range got {
		seen[p.DHGroup.ID] = true
	}
	if !seen[crypto.DH_MODP_2048] || !seen[crypto.DH_ECP_256] {
		t.Errorf("expanded D-H groups = %v, want both 14 and 19", seen)
	}
}

// A peer cannot turn the cross product into unbounded work before it authenticates. The
// cap keeps the peer's most preferred combinations and drops its least preferred ones. It
// removes candidates, so it can never admit one the peer did not offer.
func TestAltCombinationsAreBounded(t *testing.T) {
	var transforms []wire.Transform
	for i := range 40 {
		transforms = append(transforms,
			wire.Transform{Type: wire.TransformTypePRF, ID: uint16(i)},
			wire.Transform{Type: wire.TransformTypeDH, ID: uint16(i)},
		)
	}
	transforms = append(transforms,
		wire.Transform{Type: wire.TransformTypeENCR, ID: uint16(crypto.ENCR_AES_CBC), Attrs: []wire.TransformAttr{{Type: wire.AttrTypeKeyLength, Value: 256}}},
		wire.Transform{Type: wire.TransformTypeINTG, ID: uint16(crypto.AUTH_HMAC_SHA2_256_128)})

	got := wireProposalsToIKE([]wire.Proposal{{Number: 1, ProtocolID: wire.ProtocolIKE, Transforms: transforms}})
	if len(got) != maxIKECombinations {
		t.Errorf("40x40 alternatives expanded to %d parameter sets, want the cap of %d", len(got), maxIKECombinations)
	}
	// The cap keeps the peer's FIRST choices, so the most preferred set survives it.
	if got[0].DHGroup.ID != 0 || got[0].PRF.ID != 0 {
		t.Errorf("the surviving first set is D-H %d / PRF %d, want the peer's first of each", got[0].DHGroup.ID, got[0].PRF.ID)
	}
}
