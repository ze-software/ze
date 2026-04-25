package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseInactiveLeafTopLevel verifies that "inactive: <leaf> <value>;"
// at the root sets the leaf inactive while preserving the value.
//
// VALIDATES: AC-1 / AC-9 -- top-level leaf deactivation parses cleanly
// and round-trips through Tree.IsLeafInactive.
//
// PREVENTS: parseRoot rejecting `inactive:` as an unknown top-level
// keyword, which would block deactivating top-level leaves.
func TestParseInactiveLeafTopLevel(t *testing.T) {
	input := `inactive: router-id 1.2.3.4`

	p := NewParser(testSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	require.Empty(t, p.Warnings(), "leaf deactivation must not warn")

	assert.True(t, tree.IsLeafInactive("router-id"), "leaf must be marked inactive")
	v, ok := tree.Get("router-id")
	assert.True(t, ok, "value must still be present in tree.values")
	assert.Equal(t, "1.2.3.4", v, "value must be preserved verbatim, not encoded")
}

// TestParseInactiveLeafInListEntry verifies that "inactive: <leaf> <value>;"
// inside a list entry block sets the entry's leaf inactive.
//
// VALIDATES: AC-1 / AC-11 -- leaf deactivation inside `peer { ... }` /
// `neighbor { ... }` works through parser_list.parseListFieldBlock.
//
// PREVENTS: the prior warn-and-ignore branch at parser_list.go:123 from
// silently discarding the inactive flag on a leaf inside a list entry.
func TestParseInactiveLeafInListEntry(t *testing.T) {
	input := `neighbor 192.0.2.1 {
		inactive: description "test peer";
		peer-as 65001;
	}`

	p := NewParser(testSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	require.Empty(t, p.Warnings(), "leaf deactivation in list entry must not warn")

	entries := tree.GetList("neighbor")
	require.Len(t, entries, 1)
	entry := entries["192.0.2.1"]
	require.NotNil(t, entry)

	assert.True(t, entry.IsLeafInactive("description"), "description must be inactive in entry")
	v, ok := entry.Get("description")
	assert.True(t, ok)
	assert.Equal(t, "test peer", v, "value preserved verbatim")

	// Sibling leaves untouched.
	assert.False(t, entry.IsLeafInactive("peer-as"), "peer-as must remain active")
}

// TestParseInactiveLeafInContainer verifies that "inactive: <leaf> <value>;"
// inside a container block sets the leaf inactive on the container's tree.
//
// VALIDATES: AC-1 -- leaf deactivation in nested containers via
// parser.parseContainer.
//
// PREVENTS: regression in the warn-and-ignore branch at parser.go:274.
func TestParseInactiveLeafInContainer(t *testing.T) {
	// neighbor 192.0.2.1 { family { ipv4 { inactive: unicast true; } } }
	input := `neighbor 192.0.2.1 {
		family {
			ipv4 {
				inactive: unicast true;
			}
		}
	}`

	p := NewParser(testSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	require.Empty(t, p.Warnings(), "nested leaf deactivation must not warn")

	entry := tree.GetList("neighbor")["192.0.2.1"]
	require.NotNil(t, entry)
	family := entry.GetContainer("family")
	require.NotNil(t, family)
	ipv4 := family.GetContainer("ipv4")
	require.NotNil(t, ipv4)

	assert.True(t, ipv4.IsLeafInactive("unicast"), "unicast leaf inside ipv4 container must be inactive")
	v, ok := ipv4.Get("unicast")
	assert.True(t, ok)
	assert.Equal(t, "true", v, "bool value normalized and preserved")
}
