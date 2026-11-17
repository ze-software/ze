// Design: docs/architecture/core-design.md -- generated AS-path-family edits use the normal rebuild
// Related: wireu/aspath_slot.go -- ASPathEdit records automatic destination-width projection
// Related: filter_delta_handlers.go -- the real default AGGREGATOR handler

package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// TestAggregatorPartialSurvivesASPathRebuild materializes automatic AS-width
// operations through the reactor's real default handlers and one-pass writer.
// It checks encoded AGGREGATOR flags/value, the useful AS_PATH and both NLRI
// sections; complete twins and untouched same-width AGGREGATORs stay unchanged.
//
// RFC 4271 Section 5: "If a path with a recognized, transitive optional attribute
// is accepted and passed along to other BGP peers and the Partial bit in the
// Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back
// to 0 by the current AS."
// This is the automatic intent-to-rebuild boundary, not a live peer/socket proof.
func TestAggregatorPartialSurvivesASPathRebuild(t *testing.T) {
	for _, flagCase := range []struct {
		name  string
		flags byte
	}{
		{"complete", 0xc0},
		{"partial", 0xe0},
	} {
		for _, projection := range []struct {
			name       string
			sourceWide bool
			targetWide bool
			prepend    []uint32
			path       []byte
		}{
			{"narrow with protocol prepend", true, false, []uint32{65000},
				[]byte{2, 2, 0xfd, 0xe8, 0xfd, 0xea}},
			{"narrow without prepend", true, false, nil, []byte{2, 1, 0xfd, 0xea}},
			{"widen without prepend", false, true, nil, []byte{2, 1, 0, 0, 0xfd, 0xea}},
			{"same wide with protocol prepend", true, true, []uint32{65000},
				[]byte{2, 2, 0, 0, 0xfd, 0xe8, 0, 0, 0xfd, 0xea}},
			{"same old without prepend", false, false, nil, []byte{2, 1, 0xfd, 0xea}},
		} {
			t.Run(flagCase.name+"/"+projection.name, func(t *testing.T) {
				payload := partialAggregatorRebuildPayload(flagCase.flags, projection.sourceWide)
				original := append([]byte(nil), payload...)
				var mods filterapi.ModAccumulator
				var edit wireu.ASPathEdit
				_, err := edit.Record(&mods, payload, wireu.ASPathIntent{
					Prepend: projection.prepend,
					SrcASN4: projection.sourceWide,
					DstASN4: projection.targetWide,
				})
				require.NoError(t, err)
				handlers := attrModHandlersWithDefaults()
				require.NotNil(t, handlers[byte(attribute.AttrAggregator)])
				result, _, failure := buildModifiedPayload(payload, &mods, handlers, nil, nil)
				require.Equal(t, modifyFailureNone, failure)
				require.Equal(t, original, payload, "the borrowed source stays unchanged")
				if result == nil {
					// The writer's explicit no-modification result retains the source.
					result = payload
				}
				aggregator := []byte{0xfd, 0xe9, 192, 0, 2, 9}
				if projection.targetWide {
					aggregator = []byte{0, 0, 0xfd, 0xe9, 192, 0, 2, 9}
				}
				requirePartialAggregatorRebuild(t, result, flagCase.flags, projection.path, aggregator)
			})
		}
	}
}

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
