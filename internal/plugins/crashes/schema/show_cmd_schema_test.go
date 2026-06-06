package schema

import (
	"strings"
	"testing"
)

// TestCrashesCmdSchemaOwnsShowCrashes is the owner half of the
// self-containment invariant: the central show schema must NOT declare
// `show crashes`, and this package MUST. See ai/rules/plugin-self-containment.md.
func TestCrashesCmdSchemaOwnsShowCrashes(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:crashes"`,
		"container crashes",
	} {
		if !strings.Contains(ZeCrashesCmdYANG, want) {
			t.Errorf("ze-crashes-cmd.yang must declare %q so removing the crashes plugin removes the surface", want)
		}
	}
}
