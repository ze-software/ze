// Design: docs/architecture/plugin/rib-storage-design.md -- RS original source lifetimes.
// Related: retained_settlement.go -- cleanup joins before lease release.
package rs

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (rs *routeServer) callSourceLease(ctx context.Context, key workerKey, receipt uint64, action rpc.SourceLeaseAction) error {
	input := rpc.SourceLeaseInput{SourceReceipt: receipt, PeerOwner: key.owner, Action: action}
	if rs.sourceLeaseHook != nil {
		return rs.sourceLeaseHook(ctx, input)
	}
	if rs.plugin == nil {
		return errors.New("route-server source lease has no engine connection")
	}
	_, err := rs.plugin.SourceLease(ctx, input)
	return err
}

// acquireSource runs on the ordered UPDATE admission rail, before dispatch.
// A failed acquisition grants neither live nor retained replay authority.
func (rs *routeServer) acquireSource(key workerKey, receipt uint64) error {
	if receipt == 0 {
		return errors.New("route-server UPDATE has no original source receipt")
	}
	rs.mu.RLock()
	leaseActor := rs.peers[key]
	rs.mu.RUnlock()
	if leaseActor == nil {
		return errors.New("original source actor is absent")
	}
	leaseActor.leaseMu.Lock()
	defer leaseActor.leaseMu.Unlock()
	return rs.acquireSourceLocked(key, receipt)
}

func (rs *routeServer) acquireSourceLocked(key workerKey, receipt uint64) error {
	rs.mu.RLock()
	actor := rs.peers[key]
	if actor == nil {
		rs.mu.RUnlock()
		return errors.New("route-server UPDATE has no original UP actor")
	}
	if actor.StateSeen && !actor.Up {
		rs.mu.RUnlock()
		return errors.New("route-server source admission is sealed by DOWN")
	}
	owed, prior := actor.sourceReleaseOwed, actor.sourceReceipt
	if actor.sourceLease {
		same := prior == receipt
		rs.mu.RUnlock()
		if !same {
			return errors.New("route-server physical actor changed its original source receipt")
		}
		return nil
	}
	rs.mu.RUnlock()
	ctx, cancel := context.WithTimeout(rs.replayParent(), forwardCachedTimeout)
	defer cancel()
	if owed {
		if err := rs.unknownOutputFailure(); err != nil {
			return fmt.Errorf("owner %d connection %d cannot reacquire before its unresolved original enrollment joins: %w", key.owner, key.connectionID, err)
		}
		if err := rs.callSourceLease(ctx, key, prior, rpc.SourceLeaseRelease); err != nil {
			return err
		}
		rs.mu.Lock()
		actor.sourceReleaseOwed = false
		actor.sourceReceipt = 0
		actor.sourceRetained = false
		current := rs.peers[key] == actor && !(actor.StateSeen && !actor.Up)
		rs.mu.Unlock()
		if !current {
			return errors.New("route-server original source ended while its prior release joined")
		}
	}
	if err := rs.callSourceLease(ctx, key, receipt, rpc.SourceLeaseAcquire); err != nil {
		return err
	}
	rs.mu.Lock()
	actor.sourceReceipt = receipt
	actor.sourceLease = true
	current := rs.peers[key] == actor && !(actor.StateSeen && !actor.Up)
	rs.mu.Unlock()
	if !current {
		return errors.New("route-server original source ended during lease acquisition")
	}
	return nil
}

func (rs *routeServer) retainSource(key workerKey) error {
	rs.mu.RLock()
	leaseActor := rs.peers[key]
	rs.mu.RUnlock()
	if leaseActor == nil {
		return nil
	}
	leaseActor.leaseMu.Lock()
	defer leaseActor.leaseMu.Unlock()
	rs.mu.RLock()
	actor := rs.peers[key]
	if actor == nil {
		rs.mu.RUnlock()
		return nil
	}
	receipt, leased, needed, already := actor.sourceReceipt, actor.sourceLease, len(actor.retainedFamilies) != 0, actor.sourceRetained
	rs.mu.RUnlock()
	if !leased || !needed || already {
		return nil
	}
	ctx, cancel := context.WithTimeout(rs.replayParent(), forwardCachedTimeout)
	defer cancel()
	if err := rs.callSourceLease(ctx, key, receipt, rpc.SourceLeaseRetain); err != nil {
		return err
	}
	rs.mu.Lock()
	actor.sourceRetained = true
	rs.mu.Unlock()
	return nil
}

