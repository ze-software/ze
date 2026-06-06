package schema

import (
	"strings"
	"testing"
)

func TestFirewallCmdSchemaOwnsShowCommands(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:firewall-ruleset"`,
		`ze:command "ze-show:firewall-group"`,
		`ze:command "ze-show:system-conntrack"`,
		`augment "/clishowcmd:show"`,
		"container firewall",
		"container conntrack",
	} {
		if !strings.Contains(ZeFirewallCmdYANG, want) {
			t.Errorf("ze-firewall-cmd.yang must declare %q so removing firewall removes its show surface", want)
		}
	}
}
