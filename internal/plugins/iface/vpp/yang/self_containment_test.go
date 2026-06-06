package yang

import (
	"strings"
	"testing"
)

func TestVPPCmdSchemaOwnsShowVPP(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:vpp-trace-start"`,
		`ze:command "ze-show:vpp-trace-show"`,
		`ze:command "ze-show:vpp-trace-clear"`,
		`ze:command "ze-show:vpp-runtime"`,
		"container vpp",
		"container trace",
		"container runtime",
	} {
		if !strings.Contains(ZeVPPCmdYANG, want) {
			t.Errorf("ze-vpp-cmd.yang must declare %q so removing vpp removes the surface", want)
		}
	}
}
