package yang

import (
	"strings"
	"testing"
)

// TestBMPCmdSchemaOwnsShowBMP is the owner half of the self-containment invariant
// checked by internal/component/cmd/show/schema's TestShowSchemaHasNoBGPPluginCommands:
// the central show schema must NOT declare `show bmp ...`, and this package MUST.
// Together they prove the surface moved rather than vanished. See
// ai/rules/plugins.md.
func TestBMPCmdSchemaOwnsShowBMP(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:bmp-sessions"`,
		`ze:command "ze-show:bmp-peers"`,
		`ze:command "ze-show:bmp-collectors"`,
		`ze:command "ze-show:bmp-rib"`,
		"container bmp",
	} {
		if !strings.Contains(ZeBMPCmdYANG, want) {
			t.Errorf("ze-bmp-cmd.yang must declare %q so removing the bgp-bmp plugin removes the show bmp surface", want)
		}
	}
}
