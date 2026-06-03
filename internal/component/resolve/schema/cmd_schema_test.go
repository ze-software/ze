package schema

import (
	"strings"
	"testing"
)

// TestResolveCmdSchemaOwnsClearDNSCache is the owner half of the
// self-containment invariant for `clear dns cache`: the central clear schema
// must NOT declare it, and this package MUST. Removing the resolve component
// must remove the whole `clear dns cache` surface with no dangling node.
// See ai/rules/plugin-self-containment.md.
func TestResolveCmdSchemaOwnsClearDNSCache(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-clear:dns-cache"`,
		"container clear",
	} {
		if !strings.Contains(ZeResolveCmdYANG, want) {
			t.Errorf("ze-resolve-cmd.yang must declare %q so removing the resolve component removes the clear dns cache surface", want)
		}
	}
}
