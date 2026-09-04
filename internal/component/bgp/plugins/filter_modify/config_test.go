// Design: docs/architecture/core-design.md -- route modify filter config parsing
// Related: config.go -- parseAttributeDefaults, the reader of bgp/defaults/attribute
// Related: filter_modify.go -- applyBGPConfig, one delivery of the bgp subtree

package filter_modify

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: AC-7 -- a reload of bgp { defaults { attribute { } } } governs the
//
//	routes that arrive after it, and one delivery installs its base and
//	its modifiers together or installs neither.
//
// PREVENTS: a base that latches on the first delivery and never moves again; a
//
//	refused delivery leaving the modifiers of one config computing from
//	the base of another.

// attributeDefaultsTree wraps one bgp { defaults { attribute { } } } block in
// the config subtree the plugin is delivered, beside the modifier the tests
// below drive. The local-preference value is a string, which is the form a YANG
// leaf arrives in. extraModifiers is merged into the policy block, so a test
// can add a definition the parser refuses.
func attributeDefaultsTree(localPreference string, extraModifiers map[string]any) map[string]any {
	modifiers := map[string]any{
		"INC-LP": map[string]any{
			"increment": map[string]any{localPreferenceAttr: float64(50)},
		},
	}
	maps.Copy(modifiers, extraModifiers)

	return map[string]any{
		"defaults": map[string]any{
			"attribute": map[string]any{localPreferenceAttr: localPreference},
		},
		"policy": map[string]any{"modify": modifiers},
	}
}

// TestAttributeDefaultsReloadTakesEffect covers AC-7. A reload is one more
// delivery of the config subtree, so a changed default governs the routes that
// arrive after it and nothing else.
//
// The route is driven through handleFilterUpdate rather than through
// buildDynamicDelta, because the RPC is where a route reaches this plugin, and
// applyBGPConfig is the body the SDK's OnConfigure callback runs.
func TestAttributeDefaultsReloadTakesEffect(t *testing.T) {
	previousDefs := defsByName.Load()
	t.Cleanup(func() { defsByName.Store(previousDefs) })
	storeAbsentBase(t, absentBase.Load())

	update := func() string {
		out := handleFilterUpdate(&sdk.FilterUpdateInput{
			Filter:    "INC-LP",
			Direction: directionImport,
			Peer:      "10.0.0.1",
			Update:    absentSubject,
		})
		require.Equal(t, sdk.FilterModify, out.Action)
		return out.Update
	}

	require.NoError(t, applyBGPConfig(attributeDefaultsTree("100", nil)))
	require.Equal(t, "local-preference 150", update(), "100 plus 50")

	require.NoError(t, applyBGPConfig(attributeDefaultsTree("80", nil)))
	require.Equal(t, "local-preference 130", update(), "the reload moved the base to 80")

	// And back, because a reload that only ever moves one way would pass the
	// two lines above with a base that latched on the first delivery.
	require.NoError(t, applyBGPConfig(attributeDefaultsTree("100", nil)))
	require.Equal(t, "local-preference 150", update(), "the base follows every delivery")
}

// TestAttributeDefaultsRefusedConfigInstallsNothing covers the failure half of
// one delivery. A subtree the plugin cannot parse leaves the previous
// configuration running rather than half of the new one.
func TestAttributeDefaultsRefusedConfigInstallsNothing(t *testing.T) {
	previousDefs := defsByName.Load()
	t.Cleanup(func() { defsByName.Store(previousDefs) })
	storeAbsentBase(t, absentBase.Load())

	require.NoError(t, applyBGPConfig(attributeDefaultsTree("80", nil)))

	// The base moves too, so a delivery that installed the base and then
	// refused the modifiers would be caught by the assertion below.
	refused := attributeDefaultsTree("42", map[string]any{
		"BAD": map[string]any{"increment": map[string]any{"no-such-attribute": float64(1)}},
	})
	require.Error(t, applyBGPConfig(refused))

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter:    "INC-LP",
		Direction: directionImport,
		Peer:      "10.0.0.1",
		Update:    absentSubject,
	})
	require.Equal(t, sdk.FilterModify, out.Action)
	require.Equal(t, "local-preference 130", out.Update, "the accepted delivery is still the running one")
}

