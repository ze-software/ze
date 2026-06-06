package yang

import (
	"strings"
	"testing"
)

func TestStaticCmdSchemaOwnsShowStatic(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:static"`,
		"container static",
	} {
		if !strings.Contains(ZeStaticCmdYANG, want) {
			t.Errorf("ze-static-cmd.yang must declare %q so removing static removes the surface", want)
		}
	}
}
