package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// VALIDATES: an inner attribute whose own RFC 7606 action is "attribute discard" does not
// make the enclosing ATTR_SET malformed.
// PREVENTS: inverting RFC 7606's deliberate grading. It assigns attribute-discard to
// AGGREGATOR (Section 7.7) so a route survives a bad AGGREGATOR; escalating that to
// "ATTR_SET is malformed" would withdraw the route instead -- strictly worse than what
// the same attribute would suffer at the top level.
//
// RFC 6368 Section 5's third malformed condition is "The included attributes are malformed
// themselves". A discardable attribute is not malformed in that sense: RFC 7606 Section 2
// separates "attribute discard" from "treat-as-withdraw" precisely so the two are not
// conflated.
//
// The case is a 6-octet (2-octet-AS) AGGREGATOR inside the ATTR_SET. RFC 6368 Section 5
// requires the 4-octet form there, so this IS non-conforming input -- and the point is
// that non-conforming in this particular way costs the attribute, not the route.
//
// RFC requirement: RFC7606-7.16-1 positive -- an inner attribute carrying only an
// attribute-discard action leaves the ATTR_SET well-formed and the route intact.
func TestRFC7606AttrSetInnerDiscardDoesNotWithdraw(t *testing.T) {
	// AGGREGATOR, length 6: the 2-octet-AS encoding. With the inner context forced to
	// asn4=true (RFC 6368 Section 5), validateAggregatorAttr expects 8 and returns
	// RFC7606ActionAttributeDiscard.
	innerAggregator := []byte{0xc0, 0x07, 0x06, 0xfd, 0xe8, 0x0a, 0x00, 0x00, 0x01}
	require.Equal(t, RFC7606ActionAttributeDiscard,
		validateAggregatorAttr(7, 6, nil, true, true).Action,
		"guard: this fixture only discriminates while AGGREGATOR is attribute-discard")

	attrs := updateWith(optAttr(0xc0, 0x80, attrSetValue(65000, innerAggregator)))
	result := ValidateUpdateRFC7606(attrs, true, false, true)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"an inner attribute-discard must not escalate to withdrawing the route")
}
