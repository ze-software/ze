package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// RFC 6368 Section 5 judges an ATTR_SET's inner attributes in THEIR OWN context, not the
// enclosing session's. Getting that wrong does not merely mis-report: it withdraws
// conforming routes, which is the failure mode RFC 7606 exists to eliminate.
//
// Each case here failed against the first implementation of validateAttrSetDepth, which
// forwarded the session's isIBGP/asn4 and escalated any non-None inner action.

// innerASPath4Octet is an AS_PATH with one 4-octet AS_SEQUENCE entry (AS 65536).
// RFC 6368 Section 5 requires exactly this encoding inside an ATTR_SET.
var innerASPath4Octet = []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x01, 0x00, 0x00}

// VALIDATES: an ATTR_SET carrying the 4-octet AS_PATH the RFC mandates is accepted on a
// session that never negotiated RFC 6793.
// PREVENTS: reading a conforming inner AS_PATH with a 2-octet AS size and withdrawing the
// route for being malformed when it is not.
//
// RFC 6368 Section 5: "The AS_PATH and AGGREGATOR attributes contained within an ATTR_SET
// attribute MUST be encoded using 4-octet AS numbers [RFC4893], regardless of the
// capabilities advertised by the BGP speaker to which the ATTR_SET attribute is
// transmitted." The final clause is the whole point: the inner encoding does not depend on
// the session, so the inner validation must not either.
//
// RFC requirement: RFC7606-7.16-1 positive -- an ATTR_SET whose inner AS_PATH uses the
// mandated 4-octet encoding is well-formed even when the session did not negotiate ASN4.
func TestRFC7606AttrSetInnerASPathAlwaysFourOctet(t *testing.T) {
	value := attrSetValue(65000, innerASPath4Octet)
	attrs := updateWith(optAttr(0xc0, 0x80, value))

	// asn4=false: the session never negotiated RFC 6793.
	result := ValidateUpdateRFC7606(attrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"a conforming 4-octet inner AS_PATH must not be judged with the session's 2-octet AS size")

	// And it stays accepted when the session did negotiate it.
	require.Equal(t, RFC7606ActionNone,
		ValidateUpdateRFC7606(attrs, true, false, true).Action)
}

// VALIDATES: an ATTR_SET carrying the customer's LOCAL_PREF is accepted on an eBGP session.
// PREVENTS: withdrawing every route whose ATTR_SET preserves a customer's iBGP attributes,
// which is the entire purpose of the attribute.
//
// RFC 6368 is "Internal BGP as PE/CE Protocol": the ATTR_SET carries the CUSTOMER's iBGP
// attribute set across the provider, so LOCAL_PREF, ORIGINATOR_ID and CLUSTER_LIST are
// legitimate inside it regardless of the provider session's own eBGP context.
//
// RFC requirement: RFC7606-7.16-1 positive -- attributes that are legitimate in the
// customer's iBGP are not malformed merely because the carrying session is eBGP.
func TestRFC7606AttrSetInnerIBGPAttributesOnEBGPSession(t *testing.T) {
	for _, tc := range []struct {
		name  string
		inner []byte
	}{
		{"LOCAL_PREF", []byte{0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64}},
		{"ORIGINATOR_ID", []byte{0x80, 0x09, 0x04, 0xc0, 0x00, 0x02, 0x01}},
		{"CLUSTER_LIST", []byte{0x80, 0x0a, 0x04, 0x0a, 0x00, 0x00, 0x01}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attrs := updateWith(optAttr(0xc0, 0x80, attrSetValue(65000, tc.inner)))
			// isIBGP=false: the carrying session is eBGP.
			result := ValidateUpdateRFC7606(attrs, true, false, true)
			require.Equal(t, RFC7606ActionNone, result.Action,
				"an ATTR_SET preserving the customer's iBGP attributes must survive an eBGP session")
		})
	}
}

// VALIDATES: an inner attribute whose own error action is "attribute discard" does not
// escalate to a whole-UPDATE withdraw.
// PREVENTS: inverting RFC 7606's deliberate choice. It assigns attribute-discard to
// AGGREGATOR (7.7), LOCAL_PREF from eBGP (7.5), ORIGINATOR_ID (7.9) and CLUSTER_LIST
// (7.10) precisely so the route survives the error; only a TREAT-AS-WITHDRAW-or-worse
// inner result makes the ATTR_SET malformed under RFC 6368 Section 5.
//
// RFC requirement: RFC7606-7.16-1 negative -- an inner attribute that is genuinely
// malformed (ORIGIN of length 2, RFC 7606 Section 7.1) still withdraws, so the relaxation
// above did not disable the check.
func TestRFC7606AttrSetInnerMalformedStillWithdraws(t *testing.T) {
	// ORIGIN with length 2 is malformed per Section 7.1 and carries treat-as-withdraw,
	// which is strictly stronger than attribute-discard.
	inner := []byte{0x40, 0x01, 0x02, 0x00, 0x00}
	attrs := updateWith(optAttr(0xc0, 0x80, attrSetValue(65000, inner)))

	result := ValidateUpdateRFC7606(attrs, true, false, true)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(128), result.AttrCode)
	require.Contains(t, result.Description, "7.16")
}

// VALIDATES: the nesting cap admits exactly attrSetMaxDepth levels and rejects one more.
// PREVENTS: the off-by-one the first implementation had, where `depth > cap` admitted
// cap+1 levels while the error message named cap.
//
// RFC requirement: RFC7606-7.16-1 negative -- nesting beyond the supported limit is
// treated as malformed rather than recursed into, since a peer controls the depth.
func TestRFC7606AttrSetNestingCapBoundary(t *testing.T) {
	// nest(n) builds an ATTR_SET value nested n levels deep (n=0 is a bare Origin AS).
	nest := func(n int) []byte {
		v := attrSetValue(65000, nil)
		for range n {
			v = attrSetValue(65000, optAttr(0xc0, 0x80, v))
		}
		return v
	}

	// The outermost ATTR_SET is depth 0, so attrSetMaxDepth levels means
	// attrSetMaxDepth-1 nested inside it.
	deepest := updateWith(optAttr(0xc0, 0x80, nest(attrSetMaxDepth-1)))
	require.Equal(t, RFC7606ActionNone, ValidateUpdateRFC7606(deepest, true, false, true).Action,
		"nesting up to the cap must be accepted")

	tooDeep := updateWith(optAttr(0xc0, 0x80, nest(attrSetMaxDepth)))
	result := ValidateUpdateRFC7606(tooDeep, true, false, true)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action,
		"one level beyond the cap must be rejected")
	require.Contains(t, result.Description, "nested deeper")
}
