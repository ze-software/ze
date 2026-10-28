package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// formatComments parses input with schema, prints it with Serialize, and
// checks that a second parse and print writes the same bytes, so each golden
// below is also proven stable.
func formatComments(t *testing.T, schema *Schema, input string) string {
	t.Helper()
	tree, err := NewParser(schema).Parse(input)
	require.NoError(t, err)
	output := Serialize(tree, schema)

	again, err := NewParser(schema).Parse(output)
	require.NoError(t, err, "the printed text does not parse:\n%s", output)
	require.Equal(t, output, Serialize(again, schema), "a second format changed the text")
	require.Equal(t, tree.ToMap(), again.ToMap(), "the printed text changed a value")
	return output
}

// TestSerializeComments checks where Serialize prints each comment after the
// canonical order moved the statements.
//
// VALIDATES: AC-1 (an own-line comment prints directly above its leaf,
// container, list entry and `inactive:` entry, at the statement's indent),
// AC-2 (a same-line comment prints after its statement, its `{` and its `}`),
// AC-3 (closing comments print inside their block before `}`, and at the end
// of the file), AC-4 (a head group prints first in its block, then one blank
// line), AC-7 (a leaf set twice prints the comments of both occurrences).
// PREVENTS: a comment printed beside a statement it does not describe, or lost.
// Method: hand-written goldens. router-id moves from last to first and
// description above peer, so a comment that kept its line number would land
// beside the wrong statement.
func TestSerializeComments(t *testing.T) {
	input := `# header one
# header two

# lead local-as
local-as 65000 # trail local-as
# lead bgp
bgp { # open bgp
	# head bgp

	# lead peer
	peer p1 {
		peer-as 65001 # trail peer-as
		# closing peer
	} # trail peer
	# lead inactive peer
	inactive: peer p2 {
		peer-as 65002
	}
	# lead description
	description x
	# closing bgp
} # trail bgp
# lead router-id
router-id 192.0.2.1 # first router-id
router-id 192.0.2.2 # second router-id
# end of file
`
	want := `# header one
# header two

# lead router-id
# first router-id
router-id 192.0.2.2 # second router-id
# lead local-as
local-as 65000 # trail local-as
# lead bgp
bgp { # open bgp
	# head bgp

	# lead description
	description x
	# lead peer
	peer p1 {
		peer-as 65001 # trail peer-as
		# closing peer
	} # trail peer
	# lead inactive peer
	inactive: peer p2 {
		peer-as 65002
	}
	# closing bgp
} # trail bgp
# end of file
`
	require.Equal(t, want, formatComments(t, commentSchema(), input))
}

// TestSerializeKeepsStampFirst checks that the schema stamp stays the first
// line when the statement written after it moves.
//
// VALIDATES: AC-5: ScanStampRelease reads the same release from the output.
// PREVENTS: fmt turning a stamped file into an unstamped one.
func TestSerializeKeepsStampFirst(t *testing.T) {
	input := `# ze-schema: 26.10.07
bgp {
	description x
}
router-id 192.0.2.1
`
	want := `# ze-schema: 26.10.07

router-id 192.0.2.1
bgp description x
`
	output := formatComments(t, commentSchema(), input)
	require.Equal(t, want, output)
	require.Equal(t, "26.10.07", ScanStampRelease([]byte(output)))
}

// TestSerializeStampedReplacesStamp checks that a writer stamping the
// hierarchical form names one release, its own.
//
// VALIDATES: AC-5 with a writer: the parsed stamp is replaced, a head comment
// after it and the blank line detaching it are kept.
// PREVENTS: the editor commit stacking one more `# ze-schema:` line on every
// write, now that Serialize prints the stamp the parser kept as head.
func TestSerializeStampedReplacesStamp(t *testing.T) {
	stamp := FormatSchemaStamp()
	for _, tc := range []struct{ name, input, want string }{
		{"stamp only", "# ze-schema: 26.10.07\nrouter-id 192.0.2.1\n", stamp + "router-id 192.0.2.1\n"},
		{"stamp and head", "# ze-schema: 26.10.07\n# site A\n\nrouter-id 192.0.2.1\n", stamp + "# site A\n\nrouter-id 192.0.2.1\n"},
		{"no stamp", "router-id 192.0.2.1\n", stamp + "router-id 192.0.2.1\n"},
	} {
		tree, err := NewParser(commentSchema()).Parse(tc.input)
		require.NoError(t, err, tc.name)
		once := SerializeStamped(tree, commentSchema())
		require.Equal(t, tc.want, once, tc.name)
		again, err := NewParser(commentSchema()).Parse(once)
		require.NoError(t, err, tc.name)
		require.Equal(t, once, SerializeStamped(again, commentSchema()), "%s: second write", tc.name)
		require.Equal(t, ScanStampRelease([]byte(tc.input)), ScanStampRelease([]byte(Serialize(tree, commentSchema()))),
			"%s: the tree itself keeps its stamp", tc.name)
	}
}

