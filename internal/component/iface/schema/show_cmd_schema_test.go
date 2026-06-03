package schema

import (
	"strings"
	"testing"
)

// TestIfaceShowCmdSchemaOwnsKernelReads is the owner half of the
// self-containment invariant: the central show schema must NOT declare the
// kernel-table reads (show ip arp/route, show neighbors, show kernel-routes),
// and this package MUST. See ai/rules/plugin-self-containment.md.
func TestIfaceShowCmdSchemaOwnsKernelReads(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:ip-arp"`,
		`ze:command "ze-show:ip-route"`,
		`ze:command "ze-show:route-lookup"`,
		`ze:command "ze-show:neighbors"`,
		`ze:command "ze-show:kernel-routes"`,
		"container ip",
		"container neighbors",
		"container kernel-routes",
	} {
		if !strings.Contains(ZeIfaceShowCmdYANG, want) {
			t.Errorf("ze-iface-show-cmd.yang must declare %q so removing the iface kernel reads removes the surface", want)
		}
	}
}

// TestIfaceInterfaceCmdSchemaOwnsInterface is the owner half of the
// self-containment invariant for the `show interface` family and
// `monitor interface rate`: the central show and monitor schemas must NOT
// declare any of them, and this package MUST. See
// ai/rules/plugin-self-containment.md.
func TestIfaceInterfaceCmdSchemaOwnsInterface(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:interface"`,
		`ze:command "ze-show:interface-scan"`,
		`ze:command "ze-show:interface-detail"`,
		`ze:command "ze-show:interface-counters"`,
		`ze:command "ze-monitor:interface-rate"`,
		"container interface",
	} {
		if !strings.Contains(ZeIfaceInterfaceCmdYANG, want) {
			t.Errorf("ze-iface-interface-cmd.yang must declare %q so removing iface removes the interface surface", want)
		}
	}
}