// dispatchSourceUpdate owns one pending UPDATE only after worker admission.
// After success, the worker MUST call finishSourceUpdate exactly once after
// processing the admitted item. A refusal rolls back its own pending count.
func (rs *routeServer) dispatchSourceUpdate(key workerKey, item workItem, receipt uint64) error {
	rs.mu.RLock()
	leaseActor := rs.peers[key]
	rs.mu.RUnlock()
	if leaseActor == nil {
		return errors.New("original source actor is absent")
	}
	leaseActor.leaseMu.Lock()
	defer leaseActor.leaseMu.Unlock()
	rs.mu.RLock()
	sealed := len(leaseActor.cleanup) != 0
	rs.mu.RUnlock()
	if sealed {
		if err := rs.unknownOutputFailure(); err != nil {
			return fmt.Errorf("owner %d connection %d has sealed cleanup with unresolved original enrollment: %w", key.owner, key.connectionID, err)
		}
	}
	if err := rs.acquireSourceLocked(key, receipt); err != nil {
		return err
	}
	rs.mu.Lock()
	actor := rs.peers[key]
	actor.pendingUpdates++
	rs.mu.Unlock()
	if rs.workers.Dispatch(key, item) {
		return nil
	}
	rs.mu.Lock()
	actor.pendingUpdates--
	rs.mu.Unlock()
	return errors.New("original source worker admission is closed")
}

// finishSourceUpdate MUST follow each successful dispatchSourceUpdate exactly
// once, after its admitted UPDATE processing and deferred validation complete.
// It MUST NOT discharge a refused dispatch or another physical actor's item.
func (rs *routeServer) finishSourceUpdate(key workerKey) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if actor := rs.peers[key]; actor != nil && actor.pendingUpdates != 0 {
		actor.pendingUpdates--
	}
}

func (rs *routeServer) hasPendingDown(key workerKey) bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	actor := rs.peers[key]
	return actor != nil && actor.downPending
}

// finishSourceDown is called only after the captured physical worker's final
// flush and close callback joined. Only the actual pre-DOWN verdict can keep
// current routes. Lease failure never falls back to a current address peer.
func (rs *routeServer) finishSourceDown(ctx context.Context, key workerKey) (map[withdrawalKey]withdrawalEntry, error) {
	rs.mu.RLock()
	leaseActor := rs.peers[key]
	rs.mu.RUnlock()
	if leaseActor == nil {
		return nil, nil
	}
	leaseActor.leaseMu.Lock()
	defer leaseActor.leaseMu.Unlock()
	rs.mu.RLock()
	actor := rs.peers[key]
	if actor == nil {
		rs.mu.RUnlock()
		return nil, nil
	}
	if !actor.downPending {
		rs.mu.RUnlock()
		return nil, nil
	}
	receipt, leased, keep, generation, alreadyRetained := actor.sourceReceipt, actor.sourceLease, actor.retainedFamilies, actor.downGeneration, actor.sourceRetained
	rs.mu.RUnlock()
	if leased && len(keep) != 0 && !alreadyRetained {
		if err := rs.callSourceLease(ctx, key, receipt, rpc.SourceLeaseRetain); err != nil {
			return nil, fmt.Errorf("retain owner %d connection %d: %w", key.owner, key.connectionID, err)
		}
	}
	rs.withdrawalMu.Lock()
	if txn := rs.sourcePolicy[key.owner]; txn != nil && txn.source == key {
		delete(rs.sourcePolicy, key.owner)
	}
	selected := make(map[withdrawalKey]withdrawalEntry)
	addr := key.address.String()
	entries := rs.withdrawals[addr]
	for native, entry := range entries {
		if entry.owner != key.owner {
			continue
		}
		if entry.connectionID != key.connectionID {
			continue
		}
		if leased && keep[native.fam] {
			entry.retained = true
			entries[native] = entry
			continue
		}
		entry.generation = generation
		selected[native] = entry
		rs.deleteWithdrawalLocked(addr, native)
	}
	rs.withdrawalMu.Unlock()
	rs.mu.Lock()
	actor.downPending = false
	actor.retainedFamilies = nil
	rs.mu.Unlock()
	return selected, nil
}

