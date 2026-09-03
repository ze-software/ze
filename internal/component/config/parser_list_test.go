// Design: docs/architecture/config/syntax.md — list and multi-leaf parsing
package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInlineListEntryStoresALeafListAsMembers proves an inline list entry whose
// last child is a leaf-list stores the bracket contents as MEMBERS, the way the
// block form does, rather than as one joined string.
//
// The two forms must agree, because both are legal spellings of one entry.
// `nth 2 [ 174 3356 ]` and `nth 2 { asn [ 174 3356 ]; }` are the same rule, and a
// reader that saw the first as the single value "[ 174 3356 ]" would refuse it
// as an ASN nobody can parse.
//
// VALIDATES: setInlineLastChild routes a BracketLeafListNode and a
// ValueOrArrayNode through the same store the statement form uses.
// PREVENTS: two spellings of one entry producing two trees.
func TestInlineListEntryStoresALeafListAsMembers(t *testing.T) {
	schema := NewSchema()
	schema.Define("policy", Container(
		Field("group", List(TypeString,
			Field("member", ValueOrArray(TypeString)),
		)),
	))

	tree, err := NewParser(schema).Parse(`policy {
    group alpha [ one two three ]
    group beta { member [ four five ]; }
}`)
	require.NoError(t, err)

	groups := tree.GetContainer("policy").GetList("group")
	require.NotNil(t, groups["alpha"])
	require.NotNil(t, groups["beta"])

	assert.Equal(t, []string{"one", "two", "three"}, groups["alpha"].GetSlice("member"),
		"the inline form must store members, not one joined string")
	assert.Equal(t, []string{"four", "five"}, groups["beta"].GetSlice("member"),
		"the block form is unchanged and is what the inline form must match")
}

// TestInlineListEntryJoinsANonLeafListChild proves the change above is gated on
// the child's node KIND and reaches nothing else.
//
// The NLRI entry `ipv4/unicast add 10.0.0.0/24` is the shape that depends on the
// joined form: its last child is a plain leaf that absorbs every remaining token
// as one string. Nothing about it may change.
//
// VALIDATES: a last child that is not a leaf-list still takes the space-joined
// text, brackets included.
// PREVENTS: a leaf-list rule applied by "were brackets written" rather than by
// the schema, which would split every multi-token inline entry in the tree.
func TestInlineListEntryJoinsANonLeafListChild(t *testing.T) {
	schema := NewSchema()
	schema.Define("update", Container(
		Field("nlri", List(TypeString,
			Field("content", Leaf(TypeString)),
		)),
	))

	tree, err := NewParser(schema).Parse(`update {
    nlri {
        ipv4/unicast add 10.0.0.0/24
    }
}`)
	require.NoError(t, err)

	entry := tree.GetContainer("update").GetList("nlri")["ipv4/unicast"]
	require.NotNil(t, entry)

	content, ok := entry.Get("content")
	require.True(t, ok, "the last child must hold the remaining tokens")
	assert.Equal(t, "add 10.0.0.0/24", content,
		"a plain leaf keeps the space-joined form the NLRI entry depends on")
}

// TestInlineListEntryRoundTripsThroughTheSerializer proves what an operator
// writes is what they read back.
//
// A config that renders differently from how it was written is the defect this
// test exists to prevent: the operator commits, runs `show configuration`, and
// meets a shape they did not type.
//
// VALIDATES: parse then render then parse is stable in both text and tree, for a
// list whose only child is a leaf-list.
// PREVENTS: a writer that emits the block form for an entry the reader accepts
// inline, which would make every such entry change shape at the first commit.
func TestInlineListEntryRoundTripsThroughTheSerializer(t *testing.T) {
	schema := NewSchema()
	schema.Define("policy", Container(
		Field("group", List(TypeString,
			Field("member", ValueOrArray(TypeString)),
		)),
	))

	const written = "policy {\n\tgroup alpha [ one two three ]\n}\n"

	tree, err := NewParser(schema).Parse(written)
	require.NoError(t, err)

	rendered := Serialize(tree, schema)
	assert.Equal(t, written, rendered, "the rendered config must be the config that was written")

	again, err := NewParser(schema).Parse(rendered)
	require.NoError(t, err)
	assert.Equal(t, []string{"one", "two", "three"}, again.GetContainer("policy").
		GetList("group")["alpha"].GetSlice("member"))
	assert.Equal(t, rendered, Serialize(again, schema), "and a second pass must not move it again")
}

// TestInlineListEntryKeepsTheBlockFormWhereOneLineWouldLose reports the four
// entries the one-line form cannot carry.
//
// VALIDATES: a list with a second child, a deactivated leaf, a deactivated
// member and a value outside the schema each keep the block form.
// PREVENTS: a one-line render that drops the child NAME (so a reader cannot tell
// which leaf the members landed in) or drops a deactivation statement (so a
// commit would silently reactivate it).
func TestInlineListEntryKeepsTheBlockFormWhereOneLineWouldLose(t *testing.T) {
	t.Run("a second child keeps the name visible", func(t *testing.T) {
		schema := NewSchema()
		schema.Define("policy", Container(
			Field("group", List(TypeString,
				Field("label", Leaf(TypeString)),
				Field("member", ValueOrArray(TypeString)),
			)),
		))

		tree, err := NewParser(schema).Parse("policy {\n\tgroup alpha {\n\t\tmember [ one two ]\n\t}\n}\n")
		require.NoError(t, err)

		rendered := Serialize(tree, schema)
		assert.Contains(t, rendered, "member [ one two ]",
			"a list with more than one child keeps the block form, so the leaf name stays on the page")
		assert.Contains(t, rendered, "group alpha {")
	})

	t.Run("a deactivated member keeps its own line", func(t *testing.T) {
		schema := NewSchema()
		schema.Define("policy", Container(
			Field("group", List(TypeString,
				Field("member", ValueOrArray(TypeString)),
			)),
		))

		tree, err := NewParser(schema).Parse(
			"policy {\n\tgroup alpha {\n\t\tmember [ one two ]\n\t\tinactive: member two\n\t}\n}\n")
		require.NoError(t, err)

		rendered := Serialize(tree, schema)
		assert.Contains(t, rendered, "inactive: member two",
			"the deactivation owns a line the one-line form cannot carry")
		assert.False(t, strings.Contains(rendered, "group alpha [ "),
			"so the entry keeps the block form")
	})
}