// TestSerializeBracketComment checks the comment written after `[`.
//
// VALIDATES: AC-6: printed directly above the leaf, members and order kept.
// PREVENTS: the comment dropped because a bracket list prints on one line.
func TestSerializeBracketComment(t *testing.T) {
	input := "filters [ # why these filters\n zz-second aa-first ]\n"
	want := "# why these filters\nfilters [ zz-second aa-first ]\n"
	require.Equal(t, want, formatComments(t, commentSchema(), input))
}

// inlineCommentSchema holds a container the printer writes inline when it
// carries one leaf, and a ze:flatten container of list entries.
func inlineCommentSchema() *Schema {
	schema := NewSchema()
	schema.Define("local", Container(Field("ip", Leaf(TypeIPv4))))
	attach := Container(Field("process", List(TypeString, Field("run", Leaf(TypeString)))))
	attach.Flatten = true
	schema.Define("attach", attach)
	return schema
}

// TestSerializeCommentBlocksInline checks that a container whose inside carries
// a comment prints as a block, inline and ze:flatten alike, and that the
// container's own comments keep the one-line spelling.
//
// VALIDATES: AC-8.
// PREVENTS: a comment inside a container lost, or moved onto another
// statement, by the one-line spelling, which has no line for it.
// Method: each case shows the spelling without the comment first, so the
// block is shown to come from the comment alone.
func TestSerializeCommentBlocksInline(t *testing.T) {
	schema := inlineCommentSchema()
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "inline without comment",
			input: "local {\n\tip 192.0.2.1\n}\n",
			want:  "local ip 192.0.2.1\n",
		},
		{
			name:  "inline with a comment on its leaf",
			input: "local {\n\tip 192.0.2.1 # the router\n}\n",
			want:  "local {\n\tip 192.0.2.1 # the router\n}\n",
		},
		{
			name:  "inline with a comment above its leaf",
			input: "local {\n\t# the router\n\tip 192.0.2.1\n}\n",
			want:  "local {\n\t# the router\n\tip 192.0.2.1\n}\n",
		},
		{
			name:  "inline with comments of its own",
			input: "# about local\nlocal { ip 192.0.2.1 } # after local\n",
			want:  "# about local\nlocal ip 192.0.2.1 # after local\n",
		},
		{
			name:  "flatten without comment",
			input: "attach {\n\tprocess lg {\n\t\trun x\n\t}\n}\n",
			want:  "attach process lg {\n\trun x\n}\n",
		},
		{
			name:  "flatten with a comment above an entry",
			input: "attach {\n\t# the looking glass\n\tprocess lg {\n\t\trun x\n\t}\n}\n",
			want:  "attach {\n\t# the looking glass\n\tprocess lg {\n\t\trun x\n\t}\n}\n",
		},
		{
			name:  "flatten with a closing comment",
			input: "attach {\n\tprocess lg {\n\t\trun x\n\t}\n\t# more to come\n}\n",
			want:  "attach {\n\tprocess lg {\n\t\trun x\n\t}\n\t# more to come\n}\n",
		},
		{
			name:  "flatten with a comment above the container",
			input: "# processes\nattach {\n\tprocess lg {\n\t\trun x\n\t}\n}\n",
			want:  "# processes\nattach process lg {\n\trun x\n}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, formatComments(t, schema, tc.input))
		})
	}
}

// TestSerializeHeadWithoutStatements checks a block whose head group has no
// statement after it.
//
// VALIDATES: the blank line after a head is written only before a statement,
// so a second format keeps every byte.
// PREVENTS: a blank line that the next parse reads as nothing, which makes
// `ze config fmt --check` fail on fmt's own output.
func TestSerializeHeadWithoutStatements(t *testing.T) {
	schema := commentSchema()
	tree := NewTree()
	tree.addBlockHead([]string{" head"})
	tree.addBlockClosing([]string{" closing"})
	output := Serialize(tree, schema)
	require.Equal(t, "# head\n# closing\n", output)
	again, err := NewParser(schema).Parse(output)
	require.NoError(t, err)
	require.Equal(t, output, Serialize(again, schema))
}
