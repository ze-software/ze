// VALIDATES: the schema evolution that keeps a configuration saved before the
// `unmatched` default became discard on the bypass it ran with. It runs through
// migration.Evolve, the function the hub calls at start, with the evolution this
// package registers, then reads the result back with ParseIPsecConfig.
// PREVENTS: a router upgraded across the discard default losing its own BGP, SSH,
// DNS and NTP at the next start, and the evolution overriding the RFC default on a
// configuration that never ran under the old one.

package ipsec

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/migration"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// evolveUnmatched runs every registered evolution over tree as a file with stamp
// would, and parses the IPsec configuration the daemon would then apply.
func evolveUnmatched(t *testing.T, tree *config.Tree, stamp string) (*IPsecConfig, []string) {
	t.Helper()
	result, err := migration.Evolve(tree, stamp)
	if err != nil {
		t.Fatalf("Evolve(stamp %q): %v", stamp, err)
	}
	var applied []string
	if result != nil {
		tree = result.Tree
		applied = result.Applied
	}
	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig after Evolve(stamp %q): %v", stamp, err)
	}
	return cfg, applied
}

// TestUnmatchedEvolutionWritesBypassIntoAnOldFile checks that a file stamped by a
// release from before the discard default, holding `vpn ipsec` and no `unmatched`
// leaf, comes out with `unmatched bypass` written: the bypass it ran with, now a
// written word rather than a default, so no start log and no commit refusal fires.
func TestUnmatchedEvolutionWritesBypassIntoAnOldFile(t *testing.T) {
	for _, stamp := range []string{"26.09.21", unmatchedDiscardDefaultDay} {
		cfg, applied := evolveUnmatched(t, unmatchedTree(""), stamp)
		if len(applied) != 1 || applied[0] != unmatchedEvolutionName {
			t.Fatalf("stamp %s: applied %v, want [%s]", stamp, applied, unmatchedEvolutionName)
		}
		if cfg.Unmatched != dataplane.SPActionBypass {
			t.Fatalf("stamp %s: unmatched read as %d, want bypass (%d)", stamp, cfg.Unmatched, dataplane.SPActionBypass)
		}
		if cfg.UnmatchedByDefault {
			t.Fatalf("stamp %s: the evolved `unmatched bypass` is still marked as the default", stamp)
		}
	}
}

// TestUnmatchedEvolutionLeavesANewFileOnTheDefault checks the files the evolution
// must not touch: one stamped on or after the release that carries the evolution,
// one with no stamp (hand-written, or piped on stdin), and one a dev build stamped.
// None of them proves it was written under the bypass default, so each keeps the
// RFC 4301 Section 4.4.1 discard default.
func TestUnmatchedEvolutionLeavesANewFileOnTheDefault(t *testing.T) {
	for _, stamp := range []string{unmatchedBypassEvolutionRelease, "26.11.01", "", "dev"} {
		cfg, applied := evolveUnmatched(t, unmatchedTree(""), stamp)
		if len(applied) != 0 {
			t.Fatalf("stamp %q: applied %v, want none", stamp, applied)
		}
		if cfg.Unmatched != dataplane.SPActionDiscard {
			t.Fatalf("stamp %q: unmatched read as %d, want discard (%d)", stamp, cfg.Unmatched, dataplane.SPActionDiscard)
		}
		if !cfg.UnmatchedByDefault {
			t.Fatalf("stamp %q: an unset `unmatched` was not marked as the default", stamp)
		}
	}
}

// TestUnmatchedEvolutionLeavesAWrittenLeafAlone checks that an old file whose
// `unmatched` leaf is written keeps that word: the operator decided, so the
// evolution has nothing to restore.
func TestUnmatchedEvolutionLeavesAWrittenLeafAlone(t *testing.T) {
	cfg, applied := evolveUnmatched(t, unmatchedTree("discard"), "26.09.21")
	if len(applied) != 0 {
		t.Fatalf("applied %v to a written `unmatched discard`, want none", applied)
	}
	if cfg.Unmatched != dataplane.SPActionDiscard {
		t.Fatalf("written discard read as %d, want discard (%d)", cfg.Unmatched, dataplane.SPActionDiscard)
	}
}

// TestUnmatchedEvolutionLeavesABoxWithoutIPsecAlone checks that an old file with no
// `vpn ipsec` block gains none: it configured no IPsec boundary, and EmptyConfig
// already answers bypass for it.
func TestUnmatchedEvolutionLeavesABoxWithoutIPsecAlone(t *testing.T) {
	tree := config.NewTree()
	tree.GetOrCreateContainer("vpn")
	result, err := migration.Evolve(tree, "26.09.21")
	if err != nil {
		t.Fatalf("Evolve: %v", err)
	}
	if result != nil {
		t.Fatalf("applied %v to a file with no `vpn ipsec`, want none", result.Applied)
	}
}
