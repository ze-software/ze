package yang

import (
	"strings"
	"testing"
)

// TestEnvCmdSchemaOwnsShowEnv is the owner half of the self-containment
// invariant: the central show schema must NOT declare the env commands,
// and this package MUST. See ai/rules/plugins.md.
func TestEnvCmdSchemaOwnsShowEnv(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-env:show-list"`,
		`ze:command "ze-env:show-get"`,
		`ze:command "ze-env:show-registered"`,
		"container env",
	} {
		if !strings.Contains(ZeEnvCmdYANG, want) {
			t.Errorf("ze-env-cmd.yang must declare %q so removing env removes the surface", want)
		}
	}
}
