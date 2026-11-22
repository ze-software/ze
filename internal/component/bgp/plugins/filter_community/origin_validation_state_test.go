// Design: docs/architecture/wire/attributes.md -- RFC 8097 egress extended-community handler tests
// Related: handler.go -- extCommunityAttrModHandler
// RFC 8097 Section 2 -- see rfc/short/rfc8097.md

package filter_community

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestExtCommunityAttrModHandlerZeroesRFC8097Reserved proves that egress
// extended-community Set, Add, and Keep operations always zero the 5-octet
// Reserved field [2..6] of RFC 8097 0x43/0x00 entries without altering sibling
// extended communities.
func TestExtCommunityAttrModHandlerZeroesRFC8097Reserved(t *testing.T) {
	dirtyOV := []byte{0x43, 0x00, 0xde, 0xad, 0xbe, 0xef, 0x55, 0x01}
	cleanOV := []byte{0x43, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
	otherOpaque := []byte{0x43, 0x0b, 0x11, 0x22, 0x33, 0x44, 0x55, 0x02}

	// Add dirty 0x43/0x00 + sibling opaque onto empty source.
	addBuf := append(append([]byte(nil), dirtyOV...), otherOpaque...)
	outAdd, ok := planHandlerBytes(extCommunityAttrModHandler, byte(attribute.AttrExtCommunity), nil, []filterapi.AttrOp{
		{Code: byte(attribute.AttrExtCommunity), Action: filterapi.AttrModAdd, Buf: addBuf},
	})
	require.True(t, ok)
	_, _, valAdd, found := attribute.AttrFind(outAdd, attribute.AttrExtCommunity)
	require.True(t, found)
	require.Equal(t, append(append([]byte(nil), cleanOV...), otherOpaque...), valAdd)

	// Set dirty 0x43/0x00 replaces source and zeroes Reserved octets.
	outSet, ok := planHandlerBytes(extCommunityAttrModHandler, byte(attribute.AttrExtCommunity), nil, []filterapi.AttrOp{
		{Code: byte(attribute.AttrExtCommunity), Action: filterapi.AttrModSet, Buf: append([]byte(nil), dirtyOV...)},
	})
	require.True(t, ok)
	_, _, valSet, found := attribute.AttrFind(outSet, attribute.AttrExtCommunity)
	require.True(t, found)
	require.Equal(t, cleanOV, valSet)

	// Keep existing dirty 0x43/0x00 in source zeroes Reserved octets.
	srcAttr := append([]byte{0xc0, byte(attribute.AttrExtCommunity), 8}, dirtyOV...)
	outKeep, ok := planHandlerBytes(extCommunityAttrModHandler, byte(attribute.AttrExtCommunity), srcAttr, []filterapi.AttrOp{
		{Code: byte(attribute.AttrExtCommunity), Action: filterapi.AttrModAdd, Buf: otherOpaque},
	})
	require.True(t, ok)
	_, _, valKeep, found := attribute.AttrFind(outKeep, attribute.AttrExtCommunity)
	require.True(t, found)
	require.Equal(t, append(append([]byte(nil), cleanOV...), otherOpaque...), valKeep)
}
