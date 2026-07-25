//go:build linux

package iface

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestIfaceYANGDHCPBackendAnnotation verifies that the unit-level dhcp and
// dhcpv6 containers carry ze:backend "netlink" so the commit-time feature
// gate rejects `dhcp { enabled true }` (and the IPv6 counterpart) when the
// active backend is anything other than netlink -- VPP cannot honor
// address lifetimes (netlink RTM_NEWADDR valid_lft / preferred_lft are
// netlink-only).
//
// The ipv4/ipv6 containers are present on every OS (interface addressing is
// OS-portable config, edited anywhere and applied on the Linux target), so the
// assertion itself is platform-independent; it lives in a linux-tagged file
// alongside the rest of the iface schema assertions.
//
// VALIDATES: the four-case verifier extension (see iface-vpp-rejects-dhcp.ci).
// PREVENTS: a future YANG edit silently dropping the annotation, which
// would make `backend vpp; dhcp { enabled true; }` accept at verify time
// and only fail at Apply time on ReplaceAddressWithLifetime / DHCP client
// startup -- exactly the silent-wrong path exact-or-reject bans.
func TestIfaceYANGDHCPBackendAnnotation(t *testing.T) {
	s, err := config.YANGSchema()
	require.NoError(t, err)

	iface := s.Get("interface")
	require.NotNil(t, iface, "interface container missing from schema")
	ifaceCN, ok := iface.(*config.ContainerNode)
	require.True(t, ok, "interface must be a container")

	// dhcp / dhcpv6 live under the per-family containers (ipv4/ipv6) of any
	// unit-bearing list; ethernet's unit list is the canonical site shared
	// with dummy, veth, bridge, tunnel, wireguard, and loopback via
	// `uses interface-unit`. The family nesting matches how config.go parses
	// them and the path asserted by iface-vpp-rejects-dhcp.ci
	// (/interface/ethernet/<name>/unit/<n>/ipv4/dhcp).
	eth := ifaceCN.Get("ethernet")
	require.NotNil(t, eth, "interface.ethernet missing from schema")
	ethList, ok := eth.(*config.ListNode)
	require.True(t, ok, "interface.ethernet must be a list")

	unit := ethList.Get("unit")
	require.NotNil(t, unit, "interface.ethernet.unit missing from schema")
	unitList, ok := unit.(*config.ListNode)
	require.True(t, ok, "interface.ethernet.unit must be a list")

	for _, tc := range []struct{ family, child string }{
		{"ipv4", "dhcp"},
		{"ipv6", "dhcpv6"},
	} {
		fam := unitList.Get(tc.family)
		require.NotNilf(t, fam, "interface.ethernet.unit.%s missing from schema", tc.family)
		famCN, ok := fam.(*config.ContainerNode)
		require.Truef(t, ok, "interface.ethernet.unit.%s must be a container", tc.family)

		node := famCN.Get(tc.child)
		require.NotNilf(t, node, "interface.ethernet.unit.%s.%s missing from schema", tc.family, tc.child)
		cn, ok := node.(*config.ContainerNode)
		require.Truef(t, ok, "interface.ethernet.unit.%s.%s must be a container", tc.family, tc.child)
		assert.Equalf(t, []string{"netlink"}, cn.Backend,
			"interface.ethernet.unit.%s.%s must carry ze:backend \"netlink\"", tc.family, tc.child)
	}
}
