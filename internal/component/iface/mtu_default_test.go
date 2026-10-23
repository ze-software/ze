// Design: docs/architecture/iface/management.md — MTU schema coverage.

package iface

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestMTUDefaultMatchesApplyPath proves the schema declares no MTU default
// that the apply path never applies. It walks the resolved schema's interface
// lists for this OS and parses an entry with no mtu.
//
// VALIDATES: an omitted mtu leaf carries no YANG default on any registered
// interface list available on this OS, and
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

	node, err := schema.Lookup(configRootInterface)
	require.NoError(t, err)
	interfaces, ok := node.(*config.ContainerNode)
	require.True(t, ok, "interface must resolve to a ContainerNode")

	// Schema construction prunes ze:os-restricted kinds before this walk.
	// Derive coverage from its lists, not a second inventory of kind names.
	checked := 0
	for _, kind := range interfaces.Children() {
		list, ok := interfaces.Get(kind).(*config.ListNode)
		if !ok {
			continue
		}
		t.Run(kind, func(t *testing.T) {
			leaf, ok := list.Get("mtu").(*config.LeafNode)
			require.True(t, ok, "%s mtu must resolve to a LeafNode", kind)
			assert.Empty(t, leaf.Default, "%s mtu declares a default the apply path never applies", kind)
		})
		checked++
	}
	require.Positive(t, checked, "registered interface schema must expose MTU-bearing lists")

	entry, err := parseIfaceEntry("wg0", map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, 0, entry.MTU, "an omitted mtu must leave the device's MTU untouched")
}
