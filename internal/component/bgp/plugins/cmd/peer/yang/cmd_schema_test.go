package yang

import (
	"strings"
	"testing"
)

// TestPeerCmdSchemaOwnsCarvedVerbs is the owner half of the self-containment
// invariant for the BGP peer command surface that was relocated out of the
// central show/delete schemas: the central verb schemas must NOT declare these
// tokens, and this package MUST. Removing the BGP peer command owner must
// remove the whole `show bgp health` and `delete bgp peer` surface with no
// dangling node. See ai/rules/plugins.md.
func TestPeerCmdSchemaOwnsCarvedVerbs(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:bgp-health"`,
		`ze:command "ze-delete:bgp-peer"`,
		`ze:command "ze-update:bgp-peer-prefix"`,
		"container show",
		"container delete",
		"container update",
	} {
		if !strings.Contains(ZePeerCmdYANG, want) {
			t.Errorf("ze-peer-cmd.yang must declare %q so removing the BGP peer command owner removes that surface", want)
		}
	}
}
