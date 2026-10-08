// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- owner-presence half of the
// plugin-self-containment both-halves invariant for `show isis ...` / `clear isis ...`.
// Related: ze-isis-cmd.yang -- the owner command tree these tests assert.
//
// VALIDATES: the IS-IS command YANG (owned by the isis component) declares every
// ze-isis:show-* show token and every ze-isis:clear-* clear token, so removing
// the isis component removes the whole show/clear surface together with the
// handlers (the owner half of ai/rules/plugins.md). The central
// show/clear schemas assert the matching central-guard half (none of these
// tokens may appear there).
// PREVENTS: a show/clear command whose schema silently drifts out of the owner,
// leaving the command unreachable after the owner is removed.

package yang

import (
	"strings"
	"testing"
)

// TestISISCmdSchemaOwnsShowISIS asserts the owner command YANG declares the
// `show isis ...` command tokens and the containers that scaffold them.
func TestISISCmdSchemaOwnsShowISIS(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-isis:show-neighbor"`,
		`ze:command "ze-isis:show-database"`,
		`ze:command "ze-isis:show-database-detail"`,
		`ze:command "ze-isis:show-route"`,
		`ze:command "ze-isis:show-route-ipv6"`,
		`ze:command "ze-isis:show-interface"`,
		`ze:command "ze-isis:show-hostname"`,
		`ze:command "ze-isis:show-spf-log"`,
		"container isis",
		"container database",
		"container spf-log",
	} {
		if !strings.Contains(ZeIsisCmdYANG, want) {
			t.Errorf("ze-isis-cmd.yang must declare %q so removing the isis component removes the show isis surface", want)
		}
	}
}

// TestISISCmdSchemaOwnsClearISIS asserts the owner command YANG declares the
// `clear isis ...` command tokens.
func TestISISCmdSchemaOwnsClearISIS(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-isis:clear-adjacency"`,
		`ze:command "ze-isis:clear-counters"`,
		"container clear",
		"container adjacency",
		"container counters",
	} {
		if !strings.Contains(ZeIsisCmdYANG, want) {
			t.Errorf("ze-isis-cmd.yang must declare %q so removing the isis component removes the clear isis surface", want)
		}
	}
}
