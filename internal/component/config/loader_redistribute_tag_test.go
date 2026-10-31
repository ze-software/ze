// VALIDATES: spec-route-policy-0-umbrella AC-14 -- an import entry's `final` becomes the
// rule's final decision, an entry with neither filters nor final moves every route, and
// a final the enumeration does not hold is refused by name. (The `tag` leaf this file
// tested is gone: a tag is now a match filter's condition, `match T { tag [ 42 ]; }`.)
// PREVENTS: a configured chain silently importing every route, and an invalid final
// read as no chain.
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractRedistributeRulesFinal(t *testing.T) {
	registerRedistSources(t)

	tree := NewTree()
	redist := NewTree()
	tree.SetContainer("redistribute", redist)
	dest := NewTree()
	accepting := NewTree()
	accepting.SetSlice("filters", []string{"T42"})
	accepting.Set("final", "accept")
	dest.AddListEntry("import", "ebgp", accepting)
	unfiltered := NewTree()
	dest.AddListEntry("import", "ibgp", unfiltered)
	rejecting := NewTree()
	rejecting.Set("final", "reject")
	dest.AddListEntry("import", "ospf", rejecting)
	redist.AddListEntry("destination", "bgp", dest)

	rules, err := ExtractRedistributeRules(tree)
	require.NoError(t, err)
	require.Len(t, rules, 3)

	assert.Equal(t, "accept", rules[0].Final, "a named final is the rule's decision")
	assert.Equal(t, []string{"T42"}, rules[0].Filters)

	assert.Empty(t, rules[1].Final, "an entry with no chain leaves the rule unfiltered")

	assert.Equal(t, "reject", rules[2].Final, "final with no filters is a chain that decides alone")
	assert.Empty(t, rules[2].Filters)
}

func TestExtractRedistributeRulesInvalidFinal(t *testing.T) {
	registerRedistSources(t)

	tree := NewTree()
	redist := NewTree()
	tree.SetContainer("redistribute", redist)
	dest := NewTree()
	entry := NewTree()
	entry.Set("final", "maybe")
	dest.AddListEntry("import", "ebgp", entry)
	redist.AddListEntry("destination", "bgp", dest)

	_, err := ExtractRedistributeRules(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maybe")
	assert.Contains(t, err.Error(), "ebgp")
}
