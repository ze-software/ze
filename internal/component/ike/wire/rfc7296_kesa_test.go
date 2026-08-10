package wire

import (
	"errors"
	"testing"
)

// kesaProposal builds one proposal carrying the given Diffie-Hellman transform IDs.
// Passing no id at all yields a proposal that specifies no DH group, which is the
// shape RFC 7296 Section 3.4 talks about when it forbids a KE payload.
func kesaProposal(number uint8, dhIDs ...uint16) Proposal {
	p := Proposal{
		Number:     number,
		ProtocolID: ProtocolIKE,
		Transforms: []Transform{
			{Type: TransformTypeENCR, ID: 12},
			{Type: TransformTypePRF, ID: 5},
			{Type: TransformTypeINTG, ID: 12},
		},
	}
	for _, id := range dhIDs {
		p.Transforms = append(p.Transforms, Transform{Type: TransformTypeDH, ID: id})
	}
	return p
}

// kesaSA wraps proposals in an SA payload.
func kesaSA(props ...Proposal) *PayloadSA {
	return &PayloadSA{Proposals: props}
}

// VALIDATES: a Key Exchange payload whose Diffie-Hellman Group Num names a group that
// some proposal of the SA payload in the same message specifies is accepted, and the
// match is found in any proposal and at any position, not only the first.
// PREVENTS: a check that compares the KE group against one hard-coded proposal (the
// first, or the last), which would reject a conforming message whose matching group
// sits elsewhere in the offer.
// RFC requirement: RFC7296-3.4-2 positive -- PayloadSA.ValidateKEGroup (payload_sa.go)
// accepts a KE group specified by a proposal in the same SA payload.
func TestKesaKEGroupOfferedIsAccepted(t *testing.T) {
	cases := []struct {
		name string
		sa   *PayloadSA
		ke   uint16
	}{
		{"only proposal", kesaSA(kesaProposal(1, 14)), 14},
		{"first of two groups in one proposal", kesaSA(kesaProposal(1, 14, 19)), 14},
		{"second of two groups in one proposal", kesaSA(kesaProposal(1, 14, 19)), 19},
		{"group of the second proposal", kesaSA(kesaProposal(1, 14), kesaProposal(2, 19)), 19},
		{"group of the first proposal", kesaSA(kesaProposal(1, 14), kesaProposal(2, 19)), 14},
		{"real group beside NONE", kesaSA(kesaProposal(1, 0, 20)), 20},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.sa.ValidateKEGroup(&PayloadKE{DHGroup: c.ke}); err != nil {
				t.Fatalf("KE group %d is offered by the SA payload, but was rejected: %v", c.ke, err)
			}
		})
	}
}

// VALIDATES: a Key Exchange payload whose group no proposal of the same message
// specifies is rejected with ErrKEGroupNotOffered, including the case where the SA
// payload offers OTHER groups and the case where the KE names NONE.
// PREVENTS: accepting a responder that answers in a Diffie-Hellman group it never
// named. The shared secret would then be computed under a group this node never
// agreed to, which is exactly the substitution Section 3.4 exists to stop. A KE
// naming NONE is included because NONE names no group, so it can never "match a
// Diffie-Hellman group" however many the offer carries.
// RFC requirement: RFC7296-3.4-2 negative -- PayloadSA.ValidateKEGroup (payload_sa.go)
// rejects a KE group that no proposal in the same SA payload specifies.
func TestKesaKEGroupNotOfferedIsRejected(t *testing.T) {
	cases := []struct {
		name string
		sa   *PayloadSA
		ke   uint16
	}{
		{"single group, KE names another", kesaSA(kesaProposal(1, 14)), 19},
		{"two groups, KE names a third", kesaSA(kesaProposal(1, 14), kesaProposal(2, 19)), 20},
		{"KE names NONE while a real group is offered", kesaSA(kesaProposal(1, 14)), 0},
		{"KE names NONE while NONE is offered beside a real group", kesaSA(kesaProposal(1, 0, 14)), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.sa.ValidateKEGroup(&PayloadKE{DHGroup: c.ke})
			if !errors.Is(err, ErrKEGroupNotOffered) {
				t.Fatalf("KE group %d is not offered by the SA payload; got err %v, want ErrKEGroupNotOffered", c.ke, err)
			}
		})
	}
}

