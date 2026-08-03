package yang

import (
	"strings"
	"testing"
)

// TestIfaceShowCmdSchemaOwnsKernelReads is the owner half of the
// self-containment invariant: the central show schema must NOT declare the
// kernel-table reads (show route, show neighbor, show arp), and this package
// MUST. The commands are object-rooted, so there is no shared `ip` container.
// See ai/rules/plugins.md and
// docs/architecture/cli/command-namespacing.md.
func TestIfaceShowCmdSchemaOwnsKernelReads(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:route"`,
		`ze:command "ze-show:route-lookup"`,
		`ze:command "ze-show:neighbor"`,
		`ze:command "ze-show:arp"`,
		"container route",
		"container neighbor",
		"container arp",
	} {
		if !strings.Contains(ZeIfaceShowCmdYANG, want) {
			t.Errorf("ze-iface-show-cmd.yang must declare %q so removing the iface kernel reads removes the surface", want)
		}
	}
}

// TestIfaceMonitorCmdSchemaOwnsNetlink is the owner half of the
// self-containment invariant for `monitor system netlink`: the central
// monitor schema must NOT declare it, and this package MUST.
// See ai/rules/plugins.md.
func TestIfaceMonitorCmdSchemaOwnsNetlink(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-monitor:system-netlink"`,
		"container monitor",
		"container system",
		"container netlink",
	} {
		if !strings.Contains(ZeIfaceMonitorCmdYANG, want) {
			t.Errorf("ze-iface-monitor-cmd.yang must declare %q so removing iface removes the netlink monitor surface", want)
		}
	}
}

// TestIfaceInterfaceCmdSchemaOwnsInterface is the owner half of the
// self-containment invariant for the `show interface` family,
// `monitor interface rate`, and `clear interface counters`: the central show,
// monitor, and clear schemas must NOT declare any of them, and this package
// MUST. See ai/rules/plugins.md.
func TestIfaceInterfaceCmdSchemaOwnsInterface(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:interface"`,
		`ze:command "ze-show:interface-scan"`,
		`ze:command "ze-show:interface-detail"`,
		`ze:command "ze-show:interface-counters"`,
		`ze:command "ze-monitor:interface-rate"`,
		`ze:command "ze-clear:interface-counters"`,
		"container interface",
		"container clear",
	} {
		if !strings.Contains(ZeIfaceInterfaceCmdYANG, want) {
			t.Errorf("ze-iface-interface-cmd.yang must declare %q so removing iface removes the interface surface", want)
		}
	}
}
