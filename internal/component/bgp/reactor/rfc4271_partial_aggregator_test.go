// Design: docs/architecture/core-design.md -- generated AS-path-family edits use the normal rebuild
// Related: partial_aggregator_test.go -- fixture and encoded-result assertions
// Related: filter_delta_handlers.go -- the real default AGGREGATOR handler

package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
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
	// RFC requirement: RFC4271-5-4 positive -- At the component default rebuild boundary, automatic AS-width AGGREGATOR replacement preserves received Partial=1 and the exact projected value, with complete, unchanged-width, useful-attribute and NLRI controls.

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
