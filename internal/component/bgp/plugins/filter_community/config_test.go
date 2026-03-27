package filter_community

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseCommunityDefinitions verifies parsing of named community definitions.
//
// VALIDATES: AC-1, AC-2, AC-3 -- standard, large, extended communities parsed.
// PREVENTS: Config silently dropping community definitions.
func TestParseCommunityDefinitions(t *testing.T) {
	cfg := map[string]any{
		"community": map[string]any{
			"standard": map[string]any{
				"block-list": map[string]any{
					"value": []any{"65000:100", "65000:200"},
				},
			},
			"large": map[string]any{
				"transit-mark": map[string]any{
					"value": []any{"65000:1:1"},
				},
			},
			"extended": map[string]any{
				"rt-set": map[string]any{
					"value": []any{"target:65000:100"},
				},
			},
		},
	}

	defs, err := parseCommunityDefinitions(cfg)
	require.NoError(t, err)

	// Standard
	std, ok := defs["block-list"]
	require.True(t, ok, "block-list should exist")
	assert.Equal(t, communityTypeStandard, std.typ)
	assert.Equal(t, 2, len(std.wireValues), "block-list should have 2 values")

	// Large
	lg, ok := defs["transit-mark"]
	require.True(t, ok, "transit-mark should exist")
	assert.Equal(t, communityTypeLarge, lg.typ)
	assert.Equal(t, 1, len(lg.wireValues), "transit-mark should have 1 value")

	// Extended
	ext, ok := defs["rt-set"]
	require.True(t, ok, "rt-set should exist")
	assert.Equal(t, communityTypeExtended, ext.typ)
	assert.Equal(t, 1, len(ext.wireValues), "rt-set should have 1 value")
}

// TestParseCommunityMultipleValues verifies that a named community can hold
// multiple values, all stored under the same name.
//
// VALIDATES: AC-5 -- multiple values per name.
// PREVENTS: Only the last value being retained.
func TestParseCommunityMultipleValues(t *testing.T) {
	cfg := map[string]any{
		"community": map[string]any{
			"standard": map[string]any{
				"multi": map[string]any{
					"value": []any{"65000:1", "65000:2", "65000:3"},
				},
			},
		},
	}

	defs, err := parseCommunityDefinitions(cfg)
	require.NoError(t, err)

	multi, ok := defs["multi"]
	require.True(t, ok)
	assert.Equal(t, 3, len(multi.wireValues))
}

// TestRejectUndefinedCommunityRef verifies that referencing an undefined
// community name in tag/strip lists produces an error.
//
// VALIDATES: AC-4 -- undefined name rejected at config time.
// PREVENTS: Silent misconfiguration with typos in community names.
func TestRejectUndefinedCommunityRef(t *testing.T) {
	defs := communityDefs{
		"real-community": &communityDef{typ: communityTypeStandard, wireValues: [][]byte{{0, 0, 0, 1}}},
	}

	// Reference a name that doesn't exist in definitions.
	err := validateCommunityRefs(defs, []string{"nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")

	// Valid reference should succeed.
	err = validateCommunityRefs(defs, []string{"real-community"})
	require.NoError(t, err)
}

// TestParseFilterConfig verifies parsing of per-peer filter tag/strip lists.
//
// VALIDATES: AC-8 -- filter config parsed from peer tree.
// PREVENTS: Filter config silently ignored.
func TestParseFilterConfig(t *testing.T) {
	peerCfg := map[string]any{
		"filter": map[string]any{
			"ingress": map[string]any{
				"community": map[string]any{
					"tag":   []any{"block-list"},
					"strip": []any{"unwanted"},
				},
			},
			"egress": map[string]any{
				"community": map[string]any{
					"tag":   []any{"transit-mark"},
					"strip": []any{"internal"},
				},
			},
		},
	}

	fc := parseFilterConfig(peerCfg)
	assert.Equal(t, []string{"block-list"}, fc.ingressTag)
	assert.Equal(t, []string{"unwanted"}, fc.ingressStrip)
	assert.Equal(t, []string{"transit-mark"}, fc.egressTag)
	assert.Equal(t, []string{"internal"}, fc.egressStrip)
}
