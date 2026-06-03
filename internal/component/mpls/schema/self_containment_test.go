package schema

import (
	"strings"
	"testing"
)

func TestMPLSCmdSchemaOwnsShowCommands(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:mpls-forwarding"`,
		`augment "/clishowcmd:show"`,
		"container mpls",
	} {
		if !strings.Contains(ZeMPLSCmdYANG, want) {
			t.Errorf("ze-mpls-cmd.yang must declare %q so removing MPLS removes its show surface", want)
		}
	}
}
