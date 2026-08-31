package yang

import (
	"strings"
	"testing"
)

// TestStorageCliCmdSchemaOwnsDataCommands is the owner half of the
// self-containment invariant: the central show schema must NOT declare the
// data storage commands, and this package MUST.
// See ai/rules/plugins.md.
func TestStorageCliCmdSchemaOwnsDataCommands(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:data-list"`,
		`ze:command "ze-show:data-cat"`,
		`ze:command "ze-show:data-registered"`,
		"container data",
	} {
		if !strings.Contains(ZeStorageCliCmdYANG, want) {
			t.Errorf("ze-storage-cli-cmd.yang must declare %q so removing storage CLI removes the surface", want)
		}
	}
}