// TestAigpDefaultLeafDoesNotExist covers AC-6. There is no aigp default and
// there must not be one: RFC 7311 Section 4.1 removes a route with no AIGP TLV
// from consideration rather than scoring it, so no number stands in for the
// absence, and Section 3.4.1 forbids ze to create the attribute outside the
// AIGP administrative domain.
//
// The reason is asserted at the container, because that is where somebody about
// to add the leaf would read it.
//
// The spec's TDD plan named internal/component/config/yang/validator_test.go
// for this test. That package cannot host it: the schema lookup needs the bgp
// module, ze-bgp-conf registers from internal/component/bgp/yang, and that
// package imports internal/component/config/yang, so an in-package test there
// would be an import cycle.
func TestAigpDefaultLeafDoesNotExist(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	_, err = schema.Lookup(attributeDefaultsPath + "/" + aigpAttr)
	require.Error(t, err, "an aigp default leaf must not exist")

	node, err := schema.Lookup(attributeDefaultsPath)
	require.NoError(t, err)
	container, ok := node.(*config.ContainerNode)
	require.True(t, ok)
	require.ElementsMatch(t, []string{medAttr, localPreferenceAttr}, container.Children(),
		"the container declares the two attributes that have a base, and no third")
	require.Contains(t, container.Description, "RFC 7311",
		"the reason for the absence belongs where somebody would add the leaf")
	require.Contains(t, container.Description, aigpAttr)

	// A tree that names it anyway is refused rather than obeyed. YANG
	// validation refuses the key at config load, and this is the plugin's own
	// half of that pair: a hand-written or migrated tree never passed it.
	_, err = parseAttributeDefaults(map[string]any{
		"defaults": map[string]any{
			"attribute": map[string]any{aigpAttr: float64(10)},
		},
	})
	require.ErrorContains(t, err, aigpAttr)
}

// TestReadOptionalUint32RefusesAnIntAboveUint32 covers the coercion every
// attribute default passes through on its way out of parseAttributeDefaults.
//
// VALIDATES: each numeric form the config tree can carry is refused above the
// uint32 range rather than truncated, and 4294967295 itself is accepted.
//
// PREVENTS: the int branch reading its lower bound alone. int is 64 bits here,
// so 4294967296 wrapped to 0 and reported true, and a med default of that value
// reached the arithmetic as 0. Restore the missing upper bound check and the
// first subtest fails.
func TestReadOptionalUint32RefusesAnIntAboveUint32(t *testing.T) {
	const aboveRange = 4294967296

	refused := map[string]any{
		"int":     aboveRange,
		"int64":   int64(aboveRange),
		"uint64":  uint64(aboveRange),
		"float64": float64(aboveRange),
		"string":  "4294967296",
	}
	for name, value := range refused {
		t.Run(name, func(t *testing.T) {
			got, ok := readOptionalUint32(value)
			require.False(t, ok, "a value above the range must be refused, not truncated")
			require.Zero(t, got)
		})
	}

	// The top of the range is a value an operator can write, so the bound above
	// must not have moved it.
	top, ok := readOptionalUint32(int(4294967295))
	require.True(t, ok)
	require.Equal(t, uint32(4294967295), top)
}

// TestAttributeDefaultLeafRangeHoldsAtTheBoundaries covers the spec's numeric
// boundary table for both leaves. ValidateLeafValue is the check the config file
// parse path runs (parser_list.go), so the refusal an operator meets is the one
// asserted here.
//
// 0 is VALID for both, and it is med's declared default, so the range cannot be
// written as 1..4294967295 the way an increment value is.
func TestAttributeDefaultLeafRangeHoldsAtTheBoundaries(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	for _, name := range []string{medAttr, localPreferenceAttr} {
		t.Run(name, func(t *testing.T) {
			node, err := schema.Lookup(attributeDefaultsPath + "/" + name)
			require.NoError(t, err)
			leaf, ok := node.(*config.LeafNode)
			require.True(t, ok)
			require.NotEmpty(t, leaf.Ranges, "the leaf states its bound rather than leaving it to the type")

			require.NoError(t, config.ValidateLeafValue(leaf, "0"), "0 is a value an operator can write")
			require.NoError(t, config.ValidateLeafValue(leaf, "4294967295"), "the top of the range")
			require.Error(t, config.ValidateLeafValue(leaf, "4294967296"), "one above the range")
			require.Error(t, config.ValidateLeafValue(leaf, "-1"), "below the range")
		})
	}
}
