package yang

import (
	"strings"
	"testing"
)

func TestAAACmdSchemaOwnsShowCommands(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-aaa:show-accounting"`,
		`clishowcmd:show`,
		"container aaa",
	} {
		if !strings.Contains(ZeAAACmdYANG, want) {
			t.Errorf("ze-aaa-cmd.yang must declare %q so removing AAA removes its show surface", want)
		}
	}
}
