// Design: docs/architecture/config/syntax.md — Tree data structure

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bracketFormats names every text form a tree is saved in, with the parser
// that reads it back. A deactivated member and a quoted member must survive
// each one.
var bracketFormats = []struct {
	name  string
	write func(*Tree, *Schema) string
	read  func(*Schema, string) (*Tree, error)
}{
	{"hierarchical", Serialize, func(s *Schema, text string) (*Tree, error) {
		return NewParser(s).Parse(text)
	}},
	{"set", SerializeSet, func(s *Schema, text string) (*Tree, error) {
		return NewSetParser(s).Parse(text)
	}},
	{"set-with-meta", func(tree *Tree, s *Schema) string {
		return SerializeSetWithMeta(tree, NewMetaTree(), s)
	}, func(s *Schema, text string) (*Tree, error) {
		tree, _, err := NewSetParser(s).ParseWithMeta(text)
		return tree, err
	}},
}

// TestBracketLeafListInactiveMemberRoundTrip verifies that a deactivated
// member of a bracket leaf-list is saved and read back as deactivated in every
// format.
//
// VALIDATES: `items [ a b c ]`, deactivate b, save and reload: every format
// answers a, b (inactive), c, and GetSlice answers a c. A whole-leaf inactive
// marker rides along with a member marker.
// PREVENTS: the bracket serializers writing the scalar mirror (active members
// only), so deactivating a member deleted it on the next save.
func TestBracketLeafListInactiveMemberRoundTrip(t *testing.T) {
	schema := NewSchema()
	schema.Define("items", BracketLeafList(TypeString))
	want := []MemberState{{Value: "a"}, {Value: "b", Inactive: true}, {Value: "c"}}

	for _, format := range bracketFormats {
		t.Run(format.name, func(t *testing.T) {
			tree, err := NewParser(schema).Parse(`items [ a b c ]`)
			require.NoError(t, err)
			require.NoError(t, tree.DeactivateMultiValue("items", "b"))

			text := format.write(tree, schema)
			reread, err := format.read(schema, text)
			require.NoError(t, err, "reparse of:\n%s", text)
			assert.Equal(t, want, reread.GetMultiValuesState("items"), "saved:\n%s", text)
			assert.Equal(t, []string{"a", "c"}, reread.GetSlice("items"))
			assert.False(t, reread.IsLeafInactive("items"))

			reread.SetLeafInactive("items", true)
			text = format.write(reread, schema)
			again, err := format.read(schema, text)
			require.NoError(t, err, "reparse of:\n%s", text)
			assert.Equal(t, want, again.GetMultiValuesState("items"), "saved:\n%s", text)
			assert.True(t, again.IsLeafInactive("items"), "saved:\n%s", text)
		})
	}
}

// TestBracketLeafListAllMembersInactiveRoundTrip verifies that a bracket
// leaf-list whose every member is deactivated is still saved.
//
// VALIDATES: `items [ a ]`, deactivate a, save and reload answers a (inactive).
// PREVENTS: the empty mirror making the serializers skip the leaf entirely.
func TestBracketLeafListAllMembersInactiveRoundTrip(t *testing.T) {
	schema := NewSchema()
	schema.Define("items", BracketLeafList(TypeString))
	for _, format := range bracketFormats {
		t.Run(format.name, func(t *testing.T) {
			tree, err := NewParser(schema).Parse(`items [ a ]`)
			require.NoError(t, err)
			require.NoError(t, tree.DeactivateMultiValue("items", "a"))
			text := format.write(tree, schema)
			reread, err := format.read(schema, text)
			require.NoError(t, err, "reparse of:\n%s", text)
			assert.Equal(t, []MemberState{{Value: "a", Inactive: true}}, reread.GetMultiValuesState("items"), "saved:\n%s", text)
		})
	}
}

