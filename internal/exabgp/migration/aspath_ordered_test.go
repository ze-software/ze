// Design: docs/architecture/core-design.md — external format translation

package migration

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestExaBGPNamedVPLSPreservesASPathDuplicates pins that a named VPLS block's
// as-path keeps duplicate ASNs (prepends) instead of collapsing them as a set.
// The exabgp route-attributes grouping marks as-path ze:ordered; this proves the
// extension survives `uses` expansion into the flex vpls container's child node.
func TestExaBGPNamedVPLSPreservesASPathDuplicates(t *testing.T) {
	// Schema-level: the vpls container's as-path child must be Ordered. l2vpn lives
	// under the neighbor list, not at the top level.
	schema := exaBGPSchema()
	require.NotNil(t, schema)
	neighbor, ok := schema.Get("neighbor").(*config.ListNode)
	require.True(t, ok, "neighbor should be a list")
	l2vpn, ok := neighbor.Get("l2vpn").(*config.ContainerNode)
	require.True(t, ok, "neighbor/l2vpn should be a container")
	vpls, ok := l2vpn.Get("vpls").(*config.FlexNode)
	require.True(t, ok, "l2vpn/vpls should be a flex node")
	asPath, ok := vpls.Get("as-path").(*config.ValueOrArrayNode)
	require.True(t, ok, "vpls/as-path should be a value-or-array node")
	require.True(t, asPath.Ordered, "vpls/as-path must be Ordered (ze:ordered via uses route-attributes)")

	// Parse-level: a named vpls block with duplicate as-path keeps all copies.
	input := `neighbor 127.0.0.1 {
	router-id 127.0.0.1;
	local-as 65000;
	peer-as 65000;
	l2vpn {
		vpls site5 {
			endpoint 5;
			base 10732;
			as-path [ 30740 30740 30740 ];
		}
	}
}`
	tree, err := ParseExaBGPConfig(input)
	require.NoError(t, err)
	nbrs := tree.GetListOrdered("neighbor")
	require.Len(t, nbrs, 1)
	l2vpnTree := nbrs[0].Value.GetContainer("l2vpn")
	require.NotNil(t, l2vpnTree)
	vplsEntries := l2vpnTree.GetListOrdered("vpls")
	require.Len(t, vplsEntries, 1)
	require.Equal(t, []string{"30740", "30740", "30740"}, vplsEntries[0].Value.GetSlice("as-path"))
}
