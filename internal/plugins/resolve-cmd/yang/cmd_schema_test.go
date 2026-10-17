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
		`ze:command "ze-resolve:clear-dns-cache"`,
		`ze:command "ze-resolve:clear-dns-cache-record"`,
		`ze:command "ze-resolve:clear-dns-cache-stats"`,
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
		`ze:command "ze-resolve:show-dns-lookup"`,
		`ze:command "ze-resolve:show-dns-cache-stats"`,
		`ze:command "ze-resolve:show-dns-cache-list"`,
		`ze:command "ze-resolve:show-dns-cache-record"`,
		"container show",
	} {
		if !strings.Contains(ZeResolveCmdYANG, want) {
			t.Errorf("ze-resolve-cmd.yang must declare %q so removing the resolve component removes the show dns surface", want)
		}
	}
}

// TestResolveCmdSchemaOwnsClearClientCaches is the owner half of the
// self-containment invariant for clear peeringdb, irr and cymru cache, and it
// holds the owner's rule that a bare prefix never clears: each cache node
// declares an all form and a keyed form, and no command of its own.
//
// VALIDATES: the six commands live here, and the bare cache nodes carry none.
// PREVENTS: clear irr cache, typed short, emptying every cache.
func TestResolveCmdSchemaOwnsClearClientCaches(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-resolve:clear-peeringdb-cache-all"`,
		`ze:command "ze-resolve:clear-peeringdb-cache-asn"`,
		`ze:command "ze-resolve:clear-irr-cache-all"`,
		`ze:command "ze-resolve:clear-irr-cache-as-set"`,
		`ze:command "ze-resolve:clear-cymru-cache-all"`,
		`ze:command "ze-resolve:clear-cymru-cache-address"`,
	} {
		if !strings.Contains(ZeResolveCmdYANG, want) {
			t.Errorf("ze-resolve-cmd.yang must declare %q", want)
		}
	}
	for _, banned := range []string{
		`"ze-resolve:clear-peeringdb-cache"`,
		`"ze-resolve:clear-irr-cache"`,
		`"ze-resolve:clear-cymru-cache"`,
	} {
		if strings.Contains(ZeResolveCmdYANG, banned) {
			t.Errorf("ze-resolve-cmd.yang declares %s on the bare cache node: the bare prefix must not clear, give all explicitly", banned)
		}
	}
}
