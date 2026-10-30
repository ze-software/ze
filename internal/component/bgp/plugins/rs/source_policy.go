// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
// Related: server_handlers.go -- peer down drops the transaction; enqueueDeparture is drained at prepare
// Related: server_withdrawal.go -- processForward runs the settle marker
// Related: server_validation.go -- the receive store's eligibility relays the settle covers

package rs

import (
	"context"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/sourcepolicy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sourcePolicyOwner opens every refusal bgp-rs's ledger returns.
const sourcePolicyOwner sourcepolicy.Owner = "bgp-rs"

// sourcePolicyTxn is one source's transaction and its undo pins. The pins are
// the withdrawal-map keys the source held at prepare: no route is copied, and
// a pin acts only while the map still holds the key at or below the captured
// epoch, so a key the source withdrew, a key its session lost, and a key a
// later UPDATE re-announced under a newer epoch are never restamped. Guarded
// by routeServer.withdrawalMu.
type sourcePolicyTxn struct {
	ledger sourcepolicy.Ledger
	pins   map[withdrawalKey]struct{}
}

// ledgerOf is the transaction's ledger, nil when the source holds none.
func (txn *sourcePolicyTxn) ledgerOf() *sourcepolicy.Ledger {
	if txn == nil {
		return nil
	}
	return &txn.ledger
}

// handleSourcePolicy is bgp-rs's source-policy lifecycle handler. Source.Peer
// is matched as the peer address the received-UPDATE events carry, in the
// netip.Addr string form that keys the withdrawal map and the source workers.
// It runs on the SDK source-policy worker, concurrently with event delivery:
// each phase changes the withdrawal map under withdrawalMu and waits for the
// source's worker with no lock held.
//
// What bgp-rs owns of a source is its forwarding work and its withdrawal
// inventory; it holds no attributes and runs no election. A phase settles
// that work before it answers: every forward queued for the source has been
// handed to the engine. A relay of the receive store's (bgp-adj-rib-in)
// eligibility change is covered too, because the coordinator calls the
// captured owners in their dependency order, bgp-rs after bgp-adj-rib-in and
// bgp-rib, and drains the validation-change subscribers after each owner's
// call: the relay is queued on the source's worker before this phase starts,
// and this phase's settle covers it. The election that replaces or withdraws
// a forwarded copy at each destination, and the restamped re-announcement an
// abort makes, belong to bgp-rib, which bgp-rs depends on: its sent inventory
// holds every copy bgp-rs forwarded (rib/source_policy.go). Safe for
// concurrent use.
func (rs *routeServer) handleSourcePolicy(ctx context.Context, input rpc.SourcePolicyInput) error {
	return sourcePolicyOwner.Handle(input, sourcepolicy.Phases{
		Prepare: func(peer netip.Addr, input rpc.SourcePolicyInput) error {
			return rs.prepareSourcePolicy(ctx, peer.String(), input)
		},
		Apply: func(peer netip.Addr, input rpc.SourcePolicyInput) error {
			return rs.applySourcePolicy(ctx, peer.String(), input)
		},
		Abort: func(peer netip.Addr, input rpc.SourcePolicyInput) error {
			return rs.abortSourcePolicy(ctx, peer.String(), input)
		},
		Finalize: func(peer netip.Addr, input rpc.SourcePolicyInput) error {
			return rs.finalizeSourcePolicy(peer.String(), input)
		},
	})
}

// prepareSourcePolicy validates the captured identity, drains the source's
// in-flight work (a departure still queued for it and every item on its
// worker), then pins the keys its withdrawal map holds. It refuses a source
// whose session is down, a token older than a route already forwarded, a
// source another open transaction holds, and a request whose deadline passed
// while it settled, before pinning anything.
func (rs *routeServer) prepareSourcePolicy(ctx context.Context, source string, input rpc.SourcePolicyInput) error {
	rs.withdrawalMu.Lock()
	done, err := sourcePolicyOwner.Reprepare(rs.sourcePolicy[source].ledgerOf(), input)
	rs.withdrawalMu.Unlock()
	if done {
		return err
	}
	if !rs.sourceUp(source) {
		return errors.New("bgp-rs: source " + source + " has no established session to prepare")
	}
	if err := rs.settleSource(ctx, source); err != nil {
		return errors.New("bgp-rs: source " + source + ": its queued forwarding work did not finish before prepare: " + err.Error())
	}
	// The coordinator stops waiting at the request's deadline, and a settle
	// that finished at it must not pin: the coordinator has already sent
	// this transaction's abort or moved on, and a pin recorded now would hold
	// the source against every later transaction.
	if err := ctx.Err(); err != nil {
		return errors.New("bgp-rs: transaction " + input.TransactionID + " for source " + source +
			" reached its deadline while settling, so nothing was pinned: " + err.Error())
	}

	rs.withdrawalMu.Lock()
	txn := &sourcePolicyTxn{ledger: sourcepolicy.NewLedger(input), pins: make(map[withdrawalKey]struct{})}
	for key, entry := range rs.withdrawals[source] {
		if entry.sourcePolicyEpoch > input.Source.PolicyEpoch {
			rs.withdrawalMu.Unlock()
			return errors.New("bgp-rs: transaction " + input.TransactionID + " captured policy epoch of source " +
				source + " is older than a route already forwarded; capture the source again")
		}
		txn.pins[key] = struct{}{}
	}
	if rs.sourcePolicy == nil {
		rs.sourcePolicy = make(map[string]*sourcePolicyTxn)
	}
	rs.sourcePolicy[source] = txn
	err = txn.ledger.Record(input, sourcepolicy.StagePrepared, nil)
	rs.withdrawalMu.Unlock()

	// A down that ran between the check above and the install found no
	// transaction to drop; this one would outlive its session.
	if !rs.sourceUp(source) {
		rs.withdrawalMu.Lock()
		if rs.sourcePolicy[source] == txn {
			delete(rs.sourcePolicy, source)
		}
		rs.withdrawalMu.Unlock()
		return errors.New("bgp-rs: source " + source + " went down while its transaction was prepared")
	}
	return err
}

// applySourcePolicy records the exclusion of every route the source held at
// or below the captured epoch, then settles the source's worker, so every
// forward queued for the source, and each withdrawal relay the receive store
// queued before this phase started (the coordinator's dependency order and
// validation-change drain), has reached the engine before the receipt. The
// inventory keeps the keys, so abort can restamp them. Forwards the source
// admits later carry a newer epoch and are untouched, and so is every other
// source.
//
// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an
// EBGP peer SHALL NOT be considered eligible in the Decision Process if no
// explicit Import Policy has been applied."
func (rs *routeServer) applySourcePolicy(ctx context.Context, source string, input rpc.SourcePolicyInput) error {
	rs.withdrawalMu.Lock()
	txn := rs.sourcePolicy[source]
	duplicate, err := sourcePolicyOwner.Admit(txn.ledgerOf(), input)
	rs.withdrawalMu.Unlock()
	if err != nil {
		return err
	}
	if duplicate {
		return nil
	}
	settled := rs.settleSource(ctx, source)
	if settled != nil {
		settled = errors.New("bgp-rs: source " + source + ": the relays of its excluded routes did not reach the engine: " + settled.Error())
	}
	rs.withdrawalMu.Lock()
	defer rs.withdrawalMu.Unlock()
	return txn.ledger.Record(input, sourcepolicy.StageApplied, settled)
}

// abortSourcePolicy releases the pins of a transaction that was never applied
// (target zero) and changes nothing else. With a target it restamps each
// pinned key the inventory still holds at or below the captured epoch to the
// abort's fresh epoch, releases the pins, and settles the source's worker so
// every re-announcement relay the receive store queued for the restored
// routes has reached the engine before the receipt.
//
// The abort reverses the denial apply made under:
//
// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an
// EBGP peer SHALL NOT be considered eligible in the Decision Process if no
// explicit Import Policy has been applied."
func (rs *routeServer) abortSourcePolicy(ctx context.Context, source string, input rpc.SourcePolicyInput) error {
	rs.withdrawalMu.Lock()
	txn := rs.sourcePolicy[source]
	duplicate, err := sourcePolicyOwner.Admit(txn.ledgerOf(), input)
	if err != nil {
		rs.withdrawalMu.Unlock()
		return err
	}
	if duplicate {
		rs.withdrawalMu.Unlock()
		return nil
	}
	if input.NextPolicyEpoch == 0 {
		// The ledger admits a zero target only before apply: nothing was
		// excluded and the epoch did not move, so only the pins go.
		txn.pins = nil
		err = txn.ledger.Record(input, sourcepolicy.StageAborted, nil)
		rs.withdrawalMu.Unlock()
		return err
	}
	bound := txn.ledger.Source().PolicyEpoch
	entries := rs.withdrawals[source]
	for key := range txn.pins {
		entry, held := entries[key]
		if !held {
			continue
		}
		if entry.sourcePolicyEpoch > bound {
			continue
		}
		entry.sourcePolicyEpoch = input.NextPolicyEpoch
		entries[key] = entry
	}
	txn.pins = nil
	rs.withdrawalMu.Unlock()

	settled := rs.settleSource(ctx, source)
	if settled != nil {
		settled = errors.New("bgp-rs: source " + source + ": the relays of its restored routes did not reach the engine: " + settled.Error())
	}
	rs.withdrawalMu.Lock()
	defer rs.withdrawalMu.Unlock()
	return txn.ledger.Record(input, sourcepolicy.StageAborted, settled)
}

// finalizeSourcePolicy accepts an applied transaction (the ledger admits it
// only after apply) and removes every key the source holds at or below the
// captured epoch from the withdrawal map, so no denied state is kept for a
// later recovery and a later departure owes none of them. The routes were
// already excluded and their copies settled at apply, so nothing is sent.
func (rs *routeServer) finalizeSourcePolicy(source string, input rpc.SourcePolicyInput) error {
	rs.withdrawalMu.Lock()
	defer rs.withdrawalMu.Unlock()
	txn := rs.sourcePolicy[source]
	duplicate, err := sourcePolicyOwner.Admit(txn.ledgerOf(), input)
	if err != nil {
		return err
	}
	if duplicate {
		return nil
	}
	bound := txn.ledger.Source().PolicyEpoch
	entries := rs.withdrawals[source]
	for key, entry := range entries {
		if entry.sourcePolicyEpoch <= bound {
			delete(entries, key)
		}
	}
	txn.pins = nil
	return txn.ledger.Record(input, sourcepolicy.StageFinalized, nil)
}

// sourceUp reports whether the source's session is up as this plugin last
// saw it.
func (rs *routeServer) sourceUp(source string) bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	peer := rs.peers[source]
	return peer != nil && peer.Up
}

