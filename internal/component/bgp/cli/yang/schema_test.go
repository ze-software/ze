package yang

import (
	"strings"
	"testing"
)

// TestBGPToolsSchemaOwnsDecodeEncode is the owner half of the self-containment
// invariant checked by internal/component/cmd/show/schema's
// TestShowSchemaHasNoBGPPluginCommands: the central show schema must NOT declare
// `show bgp decode`/`encode`, and this package MUST. Together they prove the
// surface moved rather than vanished. See ai/rules/plugins.md.
func TestBGPToolsSchemaOwnsDecodeEncode(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:bgp-decode"`,
		`ze:command "ze-show:bgp-encode"`,
		"container bgp",
		"container decode",
		"container encode",
	} {
		if !strings.Contains(ZeBGPToolsCmdYANG, want) {
			t.Errorf("ze-bgp-tools-cmd.yang must declare %q so removing cmd/ze/bgp removes the show bgp decode/encode surface", want)
		}
	}
}
