// Design: docs/architecture/core-design.md -- LLGR_STALE preservation on egress
// RFC: rfc/short/rfc9494.md -- Section 4.3, LLGR_STALE MUST NOT be removed on readvertisement
// Related: handler.go -- genericCommunityHandler

package filter_community

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestRFC9494LLGRStaleNotRemovedOnEgress verifies that egress community
// suppression (send-community none/extended), explicit AttrModRemove, and
// AttrModSet never strip LLGR_STALE (0xFFFF0006) from a readvertised route,
// while still removing ordinary communities as configured.
//
// RFC 9494 Section 4.3: "The LLGR_STALE community MUST NOT be removed when the
// route is further advertised."
//
// RFC requirement: RFC9494-4.3-2 positive -- AttrModSuppress (send-community
// none), AttrModRemove targeting 0xFFFF0006, and AttrModSet all preserve
// LLGR_STALE (0xFFFF0006) in the emitted COMMUNITIES attribute while dropping
// non-LLGR_STALE communities.
// RFC requirement: RFC9494-4.3-2 negative -- when LLGR_STALE is absent,
// AttrModSuppress and AttrModRemove strip ordinary communities normally and
// drop an emptied COMMUNITIES attribute.
func TestRFC9494LLGRStaleNotRemovedOnEgress(t *testing.T) {
	t.Parallel()

	const otherComm = uint32(0x0064_0001) // 100:1
	const llgrStale = uint32(attribute.CommunityLLGRStale)
	const replacement = uint32(0x00C8_0002) // 200:2

	withLLGR := buildFullCommunityAttr(buildCommunityValues(otherComm, llgrStale))
	withoutLLGR := buildFullCommunityAttr(buildCommunityValues(otherComm))

	t.Run("suppress-preserves-llgr-stale", func(t *testing.T) {
		ops := []filterapi.AttrOp{{Code: byte(attribute.AttrCommunity), Action: filterapi.AttrModSuppress}}
		out, ok := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), withLLGR, ops)
		require.True(t, ok, "COMMUNITIES carrying LLGR_STALE must not be dropped by suppress")
		require.Len(t, out, 8, "header(4) + LLGR_STALE(4)")
		assert.Equal(t, llgrStale, binary.BigEndian.Uint32(out[4:8]))

		outPlain, okPlain := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), withoutLLGR, ops)
		assert.False(t, okPlain, "ordinary COMMUNITIES without LLGR_STALE must still be suppressed")
		assert.Empty(t, outPlain)
	})

	t.Run("remove-preserves-llgr-stale", func(t *testing.T) {
		rmBoth := buildCommunityValues(otherComm, llgrStale)
		ops := []filterapi.AttrOp{{Code: byte(attribute.AttrCommunity), Action: filterapi.AttrModRemove, Buf: rmBoth}}
		out, ok := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), withLLGR, ops)
		require.True(t, ok, "explicit remove targeting LLGR_STALE must preserve it")
		require.Len(t, out, 8, "header(4) + LLGR_STALE(4)")
		assert.Equal(t, llgrStale, binary.BigEndian.Uint32(out[4:8]))

		outPlain, okPlain := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), withoutLLGR, ops)
		assert.False(t, okPlain, "ordinary communities matching remove must still be removed")
		assert.Empty(t, outPlain)
	})

	t.Run("set-preserves-llgr-stale", func(t *testing.T) {
		ops := []filterapi.AttrOp{{
			Code:   byte(attribute.AttrCommunity),
			Action: filterapi.AttrModSet,
			Buf:    buildCommunityValues(replacement),
		}}
		out, ok := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), withLLGR, ops)
		require.True(t, ok)
		require.Len(t, out, 12, "header(4) + replacement(4) + LLGR_STALE(4)")
		assert.Equal(t, replacement, binary.BigEndian.Uint32(out[4:8]))
		assert.Equal(t, llgrStale, binary.BigEndian.Uint32(out[8:12]))

		outPlain, okPlain := planHandlerBytes(communityAttrModHandler, byte(attribute.AttrCommunity), withoutLLGR, ops)
		require.True(t, okPlain)
		require.Len(t, outPlain, 8, "header(4) + replacement(4)")
		assert.Equal(t, replacement, binary.BigEndian.Uint32(outPlain[4:8]))
	})
}
