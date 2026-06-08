package yang

import (
	"strings"
	"testing"
)

func TestFirewallCmdSchemaOwnsShowCommands(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:firewall-ruleset"`,
		`ze:command "ze-show:firewall-group"`,
		`ze:command "ze-show:system-conntrack"`,
		"container show {",
		"container firewall {",
		"container conntrack {",
		"container system {",
	} {
		if !strings.Contains(ZeFirewallCmdYANG, want) {
			t.Errorf("ze-firewall-cmd.yang must declare %q so removing firewall removes its show surface", want)
		}
	}
}
