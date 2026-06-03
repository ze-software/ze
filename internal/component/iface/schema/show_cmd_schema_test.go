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
