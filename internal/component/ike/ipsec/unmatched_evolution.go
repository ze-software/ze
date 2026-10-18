// Design: docs/guide/ipsec.md -- the unmatched catch-all, and what an upgrade keeps
// Related: register.go -- registers the evolution with the config migration framework
// Related: config.go -- parseUnmatched, where an unset leaf becomes the discard default
// Related: ../../config/migration/evolution.go -- Evolution, StampedOnly, Evolve

package ipsec

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/migration"
)

// unmatchedDiscardDefaultDay is the release day whose source changed the default
// of `vpn ipsec unmatched` from bypass to discard (commit 79aaa4fe34, 2026-10-18).
// A release is the build date, so a binary built that day may hold either default.
const unmatchedDiscardDefaultDay = "26.10.18"

// unmatchedBypassEvolutionRelease is the day after unmatchedDiscardDefaultDay. The
// evolution runs on a file stamped by an older release, so every file stamped on
// or before the day the default changed is read as written under bypass: an
// upgraded router keeps the behavior it ran with, which is the owner's decision of
// 2026-10-18. Its cost is a file a same-day build wrote with the leaf unset while
// the discard was already in force: it is read as old and gains `unmatched bypass`.
const unmatchedBypassEvolutionRelease = "26.10.19"

const unmatchedEvolutionName = "ipsec-unmatched-keeps-bypass"

// unmatchedBypassNotice is what the hub tells the operator, at the default log
// level, when the evolution applies: the evolution keeps unprotected traffic
// flowing in the clear where the RFC 4301 default would discard it, which is a
// security decision the operator never made in this file.
const unmatchedBypassNotice = "wrote `vpn ipsec unmatched bypass`: the config was saved by release " +
	unmatchedDiscardDefaultDay + " or earlier, when an unset `unmatched` meant bypass, so traffic no " +
	"IPsec policy matches keeps flowing in the clear as before; to discard it as RFC 4301 " +
	"Section 4.4.1 requires, first add a `bypass` policy for each flow that must stay in the clear " +
	"(BGP, SSH, ICMP: see the IPsec guide), then run `set vpn ipsec unmatched discard` and commit"

// unmatchedBypassEvolution writes `unmatched bypass` into a `vpn ipsec` block that
// leaves the leaf unset, in a file stamped before the default became discard. That
// file ran with a bypass catch-all (the default before 2026-10-18, and before
// 2026-09-21 no catch-all at all, which the kernel answers the same way), so
// writing the word keeps its behavior and turns the old default into a decision
// the file states. StampedOnly, because an unstamped file proves no release.
func unmatchedBypassEvolution() migration.Evolution {
	return migration.Evolution{
		Release:     unmatchedBypassEvolutionRelease,
		Name:        unmatchedEvolutionName,
		Description: "Write `vpn ipsec unmatched bypass` into a config saved while bypass was the default",
		Detect:      unmatchedLeftToDefault,
		Apply:       writeUnmatchedBypass,
		StampedOnly: true,
		Notice:      unmatchedBypassNotice,
	}
}

// unmatchedLeftToDefault answers whether tree holds a `vpn ipsec` block whose
// `unmatched` leaf is unset. A tree with no `vpn ipsec` configured no IPsec
// boundary, and EmptyConfig already answers bypass for it.
func unmatchedLeftToDefault(tree *config.Tree) bool {
	vpn := tree.GetContainer("vpn")
	if vpn == nil {
		return false
	}
	ipsec := vpn.GetContainer("ipsec")
	if ipsec == nil {
		return false
	}
	_, written := ipsec.Get("unmatched")
	return !written
}

// writeUnmatchedBypass sets `unmatched bypass` in tree, which Evolve hands over as
// its own working copy. It MUST be called only when unmatchedLeftToDefault is true.
func writeUnmatchedBypass(tree *config.Tree) (*config.Tree, error) {
	tree.GetContainer("vpn").GetContainer("ipsec").Set("unmatched", "bypass")
	return tree, nil
}
