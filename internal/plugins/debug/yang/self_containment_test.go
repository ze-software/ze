package yang

import (
	"strings"
	"testing"
)

func TestDebugCmdSchemaOwnsShowDebug(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-debug:debug-state"`,
		"container debug",
		"container show",
	} {
		if !strings.Contains(ZeDebugCmdYANG, want) {
			t.Errorf("ze-debug-cmd.yang must declare %q so removing debug plugin removes the show debug command surface", want)
		}
	}
}
