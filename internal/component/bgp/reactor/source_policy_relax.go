// Design: docs/architecture/behavior/peer-lifecycle.md -- in-place import invalidation
// Overview: peer_settings_apply.go -- the journaled swap that triggers it
// Related: source_policy_reload.go -- the invalidation a relaxation's undo reuses
// Related: peer_derived_bindings.go -- requestRouteRefreshAll, the recovery it sends
// RFC: rfc/short/rfc8212.md -- the default an import relaxation lifts

package reactor

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"
)

// importDecisionRises reports the D-1 transition: an effective import decision
// that rejects before the swap and accepts after it. Like importDecisionFalls
// it judges what the swap actually publishes (copy applied to the running
// settings), never the raw final value, and it refuses to guess for a next
// whose import policy was never resolved (unresolved).
func (p *Peer) importDecisionRises(next *PeerSettings, copy settingsCopier, unresolved bool) (bool, error) {
	current := p.settingsSnapshot()
	// refuse-by-default: guards swapImportRising -- a peer with no published settings has no rejecting policy to lift
	if current == nil {
		return false, nil
	}
	candidate := *current
	copy(&candidate, next)
	external := current.IsEBGP()
	// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated
	// with an EBGP peer SHALL NOT be considered eligible in the Decision
	// Process if no explicit Import Policy has been applied." A decision
	// rises only when the current import policy rejects at its end and the
	// candidate accepts.
	if current.ImportPolicy.AcceptsAtEnd(external) {
		return false, nil
	}
	if !candidate.ImportPolicy.AcceptsAtEnd(external) {
		return false, nil
	}
	if unresolved {
		return false, fmt.Errorf("peer %s: the change was built from the peer's config leaves alone, which carry no "+
			"filter chain, so its effective import policy is unknown and nothing was changed: lifting a rejecting "+
			"import policy is never guessed (RFC 8212 Section 3). The BGP reactor has no config file to resolve the "+
			"candidate from (config path or reload function unset); start ze with a config file, or report the "+
			"commit's diff if it has one", p.addrString)
	}
	return true, nil
}

// refuseRelaxationWithoutRefresh is D-1's refusal: an Established session that
// did not negotiate Route Refresh cannot be asked again for the routes the
// rejecting policy discarded, so lifting the rejection live would leave them
// missing while the policy reports accept. The change is refused before
// anything is published, and the error names the hold-down sequence that
// applies it: a session established under the accepting policy receives every
// route again.
func (p *Peer) refuseRelaxationWithoutRefresh(session *Session) error {
	neg, err := p.sessionNegotiated(session)
	if err != nil {
		return fmt.Errorf("import policy change judged against no live negotiation, so nothing was changed: %w", err)
	}
	// RFC 2918 Section 4: "A BGP speaker may send a ROUTE-REFRESH message to
	// its peer only if it has received the Route Refresh Capability from its
	// peer."
	if neg.RouteRefresh {
		return nil
	}
	return fmt.Errorf("peer %s: this change makes its import policy accept routes that the default for a peer "+
		"with no explicit import policy rejected (RFC 8212 Section 3). The routes the peer already sent were "+
		"discarded, and the session did not negotiate Route Refresh (RFC 2918), so Ze cannot ask for them again "+
		"and nothing was changed. Apply it with the session down: `request peer %s disable`, wait until the "+
		"session is down, commit this change, then `request peer %s enable`",
		p.addrString, p.addrString, p.addrString)
}

// verifyImportRelaxation is D-1's verify-stage check for one configured peer:
// a swap whose import decision rises on an Established session without Route
// Refresh is refused before any plugin applies the transaction. A change the
// session cannot take restarts it, and the new session sends every route, so
// only a swap is judged. The apply judges again (swapImportRising): the
// session can change between the two.
func (a *reactorAPIAdapter) verifyImportRelaxation(next *PeerSettings, resolved bool) error {
	a.r.mu.RLock()
	peer := a.r.peers[next.PeerKey()]
	a.r.mu.RUnlock()
	// A peer the reload creates has no session and discarded no routes, so
	// its first session admits under the published policy, and the apply
	// judges any swap again, so verify has nothing to refuse here.
	// refuse-by-default: guards swapImportRising -- a created peer has no session; apply judges the swap again
	if peer == nil {
		return nil
	}
	current := peer.settingsSnapshot()
	// refuse-by-default: guards verifyRelaxationSwap -- a peer with no published settings has no rejecting policy to lift
	if current == nil {
		return nil
	}
	// A dynamic peer is judged against its group's template (verifyDynamicRelaxations).
	if current.IsDynamic {
		return nil
	}
	session := peer.currentSession()
	apply, reason := peerSettingsSwapPlan(current, next, session)
	if reason != "" {
		return nil
	}
	return peer.verifyRelaxationSwap(next, apply, !resolved, session)
}

