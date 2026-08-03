package yang

import (
	"strings"
	"testing"
)

// TestPingCmdSchemaOwnsPing is the owner half of the self-containment invariant.
// The central show, monitor, and resolve schemas must NOT declare any `ping`
// command node, and this dedicated ping module MUST declare all of them.
// Together they prove the ping surface moved rather than vanished. See
// ai/rules/plugins.md.
func TestPingCmdSchemaOwnsPing(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:ping"`,
		`ze:command "ze-monitor:ping"`,
		`ze:command "ze-resolve:ping"`,
		"container ping",
	} {
		if !strings.Contains(ZePingCmdYANG, want) {
			t.Errorf("ze-ping-cmd.yang must declare %q so the ping feature owns its whole command surface", want)
		}
	}
}
