package yang

import (
	"strings"
	"testing"
)

func TestCrashesCmdSchemaOwnsShowCrashes(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:crashes"`,
		"container crashes",
	} {
		if !strings.Contains(ZeCrashesCmdYANG, want) {
			t.Errorf("ze-crashes-cmd.yang must declare %q so removing crashes removes the surface", want)
		}
	}
}