// verifyRelaxationSwap judges one swap the verify stage found.
func (p *Peer) verifyRelaxationSwap(next *PeerSettings, apply settingsCopier, unresolved bool, session *Session) error {
	// RFC 8212 Section 3
	rises, err := p.importDecisionRises(next, apply, unresolved)
	if err != nil {
		return err
	}
	if !rises {
		return nil
	}
	if p.State() != PeerStateEstablished {
		return nil
	}
	return p.refuseRelaxationWithoutRefresh(session)
}

// verifyImportRelaxations runs verifyImportRelaxation over a whole candidate:
// every configured peer it names, and every running dynamic peer whose group
// template it changes.
func (a *reactorAPIAdapter) verifyImportRelaxations(loaded loadedConfig) error {
	var result error
	for _, next := range loaded.peers {
		result = errors.Join(result, a.verifyImportRelaxation(next, true))
	}
	return errors.Join(result, a.verifyDynamicRelaxations(loaded.groups))
}

// verifyDynamicRelaxations judges each running dynamic peer the way
// planDynamicGroupsLocked would swap it, comparing the templates built for its
// address, without changing anything.
func (a *reactorAPIAdapter) verifyDynamicRelaxations(groups dynamicGroupsLoad) error {
	if !groups.read {
		return nil
	}
	r := a.r
	nextByName := make(map[string]*DynamicGroupConfig, len(groups.groups))
	for _, g := range groups.groups {
		nextByName[g.GroupName] = g
	}
	type dynamicCheck struct {
		peer *Peer
		next *DynamicGroupConfig
		old  *DynamicGroupConfig
		addr netip.Addr
	}
	var checks []dynamicCheck
	r.mu.RLock()
	oldByName := make(map[string]*DynamicGroupConfig, len(r.dynamicGroups))
	for _, g := range r.dynamicGroups {
		oldByName[g.GroupName] = g
	}
	for _, peer := range r.peers {
		settings := peer.Settings()
		if !settings.IsDynamic {
			continue
		}
		next, kept := nextByName[settings.GroupName]
		if !kept {
			continue
		}
		checks = append(checks, dynamicCheck{peer: peer, next: next, old: oldByName[settings.GroupName], addr: settings.Address})
	}
	r.mu.RUnlock()
	var result error
	for _, check := range checks {
		oldBuilt, nextBuilt, reason := r.dynamicTemplatePair(check.old, check.next, check.addr)
		if reason != "" {
			continue
		}
		session := check.peer.currentSession()
		apply, reason := peerSettingsSwapPlan(oldBuilt, nextBuilt, session)
		if reason != "" {
			continue
		}
		result = errors.Join(result, check.peer.verifyRelaxationSwap(nextBuilt, apply, false, session))
	}
	return result
}

// swapImportRising delivers a swap that lifts the peer's rejecting import
// decision (D-1). A peer that is not Established admits nothing, so the policy
// is published under establishGate and its next session sends every route. An
// Established peer is judged again on the session it holds now: without Route
// Refresh the change is refused before anything is published. Otherwise the
// policy is published and one ROUTE-REFRESH per negotiated family goes out on
// that same session; a send that fails undoes the change and fails the apply.
//
// The undo is undoImportRelaxation, for a failed send and for the journal's
// rollback alike: it restores the rejecting policy and invalidates the paths
// the candidate admitted.
func (a *reactorAPIAdapter) swapImportRising(peer *Peer, next *PeerSettings, apply settingsCopier, previous *PeerSettings, j configJournal) error {
	var txID string
	var deadline time.Time
	if tx, ok := j.(sourcePolicyTransaction); ok {
		txID, deadline = tx.SourcePolicyTransaction()
	}
	undo := func() error { return a.undoImportRelaxation(peer, apply, previous, txID, deadline) }
	publish := func() error {
		if err := peer.applyHotSwappableSettings(next, apply); err != nil {
			return errors.Join(err, peer.restoreHotSwappableSettings(previous, apply))
		}
		return nil
	}

	peer.establishGate.Lock()
	if peer.State() != PeerStateEstablished {
		err := j.Record(publish, undo)
		peer.establishGate.Unlock()
		return err
	}
	peer.establishGate.Unlock()

	session := peer.currentSession()
	if err := peer.refuseRelaxationWithoutRefresh(session); err != nil {
		return err
	}
	if txID == "" {
		return importRiseNeedsTransaction(peer)
	}
	return j.Record(
		func() error {
			if err := publish(); err != nil {
				return err
			}
			// RFC 7313 Section 3.2: "0 - Normal route refresh request
			// [RFC2918]". requestRouteRefreshAll asks once per negotiated
			// family, on the session judged above.
			if err := peer.requestRouteRefreshAll(session); err != nil {
				return relaxationRefreshFailed(peer, err, undo())
			}
			return nil
		},
		undo,
	)
}