// settleSource returns once a departure still queued for source has been sent
// and every item queued on its worker before the call has been handled,
// including the worker's forward batch. It returns ctx's error when ctx ends
// first. MUST NOT be called with withdrawalMu or mu held: the worker takes
// both.
func (rs *routeServer) settleSource(ctx context.Context, source string) error {
	if err := rs.awaitDeparture(ctx, source); err != nil {
		return errors.New("its departure was still being sent: " + err.Error())
	}
	barrier := make(chan struct{})
	if !rs.workers.Dispatch(workerKey{sourcePeer: source}, workItem{sourcePeer: source, barrier: barrier}) {
		return errors.New("its worker is closing (the session went down) or the plugin is stopping")
	}
	select {
	case <-barrier:
		return nil
	case <-ctx.Done():
		return errors.New("its worker did not finish its queue: " + ctx.Err().Error())
	}
}

// awaitDeparture waits for a departure queued for source to be sent. With no
// departure ever queued there is nothing to wait for.
func (rs *routeServer) awaitDeparture(ctx context.Context, source string) error {
	rs.departureMu.Lock()
	departures := rs.departures
	rs.departureMu.Unlock()
	// refuse-by-default: guards awaitDeparture -- the workers start with the first departure ever queued, so with none there is no departure to wait for
	if departures == nil {
		return nil
	}
	return departures.Await(ctx, source)
}