// VALIDATES: when no proposal of the SA payload specifies a Diffie-Hellman group, a
// message carrying a KE payload is rejected, and one carrying none is accepted. A
// proposal whose only DH transform is NONE counts as specifying no group.
// PREVENTS: treating the Transform ID NONE as a group. That reading would let a KE
// payload ride along with a DH-less offer, which is the precise combination the
// section forbids, and it would also contradict Section 3.3.6's instruction to omit
// the KE payload when NONE is selected.
// RFC requirement: RFC7296-3.4-3 negative -- PayloadSA.ValidateKEGroup (payload_sa.go)
// rejects a KE payload when no proposal specifies a Diffie-Hellman group.
func TestKesaKEWithoutDHProposalIsRejected(t *testing.T) {
	cases := []struct {
		name string
		sa   *PayloadSA
	}{
		{"no DH transform at all", kesaSA(kesaProposal(1))},
		{"only NONE", kesaSA(kesaProposal(1, 0))},
		{"NONE in every one of two proposals", kesaSA(kesaProposal(1, 0), kesaProposal(2, 0))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.sa.ValidateKEGroup(&PayloadKE{DHGroup: 14})
			if !errors.Is(err, ErrKEWithoutDHProposal) {
				t.Fatalf("SA payload specifies no DH group but a KE payload was accepted; got err %v, want ErrKEWithoutDHProposal", err)
			}
		})
	}
}

// VALIDATES: the absence of a KE payload never violates Section 3.4, whether or not
// the SA payload specifies a Diffie-Hellman group.
// PREVENTS: a validator that turns the "MUST NOT be present" rule into a "MUST be
// present" rule. Section 1.2 decides when a KE payload is required; this check must
// not answer that question, or an IKE_AUTH SA payload (which never carries a KE)
// would start failing.
// RFC requirement: RFC7296-3.4-3 positive -- PayloadSA.ValidateKEGroup (payload_sa.go)
// accepts a message that carries no KE payload.
func TestKesaAbsentKEIsAlwaysAllowed(t *testing.T) {
	cases := []struct {
		name string
		sa   *PayloadSA
	}{
		{"offer specifies a group", kesaSA(kesaProposal(1, 14))},
		{"offer specifies no group", kesaSA(kesaProposal(1))},
		{"offer specifies only NONE", kesaSA(kesaProposal(1, 0))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.sa.ValidateKEGroup(nil); err != nil {
				t.Fatalf("a message with no KE payload was rejected: %v", err)
			}
		})
	}
}

// VALIDATES: SpecifiesDHGroup reports true only when some proposal names a real
// group, and false when the offer carries no DH transform or only NONE.
// PREVENTS: the two Section 3.4 rules diverging from Section 3.3.6's reading of
// NONE. Both rest on this predicate, so it is asserted directly rather than only
// through the two validators above.
func TestKesaSpecifiesDHGroup(t *testing.T) {
	cases := []struct {
		name string
		sa   *PayloadSA
		want bool
	}{
		{"no DH transform", kesaSA(kesaProposal(1)), false},
		{"only NONE", kesaSA(kesaProposal(1, 0)), false},
		{"NONE in both proposals", kesaSA(kesaProposal(1, 0), kesaProposal(2, 0)), false},
		{"one real group", kesaSA(kesaProposal(1, 14)), true},
		{"NONE beside a real group", kesaSA(kesaProposal(1, 0, 14)), true},
		{"real group in the second proposal only", kesaSA(kesaProposal(1), kesaProposal(2, 19)), true},
		{"no proposals at all", kesaSA(), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.sa.specifiesDHGroup(); got != c.want {
				t.Fatalf("SpecifiesDHGroup() = %v, want %v", got, c.want)
			}
		})
	}
}