// importRiseNeedsTransaction is the refusal of a D-1 transition on an
// Established session outside a config transaction (swapImportRising,
// refuseImportSwapEarly).
func importRiseNeedsTransaction(peer *Peer) error {
	return fmt.Errorf("peer %s: this change makes its import policy accept routes the session's RFC 8212 "+
		"default rejected, which needs a config transaction operation to undo it in place if the commit fails; "+
		"it reached BGP on a path that carries none, so nothing was changed. Commit the import policy change "+
		"on its own from the config editor (`commit`). If that commit is refused with this message too, it is "+
		"a defect: report it with the commit's diff", peer.addrString)
}

// refuseImportSwapEarly judges one swap, before any irreversible step, the way
// swapPeerJournaled's apply would refuse it: a rise on an Established session
// that did not negotiate Route Refresh (D-1) or that belongs to no config
// transaction, and a fall on an Established session outside one (D-2). The
// dynamic step runs it while planning under r.mu, before a peer is removed or
// restarted, so a refused step leaves every peer, group and session as it
// was. The apply still judges again: the session can change in between.
func (p *Peer) refuseImportSwapEarly(next *PeerSettings, apply settingsCopier, transactional bool) error {
	// RFC 8212 Section 3
	falls, err := p.importDecisionFalls(next, apply, false)
	if err != nil {
		return err
	}
	// RFC 8212 Section 3
	rises, err := p.importDecisionRises(next, apply, false)
	if err != nil {
		return err
	}
	if p.State() != PeerStateEstablished {
		return nil
	}
	if falls {
		if transactional {
			return nil
		}
		return importFallNeedsTransaction(p)
	}
	if !rises {
		return nil
	}
	if err := p.refuseRelaxationWithoutRefresh(p.currentSession()); err != nil {
		return err
	}
	if transactional {
		return nil
	}
	return importRiseNeedsTransaction(p)
}

// relaxationRefreshFailed reports a relaxation whose ROUTE-REFRESH was not
// sent, worded from what its undo (undoErr) achieved.
func relaxationRefreshFailed(peer *Peer, err, undoErr error) error {
	if undoErr == nil {
		return fmt.Errorf("peer %s: the import policy accepted its routes, but asking the peer to send them "+
			"again failed, so the change was undone: %w", peer.addrString, err)
	}
	return errors.Join(fmt.Errorf("peer %s: the import policy accepted its routes, but asking the peer to send "+
		"them again failed, and undoing the change did not complete: %w", peer.addrString, err), undoErr)
}

