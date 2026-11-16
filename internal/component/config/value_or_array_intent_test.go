// Design: docs/architecture/config/change-file-structural-ops.md -- whole-list intent.

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValueOrArrayWholeAssignmentHasExplicitIntent proves a whole assignment
// retains one ownership record and its exact sequence through both metadata
// and change-file readers, independently of ordinary additive set syntax.
func TestValueOrArrayWholeAssignmentHasExplicitIntent(t *testing.T) {
	schema := NewSchema()
	schema.Define("plain", &ValueOrArrayNode{Type: TypeString, Ordered: true})
	tree := NewTree()
	members := []string{"second", "first", "two words", "first"}
	tree.SetLeafListMembers("plain", members)
	meta := NewMetaTree()
	entry := MetaEntry{User: "alice", Source: "api", Time: time.Date(2026, 11, 15, 20, 0, 0, 0, time.UTC), Value: JoinLeafListMembers(members)}
	meta.SetEntry("plain", entry)
	content := SerializeChangeFile(tree, meta, nil, schema)
	require.Contains(t, content, "replace-members plain [ second first \"two words\" first ]")
	for cycle := range 2 {
		got, ownership, ops, err := ParseChangeFile(content, NewSetParser(schema))
		require.NoError(t, err)
		assert.Empty(t, ops)
		assert.Equal(t, members, got.GetMultiValues("plain"))
		entries := ownership.SessionEntries(schema, entry.SessionKey())
		require.Len(t, entries, 1)
		assert.Empty(t, entries[0].Entry.Member)
		assert.Equal(t, entry.Value, entries[0].Entry.Value)
		content = SerializeChangeFile(got, ownership, ops, schema)
		assert.Contains(t, content, "replace-members", "cycle %d lost explicit intent", cycle)
	}
}

// TestValueOrArrayOrdinaryMembersKeepIndependentOwnership protects the
// additive set contract, including the ordinary singleton printer spelling.
func TestValueOrArrayOrdinaryMembersKeepIndependentOwnership(t *testing.T) {
	schema := candidateChangeSchema()
	tree, meta, err := NewSetParser(schema).ParseWithMeta("#alice @api set plain [ red blue ]\n")
	require.NoError(t, err)
	entries := meta.SessionEntries(schema, "alice@api")
	require.Len(t, entries, 2)
	assert.NotEmpty(t, entries[0].Entry.Member)
	assert.NotEmpty(t, entries[1].Entry.Member)
	assert.NotContains(t, SerializeChangeFile(tree, meta, nil, schema), "replace-members")
	one := NewTree()
	one.SetLeafListMembers("plain", []string{"red"})
	assert.Equal(t, "set plain red\n", SerializeSetWithMeta(one, nil, schema))
	committed := NewMetaTree()
	committed.SetEntry("plain", MetaEntry{User: "alice", Time: time.Date(2026, 11, 15, 20, 0, 0, 0, time.UTC)})
	content := SerializeSetWithMeta(one, committed, schema)
	assert.NotContains(t, content, "replace-members")
	got, annotations, err := NewSetParser(schema).ParseWithMeta(content)
	require.NoError(t, err)
	assert.Equal(t, []string{"red"}, got.GetMultiValues("plain"))
	assert.Empty(t, annotations.SessionEntries(schema, "alice@api"))
}

