package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPruneInactiveLeaf verifies that a leaf marked via SetLeafInactive
// is removed from tree.values after PruneInactive runs.
//
// VALIDATES: AC-5 -- components see the deactivated leaf as absent.
//
// PREVENTS: PruneInactive ignoring leaf-level deactivation, leaving
// deactivated leaves visible to consumers.
func TestPruneInactiveLeaf(t *testing.T) {
	schema := testSchema()
	tree := NewTree()
	tree.Set("router-id", "1.2.3.4")
	tree.Set("local-as", "65000")
	tree.SetLeafInactive("router-id", true)

	PruneInactive(tree, schema)

	if _, ok := tree.Get("router-id"); ok {
		t.Fatalf("inactive leaf must be pruned from values")
	}
	v, ok := tree.Get("local-as")
	require.True(t, ok, "active leaf must remain")
	assert.Equal(t, "65000", v)

	// After pruning, the inactive marker is implementation-detail; the
	// observable invariant is that tree.Get returns absent. We assert
	// the marker is also cleared so a subsequent PruneInactive on a
	// re-parsed tree starts from a clean slate.
	assert.False(t, tree.IsLeafInactive("router-id"),
		"prune should also clear the inactive marker for the removed leaf")
}

// TestPruneInactiveLeafInListEntry verifies leaf pruning recurses into
// list entries.
//
// VALIDATES: AC-5 inside a list-entry tree (parser stores the marker
// on the entry tree, not on the parent).
func TestPruneInactiveLeafInListEntry(t *testing.T) {
	schema := testSchema()
	input := `neighbor 192.0.2.1 {
		inactive: description "ignored";
		peer-as 65001;
	}`
	tree, err := NewParser(schema).Parse(input)
	require.NoError(t, err)

	entry := tree.GetList("neighbor")["192.0.2.1"]
	require.NotNil(t, entry)
	require.True(t, entry.IsLeafInactive("description"))

	PruneInactive(tree, schema)

	entry = tree.GetList("neighbor")["192.0.2.1"]
	require.NotNil(t, entry, "list entry itself must survive (only the leaf is inactive)")
	if _, ok := entry.Get("description"); ok {
		t.Fatalf("inactive leaf inside list entry must be pruned")
	}
	v, ok := entry.Get("peer-as")
	require.True(t, ok)
	assert.Equal(t, "65001", v)
}

// TestPruneInactiveLeafInsideInactiveContainer ensures the recursive
// container-level prune still wins over leaf-level prune: an inactive
// container is removed wholesale, regardless of its children's
// individual inactive markers. This guards against double-handling that
// could leave half-pruned state behind.
func TestPruneInactiveLeafInsideInactiveContainer(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	input := `bgp {
		session {
			asn {
				local 65000
			}
		}
		router-id 1.2.3.4
		peer host-with-inactive-leaf {
			connection {
				remote {
					ip 10.0.0.2
				}
			}
			session {
				asn {
					remote 65002
				}
			}
			inactive enable
		}
	}`
	tree, err := NewParser(schema).Parse(input)
	require.NoError(t, err)

	PruneInactive(tree, schema)

	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	peers := bgp.GetList("peer")
	assert.Empty(t, peers, "inactive peer entry must be removed wholesale")
}