// releaseEmptySource releases the exact existing lease only when no current
// membership of that original receipt remains. A failed release stays owed on
// the same actor; no completion history or synthetic lease is created.
func (rs *routeServer) releaseEmptySource(ctx context.Context, key workerKey) error {
	rs.mu.RLock()
	leaseActor := rs.peers[key]
	rs.mu.RUnlock()
	if leaseActor == nil {
		return nil
	}
	leaseActor.leaseMu.Lock()
	defer leaseActor.leaseMu.Unlock()
	rs.withdrawalMu.Lock()
	if txn := rs.sourcePolicy[key.owner]; txn != nil && txn.source == key && len(txn.pins) != 0 {
		rs.withdrawalMu.Unlock()
		return nil
	}
	if rs.hasWithdrawalForSourceLocked(key) {
		rs.withdrawalMu.Unlock()
		return nil
	}
	rs.mu.Lock()
	actor := rs.peers[key]
	if actor == nil {
		rs.mu.Unlock()
		rs.withdrawalMu.Unlock()
		return nil
	}
	receipt, ended := actor.sourceReceipt, actor.StateSeen && !actor.Up
	if actor.downPending || actor.pendingUpdates+actor.pendingAdoptions != 0 {
		rs.mu.Unlock()
		rs.withdrawalMu.Unlock()
		return nil
	}
	// Release invalidates at its producer before waiting for old writers. The
	// local actor MUST stop authorizing new work before the RPC can escape.
	if actor.sourceLease {
		actor.sourceLease = false
		actor.sourceReleaseOwed = true
	}
	owed := actor.sourceReleaseOwed
	mayEnroll := !ended
	rs.mu.Unlock()
	rs.withdrawalMu.Unlock()
	if mayEnroll {
		if err := rs.unknownOutputFailure(); err != nil {
			return fmt.Errorf("release owner %d connection %d cannot discharge unresolved live enrollment: %w", key.owner, key.connectionID, err)
		}
	}
	if owed {
		if err := rs.callSourceLease(ctx, key, receipt, rpc.SourceLeaseRelease); err != nil {
			return fmt.Errorf("release owner %d connection %d: %w", key.owner, key.connectionID, err)
		}
	}
	rs.mu.Lock()
	actor.sourceReleaseOwed = false
	actor.sourceReceipt = 0
	actor.sourceRetained = false
	if ended && rs.peers[key] == actor && len(actor.cleanup) == 0 {
		delete(rs.peers, key)
	}
	rs.mu.Unlock()
	rs.withdrawalMu.Lock()
	if ended {
		if txn := rs.sourcePolicy[key.owner]; txn != nil && txn.source == key && len(txn.pins) == 0 {
			delete(rs.sourcePolicy, key.owner)
		}
	}
	rs.withdrawalMu.Unlock()
	rs.batches.Delete(key)
	return nil
}

func (rs *routeServer) recordDeparture(key workerKey, entries map[withdrawalKey]withdrawalEntry) {
	rs.mu.Lock()
	actor := rs.peers[key]
	if actor == nil {
		rs.mu.Unlock()
		return
	}
	if len(entries) != 0 {
		if actor.departure == nil {
			actor.departure = &sourceDeparture{routes: make(map[withdrawalKey]withdrawalEntry), targets: make(map[rpc.ForwardTarget]error)}
		}
		maps.Copy(actor.departure.routes, entries)
		for targetKey, peer := range rs.peers {
			if targetKey.owner == key.owner || !peer.updateTarget() {
				continue
			}
			target, err := targetOf(targetKey, peer)
			if err == nil {
				actor.departure.targets[target] = nil
			}
		}
	}
	rs.mu.Unlock()
}

type sourceDeparture struct {
	routes  map[withdrawalKey]withdrawalEntry
	targets map[rpc.ForwardTarget]error
}

func (rs *routeServer) recoverBound(ctx context.Context, target rpc.ForwardTarget, command, source string, owner, generation uint64) error {
	work, finish, err := rs.beginTargetOutput(ctx, []rpc.ForwardTarget{target})
	if err != nil {
		return err
	}
	defer finish()
	ctx = work
	if rs.updateRouteHook != nil {
		rs.updateRouteHook(target.Destination, command)
		return nil
	}
	meta := map[string]any{"recovery-source": source, "recovery-source-owner": fmt.Sprint(owner), "recovery-generation": fmt.Sprint(generation)}
	_, _, err = rs.plugin.UpdateRouteRequest(ctx, rpc.UpdateRouteInput{PeerSelector: target.Destination, Command: command, Meta: meta, DestinationOwner: target.DestinationOwner, DestinationSession: target.DestinationSession})
	rs.rememberUnknownOutput([]rpc.ForwardTarget{target}, err)
	return err
}

// settleDeparture reuses the existing current native inventory and immutable
// original target set. Failed targets stay owed; a same-address successor never
// receives the old cleanup. Ended target Sessions have no remaining output.
func (rs *routeServer) settlePendingDown(ctx context.Context, key workerKey) error {
	rs.mu.RLock()
	actor := rs.peers[key]
	rs.mu.RUnlock()
	if actor == nil {
		return nil
	}
	actor.nativeMu.Lock()
	defer actor.nativeMu.Unlock()
	entries, err := rs.finishSourceDown(ctx, key)
	if err != nil {
		return err
	}
	rs.recordDeparture(key, entries)
	return rs.settleDepartureLocked(ctx, key)
}

