package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// RFC 7606 Sections 7.13, 7.15 and 7.16: the three optional attributes that had no
// validator at all. An unregistered code returns nil from validateAttribute, so ANY length
// was accepted -- which is why these three sections were disclosed as gaps in
// docs/features/rfc-status.md rather than enforced.
//
// Every case drives ValidateUpdateRFC7606, the RFC 7606 decision point, rather than the
// per-attribute validator directly: a validator nobody reaches is not compliance
// (ai/rules/evidence.md, "drive the guard's test from its entry point").

// rfc7606MandatoryAttrs is the well-known mandatory prefix every conforming UPDATE carries,
// so a case cannot pass or fail for want of ORIGIN/AS_PATH/NEXT_HOP.
var rfc7606MandatoryAttrs = []byte{
	0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
	0x40, 0x02, 0x00, // AS_PATH (empty)
	0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP = 192.0.2.1
}

// optAttr builds one optional attribute with a single-octet length.
func optAttr(flags, code byte, value []byte) []byte {
	out := []byte{flags, code, byte(len(value))}
	return append(out, value...)
}

// updateWith concatenates the mandatory attributes with extra ones.
func updateWith(extra ...[]byte) []byte {
	out := append([]byte{}, rfc7606MandatoryAttrs...)
	for _, e := range extra {
		out = append(out, e...)
	}
	return out
}

// --------------------------------------------------------------------------
// Section 7.13 -- Traffic Engineering path attribute (code 24)
// --------------------------------------------------------------------------

// VALIDATES: a Traffic Engineering attribute too short to hold one RFC 5543 descriptor is
// treat-as-withdraw.
// PREVENTS: accepting any length for code 24, which is what happened while the code had no
// validator registered at all.
//
// RFC 5543 Section 3: a descriptor is Switching Cap(1) + Encoding(1) + Reserved(2) + eight
// 4-octet Max LSP Bandwidth values = 36 fixed octets, and "the attribute contains one or
// more of the following". RFC 7606 Section 7.13 notes RFC 5543 "does not detail what
// constitutes malformation" and binds an implementation that determines it "for whatever
// reason", so the check is deliberately minimal: too short to be one descriptor.
//
// RFC requirement: RFC7606-7.13-1 negative -- a malformed Traffic Engineering attribute is
// treat-as-withdraw, not accepted and not a session reset.
func TestRFC7606TrafficEngineeringTooShort(t *testing.T) {
	for _, tc := range []struct {
		name   string
		length int
	}{
		{"zero length", 0},
		{"one octet", 1},
		{"one octet short of a descriptor", 35},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Optional non-transitive per RFC 5543 Section 3.
			attrs := updateWith(optAttr(0x80, 0x18, make([]byte, tc.length)))
			result := ValidateUpdateRFC7606(attrs, true, false, false)
			require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
			require.Equal(t, uint8(24), result.AttrCode)
			require.Contains(t, result.Description, "7.13")
		})
	}
}

// VALIDATES: a Traffic Engineering attribute holding at least one descriptor is accepted.
// PREVENTS: the length check over-firing. RFC 7606 Section 7.13 gives no license to reject
// a well-formed TE attribute, and blackholing valid routes is worse than under-validating
// an attribute ze does not act on.
//
// RFC requirement: RFC7606-7.13-1 positive -- a Traffic Engineering attribute long enough
// to carry a descriptor is not malformed, so the UPDATE is accepted.
func TestRFC7606TrafficEngineeringValid(t *testing.T) {
	for _, tc := range []struct {
		name   string
		length int
	}{
		{"exactly one descriptor", 36},
		{"descriptor with switching-capability-specific information", 44},
		{"two descriptors", 72},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attrs := updateWith(optAttr(0x80, 0x18, make([]byte, tc.length)))
			result := ValidateUpdateRFC7606(attrs, true, false, false)
			require.Equal(t, RFC7606ActionNone, result.Action)
		})
	}
}

// --------------------------------------------------------------------------
// Section 7.15 -- IPv6 Address Specific Extended Community (code 25)
// --------------------------------------------------------------------------

// VALIDATES: an IPv6 Address Specific Extended Community whose length is not a non-zero
// multiple of 20 is treat-as-withdraw.
// PREVENTS: accepting a truncated or zero-length attribute, which the code did while it
// had no validator.
//
// RFC 7606 Section 7.15: "The IPv6 Address Specific Extended Community attribute SHALL be
// considered malformed if its length is not a non-zero multiple of 20." Zero fails the
// non-zero half, and 0 % 20 == 0 would pass a multiple-of-20 test alone.
//
// RFC requirement: RFC7606-7.15-1 negative -- a length that is not a non-zero multiple of
// 20 is malformed, so treat-as-withdraw.
func TestRFC7606IPv6ExtCommunityBadLength(t *testing.T) {
	for _, tc := range []struct {
		name   string
		length int
	}{
		{"zero length fails the non-zero clause", 0},
		{"one octet short", 19},
		{"one octet long", 21},
		{"not a multiple of 20", 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Optional transitive per RFC 5701 Section 2.
			attrs := updateWith(optAttr(0xc0, 0x19, make([]byte, tc.length)))
			result := ValidateUpdateRFC7606(attrs, true, false, false)
			require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
			require.Equal(t, uint8(25), result.AttrCode)
			require.Contains(t, result.Description, "7.15")
		})
	}
}

