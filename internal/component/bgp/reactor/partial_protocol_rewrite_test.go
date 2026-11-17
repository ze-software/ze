// Design: docs/architecture/bgp/egress-attribute-rules.md -- surviving attribute flags
// Related: forward_prefix_sid.go -- automatic next-hop companion cleanup
// Related: forward_build.go -- the default registered-handler rebuild
// Related: rfc4271_partial_protocol_rewrite_test.go -- the eight-case cleanup proof

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// partialPrefixSIDPayload supplies a valid canonical path and a Prefix-SID whose
// TLV bytes come from the existing propagation fixture, without running its rails.
func partialPrefixSIDPayload(flags byte, value []byte) []byte {
	attrs := collapseAttr(0x40, 1, []byte{0})
	attrs = append(attrs, collapseAttr(0x40, 2, []byte{2, 1, 0, 0, 0xfd, 0xe9})...)
	attrs = append(attrs, collapseAttr(0x40, 3, []byte{192, 0, 2, 1})...)
	attrs = append(attrs, collapseAttr(0x80, 4, []byte{0, 0, 0, 10})...)
	attrs = append(attrs, collapseAttr(flags, byte(attribute.AttrPrefixSID), value)...)
	attrs = append(attrs, collapseAttr(0xe0, 250, []byte{0x42})...)
	return makeUpdateBody(nil, attrs, []byte{24, 203, 0, 113})
}

// requirePartialPrefixSIDResult checks the actual next hop, retained route and
// TLV bytes before the final flag assertion; legitimate empty-result removal wins.
func requirePartialPrefixSIDResult(t *testing.T, payload []byte, nextHop netip.Addr, value []byte, flags byte) {
	t.Helper()
	sections, err := wire.ParseUpdateSections(payload)
	require.NoError(t, err)
	require.Empty(t, sections.Withdrawn(payload))
	require.Equal(t, []byte{24, 203, 0, 113}, sections.NLRI(payload))
	attrs := sections.Attrs(payload)
	for _, control := range []struct {
		code  attribute.AttributeCode
		flags byte
		value []byte
	}{
		{attribute.AttrOrigin, 0x40, []byte{0}},
		{attribute.AttrASPath, 0x40, []byte{2, 1, 0, 0, 0xfd, 0xe9}},
		{attribute.AttrNextHop, 0x40, nextHop.AsSlice()},
		{attribute.AttrMED, 0x80, []byte{0, 0, 0, 10}},
		{attribute.AttributeCode(250), 0xe0, []byte{0x42}},
	} {
		_, gotFlags, gotValue, found := attribute.AttrFind(attrs, control.code)
		require.True(t, found, "useful attribute %d survives", control.code)
		require.Equal(t, control.value, gotValue, "attribute %d value", control.code)
		require.Equal(t, control.flags, byte(gotFlags), "attribute %d flags", control.code)
	}
	_, gotFlags, gotValue, found := attribute.AttrFind(attrs, attribute.AttrPrefixSID)
	if value == nil {
		require.False(t, found, "Service-only cleanup may remove the whole attribute")
		return
	}
	require.True(t, found, "useful non-Service TLVs remain")
	require.Equal(t, value, gotValue, "retained TLVs remain byte-identical")
	require.Equal(t, flags, byte(gotFlags), "the surviving Prefix-SID retains its Partial bit")
}