func (rs *routeServer) settleDeparture(ctx context.Context, key workerKey) error {
	rs.mu.RLock()
	nativeActor := rs.peers[key]
	rs.mu.RUnlock()
	if nativeActor == nil {
		return nil
	}
	nativeActor.nativeMu.Lock()
	defer nativeActor.nativeMu.Unlock()
	return rs.settleDepartureLocked(ctx, key)
}

func (rs *routeServer) settleDepartureLocked(ctx context.Context, key workerKey) error {
	rs.mu.RLock()
	actor := rs.peers[key]
	if actor == nil {
		rs.mu.RUnlock()
		return nil
	}
	owed := actor.departure
	rs.mu.RUnlock()
	if owed != nil {
		rs.mu.RLock()
		targets := make([]rpc.ForwardTarget, 0, len(owed.targets))
		for target := range owed.targets {
			targets = append(targets, target)
		}
		rs.mu.RUnlock()
		for _, target := range targets {
			rs.mu.RLock()
			current := false
			for targetKey, peer := range rs.peers {
				if targetKey.owner == target.DestinationOwner && peer.DestinationSession == target.DestinationSession && peer.Up {
					current = true
					break
				}
			}
			rs.mu.RUnlock()
			if current {
				if err := rs.sendDepartureRoutes(ctx, key, owed.routes, []rpc.ForwardTarget{target}); err != nil {
					rs.mu.Lock()
					owed.targets[target] = err
					rs.mu.Unlock()
					return fmt.Errorf("native target %s owner %d session %d: %w", target.Destination, target.DestinationOwner, target.DestinationSession, err)
				}
			}
			rs.mu.Lock()
			delete(owed.targets, target)
			rs.mu.Unlock()
		}
		rs.mu.Lock()
		if actor.departure == owed {
			actor.departure = nil
		}
		rs.mu.Unlock()
	}
	return rs.releaseEmptySource(ctx, key)
}

// beginTargetOutput enrolls an actual bound engine call before it can escape.
// Route-bearing relay callers separately hold current membership through this
// enrollment. After success, the caller MUST invoke the returned finish exactly
// once after the final engine call and result classification, including unknown
// debt. A refusal returns no completion obligation.
func (rs *routeServer) beginTargetOutput(ctx context.Context, targets []rpc.ForwardTarget) (context.Context, func(), error) {
	done := make(chan struct{})
	work, admitted := rs.outputs.begin(ctx, done)
	if !admitted {
		return nil, nil, errors.New("original bound output was not admitted before process stop")
	}
	rs.outputMu.Lock()
	if rs.outputTargets == nil {
		rs.outputTargets = make(map[chan struct{}][]rpc.ForwardTarget)
	}
	rs.outputTargets[done] = append([]rpc.ForwardTarget(nil), targets...)
	rs.outputMu.Unlock()
	// finish MUST follow this successful beginTargetOutput exactly once, after
	// the final engine call and its result classification have completed.
	finish := func() {
		rs.outputMu.Lock()
		delete(rs.outputTargets, done)
		rs.outputMu.Unlock()
		rs.outputs.finish(done)
	}
	return work, finish, nil
}

type relayFailureKey struct {
	target rpc.ForwardTarget
	native withdrawalKey
}

// joinedOutputError is a transport receipt classification, never an inference
// from a generic error or from the absence of CancellationUnsettledError.
func joinedOutputError(err error) bool {
	if err == nil {
		return true
	}
	var completed *rpc.CachedWriteJoinedError
	var refused *rpc.CachedWriteNotDispatchedError
	return errors.As(err, &completed) || errors.As(err, &refused)
}

func (rs *routeServer) rememberUnknownOutput(targets []rpc.ForwardTarget, cause error) {
	if joinedOutputError(cause) {
		return
	}
	rs.outputMu.Lock()
	defer rs.outputMu.Unlock()
	if rs.unknownOutputs == nil {
		rs.unknownOutputs = make(map[rpc.ForwardTarget]error)
	}
	for _, target := range targets {
		rs.unknownOutputs[target] = cause
	}
}

func (rs *routeServer) unknownOutputFailure() error {
	rs.outputMu.Lock()
	defer rs.outputMu.Unlock()
	for target, cause := range rs.unknownOutputs {
		return fmt.Errorf("output completion unknown at %s owner %d session %d; join the original connection before retrying cleanup: %w", target.Destination, target.DestinationOwner, target.DestinationSession, cause)
	}
	return nil
}

