// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-9 -- the `tag` leaf under a
// redistribute import entry becomes the rule's tag filter, an absent leaf leaves the
// rule unfiltered, and a value the uint32 range does not hold is refused by name.
// PREVENTS: a configured tag filter silently importing every route, and `tag 0` being
// read as "no filter".
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractRedistributeRulesTagFilter(t *testing.T) {
	registerRedistSources(t)

	tree := NewTree()
	redist := NewTree()
	tree.SetContainer("redistribute", redist)
	dest := NewTree()
	tagged := NewTree()
	tagged.Set("tag", "42")
	dest.AddListEntry("import", "ebgp", tagged)
	untagged := NewTree()
	dest.AddListEntry("import", "ibgp", untagged)
	zero := NewTree()
	zero.Set("tag", "0")
	dest.AddListEntry("import", "ospf", zero)
	redist.AddListEntry("destination", "bgp", dest)

	rules, err := ExtractRedistributeRules(tree)
	require.NoError(t, err)
	require.Len(t, rules, 3)

	assert.True(t, rules[0].MatchTag, "a named tag filters")
	assert.Equal(t, uint32(42), rules[0].Tag)

	assert.False(t, rules[1].MatchTag, "an absent tag leaf leaves the rule unfiltered")
	assert.Equal(t, uint32(0), rules[1].Tag)

	assert.True(t, rules[2].MatchTag, "tag 0 selects the untagged routes")
	assert.Equal(t, uint32(0), rules[2].Tag)
}

func TestExtractRedistributeRulesTagOutOfRange(t *testing.T) {
	registerRedistSources(t)

	tree := NewTree()
	redist := NewTree()
	tree.SetContainer("redistribute", redist)
	dest := NewTree()
	entry := NewTree()
	entry.Set("tag", "4294967296")
	dest.AddListEntry("import", "ebgp", entry)
	redist.AddListEntry("destination", "bgp", dest)

	_, err := ExtractRedistributeRules(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "4294967296")
	assert.Contains(t, err.Error(), "ebgp")
}
