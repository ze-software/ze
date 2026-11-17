// Design: docs/architecture/edge-cases/as4.md -- retained AGGREGATOR flags during AS-width conversion
// Related: aspath_collapse.go -- OLD-speaker receive reconciliation
// Related: aspath_transcode.go -- canonical four-to-two-octet projection

package wireu

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// TestAggregatorPartialSurvivesAS4Collapse widens a valid mappable AGGREGATOR
// through the receive collapse and checks its actual encoded flags and value.
// Complete twins and the NEW-speaker no-work path retain their original bytes.
//
// RFC 4271 Section 5: "If a path with a recognized, transitive optional attribute
// is accepted and passed along to other BGP peers and the Partial bit in the
// Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back
// to 0 by the current AS."
// This is a direct conversion regression, not a Session or socket proof.
func TestAggregatorPartialSurvivesAS4Collapse(t *testing.T) {
	for _, flagCase := range []struct {
		name  string
		flags byte
	}{
		{"complete", 0xc0},
		{"partial", 0xe0},
	} {
		for _, source := range []struct {
			name string
			wide bool
		}{
			{"old source widens", false},
			{"new source fast path", true},
		} {
			t.Run(flagCase.name+"/"+source.name, func(t *testing.T) {
				payload := partialAggregatorPayload(flagCase.flags, source.wide)
				original := append([]byte(nil), payload...)
				dst := make([]byte, CollapseAS4FamilySize(payload))
				n, discards, err := CollapseAS4Family(dst, payload, source.wide)
				require.NoError(t, err)
				require.Empty(t, discards, "the mappable AGGREGATOR is valid")
				require.Equal(t, original, payload, "the borrowed source stays unchanged")
				result := payload
				if source.wide {
					require.Zero(t, n, "a NEW source without AS4 companions needs no collapse")
					require.Equal(t, make([]byte, len(dst)), dst)
				} else {
					require.Positive(t, n, "the OLD source must be widened")
					require.LessOrEqual(t, n, len(dst))
					result = dst[:n]
				}
				requirePartialAggregatorProjection(t, result, flagCase.flags,
					[]byte{2, 1, 0, 0, 0xfd, 0xea},
					[]byte{0, 0, 0xfd, 0xe9, 192, 0, 2, 9})
			})
		}
	}
}

// TestAggregatorPartialSurvivesASWidthTranscode narrows canonical four-octet
// input without changing its mappable AGGREGATOR's received Partial bit.
// Matching-width controls keep their bytes; widening is not this API's contract.
// RFC 4271 Section 5's MUST NOT clear Partial applies to the surviving attribute.
func TestAggregatorPartialSurvivesASWidthTranscode(t *testing.T) {
	for _, flagCase := range []struct {
		name  string
		flags byte
	}{
		{"complete", 0xc0},
		{"partial", 0xe0},
	} {
		for _, widths := range []struct {
			name       string
			sourceWide bool
			targetWide bool
		}{
			{"canonical to old", true, false},
			{"same canonical width", true, true},
			{"same old width", false, false},
		} {
			t.Run(flagCase.name+"/"+widths.name, func(t *testing.T) {
				payload := partialAggregatorPayload(flagCase.flags, widths.sourceWide)
				original := append([]byte(nil), payload...)
				dst := make([]byte, CollapseAS4FamilySize(payload))
				n, err := TranscodeASPath(dst, payload, widths.sourceWide, widths.targetWide)
				require.NoError(t, err)
				require.Equal(t, original, payload, "the borrowed source stays unchanged")
				result := payload
				if widths.sourceWide == widths.targetWide {
					require.Zero(t, n, "equal widths need no projection")
					require.Equal(t, make([]byte, len(dst)), dst)
				} else {
					require.Positive(t, n, "canonical input must be narrowed")
					require.LessOrEqual(t, n, len(dst))
					result = dst[:n]
				}
				path := []byte{2, 1, 0xfd, 0xea}
				aggregator := []byte{0xfd, 0xe9, 192, 0, 2, 9}
				if widths.targetWide {
					path = []byte{2, 1, 0, 0, 0xfd, 0xea}
					aggregator = []byte{0, 0, 0xfd, 0xe9, 192, 0, 2, 9}
				}
				requirePartialAggregatorProjection(t, result, flagCase.flags, path, aggregator)
			})
		}
	}
}

// partialAggregatorPayload carries mappable AS65001, a useful AS_SEQUENCE65002,
// unrelated complete/partial attributes, withdrawn NLRI and advertised NLRI.
// No AS4 companion is present, so selected-companion flag ownership is not assumed.
func partialAggregatorPayload(flags byte, wide bool) []byte {
	path := []byte{0x40, 2, 4, 2, 1, 0xfd, 0xea}
	aggregator := []byte{flags, 7, 6, 0xfd, 0xe9, 192, 0, 2, 9}
	if wide {
		path = []byte{0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xea}
		aggregator = []byte{flags, 7, 8, 0, 0, 0xfd, 0xe9, 192, 0, 2, 9}
	}
	attrs := concatAttrs(
		[]byte{0x40, 1, 1, 0}, path,
		[]byte{0x40, 3, 4, 192, 0, 2, 254},
		[]byte{0x80, 4, 4, 0, 0, 0, 10}, aggregator,
		[]byte{0xe0, 250, 1, 0x42},
	)
	return buildPayload([]byte{24, 198, 51, 100}, attrs, []byte{24, 203, 0, 113})
}

// requirePartialAggregatorProjection checks useful output before the Partial
// assertion, so a missing route or a wrong-width value cannot masquerade as it.
func requirePartialAggregatorProjection(t *testing.T, payload []byte, flags byte, path, aggregator []byte) {
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
