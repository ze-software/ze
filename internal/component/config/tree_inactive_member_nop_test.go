package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHierarchicalInactiveMemberLikeNop: `inactive: <leaf-list> <member>` is
// the hierarchical spelling of the set format's `nop <leaf-list> <member>`
// (owner decision 2026-10-11), on a bracket leaf-list and a plain one alike.
//
// VALIDATES: a member the list does not hold is added deactivated, the
// whole leaf stays active, and both formats parse to the same member states.
// PREVENTS: a bracket list refusing the statement, and a plain list adding
// the member active and deactivating the whole leaf.
func TestHierarchicalInactiveMemberLikeNop(t *testing.T) {
	schema := NewSchema()
	schema.Define("c", Container(Field("items", BracketLeafList(TypeString)), Field("plain", ValueOrArray(TypeString))))
	cases := []struct {
		name, leaf, hier, set string
	}{
		{"bracket absent member", "items", "c {\n items [ a b ];\n inactive: items x;\n}\n", "set c items [ a b ]\nnop c items x\n"},
		{"bracket present member", "items", "c {\n items [ a b ];\n inactive: items a;\n}\n", "set c items [ a b ]\nnop c items a\n"},
		{"plain absent member", "plain", "c {\n plain [ a b ];\n inactive: plain x;\n}\n", "set c plain a\nset c plain b\nnop c plain x\n"},
		{"plain empty list", "plain", "c {\n inactive: plain x;\n}\n", "nop c plain x\n"},
		{"plain present member", "plain", "c {\n plain [ a b ];\n inactive: plain b;\n}\n", "set c plain a\nset c plain b\nnop c plain b\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hier, err := NewParser(schema).Parse(tc.hier)
			require.NoError(t, err)
			set, err := NewSetParser(schema).Parse(tc.set)
			require.NoError(t, err)
			got := hier.GetContainer("c")
			want := set.GetContainer("c")
			require.NotNil(t, got)
			require.NotNil(t, want)
			assert.Equal(t, want.GetMultiValuesState(tc.leaf), got.GetMultiValuesState(tc.leaf))
			assert.False(t, got.IsLeafInactive(tc.leaf), "a member statement must not deactivate the whole leaf")
		})
	}
}

// TestHierarchicalInactiveMemberRoundTrips: a list holding a deactivated
// member serializes and reparses to the same member states, and a whole-leaf
// inactive list of one member still reparses as the whole leaf, so the new
// member reading of a single bare value does not capture the serializer's
// whole-leaf form.
//
// VALIDATES: hierarchical output of both shapes reparses unchanged.
// PREVENTS: a whole-leaf inactive single-member list coming back as an
// inactive member after a save.
func TestHierarchicalInactiveMemberRoundTrips(t *testing.T) {
	schema := testLeafListSchema(t)

	tree, err := NewParser(schema).Parse("system {\n\tname-server 9.9.9.9\n\tinactive: name-server 8.8.8.8\n}\n")
	require.NoError(t, err)
	system := tree.GetContainer("system")
	require.NotNil(t, system)
	assert.Equal(t, []MemberState{{Value: "9.9.9.9"}, {Value: "8.8.8.8", Inactive: true}}, system.GetMultiValuesState("name-server"))
	again, err := NewParser(schema).Parse(Serialize(tree, schema))
	require.NoError(t, err)
	assert.Equal(t, system.GetMultiValuesState("name-server"), again.GetContainer("system").GetMultiValuesState("name-server"))

	whole, err := NewParser(schema).Parse("system {\n\tinactive: name-server [ 9.9.9.9 ]\n}\n")
	require.NoError(t, err)
	require.True(t, whole.GetContainer("system").IsLeafInactive("name-server"))
	out := Serialize(whole, schema)
	reparsed, err := NewParser(schema).Parse(out)
	require.NoError(t, err, out)
	ws := reparsed.GetContainer("system")
	require.NotNil(t, ws, out)
	assert.True(t, ws.IsLeafInactive("name-server"), "whole-leaf inactive must survive the round-trip:\n%s", out)
	assert.Equal(t, []MemberState{{Value: "9.9.9.9"}}, ws.GetMultiValuesState("name-server"), out)
}

// TestInactiveLeafListBlameAndAnnotatedMatchSerialize: the blame and the
// annotated views print every inactive leaf-list shape exactly as the
// hierarchical serializer does, so a whole inactive leaf of one member keeps
// its brackets there too. It serializes each shape three ways and compares
// the blame and annotated lines, each with its empty gutter removed, to
// Serialize, then reparses the annotated text.
//
// VALIDATES: round-7 NOTE 3, the bracket branch in serialize_blame.go and
// serialize_annotated.go.
// PREVENTS: the blame or annotated view printing `inactive: <leaf> <member>`
// for a whole inactive leaf, which reads back as one inactive member.
func TestInactiveLeafListBlameAndAnnotatedMatchSerialize(t *testing.T) {
	schema := NewSchema()
	schema.Define("c", Container(Field("items", BracketLeafList(TypeString)), Field("plain", ValueOrArray(TypeString))))
	cases := []struct {
		name, leaf, hier, line string
	}{
		{"plain whole one", "plain", "c {\n inactive: plain [ a ];\n}\n", "\tinactive: plain [ a ]"},
		{"plain whole one plus member", "plain", "c {\n inactive: plain [ a ];\n inactive: plain x;\n}\n", "\tinactive: plain x"},
		{"plain whole all members inactive", "plain", "c {\n inactive: plain [ a ];\n inactive: plain a;\n}\n", "\tinactive: plain [ a ]"},
		{"bracket whole one", "items", "c {\n inactive: items [ a ];\n}\n", "\tinactive: items [ a ]"},
		{"bracket whole one plus member", "items", "c {\n inactive: items [ a ];\n inactive: items x;\n}\n", "\tinactive: items x"},
		{"bracket whole all members inactive", "items", "c {\n inactive: items [ a ];\n inactive: items a;\n}\n", "\tinactive: items [ a ]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := NewParser(schema).Parse(tc.hier)
			require.NoError(t, err)
			want := Serialize(tree, schema)
			require.Contains(t, strings.Split(want, "\n"), tc.line, "the shape under test was not built:\n%s", want)

			blame := strings.Split(SerializeBlame(tree, NewMetaTree(), schema), "\n")
			for i, line := range blame {
				blame[i] = strings.TrimLeft(line, " ")
			}
			assert.Equal(t, want, strings.Join(blame, "\n"), "blame differs from Serialize")

			// A column is enabled, because with none SerializeAnnotatedTree
			// returns Serialize itself and the annotated branch never runs.
			lines := strings.Split(SerializeAnnotatedTree(tree, NewMetaTree(), schema, ShowColumns{Author: true}), "\n")
			for i, line := range lines {
				lines[i] = strings.TrimLeft(line, " ")
			}
			annotated := strings.Join(lines, "\n")
			assert.Equal(t, want, annotated, "annotated differs from Serialize")
			again, err := NewParser(schema).Parse(annotated)
			require.NoError(t, err, annotated)
			c := again.GetContainer("c")
			require.NotNil(t, c, annotated)
			orig := tree.GetContainer("c")
			assert.True(t, c.IsLeafInactive(tc.leaf), "the whole leaf must read back inactive:\n%s", annotated)
			assert.Equal(t, orig.GetMultiValuesState(tc.leaf), c.GetMultiValuesState(tc.leaf), annotated)
		})
	}
}
