package schema

import (
	"strings"
	"testing"
)

// TestVPPCmdSchemaOwnsShowVPP is the owner half of the self-containment
// invariant: the central show schema must NOT declare `show vpp ...`, and this
// package MUST. See ai/rules/plugin-self-containment.md.
func TestVPPCmdSchemaOwnsShowVPP(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:vpp-trace-start"`,
		`ze:command "ze-show:vpp-trace-show"`,
		`ze:command "ze-show:vpp-trace-clear"`,
		`ze:command "ze-show:vpp-runtime"`,
		`ze:backend "vpp"`,
		"container vpp",
	} {
		if !strings.Contains(ZeVPPCmdYANG, want) {
			t.Errorf("ze-vpp-cmd.yang must declare %q so removing the VPP backend removes the show vpp surface", want)
		}
	}
}
