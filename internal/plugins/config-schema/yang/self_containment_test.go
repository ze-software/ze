package yang

import (
	"strings"
	"testing"
)

// TestSchemaCliCmdSchemaOwnsSchemaCommands is the owner half of the
// self-containment invariant: the central show schema must NOT declare the
// schema introspection commands, and this package MUST.
// See ai/rules/plugins.md.
func TestSchemaCliCmdSchemaOwnsSchemaCommands(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:schema-list"`,
		`ze:command "ze-show:schema-methods"`,
		`ze:command "ze-show:schema-events"`,
		`ze:command "ze-show:schema-handlers"`,
		`ze:command "ze-show:schema-protocol"`,
		"container schema",
	} {
		if !strings.Contains(ZeSchemaCliCmdYANG, want) {
			t.Errorf("ze-schema-cli-cmd.yang must declare %q so removing schema CLI removes the surface", want)
		}
	}
}
