package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// RFC 6368 Section 5: "The included attributes are malformed themselves."
// This malformed condition does not depend on the inner attribute's standalone
// RFC 7606 action. Section 7.16 applies to the enclosing ATTR_SET.
//
// RFC requirement: RFC7606-7.16-1 negative -- a six-octet inner AGGREGATOR
// makes ATTR_SET malformed and withdraws; its valid four-octet-AS counterpart
// survives, while the malformed standalone attribute is only discarded.
func TestRFC7606AttrSetInnerDiscardWithdraws(t *testing.T) {
	innerAggregator := []byte{0xc0, 0x07, 0x06, 0xfd, 0xe8, 0x0a, 0x00, 0x00, 0x01}
	// RFC 7606 Section 7.7: standalone AGGREGATOR uses attribute discard.
	standalone := ValidateUpdateRFC7606(updateWith(innerAggregator), true, false, true)
	require.Equal(t, RFC7606ActionAttributeDiscard, standalone.Action)
	require.Equal(t, uint8(7), standalone.AttrCode)

	attrs := updateWith(optAttr(0xc0, 0x80, attrSetValue(65000, innerAggregator)))
	// RFC 7606 Section 7.16: the malformed inner value invalidates ATTR_SET.
	result := ValidateUpdateRFC7606(attrs, true, false, true)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(128), result.AttrCode)

	validAggregator := []byte{0xc0, 0x07, 0x08, 0, 0, 0xfd, 0xe8, 0x0a, 0, 0, 1}
	validAttrs := updateWith(optAttr(0xc0, 0x80, attrSetValue(65000, validAggregator)))
	require.Equal(t, RFC7606ActionNone, ValidateUpdateRFC7606(validAttrs, true, false, true).Action)
}
