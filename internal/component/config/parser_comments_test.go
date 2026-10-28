package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// commentSchema covers each statement kind a comment can belong to: a leaf, a
// bracket leaf-list, a container and a list entry.
func commentSchema() *Schema {
	schema := NewSchema()
	schema.Define("router-id", Leaf(TypeIPv4))
	schema.Define("local-as", Leaf(TypeUint32))
	schema.Define("filters", bracketLeafList(TypeString))
	schema.Define("bgp", Container(
		Field("description", Leaf(TypeString)),
		Field("peer", List(TypeString,
			Field("description", Leaf(TypeString)),
			Field("peer-as", Leaf(TypeUint32)),
		)),
	))
	return schema
}

// parseComments parses input with commentSchema and checks that its comments
// change no value: ToMap equals that of the same text with every comment cut.
func parseComments(t *testing.T, input string) *Tree {
	t.Helper()
	tree, err := NewParser(commentSchema()).Parse(input)
	require.NoError(t, err)

	var plain strings.Builder
	for line := range strings.Lines(input) {
		before, _, cut := strings.Cut(line, "#")
		plain.WriteString(before)
		if cut {
			plain.WriteString("\n")
		}
	}
	bare, err := NewParser(commentSchema()).Parse(plain.String())
	require.NoError(t, err)
	require.Equal(t, bare.ToMap(), tree.ToMap(), "a comment changed a value")
	return tree
}

// TestParserAttachesComments checks each attachment rule on the Tree.
//
// VALIDATES: AC-1 (own-line comment leads the next statement: leaf, container,
// list entry, inactive: entry), AC-2 (same-line comment trails the statement,
// the opening `{` or the closing `}`), AC-3 (comments after a block's last
// statement close that block; after the file's last statement they close the
// root), AC-4 (a group detached from the first statement by a blank line is the
// block's head), AC-5 (a first-line schema stamp is the root head even with no
// blank line after it).
// PREVENTS: a comment attached to the wrong statement, or to none.
func TestParserAttachesComments(t *testing.T) {
	tree := parseComments(t, `# ze-schema: 26.10.07
# header

# lead router-id
router-id 192.0.2.1 # trail router-id
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
# lead local-as
inactive: local-as 65000
# end of file
`)

	require.Equal(t, blockComments{
		Head:    []string{" ze-schema: 26.10.07", " header"},
		Closing: []string{" end of file"},
	}, tree.commentsOfBlock())
	require.Equal(t, statementComments{
		Leading:  []string{" lead router-id"},
		Trailing: []string{" trail router-id"},
	}, tree.commentsOfLeaf("router-id"))
	require.Equal(t, statementComments{Leading: []string{" lead local-as"}}, tree.commentsOfLeaf("local-as"))

	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	require.Equal(t, blockComments{
		Statement: statementComments{Trailing: []string{" trail bgp"}},
		Opening:   []string{" open bgp"},
		Head:      []string{" head bgp"},
		Closing:   []string{" closing bgp"},
	}, bgp.commentsOfBlock())
	require.Equal(t, statementComments{Leading: []string{" lead description"}}, bgp.commentsOfLeaf("description"))

	p1 := bgp.GetList("peer")["p1"]
	require.NotNil(t, p1)
	require.Equal(t, blockComments{
		Statement: statementComments{Leading: []string{" lead peer"}, Trailing: []string{" trail peer"}},
		Closing:   []string{" closing peer"},
	}, p1.commentsOfBlock())
	require.Equal(t, statementComments{Trailing: []string{" trail peer-as"}}, p1.commentsOfLeaf("peer-as"))

	p2 := bgp.GetList("peer")["p2"]
	require.NotNil(t, p2)
	require.True(t, p2.IsInactive())
	require.Equal(t, blockComments{
		Statement: statementComments{Leading: []string{" lead inactive peer"}},
	}, p2.commentsOfBlock())

	t.Run("stamp is head without a blank line", func(t *testing.T) {
		tree := parseComments(t, "# ze-schema: 26.10.07\n# lead\nrouter-id 192.0.2.1\n")
		require.Equal(t, blockComments{Head: []string{" ze-schema: 26.10.07"}}, tree.commentsOfBlock())
		require.Equal(t, statementComments{Leading: []string{" lead"}}, tree.commentsOfLeaf("router-id"))
	})
	t.Run("no blank line, no head", func(t *testing.T) {
		tree := parseComments(t, "# about router-id\nrouter-id 192.0.2.1\n")
		require.Equal(t, blockComments{}, tree.commentsOfBlock())
		require.Equal(t, statementComments{Leading: []string{" about router-id"}}, tree.commentsOfLeaf("router-id"))
	})
	t.Run("comment-only block closes it", func(t *testing.T) {
		tree := parseComments(t, "bgp {\n\t# nothing yet\n}\n")
		require.Equal(t, blockComments{Closing: []string{" nothing yet"}}, tree.GetContainer("bgp").commentsOfBlock())
	})
}

// TestParserBracketComment checks a comment written inside a bracket
// leaf-list, the one in-list position the grammar accepts (A-3), and the
// comment of an `inactive:` member line.
//
// VALIDATES: AC-6: a comment after `[` leads the leaf statement and the
// members keep their order; a comment above `inactive: <leaf-list> <member>`
// belongs to that member line.
// PREVENTS: a comment inside brackets dropped, or moved to another statement.
func TestParserBracketComment(t *testing.T) {
	tree := parseComments(t, `# about filters
filters [ # members
zz-second aa-first ]
# about the member line
inactive: filters zz-second
`)
	require.Equal(t, []MemberState{
		{Value: "zz-second", Inactive: true},
		{Value: "aa-first"},
	}, tree.GetMultiValuesState("filters"))
	require.Equal(t, statementComments{Leading: []string{" about filters", " members"}}, tree.commentsOfLeaf("filters"))
	require.Equal(t, statementComments{Leading: []string{" about the member line"}}, tree.commentsOfMember("filters", "zz-second"))
}

// TestParserMergedStatementComments checks statements the parser merges.
//
// VALIDATES: AC-7: two blocks of one container, and a leaf set twice, keep
// every comment, in source order, on the merged statement.
// PREVENTS: the second block's comments replacing the first's.
func TestParserMergedStatementComments(t *testing.T) {
	tree := parseComments(t, `# first bgp
bgp {
	# first description
	description a
} # trail first
# second bgp
bgp { # open second
	# second description
	description b
} # trail second
# first local-as
local-as 1
# second local-as
local-as 2 # trail local-as
`)
	require.Equal(t, statementComments{
		Leading:  []string{" first local-as", " second local-as"},
		Trailing: []string{" trail local-as"},
	}, tree.commentsOfLeaf("local-as"))

	bgp := tree.GetContainer("bgp")
	require.Equal(t, blockComments{
		Statement: statementComments{
			Leading:  []string{" first bgp", " second bgp"},
			Trailing: []string{" trail first", " trail second"},
		},
		Opening: []string{" open second"},
	}, bgp.commentsOfBlock())
	require.Equal(t, statementComments{
		Leading: []string{" first description", " second description"},
	}, bgp.commentsOfLeaf("description"))
}
