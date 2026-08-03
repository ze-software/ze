package yang

import (
	"strings"
	"testing"
)

// TestRSVPTECmdSchemaOwnsShowRSVPTE is the owner half of the self-containment
// invariant: the central show schema must NOT declare `show rsvp-te ...`, and
// this package MUST. See ai/rules/plugins.md.
func TestRSVPTECmdSchemaOwnsShowRSVPTE(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:rsvp-te-lsp"`,
		`ze:command "ze-show:rsvp-te-interface"`,
		`ze:command "ze-show:rsvp-te-tunnel"`,
		"container rsvp-te",
	} {
		if !strings.Contains(ZeRSVPTECmdYANG, want) {
			t.Errorf("ze-rsvp-te-cmd.yang must declare %q so removing the rsvp-te component removes the show rsvp-te surface", want)
		}
	}
}
