package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetParseInactiveLeaf verifies the set-format `inactive` keyword
// flips the leaf-level marker without disturbing the value.
//
// VALIDATES: round-trip parity with the block-format `inactive: leaf`
// prefix when migrating between formats via `ze config dump --format set`
// or `ze config migrate`.
//
// PREVENTS: silent loss of leaf deactivation when a config is round-tripped
// through the set format (no equivalent verb existed before this).
func TestSetParseInactiveLeaf(t *testing.T) {
	input := `set router-id 1.2.3.4
inactive router-id
`
	tree, err := NewSetParser(testSchema()).Parse(input)
	require.NoError(t, err)

	v, ok := tree.Get("router-id")
	require.True(t, ok)
	assert.Equal(t, "1.2.3.4", v)
	assert.True(t, tree.IsLeafInactive("router-id"))
}

// TestSetParseInactiveContainer verifies a container path emits the
// schema-injected inactive leaf, mirroring the block-format behavior
// for container deactivation.
func TestSetParseInactiveContainer(t *testing.T) {
	input := `set neighbor 1.1.1.1 peer-as 65001
inactive neighbor 1.1.1.1
`
	tree, err := NewSetParser(testSchema()).Parse(input)
	require.NoError(t, err)
	entry := tree.GetList("neighbor")["1.1.1.1"]
	require.NotNil(t, entry)
	v, ok := entry.Get(InactiveLeafName)
	assert.True(t, ok)
	assert.Equal(t, configTrue, v)
}

// TestSetSerializeInactiveLeaf verifies the set-format serializer emits
// an `inactive <path>` line after the matching `set` line for a leaf
// that has been marked inactive.
func TestSetSerializeInactiveLeaf(t *testing.T) {
	tree := NewTree()
	tree.Set("router-id", "1.2.3.4")
	tree.SetLeafInactive("router-id", true)

	out := SerializeSet(tree, testSchema())
	assert.Contains(t, out, "set router-id 1.2.3.4")
	assert.Contains(t, out, "inactive router-id")
	assert.NotContains(t, out, "deactivate router-id",
		"single-keyword design: never emit `deactivate`")
}

// TestSetSerializeInactiveContainer verifies a container with the
// schema-injected inactive leaf set is rendered as a trailing
// `inactive <path>` rather than the legacy `set ... inactive true`.
func TestSetSerializeInactiveContainer(t *testing.T) {
	tree := NewTree()
	entry := NewTree()
	entry.Set("peer-as", "65001")
	entry.Set(InactiveLeafName, configTrue)
	tree.AddListEntry("neighbor", "1.1.1.1", entry)

	out := SerializeSet(tree, testSchema())
	assert.Contains(t, out, "set neighbor 1.1.1.1 peer-as 65001")
	assert.Contains(t, out, "inactive neighbor 1.1.1.1")
	assert.NotContains(t, out, "set neighbor 1.1.1.1 inactive true",
		"set ... inactive true must be replaced by the inactive keyword")
}

// TestSetRoundTripInactiveLeaf verifies parse->serialize->parse stays
// fixed for an inactive leaf, in the set format.
func TestSetRoundTripInactiveLeaf(t *testing.T) {
	input := `set router-id 1.2.3.4
inactive router-id
`
	schema := testSchema()
	tree1, err := NewSetParser(schema).Parse(input)
	require.NoError(t, err)

	out := SerializeSet(tree1, schema)
	require.Contains(t, out, "inactive router-id")

	tree2, err := NewSetParser(schema).Parse(out)
	require.NoError(t, err)
	assert.True(t, TreeEqual(tree1, tree2),
		"set-format round-trip must preserve leaf-inactive flag\nout:\n%s", out)
}

// TestSetParseUnknownVerb verifies the error message lists the verbs.
func TestSetParseUnknownVerb(t *testing.T) {
	_, err := NewSetParser(testSchema()).Parse("bogus router-id 1.2.3.4\n")
	require.Error(t, err)
	assert.True(t,
		strings.Contains(err.Error(), "set/delete/inactive"),
		"error must list the supported verbs, got: %v", err)
}

// TestSetParseRejectsActivateKeyword guards the design choice: there
// is no `activate` verb in the set format. To re-activate, drop the
// `inactive <path>` line and re-load.
func TestSetParseRejectsActivateKeyword(t *testing.T) {
	_, err := NewSetParser(testSchema()).Parse("activate router-id\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command: activate")
}
