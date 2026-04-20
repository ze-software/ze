//go:build linux

package iface

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestIfaceYANGDHCPBackendAnnotation verifies that the unit-level dhcp and
// dhcpv6 containers carry ze:backend "netlink" so the commit-time feature
// gate rejects `dhcp { enabled true }` (and the IPv6 counterpart) when the
// active backend is anything other than netlink -- VPP cannot honor
// address lifetimes (netlink RTM_NEWADDR valid_lft / preferred_lft are
// netlink-only).
//
// The test is linux-only because both containers are marked ze:os "linux"
// and the YANG schema walker prunes non-matching OS nodes at schema-build
// time, so the containers are simply absent from the schema on darwin.
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

	// dhcp / dhcpv6 live under any unit-bearing list; ethernet's unit
	// container is the canonical site shared with dummy, veth, bridge,
	// tunnel, wireguard, and loopback via `uses interface-unit`.
	eth := ifaceCN.Get("ethernet")
	require.NotNil(t, eth, "interface.ethernet missing from schema")
	ethList, ok := eth.(*config.ListNode)
	require.True(t, ok, "interface.ethernet must be a list")

	unit := ethList.Get("unit")
	require.NotNil(t, unit, "interface.ethernet.unit missing from schema")
	unitList, ok := unit.(*config.ListNode)
	require.True(t, ok, "interface.ethernet.unit must be a list")

	for _, name := range []string{"dhcp", "dhcpv6"} {
		node := unitList.Get(name)
		require.NotNilf(t, node, "interface.ethernet.unit.%s missing from schema", name)
		cn, ok := node.(*config.ContainerNode)
		require.Truef(t, ok, "interface.ethernet.unit.%s must be a container", name)
		assert.Equalf(t, []string{"netlink"}, cn.Backend,
			"interface.ethernet.unit.%s must carry ze:backend \"netlink\"", name)
	}
}
