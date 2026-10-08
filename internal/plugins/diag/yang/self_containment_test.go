package yang

import (
	"strings"
	"testing"
)

func TestDiagCmdSchemaOwnsCaptureAndTCPCheck(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-diag:show-tcp-check"`,
		`ze:command "ze-diag:show-capture"`,
		`ze:command "ze-diag:show-capture-raw"`,
		`ze:command "ze-diag:show-capture-interface"`,
		"container tcp-check",
		"container capture",
		"container raw",
		"container interface",
	} {
		if !strings.Contains(ZeDiagCmdYANG, want) {
			t.Errorf("ze-diag-cmd.yang must declare %q so removing the diag component removes the surface", want)
		}
	}
}
