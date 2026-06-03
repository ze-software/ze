package schema

import (
	"strings"
	"testing"
)

func TestPKICmdSchemaOwnsShowCommands(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:pki-certificates"`,
		`ze:command "ze-show:pki-certificate"`,
		`augment "/clishowcmd:show"`,
		"container pki",
	} {
		if !strings.Contains(ZePKICmdYANG, want) {
			t.Errorf("ze-pki-cmd.yang must declare %q so removing PKI removes its show surface", want)
		}
	}
}
