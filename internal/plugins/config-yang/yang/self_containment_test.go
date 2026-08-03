package yang

import (
	"strings"
	"testing"
)

// TestYangCliCmdSchemaOwnsYangCommands is the owner half of the
// self-containment invariant: the central show schema must NOT declare the
// yang introspection commands, and this package MUST.
// See ai/rules/plugins.md.
func TestYangCliCmdSchemaOwnsYangCommands(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:yang-tree"`,
		`ze:command "ze-show:yang-completion"`,
		`ze:command "ze-show:yang-doc"`,
		"container yang",
	} {
		if !strings.Contains(ZeYangCliCmdYANG, want) {
			t.Errorf("ze-yang-cli-cmd.yang must declare %q so removing yang CLI removes the surface", want)
		}
	}
}
