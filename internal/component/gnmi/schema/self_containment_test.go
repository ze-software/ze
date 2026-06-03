package schema

import (
	"strings"
	"testing"
)

func TestGNMICmdSchemaOwnsShowCommands(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:gnmi"`,
		`clishowcmd:show`,
		"container gnmi",
	} {
		if !strings.Contains(ZeGNMICmdYANG, want) {
			t.Errorf("ze-gnmi-cmd.yang must declare %q so removing gNMI removes its show surface", want)
		}
	}
}
