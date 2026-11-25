// Design: docs/architecture/plugin/rib-storage-design.md -- applied reflector cleanup.
// Related: actor.go -- live source leases; replay_lifecycle.go -- actual replay completions.
package rr

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// cleanupSource seals matching admission without waiting for RPC or a worker.
// Every escaped stored page must subsequently find its exact current row.
// settleEvents MUST join old output before the emitter may purge source/SENT.
func (rr *routeReflector) cleanupSource(envelope rpc.EventEnvelope) error {
	cleanup, err := retention.DecodeCleanup(envelope)
	if err != nil {
		return err
	}
	source, err := newPeerIdentity(cleanup.Address().String(), cleanup.Owner(), cleanup.Connection())
	if err != nil {
		return err
	}
	rr.withdrawalMu.Lock()
	for key, info := range rr.withdrawals[source.address.String()] {
		if key.owner != source.owner {
			continue
		}
		if key.connection != source.connection {
			continue
		}
		if !cleanup.MatchesFamily(info.Family) {
			continue
		}
		if cleanup.NoLLGROnly() {
			if !info.NoLLGR {
				continue
			}
		}
		info.Closing = true
		rr.withdrawals[source.address.String()][key] = info
	}
	rr.oweCleanupLocked(source)
	rr.withdrawalMu.Unlock()
	return nil
}

// settleEvents is the combined applied barrier. The caller supplies the finite
// application deadline. A failure leaves every captured cleanup obligation
// pending, including a lease release not yet acknowledged by the engine.
func (rr *routeReflector) settleEvents(ctx context.Context) error {
	if _, bounded := ctx.Deadline(); !bounded {
		return errors.New("bgp-rr: applied settlement requires a finite deadline")
	}
	if err := joinReplays(ctx, rr.replays.snapshot()); err != nil {
		return err
	}
	if err := rr.forwards.wait(ctx); err != nil {
		return err
	}
	if err := rr.reconcileEndedOutput(ctx); err != nil {
		return err
	}
	if err := rr.reconcileOriginalDepartures(ctx); err != nil {
		return err
	}
	if err := rr.settleReplays(ctx); err != nil {
		return err
	}
	if err := rr.settleValidation(ctx); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := rr.unsettledWork(); err != nil {
			return err
		}
		rr.withdrawalMu.Lock()
		pending := make(map[peerIdentity]uint64, len(rr.cleanupPending))
		for source, version := range rr.cleanupPending {
			pending[source] = version
		}
		rr.withdrawalMu.Unlock()
		if len(pending) == 0 {
			return nil
		}
		for source, version := range pending {
			if err := rr.awaitDepartures(ctx, source.owner); err != nil {
				return err
			}
			if err := rr.unsettledWork(); err != nil {
				return err
			}
			rr.mu.RLock()
			state := rr.peers[source]
			rr.mu.RUnlock()
			if state != nil {
				state.leaseMu.Lock()
			}
			rr.withdrawalMu.Lock()
			if rr.cleanupPending[source] != version {
				rr.withdrawalMu.Unlock()
				if state != nil {
					state.leaseMu.Unlock()
				}
				continue
			}
			entries := rr.withdrawals[source.address.String()]
			for key, info := range entries {
				if key.owner != source.owner {
					continue
				}
				if key.connection != source.connection {
					continue
				}
				if info.Closing {
					rr.deleteWithdrawalLocked(source.address.String(), key)
				}
			}
			if len(entries) == 0 {
				delete(rr.withdrawals, source.address.String())
			}
			held := rr.sourceMemberLocked(source)
			delete(rr.cleanupPending, source)
			rr.withdrawalMu.Unlock()
			if state != nil {
				state.leaseMu.Unlock()
			}
			var err error
			if held {
				err = rr.retainSource(ctx, source)
			} else {
				err = rr.releaseEndedSource(ctx, source)
			}
			if err != nil {
				rr.oweCleanup(source)
				return err
			}
		}
	}
}

// noteForwardFailure records only an unknown ordinary forward outcome.
// Matched writer completion and a proven pre-dispatch refusal still report
// their failures to the event caller, but own no outstanding writer history.
// Required output MUST use noteWorkFailure instead; safe current-read replay
// failures remain in the original peer actor and are reconciled at settlement.
func (rr *routeReflector) noteForwardFailure(work string, err error, original ...peerIdentity) {
	if err == nil {
		return
	}
	var joined *rpc.CachedWriteJoinedError
	if errors.As(err, &joined) {
		return
	}
	var notDispatched *rpc.CachedWriteNotDispatchedError
	if errors.As(err, &notDispatched) {
		return
	}
	if len(original) > 0 {
		rr.noteSourceOutputFailure(original[0], work, err)
		return
	}
	rr.noteWorkFailure(work, err)
}

// noteWorkFailure retains one current unresolved obligation for this plugin
// lifetime, rather than an unbounded error history. A failed final request
// cannot be replayed blindly: cached IDs were consumed once, and an unsettled
// transport has no receipt proving its old writer ended. Reconcile/restart
// the role's process before acknowledging cleanup on a new connection.
func (rr *routeReflector) noteWorkFailure(work string, err error) {
	if err == nil {
		return
	}
	rr.workMu.Lock()
	defer rr.workMu.Unlock()
	if rr.workErr == nil {
		rr.workErr = errors.New("bgp-rr: " + work + " remains owed: " + err.Error() + "; reconcile and restart this reflector process before retrying cleanup")
	}
}

