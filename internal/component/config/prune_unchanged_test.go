package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Blank imports trigger init() registration of YANG modules.
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/softver/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/yang"
)

// parseTree is a test helper that parses config text into a Tree.
func parseTree(t *testing.T, schema *Schema, text string) *Tree {
	t.Helper()
	tree, err := NewParser(schema).Parse(text)
	require.NoError(t, err)
	return tree
}

// pruneFixture parses a baseline/modified pair and prunes modified against baseline,
// returning the serialized pruned output.
func pruneFixture(t *testing.T, baseline, modified string) string {
	t.Helper()
	schema, err := YANGSchema()
	require.NoError(t, err)

	base := parseTree(t, schema, baseline)
	mod := parseTree(t, schema, modified)

	PruneUnchanged(mod, base, schema)
	return Serialize(mod, schema)
}

const pruneBaseConfig = `bgp {
    router-id 1.2.3.4
    session {
        asn {
            local 65000
        }
    }
}`

// TestPruneUnchangedKeepsChangedLeaf verifies a modified leaf survives pruning.
//
// VALIDATES: AC-1, AC-7 -- changed leaf shown.
// PREVENTS: The reported bug's inverse -- pruning away the very change the user made.
func TestPruneUnchangedKeepsChangedLeaf(t *testing.T) {
	out := pruneFixture(t, pruneBaseConfig, `bgp {
    router-id 5.6.7.8
    session {
        asn {
            local 65000
        }
    }
}`)

	assert.Contains(t, out, "router-id 5.6.7.8", "changed leaf must survive")
}

// TestPruneUnchangedDropsUnchangedSiblings verifies unchanged leaves are removed.
//
// VALIDATES: AC-1 -- the reported bug: only the changed part is presented.
// PREVENTS: show | compare rendering the entire configuration.
func TestPruneUnchangedDropsUnchangedSiblings(t *testing.T) {
	out := pruneFixture(t, pruneBaseConfig, `bgp {
    router-id 5.6.7.8
    session {
        asn {
            local 65000
        }
    }
}`)

	assert.NotContains(t, out, "local 65000", "unchanged leaf must be pruned")
	assert.NotContains(t, out, "session", "container with no changed descendant must be pruned")
}

// TestPruneUnchangedKeepsAncestors verifies enclosing containers survive so the
// change stays locatable and the output remains valid config.
//
// VALIDATES: AC-2 -- enclosing container context retained.
// PREVENTS: Emitting a bare leaf with no hierarchy, which would not re-parse.
func TestPruneUnchangedKeepsAncestors(t *testing.T) {
	out := pruneFixture(t, pruneBaseConfig, `bgp {
    router-id 5.6.7.8
    session {
        asn {
            local 65000
        }
    }
}`)

	assert.Contains(t, out, "bgp", "ancestor container must be retained as context")

	// The pruned output must still be valid, re-parseable config.
	schema, err := YANGSchema()
	require.NoError(t, err)
	_, err = NewParser(schema).Parse(out)
	require.NoError(t, err, "pruned output must re-parse as valid config")
}

// TestPruneUnchangedEmptyWhenIdentical verifies an unchanged tree prunes to nothing.
//
// VALIDATES: AC-3 -- no changes produces no output (not a full config dump).
// PREVENTS: show | compare dumping the config when nothing changed.
func TestPruneUnchangedEmptyWhenIdentical(t *testing.T) {
	out := pruneFixture(t, pruneBaseConfig, pruneBaseConfig)

	assert.Empty(t, strings.TrimSpace(out), "identical trees must prune to nothing")
}

// TestPruneUnchangedAddedContainerKeptWhole verifies a wholly new list entry keeps
// every descendant, since none of it exists in the baseline.
//
// VALIDATES: AC-5, AC-12 -- added entry shown in full.
// PREVENTS: Recursing into a new subtree and pruning its children against a nil baseline.
func TestPruneUnchangedAddedContainerKeptWhole(t *testing.T) {
	out := pruneFixture(t, pruneBaseConfig, `bgp {
    router-id 1.2.3.4
    session {
        asn {
            local 65000
        }
    }
    peer new-peer {
        connection {
            remote {
                ip 10.0.0.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}`)

	assert.Contains(t, out, "new-peer", "added entry must survive")
	assert.Contains(t, out, "10.0.0.1", "added entry keeps its descendants")
	assert.Contains(t, out, "65001", "added entry keeps all its descendants")
	assert.NotContains(t, out, "router-id", "unchanged sibling still pruned")
}

