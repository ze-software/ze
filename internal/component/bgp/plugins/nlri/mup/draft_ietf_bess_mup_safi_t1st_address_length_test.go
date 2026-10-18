package mup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
)

// t1stLengthCase is one Type 1 ST route whose only fault is one address length
// field: every other octet would parse if that length were accepted.
type t1stLengthCase struct {
	name  string
	route []byte
}

// assertT1STWithdrawnAtIngress drives the registered IPv4 MUP recognizer through
// nlritype.Retain, the call the reactor's typed NLRI pass makes at ingress, with
// the route between two well-formed ISD siblings, and requires the route to be
// withdrawn and both siblings kept.
func assertT1STWithdrawnAtIngress(t *testing.T, cases []t1stLengthCase) {
	t.Helper()
	recognize := nlritype.Get(IPv4MUP)
	require.NotNil(t, recognize, "the mup plugin registers its ingress recognizer for ipv4/mup")
	l, l2 := mupSiblings(IPv4MUP)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var scratch [nlrisplit.PrefixKeyScratchSize]byte
			r, err := nlritype.Retain(recognize, IPv4MUP, concatNLRI(l, tc.route, l2), false, true, scratch[:])
			require.NoError(t, err, "a well-framed section is never an error")
			assert.Equal(t, concatNLRI(l, l2), r.Kept, "only the malformed route leaves the section")
			assert.Equal(t, tc.route, r.Withdrawn, "the malformed route is withdrawn")
			assert.Equal(t, 1, r.WithdrawnCount)
			assert.Zero(t, r.Discarded)
		})
	}
}

// TestMUPT1STEndpointLengthIsTheOnlyFault isolates the Endpoint Address Length
// rule, which TestMUPMalformedRouteTreatedAsWithdrawnAtIngress does not: there the
// octets after the bad length are themselves malformed once the rule is gone.
//
// draft-ietf-bess-mup-safi Section 3.1.3.1: "Endpoint Address field contains of
// an IPv4 address, then the value of the Endpoint Address Length field is 32.  If
// the Endpoint Address field contains of an IPv6 Address, then the value of the
// Endpoint Address Length field is 128.  Any other value is considered as an
// invalid and a malformed Endpoint Address.  A BGP speaker MUST handle such a
// malformed NLRI as a "Treat-as-withdraw" [RFC7606]."
//
// VALIDATES: Endpoint Address Length 24 followed by Source Address Length 0 and
// nothing else, and Endpoint Address Length 24 followed by three address octets
// and Source Address Length 0, are each withdrawn at ingress. Read with zero
// endpoint octets, the first is a well-formed route; read with (24+7)/8 = 3
// endpoint octets, the second is.
// PREVENTS: the length rule disappearing, or rounding up to whole octets, while a
// later field check still withdraws the route by accident.
// MUTATION: return true for every value in addressBitsOctets, or drop the !ok
// check after it in parseBodyT1ST, and the first case is kept.
func TestMUPT1STEndpointLengthIsTheOnlyFault(t *testing.T) {
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2 positive -- a Type 1 ST route whose Endpoint Address Length is 24, and whose remaining octets form a valid route under either a zero-octet or a three-octet reading of that length, is withdrawn at ingress and its two siblings are kept
	assertT1STWithdrawnAtIngress(t, []t1stLengthCase{
		{"length 24, no address octets, source length 0", t1stRoute(nil, teid1, 24, 0)},
		{"length 24, three address octets, source length 0", t1stRoute(nil, teid1, 24, 192, 0, 2, 0)},
	})
}

// TestMUPT1STSourceLengthIsTheOnlyFault isolates the Source Address Length rule,
// which TestMUPMalformedRouteTreatedAsWithdrawnAtIngress does not: there the
// octets after the bad length overrun as a TLV once the rule is gone.
//
// draft-ietf-bess-mup-safi Section 3.1.3.1: "Source Address field contains of an
// IPv4 address, then the value of the Source Address Length field is 32.  If the
// Source Address field contains of an IPv6 Address, then the value of the Source
// Address Length field is 128.  Any other value is considered as an invalid and a
// malformed Endpoint Address.  A BGP speaker MUST handle such a malformed NLRI as
// a "Treat-as-withdraw" [RFC7606]."
//
// VALIDATES: a valid IPv4 endpoint followed by Source Address Length 16 and
// nothing else, and by Source Address Length 16 then one valid TLV (type 0xf0,
// length 1), are each withdrawn at ingress. Read with zero source octets, both
// are well-formed routes.
// PREVENTS: the source length rule disappearing while the TLV walk still
// withdraws the route by accident.
// MUTATION: drop the !sourceOK check in parseBodyT1ST, and both cases are kept.
func TestMUPT1STSourceLengthIsTheOnlyFault(t *testing.T) {
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3 positive -- a Type 1 ST route with a valid IPv4 endpoint and Source Address Length 16, followed by nothing or by one valid TLV, is withdrawn at ingress and its two siblings are kept
	assertT1STWithdrawnAtIngress(t, []t1stLengthCase{
		{"source length 16, nothing after", t1stRoute(nil, teid1, 32, 192, 0, 2, 1, 16)},
		{"source length 16, one valid TLV after", t1stRoute(nil, teid1, 32, 192, 0, 2, 1, 16, 0xf0, 1, 0xaa)},
	})
}
