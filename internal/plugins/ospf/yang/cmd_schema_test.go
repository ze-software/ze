// VALIDATES: spec-ospf-13 plugin-self-containment (owner half) -- the OSPF command schema
// owned by this component declares every `show ip ospf ...` and `clear ip ospf ...` command
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
		`ze:command "ze-show:ospf-route";`,
		`ze:command "ze-show:ospf-border-routers";`,
		`ze:command "ze-show:ospf-spf";`,
	}
	for _, tok := range want {
		if !strings.Contains(ZeOSPFCmdYANG, tok) {
			t.Errorf("OSPF cmd schema is missing show command %q (owner half of plugin-self-containment; see ai/rules/plugin-self-containment.md)", tok)
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
			t.Errorf("OSPF cmd schema is missing clear command %q (owner half of plugin-self-containment; see ai/rules/plugin-self-containment.md)", tok)
		}
	}
}
