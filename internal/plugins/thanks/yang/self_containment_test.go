package yang

import (
	"strings"
	"testing"
)

// TestThanksCmdSchemaOwnsShowThanks is the owner half of the self-containment
// invariant: the central show schema must NOT declare the thanks commands,
// and this package MUST. See ai/rules/plugins.md.
func TestThanksCmdSchemaOwnsShowThanks(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-thanks:show-list"`,
		`ze:command "ze-thanks:show-licence"`,
		"container thanks",
	} {
		if !strings.Contains(ZeThanksCmdYANG, want) {
			t.Errorf("ze-thanks-cmd.yang must declare %q so removing thanks removes the surface", want)
		}
	}
}
