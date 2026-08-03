package yang

import (
	"strings"
	"testing"
)

// TestDeleteSchemaHasNoMigratedOwnerCommands enforces ai/rules/plugins.md
// for the central `delete` verb schema. Every delete command is owned by its
// component and merges its own `delete <noun> ...` subtree onto the bare
// verb-root anchor here. The owning component's schema holds the matching
// presence test.
//
// PREVENTS: regression where an owner command's schema drifts back into the
// central delete package, which would leave a handler-less command node after
// the owner is removed.
func TestDeleteSchemaHasNoMigratedOwnerCommands(t *testing.T) {
	banned := map[string]string{
		`"ze-delete:bgp-peer"`: "BGP peer removal -> internal/component/bgp/plugins/cmd/peer/yang",
	}
	for token, owner := range banned {
		if strings.Contains(ZeCliDeleteCmdYANG, token) {
			t.Errorf("central delete schema declares owner command %q; move it to %s (see ai/rules/plugins.md)", token, owner)
		}
	}
}

// TestDeleteSchemaIsBareAnchor asserts the delete verb schema declares no
// ze:command of its own: every delete command has carved out to an owner,
// leaving only the bare `container delete` root that owners merge onto.
// See ai/rules/plugins.md.
func TestDeleteSchemaIsBareAnchor(t *testing.T) {
	if strings.Contains(ZeCliDeleteCmdYANG, "ze:command") {
		t.Error("central delete schema declares a ze:command; it must be a bare verb-root anchor (every delete command is owner-owned)")
	}
}
