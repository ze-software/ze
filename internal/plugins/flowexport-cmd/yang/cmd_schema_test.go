package yang

import (
	"strings"
	"testing"
)

// TestFlowExportCmdSchemaOwnsShowFlowExport is the owner half of the
// self-containment invariant: the central show schema must NOT declare
// `show flow export`, and this package MUST. See ai/rules/plugins.md.
func TestFlowExportCmdSchemaOwnsShowFlowExport(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:flow-export"`,
		"container flow",
		"container export",
	} {
		if !strings.Contains(ZeFlowExportCmdYANG, want) {
			t.Errorf("ze-flowexport-cmd.yang must declare %q so removing the flowexport component removes the show flow export surface", want)
		}
	}
}
