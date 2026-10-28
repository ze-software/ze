package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTreeCloneKeepsComments checks that the tree operations that move or copy
// a statement carry its comments.
//
// VALIDATES: Clone copies every comment and shares no slice with the source;
// MergeContainer concatenates both sides in order; RenameListEntry keeps the
// entry's comments under its new key; a merge that replaces a list entry keeps
// the replaced entry's own comments ahead of the new ones.
// PREVENTS: `ze config migrate` and the editor losing comments through a copy.
func TestTreeCloneKeepsComments(t *testing.T) {
	source := parseComments(t, `# head

# lead bgp
bgp { # open
	# lead peer
	peer p1 {
		peer-as 65001 # trail peer-as
	}
} # trail bgp
# lead filters
filters [ a b ]
# lead member
inactive: filters a
# eof
`)

	clone := source.Clone()
	require.Equal(t, source.commentsOfBlock(), clone.commentsOfBlock())
	require.Equal(t, source.commentsOfLeaf("filters"), clone.commentsOfLeaf("filters"))
	require.Equal(t, statementComments{Leading: []string{" lead member"}}, clone.commentsOfMember("filters", "a"))
	cloneBGP := clone.GetContainer("bgp")
	require.Equal(t, source.GetContainer("bgp").commentsOfBlock(), cloneBGP.commentsOfBlock())
	require.Equal(t, statementComments{Trailing: []string{" trail peer-as"}},
		cloneBGP.GetList("peer")["p1"].commentsOfLeaf("peer-as"))

	cloneBGP.commentSlotOfBlock().addLeading([]string{" only in the clone"})
	require.Equal(t, []string{" lead bgp"}, source.GetContainer("bgp").commentsOfBlock().Statement.Leading,
		"the clone shares a comment slice with its source")

	t.Run("MergeContainer", func(t *testing.T) {
		second := parseComments(t, "# second bgp\nbgp {\n\tdescription x # trail description\n} # second trail\n")
		merged := source.Clone()
		require.NoError(t, merged.MergeContainer("bgp", second.GetContainer("bgp"), nil))
		got := merged.GetContainer("bgp").commentsOfBlock()
		require.Equal(t, []string{" lead bgp", " second bgp"}, got.Statement.Leading)
		require.Equal(t, []string{" trail bgp", " second trail"}, got.Statement.Trailing)
		require.Equal(t, statementComments{Trailing: []string{" trail description"}},
			merged.GetContainer("bgp").commentsOfLeaf("description"))
	})

	t.Run("RenameListEntry", func(t *testing.T) {
		renamed := source.Clone()
		bgp := renamed.GetContainer("bgp")
		require.NoError(t, bgp.RenameListEntry("peer", "p1", "p9"))
		require.Equal(t, []string{" lead peer"}, bgp.GetList("peer")["p9"].commentsOfBlock().Statement.Leading)
	})

	t.Run("merge replacing a list entry", func(t *testing.T) {
		replacing := parseComments(t, "bgp {\n\t# lead new p1\n\tpeer p1 {\n\t\tpeer-as 65009\n\t}\n}\n")
		merged := source.Clone()
		require.NoError(t, merged.MergeContainer("bgp", replacing.GetContainer("bgp"), nil))
		require.Equal(t, []string{" lead peer", " lead new p1"},
			merged.GetContainer("bgp").GetList("peer")["p1"].commentsOfBlock().Statement.Leading)
	})
}

// TestTreeLeafRemovalDropsComments checks R-2: a removed leaf takes its
// comments with it, so a later leaf of the same name starts with none.
//
// VALIDATES: removeValue (prune, the change-file paths), pruneInactiveLeaves
// and RemoveMultiValueMember drop the comments of what they remove.
// PREVENTS: a comment describing a deleted leaf reattaching to the next leaf of
// that name.
func TestTreeLeafRemovalDropsComments(t *testing.T) {
	tree := parseComments(t, `# about local-as
local-as 65000
# about filters
filters [ a b ]
# about member b
inactive: filters b
# about router-id
inactive: router-id 192.0.2.1
`)

	tree.removeValue("local-as")
	tree.Set("local-as", "65001")
	require.Equal(t, statementComments{}, tree.commentsOfLeaf("local-as"))

	require.True(t, tree.RemoveMultiValueMember("filters", "b"))
	require.Equal(t, statementComments{}, tree.commentsOfMember("filters", "b"))
	require.Equal(t, statementComments{Leading: []string{" about filters"}}, tree.commentsOfLeaf("filters"),
		"removing one member keeps the leaf's own comments")
	require.True(t, tree.RemoveMultiValueMember("filters", "a"))
	require.Equal(t, statementComments{}, tree.commentsOfLeaf("filters"), "the last member took the leaf")

	tree.pruneInactiveLeaves()
	require.Equal(t, statementComments{}, tree.commentsOfLeaf("router-id"))
}
