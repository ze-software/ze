package yang

import (
	"strings"
	"testing"
)

// TestRRCmdSchemaOwnsShowRR is the owner half of the self-containment invariant
// checked by internal/component/cmd/show/schema's TestShowSchemaHasNoBGPPluginCommands:
// the central show schema must NOT declare `show rr ...`, and this package MUST.
// Together they prove the surface moved rather than vanished. See
// ai/rules/plugins.md.
func TestRRCmdSchemaOwnsShowRR(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:rr-status"`,
		`ze:command "ze-show:rr-peers"`,
		"container rr",
	} {
		if !strings.Contains(ZeRRCmdYANG, want) {
			t.Errorf("ze-rr-cmd.yang must declare %q so removing the bgp-rr plugin removes the show rr surface", want)
		}
	}
}