// TestPruneUnchangedRemovalIsVisibleOnlyViaBaseline verifies the symmetry the
// compare view depends on. The operator deleted a peer, so the peer exists ONLY in
// the baseline. Pruning the working tree can never show it; pruning the baseline
// against the working tree retains it, which is what lets the gutter render '-'.
//
// VALIDATES: AC-6, A-2 -- removals are visible, and only because both directions
// are pruned.
// PREVENTS: Pruning one direction only, which would silently drop every deletion
// from compare output.
func TestPruneUnchangedRemovalIsVisibleOnlyViaBaseline(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	baselineText := `bgp {
    router-id 1.2.3.4
    session {
        asn {
            local 65000
        }
    }
    peer doomed {
        connection {
            remote {
                ip 10.0.0.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}`

	// The operator deleted "peer doomed": it is in the baseline, not the working tree.
	working := parseTree(t, schema, pruneBaseConfig)
	baseline := parseTree(t, schema, baselineText)

	// Direction 1 (working vs baseline) -- what compare shows as '+' / '*'.
	// The deleted peer cannot appear here; it does not exist in the working tree.
	workingPruned := working.Clone()
	PruneUnchanged(workingPruned, baseline, schema)
	assert.NotContains(t, Serialize(workingPruned, schema), "doomed",
		"the working direction cannot show a node the working tree does not have")

	// Direction 2 (baseline vs working) -- what compare shows as '-'.
	// This is the only direction that retains the deletion.
	baselinePruned := baseline.Clone()
	PruneUnchanged(baselinePruned, working, schema)
	out := Serialize(baselinePruned, schema)
	assert.Contains(t, out, "doomed", "the baseline direction must retain the removed peer")
	assert.Contains(t, out, "10.0.0.1", "the removed peer keeps its descendants")
	assert.NotContains(t, out, "local 65000",
		"unchanged config is still pruned from the baseline direction")
}

// TestPruneUnchangedListEntriesIsolated verifies only the changed list entry survives.
//
// VALIDATES: AC-12 -- unchanged sibling list entries pruned.
// PREVENTS: Showing every peer when one peer changed.
func TestPruneUnchangedListEntriesIsolated(t *testing.T) {
	twoPeers := `bgp {
    router-id 1.2.3.4
    peer alpha {
        connection {
            remote {
                ip 10.0.0.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
    peer beta {
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
    }
}`

	changedAlpha := strings.Replace(twoPeers, "ip 10.0.0.1", "ip 10.9.9.9", 1)
	out := pruneFixture(t, twoPeers, changedAlpha)

	assert.Contains(t, out, "alpha", "changed entry survives")
	assert.Contains(t, out, "10.9.9.9", "changed value survives")
	assert.NotContains(t, out, "beta", "unchanged sibling entry pruned")
	assert.NotContains(t, out, "65001", "unchanged leaf inside the changed entry pruned")
}

// TestPruneUnchangedDeactivationIsAChange verifies deactivating a leaf counts as a
// change, because the serializer renders the "inactive: " prefix.
//
// VALIDATES: AC-19, R-6 -- deactivation visible in compare.
// PREVENTS: A deactivation-only edit pruning to an empty diff.
func TestPruneUnchangedDeactivationIsAChange(t *testing.T) {
	out := pruneFixture(t, pruneBaseConfig, `bgp {
    inactive: router-id 1.2.3.4
    session {
        asn {
            local 65000
        }
    }
}`)

	assert.Contains(t, out, "router-id", "deactivated leaf must survive as a change")
	assert.NotContains(t, out, "local 65000", "unchanged sibling still pruned")
}

// TestPruneUnchangedNilBaselineKeepsAll verifies a nil baseline prunes nothing:
// with nothing to compare against, every node is new.
//
// VALIDATES: Defensive contract matching PruneInactive/PruneActive nil guards.
// PREVENTS: A nil baseline silently emptying the tree.
func TestPruneUnchangedNilBaselineKeepsAll(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	mod := parseTree(t, schema, pruneBaseConfig)
	PruneUnchanged(mod, nil, schema)

	out := Serialize(mod, schema)
	assert.Contains(t, out, "router-id 1.2.3.4", "nil baseline must not prune")
	assert.Contains(t, out, "local 65000", "nil baseline must not prune")
}

// TestPruneUnchangedNilTreeNoPanic verifies the nil guards mirror PruneInactive.
//
// VALIDATES: Defensive contract.
// PREVENTS: Panic on a nil tree or schema.
func TestPruneUnchangedNilTreeNoPanic(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	require.NotPanics(t, func() { PruneUnchanged(nil, nil, schema) })
	require.NotPanics(t, func() { PruneUnchanged(NewTree(), NewTree(), nil) })
}

// TestPruneUnchangedAgreesWithSerializer verifies the prune decision is exactly
// "serializes identically" for every node kind reachable in the schema.
//
// VALIDATES: A-9, R-1 -- one comparison rule covers all node kinds, and what prunes
// never diverges from what the serializer (and therefore the operator) sees.
// PREVENTS: A per-kind dispatch drifting from the serializer's notion of equality.
func TestPruneUnchangedAgreesWithSerializer(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	// Identical trees: every child serializes the same, so all must prune.
	base := parseTree(t, schema, pruneBaseConfig)
	mod := parseTree(t, schema, pruneBaseConfig)
	require.Equal(t, Serialize(base, schema), Serialize(mod, schema), "fixture sanity")

	PruneUnchanged(mod, base, schema)
	assert.Empty(t, strings.TrimSpace(Serialize(mod, schema)),
		"identical serialization on every kind must prune everything")
}
