package schema

import (
	"strings"
	"testing"
)

// TestNTPCmdSchemaOwnsShowSystemNTP is the owner half of the
// self-containment invariant: the central show schema must NOT declare the
// NTP status commands, and this package MUST.
// See ai/rules/plugin-self-containment.md.
func TestNTPCmdSchemaOwnsShowSystemNTP(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:system-ntp"`,
		`ze:command "ze-show:system-ntp-peers"`,
		"container ntp",
	} {
		if !strings.Contains(ZeNTPCmdYANG, want) {
			t.Errorf("ze-ntp-cmd.yang must declare %q so removing the ntp plugin removes the surface", want)
		}
	}
}
