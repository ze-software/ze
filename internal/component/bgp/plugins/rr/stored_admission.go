// Design: docs/architecture/plugin/rib-storage-design.md -- current stored-route admission.
// Related: reflect.go -- original receive progress; actor.go -- the single relay request.
package rr

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// admitStoredRoute enrolls exactly one current original row before its caller
// may dispatch output. A true answer owns one forwards.leave obligation; the
// caller MUST release it after the complete target request has joined.
// The loop is bounded by the caller's finite replay/validation deadline. A
// projection race retries only its read, before any output has been admitted.
func (rr *routeReflector) admitStoredRoute(ctx context.Context, route rpc.StoredRoute) (bool, error) {
	if _, bounded := ctx.Deadline(); !bounded {
		return false, errors.New("bgp-rr: current route admission requires a finite deadline")
	}
	source, err := newPeerIdentity(route.SourcePeer, route.SourceOwner, route.SourceConnectionID)
	if err != nil {
		return false, err
	}
	key, info, validation, err := storedInventory(route)
	if err != nil {
		return false, err
	}
	rr.mu.RLock()
	state := rr.peers[source]
	rr.mu.RUnlock()
	if state == nil {
		return false, nil
	}
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		rr.withdrawalMu.Lock()
		revision, afterMsgID := state.inventoryRevision, state.processedMsgID
		member := rr.storedMemberLocked(route)
		rr.withdrawalMu.Unlock()
		if !route.Withdraw {
			var verdict storedEligibility
			if ribevents.IsFlowSpec(validation.Family) {
				verdict, err = currentStoredFlowSpec(ctx, validation, route, afterMsgID)
			} else {
				verdict, err = rr.currentStoredRoute(ctx, validation, route.MsgID, afterMsgID)
			}
			if err != nil {
				return false, err
			}
			switch verdict {
			case storedEligibilityRejected:
				return false, nil
			case storedEligibilityPending:
				if err := waitStoredEligibility(ctx); err != nil {
					return false, err
				}
				continue
			case storedEligibilityAccepted:
			case storedEligibilityUnspecified:
				return false, errors.New("bgp-rr: current eligibility has no verdict")
			default:
				return false, errors.New("bgp-rr: current eligibility has an unknown verdict")
			}
		}
		if err := rr.retainSource(ctx, source); err != nil {
			return false, err
		}
		if !member {
			if route.Withdraw {
				return false, nil
			}
			if err := rr.acquireSourceLease(ctx, source, route.SourceReceipt); err != nil {
				return false, err
			}
		}
		state.leaseMu.Lock()
		rr.mu.RLock()
		current := rr.peers[source] == state
		live, leased, receipt := state.Up, state.Leased, state.SourceReceipt
		rr.mu.RUnlock()
		if !current {
			state.leaseMu.Unlock()
			return false, nil
		}
		rr.withdrawalMu.Lock()
		if revision != state.inventoryRevision {
			rr.withdrawalMu.Unlock()
			state.leaseMu.Unlock()
			if err := waitStoredEligibility(ctx); err != nil {
				return false, err
			}
			continue
		}
		member = rr.storedMemberLocked(route)
		if !member {
			if !live {
				rr.withdrawalMu.Unlock()
				state.leaseMu.Unlock()
				return false, nil
			}
			if route.Withdraw {
				rr.withdrawalMu.Unlock()
				state.leaseMu.Unlock()
				return false, nil
			}
			rr.putWithdrawalLocked(route.SourcePeer, key, info)
			member = rr.storedMemberLocked(route)
		}
		if member {
			if !leased {
				rr.withdrawalMu.Unlock()
				state.leaseMu.Unlock()
				return false, errors.New("bgp-rr: stored membership has no process-owned source lease")
			}
			if receipt != route.SourceReceipt {
				rr.withdrawalMu.Unlock()
				state.leaseMu.Unlock()
				return false, errors.New("bgp-rr: stored membership names a different source receipt")
			}
			rr.forwards.enter()
		}
		rr.withdrawalMu.Unlock()
		state.leaseMu.Unlock()
		return member, nil
	}
}

// waitStoredEligibility bounds projection catch-up polling by the existing
// replay convergence interval and caller deadline. It never retries output.
func waitStoredEligibility(ctx context.Context) error {
	timer := time.NewTimer(replayConvergenceDelay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
