// Design: docs/architecture/config/yang-config-design.md — OS-portable interface address config
package iface

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestIfaceUnitAddressPathIsOSPortable verifies that the per-unit ipv4/ipv6
// address leaf-lists exist in the schema on every OS, not just Linux.
//
// Interface addressing is config you author anywhere and apply on the Linux
// target, so the `unit <n> ipv4 address` / `ipv6 address` syntax must be the
// same on macOS as on Linux. Before this was fixed the ipv4/ipv6 containers
// carried ze:os "linux" and were pruned at schema-build time on darwin, so the
// CLI/web editor accepted `set ... unit 0 address ...` (a path one level short
// of the real leaf), stored it under an unknown name, and the change file then
// re-read as "corrupt" (test/web/scenario-interface-setup.wb).
//
// VALIDATES: ipv4/ipv6 address leaf-lists are present regardless of runtime.GOOS.
// PREVENTS: OS-divergent interface config syntax and corrupt change files.
func TestIfaceUnitAddressPathIsOSPortable(t *testing.T) {
	s, err := config.YANGSchema()
	require.NoError(t, err)

	iface, ok := s.Get("interface").(*config.ContainerNode)
	require.True(t, ok, "interface container missing from schema")

	eth, ok := iface.Get("ethernet").(*config.ListNode)
	require.True(t, ok, "interface.ethernet list missing from schema")

	unit, ok := eth.Get("unit").(*config.ListNode)
	require.True(t, ok, "interface.ethernet.unit list missing from schema")

	for _, family := range []string{"ipv4", "ipv6"} {
		fam, ok := unit.Get(family).(*config.ContainerNode)
		require.Truef(t, ok, "interface.ethernet.unit.%s container missing on this OS", family)

		addr := fam.Get("address")
		require.NotNilf(t, addr, "interface.ethernet.unit.%s.address missing on this OS", family)
		require.Equalf(t, config.NodeLeaf, addr.Kind(),
			"interface.ethernet.unit.%s.address must be a settable leaf", family)
	}
}