// TestBracketLeafListQuotedMemberRoundTrip verifies that a member holding a
// space stays one member through every format, and that both parsers read the
// same members from the same list.
//
// VALIDATES: `items [ a "b c" ]` answers [a, "b c"] after a save and reload in
// every format; `set items [ a "b c" ]` answers the same; the mirror quotes the
// member, so Set of the mirror changes nothing.
// PREVENTS: the serializers writing the member bare, and the set parser
// splitting a quoted member on its space, so the two parsers disagreed.
func TestBracketLeafListQuotedMemberRoundTrip(t *testing.T) {
	schema := NewSchema()
	schema.Define("items", BracketLeafList(TypeString))
	want := []string{"a", "b c"}

	set, err := NewSetParser(schema).Parse(`set items [ a "b c" ]`)
	require.NoError(t, err)
	assert.Equal(t, want, set.GetSlice("items"), "set parser")

	for _, format := range bracketFormats {
		t.Run(format.name, func(t *testing.T) {
			tree, err := NewParser(schema).Parse(`items [ a "b c" ]`)
			require.NoError(t, err)
			require.Equal(t, want, tree.GetSlice("items"))

			text := format.write(tree, schema)
			reread, err := format.read(schema, text)
			require.NoError(t, err, "reparse of:\n%s", text)
			assert.Equal(t, want, reread.GetSlice("items"), "saved:\n%s", text)

			mirror, ok := reread.Get("items")
			require.True(t, ok)
			assert.Equal(t, `a "b c"`, mirror)
			reread.Set("items", mirror)
			assert.Equal(t, want, reread.GetSlice("items"), "Set of the mirror")
		})
	}
}

// TestTreeSetMirroredKeepsQuotedAndInactiveMembers verifies Tree.Set on any
// mirrored leaf-list, bracket or not: the value is the active view, spelled as
// the mirror spells it, so quoted members stay whole and a deactivated member
// the value does not name is kept, still deactivated.
//
// VALIDATES: vals [ "a b" c d ] with d deactivated: Set of the mirror changes
// nothing; Set(`"a b" e`) answers "a b", e, d (inactive); a restated
// deactivated member keeps its marker.
// PREVENTS: strings.Fields splitting "a b" into two members and Set dropping
// every deactivated member, which the mirror cannot name.
func TestTreeSetMirroredKeepsQuotedAndInactiveMembers(t *testing.T) {
	schema := NewSchema()
	schema.Define("vals", ValueOrArray(TypeString))
	tree, err := NewParser(schema).Parse(`vals [ "a b" c d ]`)
	require.NoError(t, err)
	require.NoError(t, tree.DeactivateMultiValue("vals", "d"))
	before := tree.GetMultiValuesState("vals")

	mirror, ok := tree.Get("vals")
	require.True(t, ok)
	assert.Equal(t, `"a b" c`, mirror)
	tree.Set("vals", mirror)
	assert.Equal(t, before, tree.GetMultiValuesState("vals"))

	tree.Set("vals", `"a b" e`)
	assert.Equal(t, []MemberState{{Value: "a b"}, {Value: "e"}, {Value: "d", Inactive: true}}, tree.GetMultiValuesState("vals"))

	tree.Set("vals", `d e`)
	assert.Equal(t, []MemberState{{Value: "d", Inactive: true}, {Value: "e"}}, tree.GetMultiValuesState("vals"))
	assert.Equal(t, []string{"e"}, tree.GetSlice("vals"))
}

// TestMergeContainerDedupsAndCarriesInactive verifies that a container written
// as two blocks merges its leaf-lists as one statement would: a set keeps one
// copy of a member, a member deactivated in either block stays deactivated,
// and an ordered sequence keeps its repeats.
//
// VALIDATES: two `box { }` blocks with items [ a b ] and [ b c ], c
// deactivated in the second, answer a, b, c (inactive) and the mirror a b; seq
// [ 1 1 ] then [ 1 ] answers 1 1 1.
// PREVENTS: mergeFrom appending b twice (mirror "a b b c") and dropping the
// second block's member markers.
func TestMergeContainerDedupsAndCarriesInactive(t *testing.T) {
	seq := BracketLeafList(TypeString)
	seq.Ordered = true
	schema := NewSchema()
	schema.Define("box", Container(
		Field("items", BracketLeafList(TypeString)),
		Field("seq", seq),
	))
	tree, err := NewParser(schema).Parse("box {\n\titems [ a b ];\n\tseq [ 1 1 ];\n}\nbox {\n\titems [ b c ];\n\tinactive: items c;\n\tseq [ 1 ];\n}\n")
	require.NoError(t, err)
	box := tree.GetContainer("box")
	require.NotNil(t, box)
	assert.Equal(t, []MemberState{{Value: "a"}, {Value: "b"}, {Value: "c", Inactive: true}}, box.GetMultiValuesState("items"))
	mirror, _ := box.Get("items")
	assert.Equal(t, "a b", mirror)
	assert.Equal(t, []string{"1", "1", "1"}, box.GetSlice("seq"))
}
