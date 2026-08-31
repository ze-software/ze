package yang

import (
	"strings"
	"testing"
)

// TestConfigCliCmdSchemaOwnsConfigCommands is the owner half of the
// self-containment invariant: the central show schema must NOT declare the
// config inspection commands, and this package MUST.
// See ai/rules/plugins.md.
func TestConfigCliCmdSchemaOwnsConfigCommands(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:config-dump"`,
		`ze:command "ze-show:config-diff"`,
		`ze:command "ze-show:config-history"`,
		`ze:command "ze-show:config-list"`,
		`ze:command "ze-show:config-cat"`,
		`ze:command "ze-show:config-fmt"`,
		"container config",
	} {
		if !strings.Contains(ZeConfigCliCmdYANG, want) {
			t.Errorf("ze-config-cli-cmd.yang must declare %q so removing config CLI removes the surface", want)
		}
	}
}
