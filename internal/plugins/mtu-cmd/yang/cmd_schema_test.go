package yang

import (
	"strings"
	"testing"
)

// TestMTUCmdSchemaOwnsShowMTU is the owner half of the self-containment
// invariant: the central show schema declares no `mtu` node, and this
// dedicated module declares the command, its host address and its two bare
// words. Dropping the module removes the whole surface. See ai/rules/plugins.md.
func TestMTUCmdSchemaOwnsShowMTU(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:mtu"`,
		"container mtu",
		"leaf host",
		"type zt:ip-address",
		"enum exhaustive",
		"enum detail",
	} {
		if !strings.Contains(ZeMtuCmdYANG, want) {
			t.Errorf("ze-mtu-cmd.yang must declare %q so the mtu feature owns its whole command surface", want)
		}
	}
}
