package config

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"

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

// TestExtractRedistributeRules_Basic verifies parsing of import rules with and without family filters.
//
// VALIDATES: Import rules are extracted from config tree with correct source and families.
// PREVENTS: Wrong source names, missing families, or incorrect ordering.
func TestExtractRedistributeRules_Basic(t *testing.T) {
	registerRedistSources(t)

	tree := NewTree()
	redist := NewTree()
	tree.SetContainer("redistribute", redist)

	// import ebgp { family [ ipv4/unicast ipv4/vpn ]; }
	ebgpEntry := NewTree()
	ebgpEntry.SetSlice("family", []string{"ipv4/unicast", "ipv4/vpn"})
	redist.AddListEntry("import", "ebgp", ebgpEntry)

	// import ibgp;
	ibgpEntry := NewTree()
	redist.AddListEntry("import", "ibgp", ibgpEntry)

	rules, err := ExtractRedistributeRules(tree)
	require.NoError(t, err)
	require.Len(t, rules, 2)

	assert.Equal(t, "ebgp", rules[0].Source)
	assert.Equal(t, []string{"ipv4/unicast", "ipv4/vpn"}, rules[0].Families)

	assert.Equal(t, "ibgp", rules[1].Source)
	assert.Empty(t, rules[1].Families)
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

// TestExtractRedistributeRules_EmptyRedistribute verifies nil return when container has no imports.
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
	redist.AddListEntry("import", "rip", NewTree())

	_, err := ExtractRedistributeRules(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown source")
	assert.Contains(t, err.Error(), "rip")
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

	redist.AddListEntry("import", "connected", NewTree())
	redist.AddListEntry("import", "ospf", NewTree())
	redist.AddListEntry("import", "ebgp", NewTree())

	rules, err := ExtractRedistributeRules(tree)
	require.NoError(t, err)
	require.Len(t, rules, 3)

	assert.Equal(t, "connected", rules[0].Source)
	assert.Equal(t, "ospf", rules[1].Source)
	assert.Equal(t, "ebgp", rules[2].Source)
}
