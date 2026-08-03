package yang

import (
	"strings"
	"testing"
)

// TestLDPCmdSchemaOwnsShowLDP is the owner half of the self-containment
// invariant: the central show schema must NOT declare `show ldp ...`, and this
// package MUST. See ai/rules/plugins.md.
func TestLDPCmdSchemaOwnsShowLDP(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:ldp-neighbor"`,
		`ze:command "ze-show:ldp-binding"`,
		"container ldp",
	} {
		if !strings.Contains(ZeLDPCmdYANG, want) {
			t.Errorf("ze-ldp-cmd.yang must declare %q so removing the ldp component removes the show ldp surface", want)
		}
	}
}
