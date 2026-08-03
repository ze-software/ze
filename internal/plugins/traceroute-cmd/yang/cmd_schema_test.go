package yang

import (
	"strings"
	"testing"
)

// TestTracerouteCmdSchemaOwnsTraceroute is the owner half of the
// self-containment invariant. The central show, monitor, and resolve schemas
// must NOT declare any traceroute or probe-round command node, and this
// dedicated traceroute module MUST declare all of them. Together they prove the
// traceroute surface moved rather than vanished. See
// ai/rules/plugins.md.
func TestTracerouteCmdSchemaOwnsTraceroute(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:traceroute"`,
		`ze:command "ze-show:probe-round"`,
		`ze:command "ze-monitor:traceroute"`,
		`ze:command "ze-resolve:traceroute"`,
		"container traceroute",
		"container probe-round",
	} {
		if !strings.Contains(ZeTracerouteCmdYANG, want) {
			t.Errorf("ze-traceroute-cmd.yang must declare %q so the traceroute feature owns its whole command surface", want)
		}
	}
}
