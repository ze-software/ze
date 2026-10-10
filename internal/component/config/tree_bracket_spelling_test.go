// Design: docs/architecture/config/syntax.md — Inactive prefix (deactivate / activate)

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeactivatedMemberOneSpellingPerFormat verifies that a deactivated member
// is written the same way for a bracket and a plain leaf-list in each format.
//
// VALIDATES: `items [ a b ]` (bracket) and `plain [ a b ]` (plain), b
// deactivated: the set and set-with-meta formats write `nop <leaf> b` for both
// and no `inactive` line; the hierarchical format writes `inactive: <leaf> b;`
// for both.
// PREVENTS: the bracket arm writing a second spelling, `inactive <leaf> b`,
// for what a plain leaf-list writes as `nop <leaf> b`.
func TestDeactivatedMemberOneSpellingPerFormat(t *testing.T) {
	schema := NewSchema()
	schema.Define("items", BracketLeafList(TypeString))
	schema.Define("plain", ValueOrArray(TypeString))
	tree, err := NewParser(schema).Parse("items [ a b ];\nplain [ a b ];\n")
	require.NoError(t, err)
	require.NoError(t, tree.DeactivateMultiValue("items", "b"))
	require.NoError(t, tree.DeactivateMultiValue("plain", "b"))

	for _, text := range []string{SerializeSet(tree, schema), SerializeSetWithMeta(tree, NewMetaTree(), schema)} {
		assert.Contains(t, text, "nop items b\n")
		assert.Contains(t, text, "nop plain b\n")
		assert.NotContains(t, text, "inactive ")
	}
	hierarchical := Serialize(tree, schema)
	assert.Contains(t, hierarchical, "inactive: items b\n")
	assert.Contains(t, hierarchical, "inactive: plain b\n")
}

// TestBracketMemberLegacySpellingsRead verifies that the set parsers still read
// the spellings older files hold, and that a `nop` line naming a list keeps its
// whole-leaf meaning.
//
// VALIDATES: `inactive items b` after the list line deactivates b; `nop items
// [ a b ]` deactivates the whole leaf; `nop items c`, c not a member, reads as
// the whole leaf set to c and deactivated; in both set parsers.
// PREVENTS: reading the new member spelling breaking an older file.
func TestBracketMemberLegacySpellingsRead(t *testing.T) {
	schema := NewSchema()
	schema.Define("items", BracketLeafList(TypeString))
	parsers := map[string]func(string) (*Tree, error){
		"set": NewSetParser(schema).Parse,
		"set-with-meta": func(text string) (*Tree, error) {
			tree, _, err := NewSetParser(schema).ParseWithMeta(text)
			return tree, err
		},
	}
	for name, parse := range parsers {
		tree, err := parse("set items [ a b ]\ninactive items b\n")
		require.NoError(t, err, name)
		assert.Equal(t, []MemberState{{Value: "a"}, {Value: "b", Inactive: true}}, tree.GetMultiValuesState("items"), name)
		assert.False(t, tree.IsLeafInactive("items"), name)

		tree, err = parse("nop items [ a b ]\n")
		require.NoError(t, err, name)
		assert.Equal(t, []MemberState{{Value: "a"}, {Value: "b"}}, tree.GetMultiValuesState("items"), name)
		assert.True(t, tree.IsLeafInactive("items"), name)

		tree, err = parse("nop items c\n")
		require.NoError(t, err, name)
		assert.Equal(t, []MemberState{{Value: "c"}}, tree.GetMultiValuesState("items"), name)
		assert.True(t, tree.IsLeafInactive("items"), name)
	}
}

// TestBracketLegacyQuotedContestedValue verifies that a draft written before
// each set-format token became a member still loads: a contested entry was
// spelled as one quoted value holding every member.
//
// VALIDATES: `set addr "10.0.0.0/8 192.0.2.0/24"` on a prefix-typed bracket
// leaf-list reads as the two members, in both set parsers; a quoted value of a
// string-typed bracket leaf-list stays one member.
// PREVENTS: the old spelling failing member typing and refusing the draft.
func TestBracketLegacyQuotedContestedValue(t *testing.T) {
	schema := NewSchema()
	schema.Define("addr", BracketLeafList(TypePrefix))
	schema.Define("items", BracketLeafList(TypeString))
	text := "set addr \"10.0.0.0/8 192.0.2.0/24\"\nset items \"a b\"\n"

	tree, err := NewSetParser(schema).Parse(text)
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.0/8", "192.0.2.0/24"}, tree.GetSlice("addr"))
	assert.Equal(t, []string{"a b"}, tree.GetSlice("items"))

	tree, _, err = NewSetParser(schema).ParseWithMeta("#thomas @ssh %2026-10-11T00:00:00Z " + text)
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.0/8", "192.0.2.0/24"}, tree.GetSlice("addr"))

	_, err = NewSetParser(schema).Parse("set addr \"10.0.0.0/8 nonsense\"\n")
	require.Error(t, err, "a value whose words are not all members is still refused")
}

// TestMergeRefusesRepeatedDeactivatedMember verifies that two blocks merging an
// ordered sequence refuse a deactivation the merged list makes ambiguous, and
// that a merge keeps the repeats of a leaf-list the schema does not name.
//
// VALIDATES: `box { seq [ 1 2 ]; inactive: seq 2; } box { seq [ 2 ]; }` fails
// to parse, naming seq and the member; MergeContainer with a schema that does
// not list `other` keeps `other [ x ]` + `[ x ]` as x x.
// PREVENTS: the merge carrying the marker onto a repeated member, a tree the
// serializer writes and the parser refuses, so the file would not reload; and
// keepsRepeats dropping repeats it cannot prove redundant.
func TestMergeRefusesRepeatedDeactivatedMember(t *testing.T) {
	seq := BracketLeafList(TypeString)
	seq.Ordered = true
	schema := NewSchema()
	box := Container(Field("seq", seq))
	schema.Define("box", box)

	_, err := NewParser(schema).Parse("box {\n\tseq [ 1 2 ];\n\tinactive: seq 2;\n}\nbox {\n\tseq [ 2 ];\n}\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `seq: the blocks merged repeat deactivated member "2" 2 times`)

	_, err = NewParser(schema).Parse("box {\n\tseq [ 1 ];\n}\nbox {\n\tseq [ 2 ];\n\tinactive: seq 2;\n}\n")
	require.NoError(t, err, "a marker on a member the merge does not repeat stays allowed")

	tree := NewTree()
	first := NewTree()
	first.SetSlice("other", []string{"x"})
	second := NewTree()
	second.SetSlice("other", []string{"x"})
	require.NoError(t, tree.MergeContainer("box", first, box))
	require.NoError(t, tree.MergeContainer("box", second, box))
	assert.Equal(t, []string{"x", "x"}, tree.GetContainer("box").GetSlice("other"))
}
