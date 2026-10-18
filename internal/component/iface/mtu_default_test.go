package iface

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestMTUDefaultMatchesApplyPath proves the schema declares no MTU default
// that the apply path never applies. It looks up the mtu leaf of every
// interface kind and parses an entry with no mtu.
//
// VALIDATES: an omitted mtu leaf carries no YANG default on any kind, and
// parseIfaceEntry leaves MTU at 0, so applyConfig makes no SetMTU call and
// the device keeps its kernel default (1420 for wireguard, 1476 for gre,
// 1500 for a dummy).
//
// PREVENTS: a `default 1500` the reader trusts while a wireguard or tunnel
// device runs at the kernel's own value. Applying 1500 instead would push a
// wireguard device past its 80-byte encapsulation overhead.
func TestMTUDefaultMatchesApplyPath(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	kinds := []string{"ethernet", "dummy", "veth", "bridge", "tunnel", "wireguard", "xfrm", "pppoe-client"}
	for _, kind := range kinds {
		node, err := schema.Lookup("interface/" + kind + "/mtu")
		require.NoError(t, err, kind)
		leaf, ok := node.(*config.LeafNode)
		require.True(t, ok, "%s mtu must resolve to a LeafNode", kind)
		assert.Empty(t, leaf.Default, "%s mtu declares a default the apply path never applies", kind)
	}

	entry, err := parseIfaceEntry("wg0", map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, 0, entry.MTU, "an omitted mtu must leave the device's MTU untouched")
}
