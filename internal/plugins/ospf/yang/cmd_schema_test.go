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
		`ze:command "ze-show:ospf";`,
		`ze:command "ze-show:ospf-neighbor";`,
		`ze:command "ze-show:ospf-interface";`,
		`ze:command "ze-show:ospf-database";`,
		`ze:command "ze-show:ospf-database-router";`,
		`ze:command "ze-show:ospf-database-network";`,
		`ze:command "ze-show:ospf-database-summary";`,
		`ze:command "ze-show:ospf-database-asbr-summary";`,
		`ze:command "ze-show:ospf-database-external";`,
		`ze:command "ze-show:ospf-database-nssa-external";`,
		`ze:command "ze-show:ospf-database-opaque-link";`,
		`ze:command "ze-show:ospf-database-opaque-area";`,
		`ze:command "ze-show:ospf-database-opaque-as";`,
		`ze:command "ze-show:ospf-route";`,
		`ze:command "ze-show:ospf-border-routers";`,
		`ze:command "ze-show:ospf-spf";`,
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
		`ze:command "ze-show:ospf-database-opaque-area-detail";`,
		`ze:command "ze-show:ospf-database-opaque-as-detail";`,
		`ze:command "ze-show:ospf-database-opaque-link-detail";`,
		`ze:command "ze-show:ospf-spf-detail";`,
		`ze:command "ze-show:ospf-neighbor-detail";`,
		`ze:command "ze-show:ospf-interface-detail";`,
		`ze:command "ze-debug:ospf-inject";`,
		`ze:command "ze-debug:ospf-inject-enable";`,
		`ze:command "ze-debug:ospf-inject-disable";`,
	}
	for _, tok := range want {
		if !strings.Contains(ZeOSPFCmdYANG, tok) {
			t.Errorf("OSPF cmd schema is missing new IPv4 command %q (spec-ospf-ext-14 discoverability)", tok)
		}
	}
}

func TestV3NewCommandsDiscoverable(t *testing.T) {
	want := []string{
		`ze:command "ze-show:ospfv3-database";`,
		`ze:command "ze-show:ospfv3-database-detail";`,
		`ze:command "ze-show:ospfv3-database-router-detail";`,
		`ze:command "ze-show:ospfv3-database-scope-link";`,
		`ze:command "ze-show:ospfv3-database-scope-area";`,
		`ze:command "ze-show:ospfv3-database-scope-as";`,
		`ze:command "ze-show:ospfv3-database-router-information";`,
		`ze:command "ze-show:ospfv3-database-extended";`,
		`ze:command "ze-show:ospfv3-database-segment-routing";`,
		`ze:command "ze-show:ospfv3-instance";`,
		`ze:command "ze-show:ospfv3-neighbor";`,
		`ze:command "ze-show:ospfv3-neighbor-detail";`,
		`ze:command "ze-show:ospfv3-interface-detail";`,
		`ze:command "ze-show:ospfv3-spf";`,
		`ze:command "ze-show:ospfv3-spf-detail";`,
		`ze:command "ze-debug:ospfv3-inject";`,
	}
	for _, tok := range want {
		if !strings.Contains(ZeOSPFCmdYANG, tok) {
			t.Errorf("OSPF cmd schema is missing new IPv6 command %q (spec-ospf-ext-14 discoverability)", tok)
		}
	}
}

func TestOSPFCmdSchemaOwnsClearOSPF(t *testing.T) {
	want := []string{
		`ze:command "ze-clear:ospf-process";`,
		`ze:command "ze-clear:ospf-neighbor";`,
		`ze:command "ze-clear:ospf-counters";`,
	}
	for _, tok := range want {
		if !strings.Contains(ZeOSPFCmdYANG, tok) {
			t.Errorf("OSPF cmd schema is missing clear command %q (owner half of plugin-self-containment; see ai/rules/plugins.md)", tok)
		}
	}
}
