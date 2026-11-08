// Design: docs/guide/pppoe.md -- numbering a LAN unit from a delegated prefix
// Related: delegated_prefix.go -- delegationSourceReference under test

package iface

import (
	"slices"
	"testing"
)

// TestDelegationSourceReference checks that the delegated-prefix source
// completes exactly the pppoe-clients verify accepts: those whose ipv6 dhcpv6
// is enabled and requests a delegated prefix. Method: a lowered config root
// with one client of each shape, and the answer the editor reads.
// Break: drop either check in requestsDelegation; the client without pd, or
// the one with DHCPv6 disabled, joins the answer and the test goes red.
func TestDelegationSourceReference(t *testing.T) {
	pd := map[string]any{"length": "56"}
	root := map[string]any{"interface": map[string]any{"pppoe-client": map[string]any{
		"pppoe0": map[string]any{"ipv6": map[string]any{"enabled": "true",
			"dhcpv6": map[string]any{"enabled": "true", "pd": pd}}},
		"pppoe1": map[string]any{"ipv6": map[string]any{"enabled": "true",
			"dhcpv6": map[string]any{"enabled": "true"}}},
		"pppoe2": map[string]any{"ipv6": map[string]any{"enabled": "true",
			"dhcpv6": map[string]any{"enabled": "false", "pd": pd}}},
		"pppoe3": map[string]any{},
	}}}
	answer := delegationSourceReference(root, nil)
	if !slices.Equal(answer.Names, []string{"pppoe0"}) {
		t.Fatalf("delegation sources = %v, want [pppoe0]", answer.Names)
	}
	if answer.Check == nil {
		t.Fatal("no commit judge: commit would refuse an unknown source before verify names the unit and the fix")
	}
	if err := answer.Check("wan9"); err != nil {
		t.Fatalf("reference refused %q before the interface verify could: %v", "wan9", err)
	}
	if got := delegationSourceReference(map[string]any{}, nil).Names; len(got) != 0 {
		t.Fatalf("no interface section offered %v", got)
	}
}