func (rr *routeReflector) unsettledWork() error {
	rr.workMu.Lock()
	failed := rr.workErr
	rr.workMu.Unlock()
	if failed != nil {
		return failed
	}
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	for _, state := range rr.peers {
		if state.sourceOutputDebt != nil {
			return state.sourceOutputDebt
		}
		if state.targetOutputDebt != nil {
			return state.targetOutputDebt
		}
	}
	return nil
}

// retainSource transitions only the lease this process already acquired
// while live. leaseCall serializes lease I/O without blocking finite DOWN.
func (rr *routeReflector) retainSource(ctx context.Context, source peerIdentity) error {
	rr.mu.RLock()
	state := rr.peers[source]
	rr.mu.RUnlock()
	if state == nil {
		return nil
	}
	state.leaseCall.Lock()
	defer state.leaseCall.Unlock()
	return rr.retainSourceLocked(ctx, source, state)
}

func (rr *routeReflector) retainSourceLocked(ctx context.Context, source peerIdentity, state *peerState) error {
	rr.mu.RLock()
	pending, leased, receipt := state.RetainPending, state.Leased, state.SourceReceipt
	rr.mu.RUnlock()
	if !pending {
		return nil
	}
	if !leased {
		return errors.New("bgp-rr: retained routes have no lease acquired while the original source was live")
	}
	if _, err := rr.plugin.SourceLease(ctx, rpc.SourceLeaseInput{Action: rpc.SourceLeaseRetain, PeerOwner: source.owner, SourceReceipt: receipt}); err != nil {
		return err
	}
	rr.mu.Lock()
	state.RetainPending = false
	rr.mu.Unlock()
	return nil
}

func (rr *routeReflector) releaseEndedSource(ctx context.Context, source peerIdentity) error {
	rr.mu.RLock()
	state := rr.peers[source]
	rr.mu.RUnlock()
	if state == nil {
		rr.withdrawalMu.Lock()
		if !rr.sourceMemberLocked(source) {
			delete(rr.cleanupPending, source)
		}
		rr.withdrawalMu.Unlock()
		return nil
	}
	state.leaseCall.Lock()
	defer state.leaseCall.Unlock()
	state.leaseMu.Lock()
	rr.withdrawalMu.Lock()
	held := rr.sourceMemberLocked(source)
	rr.withdrawalMu.Unlock()
	rr.mu.RLock()
	live, leased, receipt := state.Up, state.Leased, state.SourceReceipt
	rr.mu.RUnlock()
	state.leaseMu.Unlock()
	if live {
		return nil
	}
	if held {
		return nil
	}
	if leased {
		if _, err := rr.plugin.SourceLease(ctx, rpc.SourceLeaseInput{Action: rpc.SourceLeaseRelease, PeerOwner: source.owner, SourceReceipt: receipt}); err != nil {
			return err
		}
	}
	state.leaseMu.Lock()
	defer state.leaseMu.Unlock()
	rr.mu.Lock()
	state.Leased = false
	if rr.peers[source] == state {
		if state.targetOutputDebt == nil {
			if state.departureOwed == nil {
				if state.replayCalls == 0 {
					delete(rr.peers, source)
				}
			}
		}
	}
	rr.mu.Unlock()
	rr.mu.RLock()
	replaying := state.replayCalls != 0
	rr.mu.RUnlock()
	if !replaying {
		rr.withdrawalMu.Lock()
		delete(rr.cleanupPending, source)
		rr.withdrawalMu.Unlock()
	}
	return nil
}

func (rr *routeReflector) oweCleanup(source peerIdentity) {
	rr.withdrawalMu.Lock()
	defer rr.withdrawalMu.Unlock()
	rr.oweCleanupLocked(source)
}

// oweCleanupLocked versions only this original actor's current obligations.
// The caller MUST hold withdrawalMu until any matching worker is enrolled;
// settlement MUST recheck that version after waiting before it retires rows.
func (rr *routeReflector) oweCleanupLocked(source peerIdentity) {
	if rr.cleanupPending == nil {
		rr.cleanupPending = make(map[peerIdentity]uint64)
	}
	rr.cleanupPending[source]++
}

// endOwner handles only the terminal peer-removed event. Connection zero is
// never a physical wildcard; it closes this actual owner's known actors.
func (rr *routeReflector) endOwner(owner uint64) {
	rr.retained.End(owner)
	rr.mu.RLock()
	actors := make([]peerIdentity, 0)
	for source := range rr.peers {
		if source.owner == owner {
			actors = append(actors, source)
		}
	}
	rr.mu.RUnlock()
	for _, source := range actors {
		rr.mu.RLock()
		state := rr.peers[source]
		rr.mu.RUnlock()
		if state == nil {
			continue
		}
		state.leaseMu.Lock()
		rr.mu.Lock()
		state.Up = false
		state.DownSeen = true
		state.awaitingSessionReady = false
		state.RetainPending = false
		state.optionalReplayErr = nil
		rr.mu.Unlock()
		rr.withdrawalMu.Lock()
		entries := make(map[withdrawalKey]withdrawalInfo)
		for key, info := range rr.withdrawals[source.address.String()] {
			if key.owner != owner {
				continue
			}
			if key.connection != source.connection {
				continue
			}
			info.Closing = true
			info.Generation = state.PublishedGeneration
			rr.withdrawals[source.address.String()][key] = info
			entries[key] = info
		}
		rr.dropSourcePolicyLocked(owner)
		rr.withdrawalMu.Unlock()
		if !rr.enqueueDeparture(source, entries) {
			rr.noteWorkFailure("terminal source departure", errors.New("departure worker stopped before admission"))
		}
		rr.oweCleanup(source)
		state.leaseMu.Unlock()
	}
}
