// Related: rfc7606.go — validateAttributeFlags, the entry point's flags check
// Related: ../../../core/bgp/attribute/flags_spec.go — flagsSpecs, the declaration it reads
//
// VALIDATES: RFC 7606 Section 3.c on the attributes ze accepted with any flags until now,
// the optional ones whose specification fixes an Optional and a Transitive value.
// PREVENTS: an UPDATE carrying an attribute with a corrupted Transitive bit being accepted,
// after which the Partial walks classify the attribute by the corrupted bit and keep a
// Partial bit RFC 4271 Section 4.3 forbids.
package message

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// optionalAttrFlagsUpdate builds a valid IPv4 unicast UPDATE's path attributes, with one
// optional attribute appended under the flags each case supplies.
//
// The three well-known mandatory attributes are present and well-formed, so Section 3.d
// cannot fire first, and each value below is the length and shape its own RFC requires, so
// no per-attribute validator can fire either. The flags octet is the only defect.
//
// An attribute an IBGP peer alone may send carries ibgp, which adds the LOCAL_PREF an
// IBGP UPDATE owes and drives the validation with the IBGP verdict.
func optionalAttrFlagsUpdate(flags, code byte, value []byte, ibgp bool) []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH, empty
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP = 192.0.2.1
	}
	if ibgp {
		attrs = append(attrs, 0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64) // LOCAL_PREF = 100
	}
	attrs = append(attrs, flags, code, byte(len(value)))
	return append(attrs, value...)
}

// TestRFC7606FlagsOptionalAttributeConflict drives an UPDATE carrying one optional
// attribute whose flags octet conflicts with its own RFC through the receive-path entry
// point, and asserts the UPDATE is treated as a withdrawal.
//
// Isolation: the paired positive below sends the same five UPDATEs with the conforming
// flags octet and asserts RFC7606ActionNone, so each case here fails on the flags and not
// on a neighboring rule. AttrCode pins the verdict to the attribute under test.
//
// RFC requirement: RFC7606-3.c-1 negative — an OPTIONAL attribute whose Optional or
// Transitive bit conflicts with the value its own specification fixes (MULTI_EXIT_DISC,
// ORIGINATOR_ID and CLUSTER_LIST marked transitive, COMMUNITIES marked non-transitive and
// marked well-known) is treat-as-withdraw, which is what ze accepted until now.
func TestRFC7606FlagsOptionalAttributeConflict(t *testing.T) {
	for _, one := range []struct {
		name  string
		flags byte
		code  attribute.AttributeCode
		value []byte
		ibgp  bool
	}{
		{
			// RFC 4271 Section 4.3d: "This is an optional non-transitive attribute".
			name: "MULTI_EXIT_DISC marked transitive", flags: 0xc0,
			code: attribute.AttrMED, value: []byte{0x00, 0x00, 0x00, 0x64},
		},
		{
			// RFC 4456 Section 8: "ORIGINATOR_ID is a new optional, non-transitive BGP
			// attribute".
			name: "ORIGINATOR_ID marked transitive", flags: 0xc0,
			code: attribute.AttrOriginatorID, value: []byte{0xc0, 0x00, 0x02, 0x0a}, ibgp: true,
		},
		{
			// RFC 4456 Section 8: "CLUSTER_LIST is a new, optional, non-transitive BGP
			// attribute".
			name: "CLUSTER_LIST marked transitive", flags: 0xc0,
			code: attribute.AttrClusterList, value: []byte{0xc0, 0x00, 0x02, 0x0b}, ibgp: true,
		},
		{
			// RFC 1997: "the COMMUNITIES path attribute is an optional transitive
			// attribute".
			name: "COMMUNITIES marked non-transitive", flags: 0x80,
			code: attribute.AttrCommunity, value: []byte{0xff, 0xff, 0xff, 0x01},
		},
		{
			// The same attribute marked well-known, which conflicts with the Optional bit
			// rather than the Transitive one.
			name: "COMMUNITIES marked well-known", flags: 0x40,
			code: attribute.AttrCommunity, value: []byte{0xff, 0xff, 0xff, 0x01},
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			attrs := optionalAttrFlagsUpdate(one.flags, byte(one.code), one.value, one.ibgp)

			result := ValidateUpdateRFC7606(attrs, true /*hasNLRI*/, one.ibgp, true /*asn4*/)
			require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action,
				"RFC 7606 Section 3.c asks for treat-as-withdraw, and this attribute's own RFC asks for nothing else")
			require.Equal(t, byte(one.code), result.AttrCode,
				"the flags conflict must be attributed to the attribute that carries it")
			require.Contains(t, result.Description, "3.c")
		})
	}
}

// TestRFC7606FlagsOptionalAttributeAsSpecifiedAccepted is the conforming side of the same
// UPDATE: each attribute above carries the flags its specification fixes.
//
// VALIDATES: the check rejects only a conflict, so a conformant optional attribute still
// reaches the RIB.
// PREVENTS: a validator that withdraws every UPDATE carrying an optional attribute, which
// the negative cases above cannot detect.
//
// RFC requirement: RFC7606-3.c-1 positive — an optional attribute carrying the Optional and
// Transitive values its own specification fixes raises no conflict and the UPDATE is
// accepted.
func TestRFC7606FlagsOptionalAttributeAsSpecifiedAccepted(t *testing.T) {
	for _, one := range []struct {
		name  string
		flags byte
		code  attribute.AttributeCode
		value []byte
		ibgp  bool
	}{
		{"MULTI_EXIT_DISC optional non-transitive", 0x80, attribute.AttrMED, []byte{0x00, 0x00, 0x00, 0x64}, false},
		{"ORIGINATOR_ID optional non-transitive", 0x80, attribute.AttrOriginatorID, []byte{0xc0, 0x00, 0x02, 0x0a}, true},
		{"CLUSTER_LIST optional non-transitive", 0x80, attribute.AttrClusterList, []byte{0xc0, 0x00, 0x02, 0x0b}, true},
		{"COMMUNITIES optional transitive", 0xc0, attribute.AttrCommunity, []byte{0xff, 0xff, 0xff, 0x01}, false},
	} {
		t.Run(one.name, func(t *testing.T) {
			attrs := optionalAttrFlagsUpdate(one.flags, byte(one.code), one.value, one.ibgp)

			result := ValidateUpdateRFC7606(attrs, true /*hasNLRI*/, one.ibgp, true /*asn4*/)
			require.Equal(t, RFC7606ActionNone, result.Action,
				"these are the flags the attribute's own RFC fixes: %s", result.Description)
		})
	}
}
