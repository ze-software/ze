package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestFilterRegistryEmpty verifies nil policy tree returns empty registry.
//
// VALIDATES: Empty/missing policy section produces usable empty registry.
// PREVENTS: Nil pointer panic on missing policy container.
func TestFilterRegistryEmpty(t *testing.T) {
	schema := config.Container()
	reg, err := BuildFilterRegistry(nil, schema)
	require.NoError(t, err)
	assert.Equal(t, 0, reg.Len())
	assert.Empty(t, reg.Names())
}

// TestFilterRegistryNilSchema verifies nil schema returns empty registry.
//
// VALIDATES: Nil schema does not panic.
// PREVENTS: Nil pointer dereference when YANG fails to load.
func TestFilterRegistryNilSchema(t *testing.T) {
	reg, err := BuildFilterRegistry(config.NewTree(), nil)
	require.NoError(t, err)
	assert.Equal(t, 0, reg.Len())
}

// TestFilterRegistryCollectsNames verifies that filter entries are collected from lists.
//
// VALIDATES: BuildFilterRegistry iterates schema list children and collects tree entries.
// PREVENTS: Filter instances silently ignored.
func TestFilterRegistryCollectsNames(t *testing.T) {
	// Schema: policy has one list child "loop-detection" keyed by string.
	policySchema := config.Container(
		config.Field("loop-detection", config.List(config.TypeString)),
	)

	// Tree: policy has two entries under "loop-detection" list.
	policyTree := config.NewTree()
	policyTree.AddListEntry("loop-detection", "detect-peer-loops", config.NewTree())
	policyTree.AddListEntry("loop-detection", "detect-provider-loops", config.NewTree())

	reg, err := BuildFilterRegistry(policyTree, policySchema)
	require.NoError(t, err)
	assert.Equal(t, 2, reg.Len())
	assert.Equal(t, []string{"detect-peer-loops", "detect-provider-loops"}, reg.Names())
}

// TestFilterRegistryDuplicateNameError verifies duplicate names across types produce error.
//
// VALIDATES: Same filter name in two different types is rejected.
// PREVENTS: Ambiguous filter references silently resolved to wrong type.
func TestFilterRegistryDuplicateNameError(t *testing.T) {
	policySchema := config.Container(
		config.Field("loop-detection", config.List(config.TypeString)),
		config.Field("prefix-limit", config.List(config.TypeString)),
	)

	policyTree := config.NewTree()
	policyTree.AddListEntry("loop-detection", "shared-name", config.NewTree())
	policyTree.AddListEntry("prefix-limit", "shared-name", config.NewTree())

	_, err := BuildFilterRegistry(policyTree, policySchema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate filter name")
	assert.Contains(t, err.Error(), "shared-name")
}

// TestFilterRegistryLookup verifies lookup finds entry by name.
//
// VALIDATES: Lookup returns correct FilterEntry for a known name.
// PREVENTS: Registry lookup returning wrong type or entry.
func TestFilterRegistryLookup(t *testing.T) {
	policySchema := config.Container(
		config.Field("loop-detection", config.List(config.TypeString)),
	)

	policyTree := config.NewTree()
	policyTree.AddListEntry("loop-detection", "my-filter", config.NewTree())

	reg, err := BuildFilterRegistry(policyTree, policySchema)
	require.NoError(t, err)

	entry, ok := reg.Lookup("my-filter")
	assert.True(t, ok)
	assert.Equal(t, "my-filter", entry.Name)
	assert.Equal(t, "loop-detection", entry.Type)
}

// TestFilterRegistryLookupMissing verifies lookup returns false for unknown name.
//
// VALIDATES: Lookup for non-existent name returns false without panic.
// PREVENTS: Panic or incorrect match on unknown filter name.
func TestFilterRegistryLookupMissing(t *testing.T) {
	policySchema := config.Container(
		config.Field("loop-detection", config.List(config.TypeString)),
	)

	policyTree := config.NewTree()
	policyTree.AddListEntry("loop-detection", "existing", config.NewTree())

	reg, err := BuildFilterRegistry(policyTree, policySchema)
	require.NoError(t, err)

	_, ok := reg.Lookup("nonexistent")
	assert.False(t, ok)
}

// TestValidateFilterNamesAllValid verifies no error when all names exist.
//
// VALIDATES: ValidateFilterNames returns nil when every name is registered.
// PREVENTS: False positives on valid filter references.
func TestValidateFilterNamesAllValid(t *testing.T) {
	policySchema := config.Container(
		config.Field("loop-detection", config.List(config.TypeString)),
	)

	policyTree := config.NewTree()
	policyTree.AddListEntry("loop-detection", "foo", config.NewTree())
	policyTree.AddListEntry("loop-detection", "bar", config.NewTree())

	reg, err := BuildFilterRegistry(policyTree, policySchema)
	require.NoError(t, err)

	err = reg.ValidateFilterNames([]string{"foo", "bar"}, "peer 10.0.0.1 import")
	assert.NoError(t, err)
}

// TestValidateFilterNamesUnknown verifies error on unknown filter name.
//
// VALIDATES: ValidateFilterNames returns error with context for unknown name.
// PREVENTS: Silent acceptance of misspelled or missing filter references.
func TestValidateFilterNamesUnknown(t *testing.T) {
	policySchema := config.Container(
		config.Field("loop-detection", config.List(config.TypeString)),
	)

	policyTree := config.NewTree()
	policyTree.AddListEntry("loop-detection", "foo", config.NewTree())

	reg, err := BuildFilterRegistry(policyTree, policySchema)
	require.NoError(t, err)

	err = reg.ValidateFilterNames([]string{"foo", "missing"}, "peer 10.0.0.1 import")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "peer 10.0.0.1 import")
	assert.Contains(t, err.Error(), "missing")
}

