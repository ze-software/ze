// Design: docs/architecture/config/syntax.md -- multi-word leaf token boundaries.

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetMultiWordLeafKeepsTokenBoundaries compares set input with the
// hierarchical parser and reads both serializers back into typed words.
func TestSetMultiWordLeafKeepsTokenBoundaries(t *testing.T) {
	schema := NewSchema()
	schema.Define("words", &MultiLeafNode{Type: TypeString})
	words := []string{"first", "two words", "first", "literal|pipe"}
	spelling := JoinLeafListMembers(words)
	hierarchical, err := NewParser(schema).Parse("words " + spelling + ";")
	require.NoError(t, err)
	parsed, err := NewSetParser(schema).Parse("set words " + spelling + "\n")
	require.NoError(t, err)
	assert.Equal(t, words, hierarchical.GetSlice("words"))
	assert.Equal(t, words, parsed.GetSlice("words"))
	assert.Equal(t, Serialize(hierarchical, schema), Serialize(parsed, schema))
	for _, input := range []string{SerializeSet(parsed, schema), Serialize(parsed, schema)} {
		var reread *Tree
		if DetectFormat(input) == FormatHierarchical {
			reread, err = NewParser(schema).Parse(input)
		} else {
			reread, err = ParseSetFormat(schema, input, DetectFormat(input))
		}
		require.NoError(t, err)
		assert.Equal(t, words, reread.GetSlice("words"))
	}
}

// TestSetMetaMultiWordLeafKeepsTokenBoundaries proves metadata replay preserves
// quoted words and their single whole-setting owner through serialization.
func TestSetMetaMultiWordLeafKeepsTokenBoundaries(t *testing.T) {
	schema := NewSchema()
	schema.Define("words", &MultiLeafNode{Type: TypeString})
	words := []string{"first", "two words", "first"}
	spelling := JoinLeafListMembers(words)
	input := "#alice @api %2026-11-16T00:30:00Z set words " + spelling + "\n"
	for range 2 {
		tree, meta, err := NewSetParser(schema).ParseWithMeta(input)
		require.NoError(t, err)
		assert.Equal(t, words, tree.GetSlice("words"))
		entry, found := meta.GetEntry("words")
		require.True(t, found)
		assert.Equal(t, spelling, entry.Value)
		assert.Equal(t, "alice", entry.User)
		assert.Equal(t, "api", entry.Source)
		assert.Len(t, meta.SessionEntries(schema, "alice@api%2026-11-16T00:30:00Z"), 1)
		input = SerializeSetWithMeta(tree, meta, schema)
	}
}

// TestInlineMultiWordLeafKeepsItsOneToken proves an inline value stays one
// quoted word without consuming the following independent field.
func TestInlineMultiWordLeafKeepsItsOneToken(t *testing.T) {
	parent := Container(
		Field("words", &MultiLeafNode{Type: TypeString}),
		Field("label", Leaf(TypeString)),
	)
	tree, err := ParseInlineArgs(parent, []string{"words", "two words", "label", "kept"})
	require.NoError(t, err)
	assert.Equal(t, []string{"two words"}, tree.GetSlice("words"))
	value, found := tree.Get("label")
	assert.True(t, found)
	assert.Equal(t, "kept", value)
	schema := NewSchema()
	schema.Define("words", &MultiLeafNode{Type: TypeString})
	schema.Define("label", Leaf(TypeString))
	reread, err := NewSetParser(schema).Parse(SerializeSet(tree, schema))
	require.NoError(t, err)
	assert.Equal(t, []string{"two words"}, reread.GetSlice("words"))
}
