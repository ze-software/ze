// Design: docs/architecture/web-interface.md -- CLI terminal show and tree serialization.
// Related: cli_terminal.go -- serializeTreeAtPath.
// Related: handler_config_walk.go -- isBareListPath.

package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestSerializeTreeAtPathBareListWithEntries verifies that a path stopping at
// a keyed list name renders all configured entries of that list (and masks
// secrets), without leaking sibling nodes from the parent container.
//
// VALIDATES: AC-20 -- bare list paths such as ["bgp", "peer"] render their
// entries rather than silently returning "".
// PREVENTS: walkTree looking up KeyDefault on a keyed ListNode and returning
// nil, leaving web `show <list>` / ContentAtPath blank.
func TestSerializeTreeAtPathBareListWithEntries(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	const cfg = `bgp {
	router-id 192.0.2.99;
	session { asn { local 65000; } }
	group edge {
		description "edge-group";
		peer gamma {
			description "nested-peer";
		}
	}
	peer alpha {
		description "first-peer";
		connection {
			md5 {
				password "bare-list-secret-9981z";
			}
		}
	}
	peer beta {
		description "second-peer";
	}
}
`
	tree, err := config.NewParser(schema).Parse(cfg)
	require.NoError(t, err)

	out := serializeTreeAtPath(tree, schema, []string{"bgp", "peer"})
	assert.Contains(t, out, "peer alpha")
	assert.Contains(t, out, "first-peer")
	assert.Contains(t, out, "peer beta")
	assert.Contains(t, out, "second-peer")
	assert.Contains(t, out, config.SecretDataPlaceholder, "bare list rendering must keep secret leaves masked")
	assert.NotContains(t, out, "bare-list-secret-9981z")
	assert.NotContains(t, out, "9981z")
	assert.NotContains(t, out, "router-id 192.0.2.99", "bare list rendering must not leak parent container siblings")

	// Nested list inside a keyed entry also renders its entries.
	nested := serializeTreeAtPath(tree, schema, []string{"bgp", "group", "edge", "peer"})
	assert.Contains(t, nested, "peer gamma")
	assert.Contains(t, nested, "nested-peer")
	assert.NotContains(t, nested, "edge-group")

	// Specific keyed entry still renders only that entry's body.
	entryOut := serializeTreeAtPath(tree, schema, []string{"bgp", "peer", "beta"})
	assert.Contains(t, entryOut, "second-peer")
	assert.NotContains(t, entryOut, "first-peer")
}

// TestSerializeTreeAtPathBareListEmpty verifies that a bare list path with no
// configured entries returns an explicit empty-list message rather than "".
//
// VALIDATES: AC-20 -- operator can distinguish a valid empty list from an
// unshown or unknown path.
// PREVENTS: silent empty-string fall-through on empty lists.
func TestSerializeTreeAtPathBareListEmpty(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	tree, err := config.NewParser(schema).Parse(validWebConfig)
	require.NoError(t, err)

	out := serializeTreeAtPath(tree, schema, []string{"bgp", "peer"})
	assert.Equal(t, "(no bgp peer entries)", out)

	// An unknown path still returns empty string.
	assert.Empty(t, serializeTreeAtPath(tree, schema, []string{"bgp", "nonexistent-node"}))
}
