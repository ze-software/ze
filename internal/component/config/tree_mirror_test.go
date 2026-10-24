// Design: docs/architecture/config/syntax.md — Tree data structure

package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireMirrorAgrees fails unless the member store, ToMap and the scalar
// mirror of the "items" leaf-list in tree all answer want.
func requireMirrorAgrees(t *testing.T, tree *Tree, want []string) {
	t.Helper()
	const name = "items"
	assert.Equal(t, want, tree.GetSlice(name), "member store")
	scalar, ok := tree.Get(name)
	require.True(t, ok, "scalar mirror present")
	assert.Equal(t, strings.Join(want, " "), scalar, "scalar mirror")
	switch len(want) {
	case 1:
		assert.Equal(t, want[0], tree.ToMap()[name], "ToMap")
	default:
		assert.Equal(t, want, tree.ToMap()[name], "ToMap")
	}
}

// TestTreeSetKeepsMirroredLeafListConsistent verifies that Tree.Set on a
// leaf-list held as members plus a scalar mirror replaces the members, so no
// reader sees the old list.
//
// VALIDATES: parse `items [ a ]`, Set("a c"), then AppendSlice([b]) answers
// [a c b] from every reader; a deactivated member a whole Set restates keeps
// its marker, so the active view is the rest.
// PREVENTS: Set writing the scalar alone, which left the member store (read by
// GetSlice, ToMap and the editor's merge) on the old list while the serializer
// (which reads the scalar) wrote the new one: a second load merge lost members.
func TestTreeSetKeepsMirroredLeafListConsistent(t *testing.T) {
	schema := NewSchema()
	schema.Define("items", bracketLeafList(TypeString))
	tree, err := NewParser(schema).Parse(`items [ a ]`)
	require.NoError(t, err)

	tree.Set("items", "a c")
	requireMirrorAgrees(t, tree, []string{"a", "c"})
	assert.Equal(t, "items [ a c ]\n", Serialize(tree, schema))

	tree.AppendSlice("items", []string{"b"})
	requireMirrorAgrees(t, tree, []string{"a", "c", "b"})

	require.NoError(t, tree.DeactivateMultiValue("items", "c"))
	tree.Set("items", "c d")
	requireMirrorAgrees(t, tree, []string{"d"})
	assert.Equal(t, []MemberState{{Value: "c", Inactive: true}, {Value: "d"}}, tree.GetMultiValuesState("items"))

	clone := tree.Clone()
	clone.Set("items", "e")
	requireMirrorAgrees(t, clone, []string{"e"})
	assert.Equal(t, []MemberState{{Value: "e"}, {Value: "c", Inactive: true}}, clone.GetMultiValuesState("items"))
	requireMirrorAgrees(t, tree, []string{"d"})

	tree.Delete("items")
	tree.Set("items", "x y")
	assert.Nil(t, tree.GetSlice("items"), "a deleted leaf-list is no longer mirrored")
}

// TestTreeSetLeavesUnmirroredValuesWhole verifies that Set never splits a value
// whose members are not a space-joined mirror: the plugin-map singleton view
// and freeform entries keep their spaces.
//
// VALIDATES: Set("description", "two words") after AppendValue keeps one value.
// PREVENTS: the mirror rule turning a multi-word string into a member list.
func TestTreeSetLeavesUnmirroredValuesWhole(t *testing.T) {
	tree := NewTree()
	tree.Set("description", "one")
	tree.AppendValue("description", "one")
	tree.Set("description", "two words")
	v, ok := tree.Get("description")
	require.True(t, ok)
	assert.Equal(t, "two words", v)
	assert.Equal(t, []string{"one"}, tree.GetSlice("description"))
}

// TestSetParserBracketStoresMembers verifies that the set-format parsers store
// a bracket leaf-list as the hierarchical parser does, members plus the
// mirror, with or without metadata. A set line states the whole list, so a
// later line for the leaf replaces an earlier one, and a quoted token is one
// member, as the hierarchical parser reads it.
//
// VALIDATES: `set items [ a b ]` then `set items [ c d ]` answers [c d] from
// every reader; `set items "a b"` answers the one member "a b".
// PREVENTS: the set parsers writing the scalar alone, so a set-format load
// handed the editor no members to merge, and ToMap answered the joined text
// as one value.
func TestSetParserBracketStoresMembers(t *testing.T) {
	schema := NewSchema()
	schema.Define("items", bracketLeafList(TypeString))
	input := "set items [ a b ]\nset items [ c d ]\n"
	want := []string{"c", "d"}

	tree, err := NewSetParser(schema).Parse(input)
	require.NoError(t, err)
	requireMirrorAgrees(t, tree, want)

	tree, _, err = NewSetParser(schema).ParseWithMeta("#thomas %2026-10-10T19:59:37Z set items \"a b\"\n#thomas %2026-10-10T19:59:38Z set items [ c d ]\n")
	require.NoError(t, err)
	requireMirrorAgrees(t, tree, want)

	tree, err = NewSetParser(schema).Parse("set items \"a b\"\n")
	require.NoError(t, err)
	assert.Equal(t, []string{"a b"}, tree.GetSlice("items"))
	scalar, ok := tree.Get("items")
	require.True(t, ok)
	assert.Equal(t, `"a b"`, scalar, "the mirror quotes the member")
}
