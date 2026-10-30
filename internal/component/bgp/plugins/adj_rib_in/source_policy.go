// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
// Related: rib_eligibility.go -- routeEligible applies the denial bound
// Related: rib.go -- replay skips denied routes; peer down drops the transaction

package adj_rib_in

import (
	"context"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/sourcepolicy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sourcePolicyOwner opens every refusal bgp-adj-rib-in's ledger returns.
const sourcePolicyOwner sourcepolicy.Owner = "bgp-adj-rib-in"

// sourcePolicyTxn is one source's transaction and its undo pins. The pins are
// the RawRoute pointers the store held at prepare: no route is copied, and a
// pin restores only while the store still holds that same pointer, so a path
// the source withdrew, a path its session lost, and a path a later UPDATE
// replaced are never restored. previousBound is the denial bound the source
// held before apply raised it (zero for none), which an abort puts back.
// Guarded by AdjRIBInManager.mu.
type sourcePolicyTxn struct {
	ledger        sourcepolicy.Ledger
	previousBound uint64
	installed     map[compactRouteKey]*RawRoute
	pending       map[compactPendingKey]*RawRoute
}

// ledgerOf is the transaction's ledger, nil when the source holds none.
func (txn *sourcePolicyTxn) ledgerOf() *sourcepolicy.Ledger {
	if txn == nil {
		return nil
	}
	return &txn.ledger
}

// handleSourcePolicy is this owner's source-policy lifecycle handler. Source.Peer
// is matched as the peer address the received-UPDATE events carry, parsed to the
// netip.Addr that keys ribIn. It runs on the SDK source-policy worker
// concurrently with OnEvent, so every phase works under mu. No phase waits on
// anything: this store has no asynchronous worker, and its downstream is its
// own replay and eligibility answers, which read mu. The phase is therefore
// settled when the handler returns. Safe for concurrent use.
func (r *AdjRIBInManager) handleSourcePolicy(_ context.Context, input rpc.SourcePolicyInput) error {
	return sourcePolicyOwner.Handle(input, sourcepolicy.Phases{
		Prepare: r.prepareSourcePolicy, Apply: r.applySourcePolicy,
		Abort: r.abortSourcePolicy, Finalize: r.finalizeSourcePolicy,
	})
}

// prepareSourcePolicy validates the captured identity and pins the source's
// installed and pending routes. It refuses a source whose session is down, a
// token older than a route this store already holds, and a source another
// open transaction holds, before pinning anything.
func (r *AdjRIBInManager) prepareSourcePolicy(peer netip.Addr, input rpc.SourcePolicyInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if done, err := sourcePolicyOwner.Reprepare(r.sourcePolicy[peer].ledgerOf(), input); done {
		return err
	}
	if !r.peerUp[peer] {
		return errors.New("bgp-adj-rib-in: source " + input.Source.Peer + " has no established session to prepare")
	}
	txn := &sourcePolicyTxn{
		ledger:    sourcepolicy.NewLedger(input),
		installed: make(map[compactRouteKey]*RawRoute),
		pending:   make(map[compactPendingKey]*RawRoute),
	}
	stale := false
	if routes := r.ribIn[peer]; routes != nil {
		routes.Range(func(key compactRouteKey, _ uint64, route *RawRoute) bool {
			if route.SourcePolicyEpoch > input.Source.PolicyEpoch {
				stale = true
				return false
			}
			txn.installed[key] = route
			return true
		})
	}
	for key, pending := range r.pending {
		if pending.peerAddr != peer {
			continue
		}
		if pending.route.SourcePolicyEpoch > input.Source.PolicyEpoch {
			stale = true
		}
		txn.pending[key] = pending.route
	}
	if stale {
		return errors.New("bgp-adj-rib-in: transaction " + input.TransactionID + " captured policy epoch of source " +
			input.Source.Peer + " is older than a route already admitted; capture the source again")
	}
	r.setSourcePolicyLocked(peer, txn)
	return txn.ledger.Record(input, sourcepolicy.StagePrepared, nil)
}

// setSourcePolicyLocked installs txn as the source's transaction. MUST hold mu.
func (r *AdjRIBInManager) setSourcePolicyLocked(peer netip.Addr, txn *sourcePolicyTxn) {
	if r.sourcePolicy == nil {
		r.sourcePolicy = make(map[netip.Addr]*sourcePolicyTxn)
	}
	r.sourcePolicy[peer] = txn
}

// applySourcePolicy makes every route the source held at or below the
// captured epoch ineligible in this store, by recording the denial bound that
// routeEligible and replay read. Routes stay stored for abort; routes the
// source's session admits later carry a newer epoch and are untouched, and so
// is every other source.
//
// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an
// EBGP peer SHALL NOT be considered eligible in the Decision Process if no
// explicit Import Policy has been applied."
func (r *AdjRIBInManager) applySourcePolicy(peer netip.Addr, input rpc.SourcePolicyInput) error {
	r.mu.Lock()
	txn := r.sourcePolicy[peer]
	duplicate, err := sourcePolicyOwner.Admit(txn.ledgerOf(), input)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	if duplicate {
		r.mu.Unlock()
		return nil
	}
	if r.policyDenied == nil {
		r.policyDenied = make(map[netip.Addr]uint64)
	}
	txn.previousBound = r.policyDenied[peer]
	if bound := txn.ledger.Source().PolicyEpoch; bound > txn.previousBound {
		r.policyDenied[peer] = bound
	}
	r.noteSourcePolicyChangesLocked(peer, txn)
	err = txn.ledger.Record(input, sourcepolicy.StageApplied, nil)
	r.unlockValidation()
	return err
}

// abortSourcePolicy releases the pins of a transaction that was never applied
// (target zero) and changes nothing else. With a target it restores the
// pinned routes the store still holds as the same route, restamped to the abort's fresh epoch so the engine's writers
// accept them again. After an apply it drops any other denied route rather
// than keeping it, and puts back the denial bound the source held before.
// It runs no import evaluation and asks for no Route Refresh.
//
// RFC 8212 Section 3 (the decision this abort reverses).
func (r *AdjRIBInManager) abortSourcePolicy(peer netip.Addr, input rpc.SourcePolicyInput) error {
	r.mu.Lock()
	txn := r.sourcePolicy[peer]
	duplicate, err := sourcePolicyOwner.Admit(txn.ledgerOf(), input)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	if duplicate {
		r.mu.Unlock()
		return nil
	}
	if input.NextPolicyEpoch == 0 {
		// The ledger admits a zero target only before apply: no denial bound
		// was raised and the epoch did not move, so only the pins go.
		txn.installed, txn.pending = nil, nil
		err = txn.ledger.Record(input, sourcepolicy.StageAborted, nil)
		r.mu.Unlock()
		return err
	}
	bound := txn.ledger.Source().PolicyEpoch
	routes := r.ribIn[peer]
	if routes != nil {
		for key, pinned := range txn.installed {
			if current, held := routes.Get(key); held && current == pinned && current.SourcePolicyEpoch <= bound {
				current.SourcePolicyEpoch = input.NextPolicyEpoch
			}
		}
	}
	for key, pinned := range txn.pending {
		if current := r.pending[key]; current != nil && current.route == pinned && pinned.SourcePolicyEpoch <= bound {
			pinned.SourcePolicyEpoch = input.NextPolicyEpoch
		}
	}
	if txn.ledger.Stage() == sourcepolicy.StageApplied {
		r.dropDeniedLocked(peer, bound)
		r.restoreDenialLocked(peer, txn.previousBound)
	}
	r.noteSourcePolicyChangesLocked(peer, txn)
	txn.installed, txn.pending = nil, nil
	err = txn.ledger.Record(input, sourcepolicy.StageAborted, nil)
	r.unlockValidation()
	return err
}

// finalizeSourcePolicy accepts an applied transaction (abort is the path after
// a prepare that was not applied) and releases its pins. It removes the denied
// routes, so no denied state is retained for a later
// recovery; the denial bound stays, so a route still carrying the old epoch
// can never become eligible again.
func (r *AdjRIBInManager) finalizeSourcePolicy(peer netip.Addr, input rpc.SourcePolicyInput) error {
	r.mu.Lock()
	txn := r.sourcePolicy[peer]
	duplicate, err := sourcePolicyOwner.Admit(txn.ledgerOf(), input)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	if duplicate {
		r.mu.Unlock()
		return nil
	}
	// The ledger admits finalize only after apply.
	r.dropDeniedLocked(peer, txn.ledger.Source().PolicyEpoch)
	txn.installed, txn.pending = nil, nil
	err = txn.ledger.Record(input, sourcepolicy.StageFinalized, nil)
	r.unlockValidation()
	return err
}

// restoreDenialLocked puts back the source's bound from before an aborted
// apply: zero means the source held none. MUST hold mu.
func (r *AdjRIBInManager) restoreDenialLocked(peer netip.Addr, previous uint64) {
	if previous == 0 {
		delete(r.policyDenied, peer)
		return
	}
	r.policyDenied[peer] = previous
}

// dropDeniedLocked removes the source's installed and pending routes at or
// below bound. MUST hold mu and finish with unlockValidation.
func (r *AdjRIBInManager) dropDeniedLocked(peer netip.Addr, bound uint64) {
	if routes := r.ribIn[peer]; routes != nil {
		var denied []compactRouteKey
		routes.Range(func(key compactRouteKey, _ uint64, route *RawRoute) bool {
			if route.SourcePolicyEpoch <= bound {
				denied = append(denied, key)
			}
			return true
		})
		for _, key := range denied {
			r.removeInstalled(peer, key)
		}
	}
	for key, pending := range r.pending {
		if pending.peerAddr == peer && pending.route.SourcePolicyEpoch <= bound {
			delete(r.pending, key)
		}
	}
}

// noteSourcePolicyChangesLocked tells selection which keys changed
// eligibility. MUST hold mu and finish with unlockValidation.
func (r *AdjRIBInManager) noteSourcePolicyChangesLocked(peer netip.Addr, txn *sourcePolicyTxn) {
	for key := range txn.installed {
		r.noteValidationChange(peer, key)
	}
}

// sourcePolicyDeniedLocked reports whether a route of peer with epoch is
// excluded by an applied source-policy denial. MUST hold mu (read or write).
//
// RFC 8212 Section 3.
func (r *AdjRIBInManager) sourcePolicyDeniedLocked(peer netip.Addr, epoch uint64) bool {
	bound, denied := r.policyDenied[peer]
	return denied && epoch <= bound
}

// dropSourcePolicyLocked is the ordinary cleanup when the source's session
// ends: its transaction and denial bound go with its routes, and a later phase
// for that transaction is refused rather than applied to a successor session.
// Nothing is restored locally: the coordinator's later abort is refused and it
// reports the compensation incomplete (fail-closed). MUST hold mu.
func (r *AdjRIBInManager) dropSourcePolicyLocked(peer netip.Addr) {
	delete(r.sourcePolicy, peer)
	delete(r.policyDenied, peer)
}
