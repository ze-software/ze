package config

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config/redistribute"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerRedistSources registers test sources for redistribution tests.
func registerRedistSources(t *testing.T) {
	t.Helper()
	require.NoError(t, redistribute.RegisterSource(redistribute.RouteSource{Name: "ebgp", Protocol: "bgp"}))
	require.NoError(t, redistribute.RegisterSource(redistribute.RouteSource{Name: "ibgp", Protocol: "bgp"}))
	require.NoError(t, redistribute.RegisterSource(redistribute.RouteSource{Name: "ospf", Protocol: "ospf"}))
	require.NoError(t, redistribute.RegisterSource(redistribute.RouteSource{Name: "connected", Protocol: "connected"}))
}

// TestExtractRedistributeRules_Basic verifies parsing of import rules with and without a chain.
//
// VALIDATES: Import rules are extracted from config tree with correct source, filters and final.
// PREVENTS: Wrong source names, missing or reordered filters, or a lost final.
func TestExtractRedistributeRules_Basic(t *testing.T) {
	registerRedistSources(t)

	tree := NewTree()
	redist := NewTree()
	tree.SetContainer("redistribute", redist)
	dest := NewTree()
	ebgpEntry := NewTree()
	ebgpEntry.SetSlice("filters", []string{"V4", "VPN"})
	ebgpEntry.Set("final", "reject")
	dest.AddListEntry("import", "ebgp", ebgpEntry)
	ibgpEntry := NewTree()
	dest.AddListEntry("import", "ibgp", ibgpEntry)
	redist.AddListEntry("destination", "bgp", dest)

	rules, err := ExtractRedistributeRules(tree)
	require.NoError(t, err)
	require.Len(t, rules, 2)

	assert.Equal(t, "ebgp", rules[0].Source)
	assert.Equal(t, []string{"V4", "VPN"}, rules[0].Filters)
	assert.Equal(t, "reject", rules[0].Final)
	assert.Nil(t, rules[0].Chain, "the loader names the chain; the orchestrator compiles it")

	assert.Equal(t, "ibgp", rules[1].Source)
	assert.Empty(t, rules[1].Filters)
	assert.Empty(t, rules[1].Final, "an import with no chain moves every route")
}

// TestExtractRedistributeRules_NoRedistribute verifies nil return when container is absent.
//
// VALIDATES: Missing redistribute container returns nil, no error.
// PREVENTS: Panic on nil container or spurious error.
func TestExtractRedistributeRules_NoRedistribute(t *testing.T) {
	tree := NewTree()
	rules, err := ExtractRedistributeRules(tree)
	require.NoError(t, err)
	assert.Nil(t, rules)
}

// TestExtractRedistributeRules_EmptyRedistribute verifies nil return when container has no destinations.
//
// VALIDATES: Empty redistribute container returns nil, no error.
// PREVENTS: Empty slice vs nil confusion.
func TestExtractRedistributeRules_EmptyRedistribute(t *testing.T) {
	tree := NewTree()
	tree.SetContainer("redistribute", NewTree())
	rules, err := ExtractRedistributeRules(tree)
	require.NoError(t, err)
	assert.Nil(t, rules)
}

// TestExtractRedistributeRules_UnknownSource verifies error on unregistered source.
//
// VALIDATES: Unknown source name produces an error.
// PREVENTS: Silently accepting a typo in config.
func TestExtractRedistributeRules_UnknownSource(t *testing.T) {
	registerRedistSources(t)

	tree := NewTree()
	redist := NewTree()
	tree.SetContainer("redistribute", redist)
	dest := NewTree()
	dest.AddListEntry("import", "rip", NewTree())
	redist.AddListEntry("destination", "bgp", dest)

	_, err := ExtractRedistributeRules(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown source")
	assert.Contains(t, err.Error(), "rip")
}

// TestExtractRedistributeRules_FiltersWithoutFinal verifies an import naming
// filters and no final is refused.
//
// VALIDATES: a chain with no final decision is refused by name, naming the source
// and the fix (exact-or-reject).
// PREVENTS: a chain silently given a default decision. A family the operator names
// is now a match filter's condition, refused by the policy library when unknown.
func TestExtractRedistributeRules_FiltersWithoutFinal(t *testing.T) {
	registerRedistSources(t)

	tree := NewTree()
	redist := NewTree()
	tree.SetContainer("redistribute", redist)
	dest := NewTree()
	entry := NewTree()
	entry.SetSlice("filters", []string{"V4"})
	dest.AddListEntry("import", "ebgp", entry)
	redist.AddListEntry("destination", "bgp", dest)

	_, err := ExtractRedistributeRules(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no final")
	assert.Contains(t, err.Error(), "final accept")
	assert.Contains(t, err.Error(), "ebgp")
}

// TestExtractRedistributeRules_PreservesOrder verifies import rules maintain config order.
//
// VALIDATES: Rules are returned in the order they appear in config.
// PREVENTS: Map iteration randomizing rule order.
func TestExtractRedistributeRules_PreservesOrder(t *testing.T) {
	registerRedistSources(t)

	tree := NewTree()
	redist := NewTree()
	tree.SetContainer("redistribute", redist)
	dest := NewTree()
	dest.AddListEntry("import", "connected", NewTree())
	dest.AddListEntry("import", "ospf", NewTree())
	dest.AddListEntry("import", "ebgp", NewTree())
	redist.AddListEntry("destination", "bgp", dest)

	rules, err := ExtractRedistributeRules(tree)
	require.NoError(t, err)
	require.Len(t, rules, 3)

	assert.Equal(t, "connected", rules[0].Source)
	assert.Equal(t, "ospf", rules[1].Source)
	assert.Equal(t, "ebgp", rules[2].Source)
}
