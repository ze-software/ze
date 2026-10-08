// VALIDATES: spec-ospf-13 plugin-self-containment (owner half) -- the OSPF command schema
// owned by this component declares every `show ospf ...` and `clear ospf ...` command
// token. The central show/clear schemas assert the SAME tokens are ABSENT (the central-guard
// half, in internal/component/cmd/{show,clear}/yang/self_containment_test.go), so removing the
// OSPF plugin removes the whole subtree with no dangling, handler-less CLI node.
// PREVENTS: the owner schema silently losing a command token (leaving a registered CLI verb
// with no schema node) while the central guard still passes -- both halves must hold together.
package yang

import (
	"strings"
	"testing"
)

func TestOSPFCmdSchemaOwnsShowOSPF(t *testing.T) {
	want := []string{
		`ze:command "ze-ospf:show-ospf";`,
		`ze:command "ze-ospf:show-neighbor";`,
		`ze:command "ze-ospf:show-interface";`,
		`ze:command "ze-ospf:show-database";`,
		`ze:command "ze-ospf:show-database-router";`,
		`ze:command "ze-ospf:show-database-network";`,
		`ze:command "ze-ospf:show-database-summary";`,
		`ze:command "ze-ospf:show-database-asbr-summary";`,
		`ze:command "ze-ospf:show-database-external";`,
		`ze:command "ze-ospf:show-database-nssa-external";`,
		`ze:command "ze-ospf:show-database-opaque-link";`,
		`ze:command "ze-ospf:show-database-opaque-area";`,
		`ze:command "ze-ospf:show-database-opaque-as";`,
		`ze:command "ze-ospf:show-route";`,
		`ze:command "ze-ospf:show-border-routers";`,
		`ze:command "ze-ospf:show-spf";`,
	}
	for _, tok := range want {
		if !strings.Contains(ZeOSPFCmdYANG, tok) {
			t.Errorf("OSPF cmd schema is missing show command %q (owner half of plugin-self-containment; see ai/rules/plugins.md)", tok)
		}
	}
}

// TestNewCommandsDiscoverable / TestV3NewCommandsDiscoverable: spec-ospf-ext-14 R-4 -- every
// new IPv4 and IPv6 command self-documents its dispatch key in the owner schema, so it
// appears in completion and the dispatch-key listing (no hidden RPC-name-only command).
func TestNewCommandsDiscoverable(t *testing.T) {
	want := []string{
		`ze:command "ze-ospf:show-database-opaque-area-detail";`,
		`ze:command "ze-ospf:show-database-opaque-as-detail";`,
		`ze:command "ze-ospf:show-database-opaque-link-detail";`,
		`ze:command "ze-ospf:show-spf-detail";`,
		`ze:command "ze-ospf:show-neighbor-detail";`,
		`ze:command "ze-ospf:show-interface-detail";`,
		`ze:command "ze-ospf:debug-inject";`,
		`ze:command "ze-ospf:debug-inject-enable";`,
		`ze:command "ze-ospf:debug-inject-disable";`,
	}
	for _, tok := range want {
		if !strings.Contains(ZeOSPFCmdYANG, tok) {
			t.Errorf("OSPF cmd schema is missing new IPv4 command %q (spec-ospf-ext-14 discoverability)", tok)
		}
	}
}

func TestV3NewCommandsDiscoverable(t *testing.T) {
	want := []string{
		`ze:command "ze-ospf:show-ospfv3-database";`,
		`ze:command "ze-ospf:show-ospfv3-database-detail";`,
		`ze:command "ze-ospf:show-ospfv3-database-router-detail";`,
		`ze:command "ze-ospf:show-ospfv3-database-scope-link";`,
		`ze:command "ze-ospf:show-ospfv3-database-scope-area";`,
		`ze:command "ze-ospf:show-ospfv3-database-scope-as";`,
		`ze:command "ze-ospf:show-ospfv3-database-router-information";`,
		`ze:command "ze-ospf:show-ospfv3-database-extended";`,
		`ze:command "ze-ospf:show-ospfv3-database-segment-routing";`,
		`ze:command "ze-ospf:show-ospfv3-instance";`,
		`ze:command "ze-ospf:show-ospfv3-neighbor";`,
		`ze:command "ze-ospf:show-ospfv3-neighbor-detail";`,
		`ze:command "ze-ospf:show-ospfv3-interface-detail";`,
		`ze:command "ze-ospf:show-ospfv3-spf";`,
		`ze:command "ze-ospf:show-ospfv3-spf-detail";`,
		`ze:command "ze-ospf:debug-ospfv3-inject";`,
	}
	for _, tok := range want {
		if !strings.Contains(ZeOSPFCmdYANG, tok) {
			t.Errorf("OSPF cmd schema is missing new IPv6 command %q (spec-ospf-ext-14 discoverability)", tok)
		}
	}
}

func TestOSPFCmdSchemaOwnsClearOSPF(t *testing.T) {
	want := []string{
		`ze:command "ze-ospf:clear-process";`,
		`ze:command "ze-ospf:clear-neighbor";`,
		`ze:command "ze-ospf:clear-counters";`,
	}
	for _, tok := range want {
		if !strings.Contains(ZeOSPFCmdYANG, tok) {
			t.Errorf("OSPF cmd schema is missing clear command %q (owner half of plugin-self-containment; see ai/rules/plugins.md)", tok)
		}
	}
}
