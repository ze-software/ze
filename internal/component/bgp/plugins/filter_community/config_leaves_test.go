package filter_community

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ingressCommunityCfg wraps a leaf map at the path parseFilterConfig walks.
func ingressCommunityCfg(leaves map[string]any) map[string]any {
	return map[string]any{
		"filter": map[string]any{
			"ingress": map[string]any{"community": leaves},
		},
	}
}

// TestParseIngressLeaves reads each new leaf in both shapes the config path
// delivers: the framework's string form and a JSON round-trip's typed form.
//
// PREVENTS: a leaf that works from a config file and is silently ignored over the
// API, or the reverse, which is invisible until an operator uses the other
// one.
func TestParseIngressLeaves(t *testing.T) {
	t.Run("string form, as the config framework delivers leaves", func(t *testing.T) {
		fc := parseFilterConfig(ingressCommunityCfg(map[string]any{
			"relation-tag":          "true",
			"relation-function":     "64",
			"scrub-own-ga":          "true",
			"scrub-keep-function":   []string{"64", "65"},
			"blackhole-propagation": "no-advertise",
		}))

		assert.True(t, fc.relationTagEnabled())
		assert.Equal(t, uint32(64), fc.relationFunctionNumber())
		assert.True(t, fc.scrubEnabled())
		assert.Equal(t, []uint32{64, 65}, fc.scrubKeepFuncs)
		assert.Equal(t, blackholeGuardNoAdvertise, fc.blackholeGuardToken())
	})

	t.Run("typed form, as a JSON round-trip delivers leaves", func(t *testing.T) {
		fc := parseFilterConfig(ingressCommunityCfg(map[string]any{
			"relation-tag":          true,
			"relation-function":     float64(64),
			"scrub-own-ga":          false,
			"scrub-keep-function":   []any{float64(64), float64(65)},
			"blackhole-propagation": "no-export",
		}))

		assert.True(t, fc.relationTagEnabled())
		assert.Equal(t, uint32(64), fc.relationFunctionNumber())
		assert.False(t, fc.scrubEnabled())
		assert.Equal(t, []uint32{64, 65}, fc.scrubKeepFuncs)
		assert.Equal(t, blackholeGuardNoExport, fc.blackholeGuardToken())
	})
}

// TestParseIngressLeafDefaults pins the shipped defaults, which is what
// every existing config gets on upgrade.
//
// VALIDATES: AC-13 (the default half), AC-21 (the guard-off half)
// PREVENTS: a wire-visible change for an operator who configured none of this.
func TestParseIngressLeafDefaults(t *testing.T) {
	fc := parseFilterConfig(map[string]any{})

	assert.False(t, fc.relationTagEnabled())
	assert.Equal(t, uint32(3), fc.relationFunctionNumber())
	assert.False(t, fc.scrubEnabled())
	assert.Nil(t, fc.scrubKeepSet())
	assert.Equal(t, blackholeGuardNone, fc.blackholeGuardToken())
	assert.False(t, fc.hasAnyRule(), "a peer with no rule is skipped before any wire scan")
}

// TestRelationFunctionLeafBoundaries drives the relation-function leaf
// across its declared uint32 range. Spec Boundary Tests row "Function
// number leaf" (0-4294967295), where a value of more than 32 bits is rejected at
// config.
//
// A rejected value reads as ABSENT, not as zero, so the inherited or
// default number stands. Reading it as 0 would silently move the convention
// to function 0 for the whole peer.
//
// PREVENTS: a truncated or wrapped function number writing tags at a number
// nobody configured.
func TestRelationFunctionLeafBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  uint32
	}{
		{"0, the first valid value", "0", 0},
		{"4294967295, the last valid value", "4294967295", 4294967295},
		{"4294967296 is more than the range and is refused", "4294967296", defaultRelationFunction},
		{"a negative value is refused", "-1", defaultRelationFunction},
		{"a non-numeric value is refused", "three", defaultRelationFunction},
		{"a float of more than the range is refused", float64(4294967296), defaultRelationFunction},
		{"a negative float is refused", float64(-1), defaultRelationFunction},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := parseFilterConfig(ingressCommunityCfg(map[string]any{"relation-function": tt.value}))
			assert.Equal(t, tt.want, fc.relationFunctionNumber())
		})
	}
}

