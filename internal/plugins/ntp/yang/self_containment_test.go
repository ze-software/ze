package yang

import (
	"strings"
	"testing"
)

func TestNTPCmdSchemaOwnsShowNTP(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:system-ntp"`,
		`ze:command "ze-show:system-ntp-peers"`,
		"container ntp",
	} {
		if !strings.Contains(ZeNTPCmdYANG, want) {
			t.Errorf("ze-ntp-cmd.yang must declare %q so removing ntp removes the surface", want)
		}
	}
}
