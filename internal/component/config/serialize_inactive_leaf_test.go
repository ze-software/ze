package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSerializeInactiveLeaf verifies a leaf marked via SetLeafInactive
// is rendered with the "inactive: " prefix.
//
// VALIDATES: AC-9 -- serializer emits the same syntax the parser
// accepts, so output round-trips through Parse.
//
// PREVENTS: silent loss of leaf-inactive state at save time.
func TestSerializeInactiveLeaf(t *testing.T) {
	schema := testSchema()
	tree := NewTree()
	tree.Set("router-id", "1.2.3.4")
	tree.SetLeafInactive("router-id", true)

	out := Serialize(tree, schema)
	assert.Contains(t, out, "inactive: router-id 1.2.3.4",
		"inactive leaf must be prefixed; got:\n%s", out)
}

// TestSerializeActiveLeafNotPrefixed guards against false positives:
// a leaf without inactive state must not get the prefix.
func TestSerializeActiveLeafNotPrefixed(t *testing.T) {
	schema := testSchema()
	tree := NewTree()
	tree.Set("router-id", "1.2.3.4")

	out := Serialize(tree, schema)
	assert.NotContains(t, out, "inactive: router-id",
		"active leaf must not carry inactive prefix; got:\n%s", out)
	assert.Contains(t, out, "router-id 1.2.3.4")
}

// TestRoundTripInactiveLeaf verifies the parse->serialize->parse
// cycle preserves leaf-inactive state and value.
//
// VALIDATES: AC-9 (structurally-equivalent round-trip).
//
// PREVENTS: drift where the parser accepts `inactive: leaf value` but
// the serializer drops the prefix on the next save.
func TestRoundTripInactiveLeaf(t *testing.T) {
	schema := testSchema()
	input := `inactive: router-id 1.2.3.4`

	p := NewParser(schema)
	tree1, err := p.Parse(input)
	require.NoError(t, err)
	require.True(t, tree1.IsLeafInactive("router-id"))

	out := Serialize(tree1, schema)
	require.Contains(t, out, "inactive: router-id")

	tree2, err := NewParser(schema).Parse(out)
	require.NoError(t, err)
	assert.True(t, TreeEqual(tree1, tree2),
		"round-trip must preserve leaf-inactive state; serialized:\n%s", out)
}

// TestRoundTripInactiveLeafInListEntry exercises the same round-trip
// for a leaf inside a list entry, where the parser path is
// parser_list.parseListFieldBlock and the serializer path is the
// list-entry inner walk.
func TestRoundTripInactiveLeafInListEntry(t *testing.T) {
	schema := testSchema()
	input := strings.Join([]string{
		`neighbor 192.0.2.1 {`,
		`	inactive: description "test peer";`,
		`	peer-as 65001;`,
		`}`,
	}, "\n")

	p := NewParser(schema)
	tree1, err := p.Parse(input)
	require.NoError(t, err)

	out := Serialize(tree1, schema)
	require.Contains(t, out, "inactive: description")

	tree2, err := NewParser(schema).Parse(out)
	require.NoError(t, err)
	assert.True(t, TreeEqual(tree1, tree2),
		"list-entry leaf round-trip must preserve inactive flag; serialized:\n%s", out)
}
