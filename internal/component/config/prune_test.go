package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Blank imports trigger init() registration of YANG modules.
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/softver/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/schema"
)

// TestPruneInactiveContainer verifies that a container with inactive=true
// is removed from the tree after pruning.
//
// VALIDATES: AC-1 -- inactive peer not created at runtime.
// PREVENTS: Inactive containers surviving into the resolution pipeline.
func TestPruneInactiveContainer(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	input := `bgp {
    local {
        as 65000
    }
    router-id 1.2.3.4
    peer active-peer {
        remote {
            ip 10.0.0.1
            as 65001
        }
    }
    peer inactive-peer {
        inactive enable
        remote {
            ip 10.0.0.2
            as 65002
        }
    }
}`
	parser := NewParser(schema)
	tree, err := parser.Parse(input)
	require.NoError(t, err)

	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)

	// Before pruning: both peers exist.
	peers := bgp.GetList("peer")
	require.Len(t, peers, 2)

	PruneInactive(tree, schema)

	// After pruning: only active peer remains.
	peers = bgp.GetList("peer")
	require.Len(t, peers, 1, "inactive peer should be removed")
	assert.NotNil(t, peers["active-peer"], "active peer should survive")
	assert.Nil(t, peers["inactive-peer"], "inactive peer should be removed")
}

// TestPruneInactivePreservesActive verifies that active siblings are
// preserved when one sibling is inactive.
//
// VALIDATES: AC-4 -- config with no inactive leaves behaves identically.
// PREVENTS: Pruning accidentally removing active nodes.
func TestPruneInactivePreservesActive(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	input := `bgp {
    local {
        as 65000
    }
    router-id 1.2.3.4
    peer alice {
        remote {
            ip 10.0.0.1
            as 65001
        }
    }
    peer bob {
        remote {
            ip 10.0.0.2
            as 65002
        }
    }
}`
	parser := NewParser(schema)
	tree, err := parser.Parse(input)
	require.NoError(t, err)

	PruneInactive(tree, schema)

	bgp := tree.GetContainer("bgp")
	peers := bgp.GetList("peer")
	assert.Len(t, peers, 2, "both active peers should survive pruning")
}

// TestPruneInactiveDefault verifies that nodes without an inactive leaf
// (default false) are not pruned.
//
// VALIDATES: AC-4 -- default is active.
// PREVENTS: Missing inactive leaf being treated as inactive.
func TestPruneInactiveDefault(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	input := `bgp {
    local {
        as 65000
    }
    router-id 1.2.3.4
    peer alice {
        remote {
            ip 10.0.0.1
            as 65001
        }
    }
}`
	parser := NewParser(schema)
	tree, err := parser.Parse(input)
	require.NoError(t, err)

	PruneInactive(tree, schema)

	bgp := tree.GetContainer("bgp")
	peers := bgp.GetList("peer")
	assert.Len(t, peers, 1, "peer without inactive should survive")
}

// TestPruneInactiveNested verifies that an inactive parent removes its
// entire subtree (e.g., inactive group removes all its peers).
//
// VALIDATES: AC-2 -- group with inactive removes all peers.
// PREVENTS: Peers in inactive groups surviving into resolution.
func TestPruneInactiveNested(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	input := `bgp {
    local {
        as 65000
    }
    router-id 1.2.3.4
    group mygroup {
        inactive enable
        remote {
            as 65001
        }
        peer alice {
            remote {
                ip 10.0.0.1
            }
        }
        peer bob {
            remote {
                ip 10.0.0.2
            }
        }
    }
}`
	parser := NewParser(schema)
	tree, err := parser.Parse(input)
	require.NoError(t, err)

	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	groups := bgp.GetList("group")
	require.Len(t, groups, 1, "group should exist before pruning")

	PruneInactive(tree, schema)

	groups = bgp.GetList("group")
	assert.Len(t, groups, 0, "inactive group should be removed with all its peers")
}

// TestPruneInactiveListEntry verifies that individual inactive list entries
// are removed while active entries remain.
//
// VALIDATES: AC-3 -- inactive update block not announced.
// PREVENTS: Inactive update blocks leaking into route extraction.
func TestPruneInactiveListEntry(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	input := `bgp {
    local {
        as 65000
    }
    router-id 1.2.3.4
    peer alice {
        remote {
            ip 10.0.0.1
            as 65001
        }
        update {
            attribute {
                origin igp
                next-hop 10.0.0.1
            }
            nlri {
                ipv4/unicast add 10.0.0.0/24
            }
        }
        update {
            inactive enable
            attribute {
                origin igp
                next-hop 10.0.0.1
            }
            nlri {
                ipv4/unicast add 10.0.1.0/24
            }
        }
    }
}`
	parser := NewParser(schema)
	tree, err := parser.Parse(input)
	require.NoError(t, err)

	bgp := tree.GetContainer("bgp")
	peers := bgp.GetList("peer")
	alice := peers["alice"]
	require.NotNil(t, alice)
	updates := alice.GetList("update")
	require.Len(t, updates, 2, "both updates should exist before pruning")

	PruneInactive(tree, schema)

	updates = alice.GetList("update")
	assert.Len(t, updates, 1, "inactive update should be removed")
}
