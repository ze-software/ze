package schema

import (
	"strings"
	"testing"
)

// TestStaticCmdSchemaOwnsShowStatic is the owner half of the self-containment
// invariant: the central show schema must NOT declare `show static`, and this
// package MUST. See ai/rules/plugin-self-containment.md.
func TestStaticCmdSchemaOwnsShowStatic(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:static"`,
		"container static",
	} {
		if !strings.Contains(ZeStaticCmdYANG, want) {
			t.Errorf("ze-static-cmd.yang must declare %q so removing the static plugin removes the show static surface", want)
		}
	}
}
