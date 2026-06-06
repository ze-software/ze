package yang

import (
	"strings"
	"testing"
)

func TestLogCmdSchemaOwnsLogCommands(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-bgp:log-levels"`,
		`ze:command "ze-bgp:log-recent"`,
		`ze:command "ze-bgp:log-set"`,
		"container log",
		"container levels",
		"container recent",
		"container level",
	} {
		if !strings.Contains(ZeLogCmdYANG, want) {
			t.Errorf("ze-log-cmd.yang must declare %q so removing slogutil removes the log command surface", want)
		}
	}
}