// undoImportRelaxation restores the rejecting import policy a relaxation
// lifted, and removes from eligibility every path the candidate admitted: the
// "Failed import relaxation" boundary. It is the falling transition from the
// candidate back to previous, so it takes swapImportFalling's two routes: a
// peer that is not Established restores under establishGate and purges what
// Graceful Restart retained from a session that came and went; an Established
// peer takes the in-place invalidation, whose fresh epoch fences every path
// the candidate admitted and every late answer to the ROUTE-REFRESH. Its
// pause lasts until the transaction's receipt, which finalizes it
// (CompleteSourcePolicyTx, relaxationUndo), or until the hold expires at the
// receipt's completion deadline (holdUntilCompletion), which finalizes it too.
// An invalidation that failed after publishing the rejecting policy keeps it
// and the denial, and drives the exclusion to every owner apply missed
// (sourceInvalidation.fail, completeExclusion); one whose publish failed
// leaves the accepting policy in effect. Each is reported from what it
// achieved (relaxationUndoFailed), with the hold-down sequence wherever paths
// may stay eligible. A session that went down after the ROUTE-REFRESH
// failed has nothing to pause, so it is purged as a peer that is not
// Established is.
func (a *reactorAPIAdapter) undoImportRelaxation(peer *Peer, apply settingsCopier, previous *PeerSettings, txID string, deadline time.Time) error {
	restore := func() error { return peer.restoreHotSwappableSettings(previous, apply) }
	peer.establishGate.Lock()
	if peer.State() != PeerStateEstablished {
		err := restore()
		peer.establishGate.Unlock()
		if err != nil {
			return err
		}
		return a.r.purgeRetained(peer)
	}
	peer.establishGate.Unlock()
	if txID == "" {
		return errors.Join(fmt.Errorf("peer %s: the rejecting import policy is restored, but the session "+
			"established while the change ran and the change belongs to no config transaction, so the routes it "+
			"admitted stay eligible; run `request peer %s disable`, then `request peer %s enable`",
			peer.addrString, peer.addrString, peer.addrString), restore())
	}
	if deadline.IsZero() {
		deadline = a.r.clock.Now().Add(sourcePolicyTxBudget)
	}
	inv, err := a.r.prepareImportInvalidation(txID, deadline, peer)
	if err != nil {
		// A session that ended while prepare ran has nothing to pause. The
		// state is judged again under establishGate, as above, so a session
		// establishing from here on waits for the restored rejecting policy
		// (enterEstablished) instead of admitting under the candidate.
		peer.establishGate.Lock()
		if peer.State() != PeerStateEstablished {
			restoreErr := restore()
			peer.establishGate.Unlock()
			if restoreErr != nil {
				return errors.Join(err, restoreErr)
			}
			return a.r.purgeRetained(peer)
		}
		peer.establishGate.Unlock()
		return errors.Join(fmt.Errorf("peer %s: the rejecting import policy is restored, but the routes the "+
			"undone change admitted could not be withdrawn and stay eligible; run `request peer %s disable`, then "+
			"`request peer %s enable`: %w", peer.addrString, peer.addrString, peer.addrString, err), restore())
	}
	inv.relaxationUndo = true
	// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated
	// with an EBGP peer SHALL NOT be considered eligible in the Decision
	// Process if no explicit Import Policy has been applied." The restored
	// policy rejects at its end again, so the paths the candidate admitted
	// leave eligibility under a new epoch. A failed publish aborts with no
	// settings restore: re-publishing the candidate would not make the
	// receipt's report true.
	err = inv.apply(restore, nil)
	inv.holdUntilCompletion()
	if err == nil {
		return nil
	}
	return relaxationUndoFailed(peer, inv, err)
}

// unsettledClause names, inside the unexcluded report, the owners that
// withdrew the routes but may leave a copy advertised downstream; empty when
// there are none.
func unsettledClause(unsettled []string) string {
	if len(unsettled) == 0 {
		return ""
	}
	return " (source-policy owner " + strings.Join(unsettled, ", ") +
		" withdrew them, but a copy may stay advertised downstream)"
}

// relaxationUndoFailed words a relaxation undo whose invalidation failed from
// what it achieved. A failed publish aborted (aborted): the accepting policy
// stays in effect. Otherwise the rejecting policy is published and
// completeExclusion drove every owner apply missed: the owners it could not
// make apply (unexcluded) keep the paths eligible; the owners that refused
// to undo an apply they recorded with an error (unsettled) withdrew the
// paths but may leave a copy advertised downstream; when every owner applied
// and downstream settled, the undo is complete and the first failure is only
// logged; when only downstream did not settle, a copy may stay advertised.
func relaxationUndoFailed(peer *Peer, inv *sourceInvalidation, err error) error {
	label := peer.addrString
	if inv.aborted {
		return fmt.Errorf("peer %s: the rejecting import policy could not be restored, so the accepting policy stays "+
			"in effect and the routes it admitted stay eligible; run `request peer %s disable`, then `request peer "+
			"%s enable`: %w", label, label, label, err)
	}
	if len(inv.unexcluded) != 0 {
		return fmt.Errorf("peer %s: the rejecting import policy is restored, but source-policy owner %s did not "+
			"withdraw the routes the undone change admitted, so they stay eligible%s; run `request peer %s disable`, "+
			"then `request peer %s enable`: %w", label, strings.Join(inv.unexcluded, ", "),
			unsettledClause(inv.unsettled), label, label, err)
	}
	if len(inv.unsettled) != 0 {
		return fmt.Errorf("peer %s: the rejecting import policy is restored and source-policy owner %s withdrew the "+
			"routes the undone change admitted, but it reported an error before its change settled downstream, so a "+
			"copy may stay advertised; run `request peer %s disable`, then `request peer %s enable`: %w",
			label, strings.Join(inv.unsettled, ", "), label, label, err)
	}
	if inv.exclusionErr == nil {
		reactorLogger().Warn("import relaxation undo completed after a retry", "peer", label,
			"transaction", inv.txID, "error", err)
		return nil
	}
	return fmt.Errorf("peer %s: the rejecting import policy is restored and every source-policy owner withdrew the "+
		"routes the undone change admitted, but the change did not settle downstream, so a copy may stay advertised; "+
		"run `request peer %s disable`, then `request peer %s enable`: %w", label, label, label, err)
}
