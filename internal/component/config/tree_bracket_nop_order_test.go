package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetFormatMemberNopBeforeListLine proves that `nop <path> <member>` on a
// bracket leaf-list stays a member deactivation whichever line comes first,
// in both set formats, and that the reserialized text reads back the same.
// VALIDATES: round-4 NOTE 3; a member nop never deactivates the whole leaf.
// PREVENTS: `nop c items b` before `set c items [ a b ]` leaving the whole
// leaf inactive, and a nop naming a value the list lacks replacing the list.
func TestSetFormatMemberNopBeforeListLine(t *testing.T) {
	schema := NewSchema()
	schema.Define("c", Container(Field("items", bracketLeafList(TypeString))))

	cases := []struct {
		name     string
		text     string
		want     []MemberState
		wantText string
	}{
		{
			name:     "nop before the list line",
			text:     "nop c items b\nset c items [ a b ]\n",
			want:     []MemberState{{Value: "a"}, {Value: "b", Inactive: true}},
			wantText: "set c items [ a b ]\nnop c items b\n",
		},
		{
			name:     "nop after the list line",
			text:     "set c items [ a b ]\nnop c items b\n",
			want:     []MemberState{{Value: "a"}, {Value: "b", Inactive: true}},
			wantText: "set c items [ a b ]\nnop c items b\n",
		},
		{
			name:     "nop naming a value the list lacks",
			text:     "set c items [ a b ]\nnop c items c\n",
			want:     []MemberState{{Value: "a"}, {Value: "b"}, {Value: "c", Inactive: true}},
			wantText: "set c items [ a b c ]\nnop c items c\n",
		},
		{
			name:     "a later list line drops a member it does not name",
			text:     "nop c items b\nset c items [ a ]\n",
			want:     []MemberState{{Value: "a"}},
			wantText: "set c items [ a ]\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := NewSetParser(schema).Parse(tc.text)
			require.NoError(t, err)
			c := tree.GetContainer("c")
			require.NotNil(t, c)
			assert.Equal(t, tc.want, c.GetMultiValuesState("items"))
			assert.False(t, c.IsLeafInactive("items"), "a member nop must not deactivate the whole leaf")

			text := SerializeSet(tree, schema)
			assert.Contains(t, text, tc.wantText)
			again, err := NewSetParser(schema).Parse(text)
			require.NoError(t, err)
			assert.Equal(t, tc.want, again.GetContainer("c").GetMultiValuesState("items"), "round trip")

			metaTree, _, err := NewSetParser(schema).ParseWithMeta(tc.text)
			require.NoError(t, err)
			assert.Equal(t, tc.want, metaTree.GetContainer("c").GetMultiValuesState("items"), "set-with-meta format")
			assert.False(t, metaTree.GetContainer("c").IsLeafInactive("items"))
		})
	}

	// The whole-leaf spelling keeps its meaning.
	tree, err := NewSetParser(schema).Parse("set c items [ a b ]\nnop c items [ a b ]\n")
	require.NoError(t, err)
	assert.True(t, tree.GetContainer("c").IsLeafInactive("items"))
}

// TestSetFormatMemberNopOnRepeatedOrderedMember proves that a member nop on an
// ordered bracket leaf-list whose list line repeats the member is refused with
// the fix, since the deactivation marks a member by its value.
// VALIDATES: round-5 NOTE 3; the refusal setBracketLine makes reachable names
// the problem, why, and the fix (ai/rules/user-facing-errors.md).
// PREVENTS: an operator told only that the deactivation is ambiguous.
func TestSetFormatMemberNopOnRepeatedOrderedMember(t *testing.T) {
	seq := bracketLeafList(TypeString)
	seq.Ordered = true
	schema := NewSchema()
	schema.Define("c", Container(Field("items", seq)))

	_, err := NewSetParser(schema).Parse("nop c items b\nset c items [ a b b ]\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `cannot deactivate "b" in items: it appears 2 times, and items is an ordered list that keeps repeated members, `+
		`so ze cannot tell which copy to deactivate. Fix: list "b" once in items, or deactivate the whole items leaf`)
}
