package rr

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestOriginatorIDRendersTheAddressItCarries pins the formatter against the
// type the PARSER produces, which is the fact the first version of it got
// wrong.
//
// VALIDATES: a parsed ORIGINATOR_ID renders as its address, and the registry
// answers the formatter under the name ExaBGP also uses.
// PREVENTS: the silent fall-through this fixed, in both its forms. Without a
// formatter at all, appendAttributeJSON's generic arm names the attribute
// `attr-9` and prints its hex, which no reader can act on without knowing the
// wire format. With a formatter that asserts the WRONG Go type the symptom is
// identical, because a formatter answering nil falls through to that same arm:
// knownAttrParsers stores what ParseOriginatorID returns, and that is the value
// type OriginatorID, where every other registered formatter takes a pointer.
// Nothing errors, the output simply does not change, so only a test that reads
// the rendered bytes can tell the two apart.
func TestOriginatorIDRendersTheAddressItCarries(t *testing.T) {
	parsed, err := attribute.ParseOriginatorID([]byte{10, 0, 0, 1})
	require.NoError(t, err, "a four-octet ORIGINATOR_ID is well formed (RFC 4456 Section 8)")

	formatter := attribute.GetJSONFormatter(attribute.AttrOriginatorID)
	require.NotNil(t, formatter,
		"no formatter is registered for ORIGINATOR_ID, so every JSON reader gets attr-9 hex")
	assert.Equal(t, "originator-id", formatter.Key,
		"the key ze publishes is the one ExaBGP publishes, so the bridge has nothing to translate")

	rendered := formatter.AppendValue(nil, parsed)
	require.NotNil(t, rendered,
		"the formatter answered nil, which falls through to the generic attr-9 arm: "+
			"it is asserting a Go type the parser does not produce")
	assert.Equal(t, `"10.0.0.1"`, string(rendered))
}

// TestOriginatorIDFormatterRefusesAnotherAttribute pins the fail-closed half.
//
// VALIDATES: the formatter answers nil for an attribute that is not an
// ORIGINATOR_ID, so the generic arm renders it rather than this one printing
// something wrong for it.
// PREVENTS: a type assertion widened to `any` to make the test above pass,
// which would print an address for whatever it was handed.
func TestOriginatorIDFormatterRefusesAnotherAttribute(t *testing.T) {
	formatter := attribute.GetJSONFormatter(attribute.AttrOriginatorID)
	require.NotNil(t, formatter)

	other, err := attribute.ParseNextHop([]byte{192, 0, 2, 1})
	require.NoError(t, err)

	assert.Nil(t, formatter.AppendValue(nil, other),
		"a NEXT_HOP is not an ORIGINATOR_ID and this formatter must not answer for it")
}

// TestOriginatorIDIsNotTheBuilderPointerType states the trap in one assertion,
// so a future edit that reaches for the pointer form is told why it is wrong
// rather than discovering it through unchanged output.
func TestOriginatorIDIsNotTheBuilderPointerType(t *testing.T) {
	parsed, err := attribute.ParseOriginatorID([]byte{10, 0, 0, 1})
	require.NoError(t, err)

	var asValue attribute.Attribute = parsed
	_, isPointer := asValue.(*attribute.OriginatorID)
	assert.False(t, isPointer,
		"ParseOriginatorID returns a VALUE; a formatter asserting the pointer answers nil "+
			"and the reader silently gets attr-9 hex instead")

	value, isValue := asValue.(attribute.OriginatorID)
	require.True(t, isValue)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), netip.Addr(value))
}
