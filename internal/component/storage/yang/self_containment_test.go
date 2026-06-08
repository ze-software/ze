package yang

import (
	"strings"
	"testing"
)

func TestStorageCmdSchemaOwnsShowCommands(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:storage-smart"`,
		`clishowcmd:show`,
		"container storage",
	} {
		if !strings.Contains(ZeStorageCmdYANG, want) {
			t.Errorf("ze-storage-cmd.yang must declare %q so removing storage removes its show surface", want)
		}
	}
}