// TestValidateFilterNamesInactivePrefix verifies inactive: prefix is stripped.
//
// VALIDATES: "inactive:foo" passes when "foo" is registered.
// PREVENTS: Deactivated filters rejected as unknown.
func TestValidateFilterNamesInactivePrefix(t *testing.T) {
	policySchema := config.Container(
		config.Field("loop-detection", config.List(config.TypeString)),
	)

	policyTree := config.NewTree()
	policyTree.AddListEntry("loop-detection", "foo", config.NewTree())

	reg, err := BuildFilterRegistry(policyTree, policySchema)
	require.NoError(t, err)

	err = reg.ValidateFilterNames([]string{"inactive:foo"}, "peer 10.0.0.1 export")
	assert.NoError(t, err)
}

// TestValidateFilterNamesInactiveUnknown verifies inactive: with unknown base name fails.
//
// VALIDATES: "inactive:bar" fails when "bar" is not registered.
// PREVENTS: inactive: prefix used to bypass validation.
func TestValidateFilterNamesInactiveUnknown(t *testing.T) {
	policySchema := config.Container(
		config.Field("loop-detection", config.List(config.TypeString)),
	)

	policyTree := config.NewTree()
	policyTree.AddListEntry("loop-detection", "foo", config.NewTree())

	reg, err := BuildFilterRegistry(policyTree, policySchema)
	require.NoError(t, err)

	err = reg.ValidateFilterNames([]string{"inactive:bar"}, "peer 10.0.0.1 export")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bar")
}

// TestValidateFilterNamesColonSkipped verifies external plugin names (with colon) are skipped.
//
// VALIDATES: Names containing ":" skip parse-time validation.
// PREVENTS: External plugin filters rejected at config parse.
func TestValidateFilterNamesColonSkipped(t *testing.T) {
	policySchema := config.Container(
		config.Field("loop-detection", config.List(config.TypeString)),
	)

	policyTree := config.NewTree()
	policyTree.AddListEntry("loop-detection", "foo", config.NewTree())

	reg, err := BuildFilterRegistry(policyTree, policySchema)
	require.NoError(t, err)

	err = reg.ValidateFilterNames([]string{"rpki:validate", "foo"}, "peer 10.0.0.1 import")
	assert.NoError(t, err, "colon names should be skipped, plain names validated")
}

// TestValidateFilterNamesInactiveColonSkipped verifies inactive external names are skipped.
//
// VALIDATES: "inactive:plugin:filter" skips validation (strip inactive, still has colon).
// PREVENTS: Deactivated external filters rejected at parse.
func TestValidateFilterNamesInactiveColonSkipped(t *testing.T) {
	reg := &FilterRegistry{entries: make(map[string]FilterEntry)}

	err := reg.ValidateFilterNames([]string{"inactive:rpki:validate"}, "peer 10.0.0.1 import")
	assert.NoError(t, err)
}
