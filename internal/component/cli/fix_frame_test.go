// Design: docs/architecture/cli/error-surface.md -- The operator error
// Related: fix_frame.go -- FrameFix
package cli

import (
	"testing"
)

// TestFrameFix proves FrameFix writes a fix as the command the operator types
// where they are, in each of its three arms.
//
// VALIDATES: A-15 -- the relative form when the operator sits where the
// arguments are relative to, the plain root form at the root, `top` with the
// whole path from any other position.
// PREVENTS: a commit refusal inside a context printing a root-form `set` the
// context-relative `set` cannot take, or a refused relative `set` answered
// with a root path the operator did not type.
// Method: each arm is called with a fix and a position and compared whole.
func TestFrameFix(t *testing.T) {
	peer := []string{"bgp", "peer", "peer1"}
	leaf := []string{"session", "capability", "route-refresh", "require"}
	rootArgs := []string{"bgp", "peer", "peer1", "session", "capability", "route-refresh", "require"}
	cases := []struct {
		name     string
		fix      FixCommand
		operator []string
		want     string
	}{
		{"relative, at the position", FixCommand{Verb: cmdSet, At: peer, Args: leaf}, peer, "set session capability route-refresh require"},
		{"relative, operator at the root", FixCommand{Verb: cmdSet, At: peer, Args: leaf}, nil, "set bgp peer peer1 session capability route-refresh require"},
		{"relative, operator elsewhere", FixCommand{Verb: cmdSet, At: peer, Args: leaf}, []string{"ospf"}, "top set bgp peer peer1 session capability route-refresh require"},
		{"root path, operator at the root", FixCommand{Verb: cmdSet, Args: rootArgs}, nil, "set bgp peer peer1 session capability route-refresh require"},
		{"root path, operator at an empty position", FixCommand{Verb: cmdSet, Args: rootArgs}, []string{}, "set bgp peer peer1 session capability route-refresh require"},
		{"root path, operator in a context", FixCommand{Verb: cmdSet, Args: rootArgs}, peer, "top set bgp peer peer1 session capability route-refresh require"},
		{"delete of a member, in a context", FixCommand{Verb: cmdDelete, Args: []string{"bgp", "filter", "import", "nope"}}, peer, "top delete bgp filter import nope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FrameFix(tc.fix, tc.operator); got != tc.want {
				t.Errorf("FrameFix:\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}
