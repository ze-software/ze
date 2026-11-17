// Design: docs/architecture/core-design.md -- generated AS-path-family edits use the normal rebuild
// Related: wireu/aspath_slot.go -- ASPathEdit records automatic destination-width projection
// Related: filter_delta_handlers.go -- the real default AGGREGATOR handler
// Related: rfc4271_partial_aggregator_test.go -- the ten-case rebuild proof

package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// partialAggregatorRebuildPayload supplies valid mappable AGGREGATOR65001 and
// AS_SEQUENCE65002 at the source width, without any policy edit or AS4 companion.
func partialAggregatorRebuildPayload(flags byte, wide bool) []byte {
	path := []byte{2, 1, 0xfd, 0xea}
	aggregator := []byte{0xfd, 0xe9, 192, 0, 2, 9}
	if wide {
		path = []byte{2, 1, 0, 0, 0xfd, 0xea}
		aggregator = []byte{0, 0, 0xfd, 0xe9, 192, 0, 2, 9}
	}
	attrs := collapseAttr(0x40, 1, []byte{0})
	attrs = append(attrs, collapseAttr(0x40, 2, path)...)
	attrs = append(attrs, collapseAttr(0x40, 3, []byte{192, 0, 2, 254})...)
	attrs = append(attrs, collapseAttr(0x80, 4, []byte{0, 0, 0, 10})...)
	attrs = append(attrs, collapseAttr(flags, 7, aggregator)...)
	attrs = append(attrs, collapseAttr(0xe0, 250, []byte{0x42})...)
	return makeUpdateBody([]byte{24, 198, 51, 100}, attrs, []byte{24, 203, 0, 113})
}

// requirePartialAggregatorRebuild observes the actual encoded body rather than
// trusting the operation count, generator metadata or an unexercised fake handler.
func requirePartialAggregatorRebuild(t *testing.T, payload []byte, flags byte, path, aggregator []byte) {
	t.Helper()
	sections, err := wire.ParseUpdateSections(payload)
	require.NoError(t, err)
	require.Equal(t, []byte{24, 198, 51, 100}, sections.Withdrawn(payload))
	require.Equal(t, []byte{24, 203, 0, 113}, sections.NLRI(payload))
	attrs := sections.Attrs(payload)
	for _, control := range []struct {
		code  attribute.AttributeCode
		flags byte
		value []byte
	}{
		{attribute.AttrOrigin, 0x40, []byte{0}},
		{attribute.AttrASPath, 0x40, path},
		{attribute.AttrNextHop, 0x40, []byte{192, 0, 2, 254}},
		{attribute.AttrMED, 0x80, []byte{0, 0, 0, 10}},
		{attribute.AttributeCode(250), 0xe0, []byte{0x42}},
	} {
		_, gotFlags, value, found := attribute.AttrFind(attrs, control.code)
		require.True(t, found, "untouched or converted attribute %d survives", control.code)
		require.Equal(t, control.flags, byte(gotFlags), "attribute %d flags", control.code)
		require.Equal(t, control.value, value, "attribute %d value", control.code)
	}
	_, _, _, hasAS4Path := attribute.AttrFind(attrs, attribute.AttrAS4Path)
	require.False(t, hasAS4Path, "all path AS numbers are mappable")
	_, _, _, hasAS4Aggregator := attribute.AttrFind(attrs, attribute.AttrAS4Aggregator)
	require.False(t, hasAS4Aggregator, "the mappable AGGREGATOR needs no companion")
	_, gotFlags, value, found := attribute.AttrFind(attrs, attribute.AttrAggregator)
	require.True(t, found, "the accepted AGGREGATOR survives")
	require.Equal(t, aggregator, value, "the exact AS-width projection and IPv4 address survive")
	require.Equal(t, flags, byte(gotFlags), "the surviving AGGREGATOR retains its Partial bit")
}
