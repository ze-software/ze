// Design: docs/architecture/plugin/rib-storage-design.md -- live reflector input.
// Related: actor.go -- only live ordered delivery acquires a source lease.
package rr

import (
	"context"
	"errors"
	"fmt"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (rr *routeReflector) reflectWire(event *rpc.StructuredEvent, msg *bgptypes.RawMessage) error {
	source, err := newPeerIdentity(event.PeerAddress, event.PeerOwner, event.ConnectionID)
	if err != nil {
		return rr.rejectCached(msg.MessageID, err)
	}
	if msg.SourceOwner != source.owner {
		return rr.rejectCached(msg.MessageID, errors.New("bgp-rr: UPDATE owner differs from its event"))
	}
	if msg.SourceReceipt != event.SourceReceipt {
		return rr.rejectCached(msg.MessageID, errors.New("bgp-rr: UPDATE receipt differs from its event"))
	}
	ctx, cancel := context.WithTimeout(rr.replayParent(), updateRouteTimeout)
	defer cancel()
	rr.mu.RLock()
	state := rr.peers[source]
	rr.mu.RUnlock()
	if state == nil {
		return rr.rejectCached(msg.MessageID, errors.New("bgp-rr: UPDATE has no original UP actor"))
	}
	if err := rr.acquireSourceLease(ctx, source, msg.SourceReceipt); err != nil {
		return rr.rejectCached(msg.MessageID, err)
	}
	state.leaseMu.Lock()
	rr.mu.RLock()
	live := state.Up
	rr.mu.RUnlock()
	if !live {
		state.leaseMu.Unlock()
		return rr.rejectCached(msg.MessageID, errors.New("bgp-rr: original source closed before UPDATE admission"))
	}
	rr.withdrawalMu.Lock()
	if err := rr.updateWithdrawalMapWire(event.PeerAddress, msg, source.connection); err != nil {
		rr.withdrawalMu.Unlock()
		state.leaseMu.Unlock()
		return rr.rejectCached(msg.MessageID, err)
	}
	state.inventoryRevision++
	state.processedMsgID = max(state.processedMsgID, msg.MessageID)
	rr.forwards.enter()
	rr.withdrawalMu.Unlock()
	state.leaseMu.Unlock()
	targets, held := rr.targetsWithHold(source, family.Family{})
	if rr.enqueueForwardIfNeeded(source, msg.MessageID, targets, held, "cached reflection") {
		return nil
	}
	defer rr.wakeCurrentReplay()
	defer rr.forwards.leave()
	err = rr.forwardCaptured(ctx, source, msg.MessageID, targets)
	rr.noteForwardFailure("cached reflection", err, source)
	return forwardResult(err)
}

func (rr *routeReflector) reflectText(address string, owner, connection, receipt, msgID, epoch uint64, text string) error {
	source, err := newPeerIdentity(address, owner, connection)
	if err != nil {
		return rr.rejectCached(msgID, err)
	}
	ctx, cancel := context.WithTimeout(rr.replayParent(), updateRouteTimeout)
	defer cancel()
	records, err := rr.textRecords(ctx, parseTextNLRIOps(text), epoch, receipt, owner, connection, msgID, textNoLLGR(text))
	if err != nil {
		return rr.rejectCached(msgID, err)
	}
	rr.mu.RLock()
	state := rr.peers[source]
	rr.mu.RUnlock()
	if state == nil {
		return rr.rejectCached(msgID, errors.New("bgp-rr: UPDATE has no original UP actor"))
	}
	if err := rr.acquireSourceLease(ctx, source, receipt); err != nil {
		return rr.rejectCached(msgID, err)
	}
	state.leaseMu.Lock()
	rr.mu.RLock()
	live := state.Up
	rr.mu.RUnlock()
	if !live {
		state.leaseMu.Unlock()
		return rr.rejectCached(msgID, errors.New("bgp-rr: original source closed before TEXT UPDATE admission"))
	}
	rr.withdrawalMu.Lock()
	for _, record := range records {
		rr.applyInventoryLocked(address, record)
	}
	state.inventoryRevision++
	state.processedMsgID = max(state.processedMsgID, msgID)
	rr.forwards.enter()
	rr.withdrawalMu.Unlock()
	state.leaseMu.Unlock()
	targets, held := rr.targetsWithHold(source, family.Family{})
	if rr.enqueueForwardIfNeeded(source, msgID, targets, held, "cached text reflection") {
		return nil
	}
	defer rr.wakeCurrentReplay()
	defer rr.forwards.leave()
	err = rr.forwardCaptured(ctx, source, msgID, targets)
	rr.noteForwardFailure("cached text reflection", err, source)
	return forwardResult(err)
}

// forwardOutputError distinguishes an applied ordinary UPDATE whose final
// output failed from an event that could not be applied. Only forwardResult
// constructs it; its zero and nil values are invalid. The event wrapper MUST
// accept this already reported failure into the declared settlement debt;
// internal callers MUST retain the failure and its original cause.
type forwardOutputError struct{ cause error }

func (err *forwardOutputError) Error() string { return err.cause.Error() }
func (err *forwardOutputError) Unwrap() error { return err.cause }
func forwardResult(err error) error {
	if err == nil {
		return nil
	}
	return &forwardOutputError{cause: err}
}

// rejectCached owns only a request that never entered ForwardCached. The
// forward request itself owns its ACK even on error; it MUST NOT come here.
func (rr *routeReflector) rejectCached(id uint64, cause error) error {
	ctx, cancel := context.WithTimeout(rr.replayParent(), updateRouteTimeout)
	defer cancel()
	if err := rr.plugin.ReleaseCached(ctx, []uint64{id}); err != nil {
		rr.noteWorkFailure("rejected cache release", err)
		return fmt.Errorf("%w; cache release: %v", cause, err)
	}
	return cause
}
