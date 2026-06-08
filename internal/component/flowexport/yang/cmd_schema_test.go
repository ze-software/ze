package yang

import (
	"strings"
	"testing"
)

// TestFlowExportCmdSchemaOwnsShowFlowExport is the owner half of the
// self-containment invariant: the central show schema must NOT declare
// `show flow-export`, and this package MUST. See ai/rules/plugin-self-containment.md.
func TestFlowExportCmdSchemaOwnsShowFlowExport(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:flow-export"`,
		"container flow-export",
	} {
		if !strings.Contains(ZeFlowExportCmdYANG, want) {
			t.Errorf("ze-flowexport-cmd.yang must declare %q so removing the flow-export component removes the show flow-export surface", want)
		}
	}
}
