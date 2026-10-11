// Design: docs/guide/bfd.md -- Enabling BFD on a BGP peer
// Related: peer_bfd.go -- verifyPeerBFDProfiles, the commit check that warns
// Related: rfc5882_bfd_auth_warn_test.go -- the RFC-tagged units over the same Warn lines
//
// Goal: the commit Warn for an EBGP peer whose bfd block names no profile tells
// the operator the exact change that resolves it (ai/rules/user-facing-errors.md),
// not only what is wrong. Method: verifyPeerBFDProfiles runs over a bgp tree whose
// EBGP peer enables BFD with no profile, with the peer logger swapped for a
// Warn-level text sink, and the one Warn line must carry the fix attribute naming
// both config paths the operator edits.
//
// VALIDATES: the no-profile Warn's fix attribute names the bfd profile with its
// auth block and the peer's connection { bfd { profile } } leaf.
// PREVENTS: a commit warning that says what is wrong but not how to fix it.
package reactor

import (
	"strings"
	"testing"
)

// TestPeerBFDNoProfileWarnNamesFix proves the "names no profile" Warn carries a
// fix attribute that names the profile to define, with its auth block, and the
// peer leaf that selects it. Dropping the fix attribute turns it red.
func TestPeerBFDNoProfileWarnNamesFix(t *testing.T) {
	sink := rfc5882WarnSink(t)
	tree := bfdStrictTree(map[string]any{"enabled": "true"})

	if err := verifyPeerBFDProfiles(tree, rfc5882PlainSection); err != nil {
		t.Fatalf("commit refused a peer with no bfd profile: %v", err)
	}

	lines := rfc5882WarnLines(sink)
	if len(lines) != 1 {
		t.Fatalf("commit logged %d Warn lines, want 1: %q", len(lines), lines)
	}
	line := lines[0]
	for _, want := range []string{
		"names no profile",
		"peer=192.0.2.1",
		"fix=",
		"define bfd { profile <name> { auth { type keyed-sha1; key-id <0-255>; secret <secret>; } } }",
		"with the type, key-id and secret the neighbor uses",
		"then set connection { bfd { profile <name>; } } on this peer",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("Warn line %q does not carry %q", line, want)
		}
	}
}
