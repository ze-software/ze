package schema

import (
	"strings"
	"testing"
)

// TestShowSchemaHasNoBGPPluginCommands enforces ai/rules/plugin-self-containment.md
// for the central `show` verb schema.
//
// VALIDATES: the central show schema declares no part of the `show bgp ...`
// subtree. BGP peer state and RIB queries are owned by the BGP plugin schemas
// (internal/component/bgp/plugins/cmd/{peer,rib}/schema); the offline
// decode/encode diagnostics are owned by cmd/ze/bgp/schema next to their
// handlers. Removing the BGP surface must remove the whole `show bgp ...`
// branch with no dangling YANG node.
//
// PREVENTS: regression where any BGP command's schema drifts back into the
// central show package, which would leave a handler-less command node after
// the BGP surface is removed.
func TestShowSchemaHasNoBGPPluginCommands(t *testing.T) {
	// Command tokens owned by removable BGP packages; none may appear in the
	// central show schema.
	banned := map[string]string{
		"ze-rib-api:":          "BGP RIB queries -> internal/component/bgp/plugins/cmd/rib/schema",
		`"ze-bgp:peer-`:        "BGP peer state -> internal/component/bgp/plugins/cmd/peer/schema",
		`"ze-show:bgp-decode"`: "offline BGP decode -> cmd/ze/bgp/schema",
		`"ze-show:bgp-encode"`: "offline BGP encode -> cmd/ze/bgp/schema",
	}
	for token, owner := range banned {
		if strings.Contains(ZeCliShowCmdYANG, token) {
			t.Errorf("central show schema declares BGP-owned command %q; move it to %s (see ai/rules/plugin-self-containment.md)", token, owner)
		}
	}
}
