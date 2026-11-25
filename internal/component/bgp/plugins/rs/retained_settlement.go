// Design: docs/architecture/plugin/plugin-system.md -- RS applied lifecycle joins.
// Related: source_lifetime.go -- release only after final original output joins.
package rs

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func cleanupMatches(scope retention.Cleanup, native withdrawalKey, entry withdrawalEntry) bool {
	if entry.owner != scope.Owner() {
		return false
	}
	if entry.connectionID != scope.Connection() {
		return false
	}
	if !scope.MatchesFamily(native.fam) {
		return false
	}
	if scope.NoLLGROnly() && !entry.noLLGR {
		return false
	}
	return true
}

// sealCleanup does finite decode and current admission sealing only. It never
// joins a worker/page/final writer on the context-free ordinary event loop.
func (rs *routeServer) sealCleanup(delivered string) (bool, error) {
	envelope, matched := retention.AcceptsCleanup(delivered)
	if !matched {
		return false, nil
	}
	scope, err := retention.DecodeCleanup(envelope)
	if err != nil {
		return true, err
	}
	key, err := newWorkerKey(scope.Address().String(), scope.Owner(), scope.Connection())
	if err != nil {
		return true, err
	}
	rs.withdrawalMu.Lock()
	rs.mu.Lock()
	if actor := rs.peers[key]; actor != nil {
		// Repeated delivery coalesces by its actual predicate; no event ID or ended
		// owner history is retained. Existing admitted output still owns its join.
		repeated := false
		for _, old := range actor.cleanup {
			if old.NoLLGROnly() != scope.NoLLGROnly() {
				continue
			}
			equal := true
			for _, name := range family.RegisteredFamilyNames() {
				fam, _ := family.LookupFamily(name)
				if old.MatchesFamily(fam) != scope.MatchesFamily(fam) {
					equal = false
					break
				}
			}
			if equal {
				repeated = true
				break
			}
		}
		if !repeated {
			actor.cleanup = append(actor.cleanup, scope)
			actor.adoptionOpen = false
		}
	}
	rs.mu.Unlock()
	rs.withdrawalMu.Unlock()
	return true, nil
}