func (rs *routeServer) rememberRelayResult(target rpc.ForwardTarget, routes []rpc.StoredRoute, cause error) {
	rs.rememberUnknownOutput([]rpc.ForwardTarget{target}, cause)
	rs.withdrawalMu.Lock()
	defer rs.withdrawalMu.Unlock()
	for _, route := range routes {
		txn := rs.sourcePolicy[route.SourceOwner]
		if txn == nil {
			continue
		}
		if txn.source.connectionID != route.SourceConnectionID {
			continue
		}
		key, err := storedKey(route, false)
		if err != nil {
			continue
		}
		named := relayFailureKey{target: target, native: key}
		if cause == nil {
			delete(txn.relayFailures, named)
			continue
		}
		if txn.relayFailures == nil {
			txn.relayFailures = make(map[relayFailureKey]error)
		}
		txn.relayFailures[named] = cause
	}
}

func (rs *routeServer) targetLive(target rpc.ForwardTarget) bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	for key, actor := range rs.peers {
		if key.owner == target.DestinationOwner && actor.DestinationSession == target.DestinationSession && actor.Up {
			return true
		}
	}
	return false
}

func (rs *routeServer) sourceRelayFailure(owner uint64) error {
	rs.withdrawalMu.Lock()
	defer rs.withdrawalMu.Unlock()
	txn := rs.sourcePolicy[owner]
	if txn == nil {
		return nil
	}
	for key, cause := range txn.relayFailures {
		if !rs.targetLive(key.target) {
			delete(txn.relayFailures, key)
			continue
		}
		return fmt.Errorf("source-policy owner %d connection %d output remains owed to %s owner %d session %d: %w", owner, txn.source.connectionID, key.target.Destination, key.target.DestinationOwner, key.target.DestinationSession, cause)
	}
	return nil
}

func (rs *routeServer) failReplay(key workerKey, generation uint64, cause error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if actor := rs.peers[key]; actor != nil && actor.ReplayGen == generation {
		actor.replayFailure = cause
	}
}

func (rs *routeServer) finishReplayOutput(ctx context.Context, key workerKey, generation, initial uint64) {
	if err := rs.sendEOR(ctx, key, generation, initial); err != nil {
		rs.failReplay(key, generation, err)
		return
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if actor := rs.peers[key]; actor != nil && actor.ReplayGen == generation {
		actor.replayFailure = nil
	}
}

// retryCurrentReplays reads the authoritative current stores again for only
// the original still-live UP whose useful initial output remains owed. It
// never transfers an earlier failed replay to a same-address replacement.
func (rs *routeServer) retryCurrentReplays(ctx context.Context) error {
	rs.mu.RLock()
	var keys []workerKey
	for key, actor := range rs.peers {
		if actor.Up && actor.replayFailure != nil {
			keys = append(keys, key)
		}
	}
	rs.mu.RUnlock()
	for _, key := range keys {
		rs.mu.Lock()
		actor := rs.peers[key]
		if actor == nil || !actor.Up || actor.replayFailure == nil {
			rs.mu.Unlock()
			continue
		}
		target, err := targetOf(key, actor)
		if err != nil {
			rs.mu.Unlock()
			return err
		}
		generation, cut, initial := actor.ReplayGen, actor.ForwardFrom, actor.InitialReplay
		done := make(chan struct{})
		actor.replayDone = done
		rs.mu.Unlock()
		work, admitted := rs.replays.begin(ctx, done)
		if !admitted {
			rs.endReplay(key, done)
			return errors.New("original replay retry was not admitted")
		}
		rs.replayForPeer(context.WithValue(work, replayTargetKey{}, target), key, generation, cut, initial, done)
		rs.mu.RLock()
		cause := actor.replayFailure
		rs.mu.RUnlock()
		if cause != nil {
			return fmt.Errorf("original replay to %s owner %d session %d remains owed: %w", target.Destination, target.DestinationOwner, target.DestinationSession, cause)
		}
	}
	return nil
}

type departureWork struct {
	entries  map[withdrawalKey]withdrawalEntry
	revision uint64
}

func (rs *routeServer) sendDepartureWork(key workerKey, work departureWork) {
	defer func() {
		rs.departureMu.Lock()
		if rs.departureRevisions[key] == work.revision {
			delete(rs.departureRevisions, key)
		}
		rs.departureMu.Unlock()
	}()
	rs.sendBatchedWithdrawals(key, work.entries)
}