// TestValueOrArrayReplacementRefusesOtherIntents checks the explicit command
// cannot replace a scalar, create an empty path, or omit session ownership.
func TestValueOrArrayReplacementRefusesOtherIntents(t *testing.T) {
	schema := candidateChangeSchema()
	schema.Define("block", Container(Field("plain", &ValueOrArrayNode{Type: TypeString})))
	schema.Define("group", List(TypeString, Field("plain", &ValueOrArrayNode{Type: TypeString})))
	schema.Define("inline", InlineList(TypeString, Field("plain", &ValueOrArrayNode{Type: TypeString})))
	for _, input := range []string{
		"replace-members plain [ red ]",
		"#alice replace-members plain [ red ]",
		"#alice @api replace-members color red",
		"#alice @api replace-members words red",
		"#alice @api replace-members bracket [ red ]",
		"#alice @api replace-members plain",
		"#alice @api replace-members plain [ red",
		"#alice @api replace-members block",
		"#alice @api replace-members group",
		"#alice @api replace-members group named",
		"#alice @api replace-members inline",
		"#alice @api replace-members inline named",
	} {
		t.Run(input, func(t *testing.T) {
			parser := NewSetParser(schema)
			_, _, err := parser.ParseWithMeta(input)
			require.Error(t, err)
			_, restored, err := parser.ParseWithMeta("#alice @api set plain [ red blue ]")
			require.NoError(t, err)
			assert.Len(t, restored.SessionEntries(schema, "alice@api"), 2, "refused replacement leaked parser state")
		})
	}
	_, err := NewSetParser(schema).Parse("replace-members plain [ red ]")
	assert.Error(t, err, "the ordinary configuration parser must not accept session intent")
}

// TestValueOrArrayReplacementRestoresInvalidCandidate preserves an invalid
// value for correction but keeps the ordinary metadata reader strict.
func TestValueOrArrayReplacementRestoresInvalidCandidate(t *testing.T) {
	schema := candidateChangeSchema()
	content := "#alice @api replace-members plain [ red bogus ]\n"
	_, _, _, err := ParseChangeFile(content, NewSetParser(schema))
	require.Error(t, err)
	for range 2 {
		tree, meta, ops, err := ParseCandidateChangeFile(content, schema)
		require.NoError(t, err)
		assert.Equal(t, []string{"red", "bogus"}, tree.GetMultiValues("plain"))
		require.Len(t, meta.SessionEntries(schema, "alice@api"), 1)
		content = SerializeChangeFile(tree, meta, ops, schema)
	}
}

// TestValueOrArrayReplacementChecksDelimiters rejects broken syntax even when
// the string member type would otherwise accept the bracket as a value.
func TestValueOrArrayReplacementChecksDelimiters(t *testing.T) {
	schema := NewSchema()
	schema.Define("plain", &ValueOrArrayNode{Type: TypeString, Ordered: true})
	for _, input := range []string{
		"#alice @api replace-members plain [ first",
		"#alice @api replace-members plain first ]",
		"#alice @api replace-members plain [ [ first ]",
	} {
		_, _, _, err := ParseChangeFile(input, NewSetParser(schema))
		require.Error(t, err, input)
		_, _, _, err = ParseCandidateChangeFile(input, schema)
		require.Error(t, err, input)
	}
	content := "#alice @api replace-members plain [ \"[\" \"]\" ]"
	tree, _, _, err := ParseChangeFile(content, NewSetParser(schema))
	require.NoError(t, err)
	assert.Equal(t, []string{"[", "]"}, tree.GetMultiValues("plain"))
}

// TestValueOrArrayReplacementKeepsInactiveSequence checks deactivation lines
// do not append extra copies to an already materialized ordered sequence.
func TestValueOrArrayReplacementKeepsInactiveSequence(t *testing.T) {
	schema := NewSchema()
	schema.Define("plain", &ValueOrArrayNode{Type: TypeString, Ordered: true})
	tree := NewTree()
	members := []string{"second", "first", "first"}
	tree.SetLeafListMembers("plain", members)
	require.NoError(t, tree.DeactivateMultiValue("plain", "second"))
	tree.SetLeafInactive("plain", true)
	meta := NewMetaTree()
	meta.SetEntry("plain", MetaEntry{User: "alice", Source: "api", Value: JoinLeafListMembers(members)})
	content := SerializeSetWithMeta(tree, meta, schema)
	got, ownership, err := NewSetParser(schema).ParseWithMeta(content)
	require.NoError(t, err)
	assert.Equal(t, []MemberState{{Value: "second", Inactive: true}, {Value: "first"}, {Value: "first"}}, got.GetMultiValuesState("plain"))
	assert.Equal(t, []string{"first", "first"}, got.GetMultiValues("plain"))
	assert.True(t, got.IsLeafInactive("plain"))
	_, inactive := got.MultiValueMemberState("plain", "second")
	assert.True(t, inactive)
	assert.Len(t, ownership.SessionEntries(schema, "alice@api"), 1)
}
