package schema

import (
	"strings"
	"testing"
)

// TestShowSchemaHasNoBGPPluginCommands enforces ai/rules/plugin-self-containment.md
// for the central `show` verb schema.
//
// VALIDATES: RPC-backed BGP commands (peer state, RIB queries) are declared by
// the BGP plugin schemas (internal/component/bgp/plugins/cmd/{peer,rib}/schema),
// not by this central verb package, so that removing the BGP plugin removes the
// whole `show bgp peer ...` / `show bgp rib ...` surface with no dangling YANG.
//
// PREVENTS: regression where a BGP command's schema drifts back into the central
// show package, which would leave a handler-less command node after the BGP
// plugin is removed.
func TestShowSchemaHasNoBGPPluginCommands(t *testing.T) {
	// WireMethod prefixes owned by removable BGP plugin packages.
	banned := map[string]string{
		"ze-rib-api:":   "BGP RIB queries -> internal/component/bgp/plugins/cmd/rib/schema",
		`"ze-bgp:peer-`: "BGP peer state -> internal/component/bgp/plugins/cmd/peer/schema",
	}
	for token, owner := range banned {
		if strings.Contains(ZeCliShowCmdYANG, token) {
			t.Errorf("central show schema declares plugin-owned command %q; move it to %s (see ai/rules/plugin-self-containment.md)", token, owner)
		}
	}

	// The offline decode/encode wire diagnostics are the only `show bgp ...`
	// commands allowed to remain central; their handlers live in cmd/ze/bgp.
	// Assert they are still present so a future migration that relocates them
	// updates this expectation deliberately rather than silently.
	for _, want := range []string{"ze-show:bgp-decode", "ze-show:bgp-encode"} {
		if !strings.Contains(ZeCliShowCmdYANG, want) {
			t.Errorf("expected offline BGP diagnostic %q to remain in the central show schema", want)
		}
	}
}