// VALIDATES: lengths 20 and 40 are accepted.
// PREVENTS: a validator that rejected every IPv6 Extended Community would still satisfy
// TestRFC7606IPv6ExtCommunityBadLength.
//
// RFC requirement: RFC7606-7.15-1 positive -- a non-zero multiple of 20 is well-formed, so
// the UPDATE is accepted.
func TestRFC7606IPv6ExtCommunityValidLength(t *testing.T) {
	for _, tc := range []struct {
		name   string
		length int
	}{
		{"one community", 20},
		{"two communities", 40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attrs := updateWith(optAttr(0xc0, 0x19, make([]byte, tc.length)))
			result := ValidateUpdateRFC7606(attrs, true, false, false)
			require.Equal(t, RFC7606ActionNone, result.Action)
		})
	}
}

// VALIDATES: an unrecognized IPv6 Extended Community Type or Sub-Type is NOT an error.
// PREVENTS: adding a Type/Sub-Type allowlist to the new length validator, which would drop
// routes carrying communities that merely postdate this code.
//
// RFC 7606 Section 7.15: "Note that a BGP speaker MUST NOT treat an unrecognized IPv6
// Address Specific Extended Community Type or Sub-Type as an error."
//
// This requirement was previously satisfied only BY OMISSION -- nothing validated code 25,
// so nothing could error on an unknown type. Now that a length check exists the
// prohibition is load-bearing: the validator must read LENGTH ONLY. The length here stays
// valid (20) so the Section 7.15-1 clause cannot fire and mask the type question.
//
// RFC requirement: RFC7606-7.15-2 positive -- an unrecognized Type/Sub-Type with a valid
// length is accepted, not an error.
func TestRFC7606IPv6ExtCommunityUnrecognizedType(t *testing.T) {
	// RFC 5701 Section 2: Type(1) Sub-Type(1) Global Administrator(16, an IPv6 address)
	// Local Administrator(2). Type 0x3f / Sub-Type 0xee is not a Type ze recognizes.
	value := []byte{0x3f, 0xee}
	value = append(value, make([]byte, 16)...) // Global Administrator
	value = append(value, 0x00, 0x01)          // Local Administrator

	attrs := updateWith(optAttr(0xc0, 0x19, value))
	result := ValidateUpdateRFC7606(attrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// --------------------------------------------------------------------------
// Section 7.16 -- ATTR_SET (code 128)
// --------------------------------------------------------------------------

// attrSetValue builds an ATTR_SET value: 4-octet Origin AS then a path-attribute stream.
func attrSetValue(originAS uint32, inner []byte) []byte {
	v := []byte{
		byte(originAS >> 24), byte(originAS >> 16), byte(originAS >> 8), byte(originAS),
	}
	return append(v, inner...)
}

// VALIDATES: every RFC 6368 Section 5 malformed condition selects treat-as-withdraw.
// PREVENTS: accepting an ATTR_SET of any shape, which the code did while code 128 had no
// validator.
//
// RFC 6368 Section 5 defines malformed as: length under 4 octets; the contained attributes
// include MP_REACH or MP_UNREACH; or the included attributes are malformed themselves.
// RFC 7606 Section 7.16 replaces only the ACTION, which is now always "treat as withdraw"
// rather than the old Partial/Neighbor-Complete branch.
//
// RFC requirement: RFC7606-7.16-1 negative -- a malformed ATTR_SET is treat-as-withdraw.
func TestRFC7606AttrSetMalformed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value []byte
	}{
		{"shorter than the 4-octet Origin AS", []byte{0x00, 0x00, 0x00}},
		{"empty", nil},
		{
			"contains MP_REACH_NLRI",
			attrSetValue(65000, optAttr(0x80, 0x0e, []byte{0x00, 0x01, 0x01, 0x00})),
		},
		{
			"contains MP_UNREACH_NLRI",
			attrSetValue(65000, optAttr(0x80, 0x0f, []byte{0x00, 0x01, 0x01})),
		},
		{
			"inner ORIGIN is itself malformed",
			// ORIGIN with length 2: RFC 7606 Section 7.1 makes it malformed.
			attrSetValue(65000, []byte{0x40, 0x01, 0x02, 0x00, 0x00}),
		},
		{
			"inner attribute stream is truncated",
			// Declares an 8-octet ORIGIN but only 2 octets follow.
			attrSetValue(65000, []byte{0x40, 0x01, 0x08, 0x00, 0x00}),
		},
		{
			"inner attribute header is truncated",
			attrSetValue(65000, []byte{0x40}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attrs := updateWith(optAttr(0xc0, 0x80, tc.value))
			result := ValidateUpdateRFC7606(attrs, true, false, false)
			require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
			require.Equal(t, uint8(128), result.AttrCode)
			require.Contains(t, result.Description, "7.16")
		})
	}
}

// VALIDATES: a well-formed ATTR_SET is accepted, including one nested within the cap.
// PREVENTS: a validator that rejected every ATTR_SET would still satisfy
// TestRFC7606AttrSetMalformed.
//
// RFC requirement: RFC7606-7.16-1 positive -- a well-formed ATTR_SET carrying valid inner
// attributes is not malformed, so the UPDATE is accepted.
func TestRFC7606AttrSetValid(t *testing.T) {
	inner := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
	}
	for _, tc := range []struct {
		name  string
		value []byte
	}{
		{"origin AS only, no inner attributes", attrSetValue(65000, nil)},
		{"origin AS with valid inner attributes", attrSetValue(65000, inner)},
		{
			"one level of nesting, within the cap",
			attrSetValue(65000, optAttr(0xc0, 0x80, attrSetValue(65001, inner))),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attrs := updateWith(optAttr(0xc0, 0x80, tc.value))
			result := ValidateUpdateRFC7606(attrs, true, false, false)
			require.Equal(t, RFC7606ActionNone, result.Action)
		})
	}
}
