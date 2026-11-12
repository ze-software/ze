// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Related: ../../firewall/commit_observer.go -- CommitObserver, run around every Backend.Apply

package engine

import (
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// init registers holdESPFormFastPathForCommit with the firewall, so every
// reconcile Ze itself commits holds the ESP form fast path off while it lands.
//
//nolint:gochecknoinits // registration: the observer is declared from init, before any ApplyAll
func init() {
	firewall.RegisterCommitObserver(holdESPFormFastPathForCommit)
}

// holdESPFormFastPathForCommit is the firewall commit observer of the ESP form
// fast path. It takes the dataplane's hold just before Backend.Apply and answers
// the release, which the firewall calls once Apply has returned, on success and
// on failure alike.
//
// Why it is needed: the fast path rewrites a peer's bare ESP at tc ingress, before
// netfilter, so it may run only while no input-path rule could drop that ESP. It
// learns of a ruleset change from the nftables monitor, after the commit has
// landed. A rule Ze's own commit adds would otherwise be bypassed for the time the
// monitor takes to deliver the commit and the fast path takes to re-read the
// ruleset. The hold turns the fast path off before the commit, and it is turned
// back on only by an evaluation that started after the release.
//
// The forked-engine limit: the observer acts only in the process that holds the
// dataplane, which is the one runEngine loaded it in (dataplane.Load). With the
// engine forked (ze plugin ike, internal/component/plugin/cli), this init runs in
// the daemon too, the same binary, but there dataplane.Get() answers nil, or,
// when OSPF loaded the XFRM backend for RFC 4552, a backend with no dual-form
// receiver, whose release is a no-op: the daemon's reconciles, the operator's
// firewall and every other owner's, take no hold. Only the monitor then covers
// them, with the window it carries. In the forked process the hold covers only
// the reconciles that process commits itself (the tunnel-source guard).
//
// It never calls into the firewall package: it runs inside reconcileMu, and a
// call back would deadlock (firewall.CommitObserver).
func holdESPFormFastPathForCommit() func(error) {
	holder, ok := dataplane.Get().(dataplane.ESPFormFirewallHolder)
	// refuse-by-default: guards firewall.CommitObserver -- a nil after is the observer's "nothing taken" answer: no dataplane loaded in this process, or a backend (VPP, a test fake) with no fast path to hold off
	if !ok {
		return nil
	}
	release := holder.HoldESPFormFastPath()
	// refuse-by-default: guards firewall.CommitObserver -- a holder that took nothing answers no release, and a nil after asks the firewall to call nothing
	if release == nil {
		return nil
	}
	return func(error) { release() }
}