// TestScrubKeepFunctionLeafDropsUnparseableValues pins that a value which
// does not parse is DROPPED rather than defaulted to 0.
//
// The keep-list is a security control and 0 is a real function number. So a
// silent zero would keep a value the operator never listed.
//
// PREVENTS: the zero-value trap of ai/rules/evidence.md in a keep-list.
func TestScrubKeepFunctionLeafDropsUnparseableValues(t *testing.T) {
	fc := parseFilterConfig(ingressCommunityCfg(map[string]any{
		"scrub-keep-function": []string{"64", "not-a-number", "4294967296"},
	}))

	assert.Equal(t, []uint32{64}, fc.scrubKeepFuncs)
	assert.Equal(t, map[uint32]bool{64: true}, fc.scrubKeepSet())
}

// TestMergeScalarLeavesOverrideOnlyWhenSet verifies the inheritance rule: a
// level that said nothing leaves the inherited value standing, and a level
// that spoke replaces it.
//
// PREVENTS: a group-level setting being canceled by every peer under it, which is
// what treating an unset leaf as false would do.
func TestMergeScalarLeavesOverrideOnlyWhenSet(t *testing.T) {
	group := parseFilterConfig(ingressCommunityCfg(map[string]any{
		"scrub-own-ga":          "true",
		"relation-function":     "64",
		"blackhole-propagation": "no-export",
	}))

	t.Run("a silent peer level inherits", func(t *testing.T) {
		merged := mergeFilterConfigs(group, parseFilterConfig(map[string]any{}))
		assert.True(t, merged.scrubEnabled())
		assert.Equal(t, uint32(64), merged.relationFunctionNumber())
		assert.Equal(t, blackholeGuardNoExport, merged.blackholeGuardToken())
	})

	t.Run("a peer level that speaks overrides", func(t *testing.T) {
		peer := parseFilterConfig(ingressCommunityCfg(map[string]any{
			"scrub-own-ga":          "false",
			"blackhole-propagation": "no-advertise",
		}))
		merged := mergeFilterConfigs(group, peer)
		assert.False(t, merged.scrubEnabled(), "an explicit false does override")
		assert.Equal(t, uint32(64), merged.relationFunctionNumber(), "unspoken, so inherited")
		assert.Equal(t, blackholeGuardNoAdvertise, merged.blackholeGuardToken())
	})
}

// TestMergeKeepListIsCumulative verifies the keep-list follows
// ze:cumulative, as the other leaf-lists in this module do: levels combine
// rather than replace.
func TestMergeKeepListIsCumulative(t *testing.T) {
	base := parseFilterConfig(ingressCommunityCfg(map[string]any{"scrub-keep-function": []string{"64"}}))
	overlay := parseFilterConfig(ingressCommunityCfg(map[string]any{"scrub-keep-function": []string{"65", "64"}}))

	merged := mergeFilterConfigs(base, overlay)
	assert.Equal(t, []uint32{64, 65}, merged.scrubKeepFuncs, "combined, with no duplicate")
}

// TestValidateScrubKeepListRefusesTheRelationFunction verifies the config
// is REFUSED rather than silently corrected, so an operator holding a false
// belief is told about it.
//
// VALIDATES: AC-5 (the config-surface half)
// PREVENTS: a keep-list line that looks in force and is ignored, which is how a
// security control comes to be trusted and never applied.
func TestValidateScrubKeepListRefusesTheRelationFunction(t *testing.T) {
	err := validateScrubKeepList(filterConfig{
		relationTag:    new(true),
		scrubKeepFuncs: []uint32{3, 64},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "relation-function")

	t.Run("at a moved function number", func(t *testing.T) {
		assert.Error(t, validateScrubKeepList(filterConfig{
			relationTag:      new(true),
			relationFunction: new(uint32(64)),
			scrubKeepFuncs:   []uint32{64},
		}))
		assert.NoError(t, validateScrubKeepList(filterConfig{
			relationTag:      new(true),
			relationFunction: new(uint32(64)),
			scrubKeepFuncs:   []uint32{3},
		}), "function 3 is free once the convention has moved")
	})

	t.Run("with the tag off the number is the operator's to use", func(t *testing.T) {
		assert.NoError(t, validateScrubKeepList(filterConfig{scrubKeepFuncs: []uint32{3}}))
	})
}
