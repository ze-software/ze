package yang

import (
	"strings"
	"testing"
)

// TestResolveCmdSchemaOwnsClearDNSCache is the owner half of the
// self-containment invariant for the clear dns cache command family: the
// central clear schema must NOT declare it, and this package MUST. Removing
// the resolve component must remove the whole clear dns cache surface with no
// dangling node.
// See ai/rules/plugins.md.
func TestResolveCmdSchemaOwnsClearDNSCache(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-clear:dns-cache"`,
		`ze:command "ze-clear:dns-cache-record"`,
		`ze:command "ze-clear:dns-cache-stats"`,
		"container clear",
	} {
		if !strings.Contains(ZeResolveCmdYANG, want) {
			t.Errorf("ze-resolve-cmd.yang must declare %q so removing the resolve component removes the clear dns cache surface", want)
		}
	}
}

// TestResolveCmdSchemaOwnsShowDNS is the owner half of the self-containment
// invariant for show dns lookup and the show dns cache command family: the
// central show schema must NOT declare them, and this package MUST. Removing
// the resolve component must remove the whole show dns surface with no dangling
// node.
// See ai/rules/plugins.md.
func TestResolveCmdSchemaOwnsShowDNS(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:dns-lookup"`,
		`ze:command "ze-show:dns-cache-stats"`,
		`ze:command "ze-show:dns-cache-list"`,
		`ze:command "ze-show:dns-cache-record"`,
		"container show",
	} {
		if !strings.Contains(ZeResolveCmdYANG, want) {
			t.Errorf("ze-resolve-cmd.yang must declare %q so removing the resolve component removes the show dns surface", want)
		}
	}
}