// settleAll captures already admitted workers including closing physical lanes.
// A marker flushes the real cached batch and settles coalesced validations.
func (wp *workerPool) settleAll(ctx context.Context) error {
	wp.mu.Lock()
	pending := make([]chan struct{}, 0, len(wp.workers))
	for key, worker := range wp.workers {
		select {
		case <-worker.closeCh:
			pending = append(pending, worker.done)
		default:
			marker := make(chan struct{})
			if !wp.dispatchLocked(key, workItem{barrier: marker}) {
				wp.mu.Unlock()
				return errWorkerPoolClosing
			}
			pending = append(pending, marker)
		}
	}
	wp.mu.Unlock()
	for _, done := range pending {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return ctx.Err()
}

// settleRetainedEvents is one combined applied-event callback. The caller's
// actual work deadline bounds every wait, rather than a background substitute.
// A failed join preserves all current cleanup predicates and original leases.
func (rs *routeServer) settleRetainedEvents(ctx context.Context) (result error) {
	defer func() {
		if result != nil {
			result = rs.settlementFailure(workerKey{}, result)
		} else {
			rs.outputMu.Lock()
			rs.settlementOwed = nil
			rs.outputMu.Unlock()
		}
	}()
	if _, bounded := ctx.Deadline(); !bounded {
		return errors.New("route-server settlement requires an applied work deadline")
	}
	rs.mu.RLock()
	keys := make([]workerKey, 0, len(rs.peers))
	for key := range rs.peers {
		keys = append(keys, key)
	}
	rs.mu.RUnlock()
	rs.departureMu.Lock()
	departures := rs.departures
	for key := range rs.departureRevisions {
		found := false
		for _, old := range keys {
			if old == key {
				found = true
				break
			}
		}
		if !found {
			keys = append(keys, key)
		}
	}
	rs.departureMu.Unlock()
	if departures != nil {
		for _, key := range keys {
			if err := departures.Await(ctx, key); err != nil {
				return rs.settlementFailure(key, err)
			}
		}
	}
	if rs.workers != nil {
		if err := rs.workers.settleAll(ctx); err != nil {
			return rs.settlementFailure(workerKey{}, err)
		}
	}
	for _, key := range keys {
		if rs.hasPendingDown(key) {
			if err := rs.settlePendingDown(ctx, key); err != nil {
				return rs.settlementFailure(key, err)
			}
		}
	}
	// Every replay phase, superseded UP replay and refresh is admitted before
	// launch. This join includes escaped store pages and their actual relays.
	if err := rs.settleReplays(ctx); err != nil {
		return rs.settlementFailure(workerKey{}, err)
	}
	if err := rs.unknownOutputFailure(); err != nil {
		return err
	}
	if err := rs.retryCurrentReplays(ctx); err != nil {
		return err
	}
	if err := joinReplays(ctx, rs.outputs.snapshot()); err != nil {
		return rs.settlementFailure(workerKey{}, err)
	}
	if err := ctx.Err(); err != nil {
		return rs.settlementFailure(workerKey{}, err)
	}
	rs.withdrawalMu.Lock()
	rs.mu.Lock()
	for _, key := range keys {
		actor := rs.peers[key]
		if actor == nil {
			continue
		}
		addr := key.address.String()
		entries := rs.withdrawals[addr]
		for native, entry := range entries {
			for _, scope := range actor.cleanup {
				if cleanupMatches(scope, native, entry) {
					rs.deleteWithdrawalLocked(addr, native)
					break
				}
			}
		}
		actor.cleanup = nil
	}
	rs.mu.Unlock()
	rs.withdrawalMu.Unlock()
	for _, key := range keys {
		if err := rs.settleDeparture(ctx, key); err != nil {
			return rs.settlementFailure(key, err)
		}
	}
	return ctx.Err()
}

type rsSettlementError struct {
	cause   error
	message string
}

func (e *rsSettlementError) Error() string { return e.message }
func (e *rsSettlementError) Unwrap() error { return e.cause }

func (rs *routeServer) settlementFailure(source workerKey, cause error) error {
	var existing *rsSettlementError
	if errors.As(cause, &existing) {
		return cause
	}
	rs.outputMu.Lock()
	rs.settlementOwed = cause
	count := 0
	targets := make([]string, 0, 8)
	for _, batch := range rs.outputTargets {
		count += len(batch)
		for _, target := range batch {
			if len(targets) == 8 {
				break
			}
			targets = append(targets, fmt.Sprintf("%s owner=%d session=%d", target.Destination, target.DestinationOwner, target.DestinationSession))
		}
	}
	rs.outputMu.Unlock()
	detail := cause.Error()
	if len(detail) > 1024 {
		detail = "cause exceeds this receipt's detail limit"
	}
	message := fmt.Sprintf("route-server owner %d connection %d settlement remains owed; current targets [%s], omitted %d; %s; full current details: ze show bgp rs status", source.owner, source.connectionID, strings.Join(targets, ", "), count-len(targets), detail)
	return &rsSettlementError{cause: cause, message: message}
}

// lifecycleStatus exposes only current actors, active output and owed work.
// A successful settlement clears its current diagnostic; it keeps no history.
func (rs *routeServer) lifecycleStatus() any {
	rs.mu.RLock()
	sources := make([]map[string]any, 0, len(rs.peers))
	for key, actor := range rs.peers {
		replayFailure := ""
		if actor.replayFailure != nil {
			replayFailure = actor.replayFailure.Error()
		}
		sources = append(sources, map[string]any{"source": key.address.String(), "owner": key.owner, "connection": key.connectionID, "lease": actor.sourceLease, "release-owed": actor.sourceReleaseOwed, "source-receipt": actor.sourceReceipt, "down-pending": actor.downPending, "cleanup-scopes": len(actor.cleanup), "replay-failure": replayFailure})
	}
	rs.mu.RUnlock()
	rs.outputMu.Lock()
	active := make([]rpc.ForwardTarget, 0, len(rs.outputTargets))
	for _, batch := range rs.outputTargets {
		active = append(active, batch...)
	}
	unknown := make([]map[string]any, 0, len(rs.unknownOutputs))
	for target, cause := range rs.unknownOutputs {
		unknown = append(unknown, map[string]any{"destination": target, "cause": cause.Error()})
	}
	failure := ""
	if rs.settlementOwed != nil {
		failure = rs.settlementOwed.Error()
	}
	rs.outputMu.Unlock()
	return map[string]any{"sources": sources, "active-output": active, "unknown-output": unknown, "settlement-owed": failure}
}
