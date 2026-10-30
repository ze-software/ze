// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
// Related: rr.go -- peer down drops the transaction; forwards pass the forward gate
// Related: departure_wait.go -- the departures each phase waits for
// Related: validation.go -- the revalidation relays each phase waits for
// Related: withdrawal.go -- the per-source route inventory the pins name

package rr

import (
	"context"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/sourcepolicy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sourcePolicyOwner opens every refusal bgp-rr's ledger returns.
const sourcePolicyOwner sourcepolicy.Owner = "bgp-rr"

// sourcePolicyTxn is one source's transaction and its undo pins. The pins are
// the withdrawal-map entries the source held at prepare, by route key: a pin
// restores only while the map still holds an equal entry, so a route the
// source withdrew, or that a later UPDATE replaced, is never restored.
// Guarded by routeReflector.withdrawalMu.
type sourcePolicyTxn struct {
	ledger sourcepolicy.Ledger
	pins   map[string]withdrawalInfo
}

// ledgerOf is the transaction's ledger, nil when the source holds none.
func (txn *sourcePolicyTxn) ledgerOf() *sourcepolicy.Ledger {
	if txn == nil {
		return nil
	}
	return &txn.ledger
}

// handleSourcePolicy is bgp-rr's source-policy lifecycle handler. Source.Peer
// is matched as the peer address the received-UPDATE events carry, which keys
// the withdrawal map. It runs on the SDK source-policy worker, concurrently
// with event delivery: each phase changes state under withdrawalMu and waits
// for the reflector's work with no lock held.
//
// What bgp-rr owns of a source is its forwarding work and its withdrawal
// inventory; it holds no attributes and runs no election, and it writes
// nothing downstream for a phase. A phase settles that work before it
// answers (settleSource): every departure queued for the source, every
// UPDATE being reflected, and every revalidation relay the receive store
// (bgp-adj-rib-in) queued when the same phase changed the path's eligibility
// there. Withdrawing a reflected copy at each client, electing what replaces
// it, and the restamped re-announcement an abort makes belong to bgp-rib,
// which bgp-rr depends on: its sent inventory holds every copy bgp-rr
// reflected (rib/source_policy.go). Safe for concurrent use.
func (rr *routeReflector) handleSourcePolicy(ctx context.Context, input rpc.SourcePolicyInput) error {
	return sourcePolicyOwner.Handle(input, sourcepolicy.Phases{
		Prepare: func(_ netip.Addr, input rpc.SourcePolicyInput) error {
			return rr.prepareSourcePolicy(ctx, input)
		},
		Apply: func(_ netip.Addr, input rpc.SourcePolicyInput) error {
			return rr.applySourcePolicy(ctx, input)
		},
		Abort: func(_ netip.Addr, input rpc.SourcePolicyInput) error {
			return rr.abortSourcePolicy(ctx, input)
		},
		Finalize: func(_ netip.Addr, input rpc.SourcePolicyInput) error {
			return rr.finalizeSourcePolicy(input)
		},
	})
}

// prepareSourcePolicy validates the captured identity, settles the
// reflector's work for the source, then pins the routes its withdrawal map
// holds. It refuses a source whose session is down, a token older than a
// route this reflector already holds, a source another open transaction
// holds, and a request whose deadline passed while it settled, before pinning
// anything. Event application before the request is
// the coordinator's to drain (sdk.OnSourcePolicy).
func (rr *routeReflector) prepareSourcePolicy(ctx context.Context, input rpc.SourcePolicyInput) error {
	source := input.Source.Peer
	rr.withdrawalMu.Lock()
	done, err := sourcePolicyOwner.Reprepare(rr.sourcePolicy[source].ledgerOf(), input)
	rr.withdrawalMu.Unlock()
	if done {
		return err
	}
	if !rr.peerUp(source) {
		return errors.New("bgp-rr: source " + source + " has no established session to prepare")
	}
	if err := rr.settleSource(ctx, source); err != nil {
		return err
	}
	// The coordinator stops waiting at the request's deadline, and a settle
	// that finished at it must not pin: the coordinator has already sent
	// this transaction's abort or moved on, and a pin recorded now would hold
	// the source against every later transaction.
	if err := ctx.Err(); err != nil {
		return errors.New("bgp-rr: transaction " + input.TransactionID + " for source " + source +
			" reached its deadline while settling, so nothing was pinned: " + err.Error())
	}

	rr.withdrawalMu.Lock()
	txn := &sourcePolicyTxn{ledger: sourcepolicy.NewLedger(input), pins: make(map[string]withdrawalInfo)}
	for key, info := range rr.withdrawals[source] {
		if info.SourcePolicyEpoch > input.Source.PolicyEpoch {
			rr.withdrawalMu.Unlock()
			return errors.New("bgp-rr: transaction " + input.TransactionID + " captured policy epoch of source " +
				source + " is older than a route already reflected; capture the source again")
		}
		txn.pins[key] = info
	}
	if rr.sourcePolicy == nil {
		rr.sourcePolicy = make(map[string]*sourcePolicyTxn)
	}
	rr.sourcePolicy[source] = txn
	err = txn.ledger.Record(input, sourcepolicy.StagePrepared, nil)
	rr.withdrawalMu.Unlock()

	// A down that ran between the check above and the install found no
	// transaction to drop; this one would outlive its session.
	if !rr.peerUp(source) {
		rr.withdrawalMu.Lock()
		if rr.sourcePolicy[source] == txn {
			delete(rr.sourcePolicy, source)
		}
		rr.withdrawalMu.Unlock()
		return errors.New("bgp-rr: source " + source + " went down while its transaction was prepared")
	}
	return err
}

// peerUp reports whether the reflector last saw peer's session up.
func (rr *routeReflector) peerUp(peer string) bool {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	state := rr.peers[peer]
	return state != nil && state.Up
}

// applySourcePolicy records the denial of every route the source held at or
// below the captured epoch, which the ledger keeps as the captured identity,
// keeping the routes in the withdrawal map for abort, then settles the
// reflector's work,
// so each revalidation relay the receive store queued when it made those
// routes ineligible has reached the engine before the receipt. Routes the
// source's session admits later carry a newer epoch and are untouched, and so
// is every other source.
//
// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an
// EBGP peer SHALL NOT be considered eligible in the Decision Process if no
// explicit Import Policy has been applied."
func (rr *routeReflector) applySourcePolicy(ctx context.Context, input rpc.SourcePolicyInput) error {
	source := input.Source.Peer
	rr.withdrawalMu.Lock()
	txn := rr.sourcePolicy[source]
	duplicate, err := sourcePolicyOwner.Admit(txn.ledgerOf(), input)
	if err != nil {
		rr.withdrawalMu.Unlock()
		return err
	}
	if duplicate {
		rr.withdrawalMu.Unlock()
		return nil
	}
	rr.withdrawalMu.Unlock()

	// RFC 8212 Section 3: the denied routes' relays reach the engine before the receipt.
	settled := rr.settleSource(ctx, source)
	rr.withdrawalMu.Lock()
	defer rr.withdrawalMu.Unlock()
	return txn.ledger.Record(input, sourcepolicy.StageApplied, settled)
}

// abortSourcePolicy releases the pins of a transaction that was never applied
// (target zero) and changes nothing else. With a target it restamps each
// pinned route the withdrawal map still holds unchanged to the abort's fresh
// epoch, and after an apply it also drops every other denied route rather than
// keeping it. It then
// settles the reflector's work, so every re-announcement relay the receive
// store queued for the restored routes has reached the engine before the
// receipt. It runs no import evaluation and asks for no Route Refresh.
//
// The abort reverses the denial apply made under:
//
// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an
// EBGP peer SHALL NOT be considered eligible in the Decision Process if no
// explicit Import Policy has been applied."
func (rr *routeReflector) abortSourcePolicy(ctx context.Context, input rpc.SourcePolicyInput) error {
	source := input.Source.Peer
	rr.withdrawalMu.Lock()
	txn := rr.sourcePolicy[source]
	duplicate, err := sourcePolicyOwner.Admit(txn.ledgerOf(), input)
	if err != nil {
		rr.withdrawalMu.Unlock()
		return err
	}
	if duplicate {
		rr.withdrawalMu.Unlock()
		return nil
	}
	if input.NextPolicyEpoch == 0 {
		// The ledger admits a zero target only before apply: nothing was
		// denied and the epoch did not move, so only the pins go.
		txn.pins = nil
		err = txn.ledger.Record(input, sourcepolicy.StageAborted, nil)
		rr.withdrawalMu.Unlock()
		return err
	}
	target, bound := input.NextPolicyEpoch, txn.ledger.Source().PolicyEpoch
	routes := rr.withdrawals[source]
	for key, pinned := range txn.pins {
		if current, held := routes[key]; held && current == pinned {
			current.SourcePolicyEpoch = target
			routes[key] = current
		}
	}
	if txn.ledger.Stage() == sourcepolicy.StageApplied {
		rr.dropDeniedLocked(source, bound)
	}
	txn.pins = nil
	rr.withdrawalMu.Unlock()

	// RFC 8212 Section 3: the restored routes' relays reach the engine before the receipt.
	settled := rr.settleSource(ctx, source)
	rr.withdrawalMu.Lock()
	defer rr.withdrawalMu.Unlock()
	return txn.ledger.Record(input, sourcepolicy.StageAborted, settled)
}

// finalizeSourcePolicy accepts an applied transaction. The ledger admits
// finalize only after apply (abort is the path after a prepare that was not
// applied). It removes the denied routes from the withdrawal map, one pass
// over the source's own entries, so no denied state is retained and a later
// departure owes none of them, and releases the pins.
// The work was settled at apply, so nothing is waited for or sent.
func (rr *routeReflector) finalizeSourcePolicy(input rpc.SourcePolicyInput) error {
	rr.withdrawalMu.Lock()
	defer rr.withdrawalMu.Unlock()
	txn := rr.sourcePolicy[input.Source.Peer]
	duplicate, err := sourcePolicyOwner.Admit(txn.ledgerOf(), input)
	if err != nil {
		return err
	}
	if duplicate {
		return nil
	}
	rr.dropDeniedLocked(input.Source.Peer, txn.ledger.Source().PolicyEpoch)
	txn.pins = nil
	return txn.ledger.Record(input, sourcepolicy.StageFinalized, nil)
}

// dropDeniedLocked removes the source's routes at or below bound from the
// withdrawal map. MUST hold withdrawalMu.
func (rr *routeReflector) dropDeniedLocked(source string, bound uint64) {
	routes := rr.withdrawals[source]
	for key, info := range routes {
		if info.SourcePolicyEpoch <= bound {
			delete(routes, key)
		}
	}
}

// settleSource returns once no departure of source is queued or running, no
// UPDATE is being reflected, and every revalidation the validation worker held
// when it was called has been relayed. It refuses, naming the source and the
// work, when ctx ends first. MUST NOT be called with withdrawalMu or mu held:
// the work it waits for takes both.
func (rr *routeReflector) settleSource(ctx context.Context, source string) error {
	if err := rr.awaitDepartures(ctx, source); err != nil {
		return err
	}
	if err := rr.forwards.wait(ctx); err != nil {
		return errors.New("bgp-rr: source " + source + ": an UPDATE being reflected did not reach the engine " +
			"before the source-policy deadline: " + err.Error())
	}
	if err := rr.settleValidation(ctx); err != nil {
		return errors.New("bgp-rr: source " + source + ": " + err.Error())
	}
	return nil
}

// dropSourcePolicyLocked is the ordinary cleanup when the source's session
// ends: its transaction goes with its routes, and a later phase
// for that transaction is refused rather than applied to a successor session.
// Nothing is restored locally: the coordinator's later abort is refused and it
// reports the compensation incomplete (fail-closed). MUST hold withdrawalMu.
func (rr *routeReflector) dropSourcePolicyLocked(source string) {
	delete(rr.sourcePolicy, source)
}
