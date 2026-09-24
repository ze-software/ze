// Design: docs/architecture/plugin/rib-storage-design.md — RPKI validation gate
// RFC: rfc/short/rfc6811.md -- BGP prefix origin validation states
// Overview: rib.go — core types, event handlers, and raw hex storage
// Related: rib_commands.go — command handlers including validation commands
package adj_rib_in

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Validation state constants (RFC 6811 + internal states).
const (
	ValidationNotValidated uint8 = 0 // Default or timeout (fail-open)
	ValidationValid        uint8 = 1 // Origin AS matches a covering VRP
	ValidationNotFound     uint8 = 2 // No covering VRP exists
	ValidationInvalid      uint8 = 3 // Covering VRP exists but no match
	ValidationPending      uint8 = 4 // Awaiting validation (internal only)
)

// defaultValidationTimeout is the fail-open timeout for pending routes.
const defaultValidationTimeout = 30 * time.Second

// pendingRoute stores a route awaiting validation.
type pendingRoute struct {
	peerAddr   netip.Addr
	family     family.Family
	prefix     string
	routeKey   compactRouteKey // Key for insertion into installed seqmap
	route      *RawRoute       // The raw route data
	receivedAt time.Time
	state      uint8
}

// pendingKey builds a lookup key for the pending routes map.
func pendingKey(peerAddr netip.Addr, routeKey compactRouteKey) compactPendingKey {
	return compactPendingKey{PeerAddr: peerAddr, Route: routeKey}
}

// promoteToInstalled moves a pending route to the installed ribIn map.
// Caller must hold r.mu write lock.
func (r *AdjRIBInManager) promoteToInstalled(pr *pendingRoute, validationState uint8) {
	pr.route.ValidationState = validationState

	if r.ribIn[pr.peerAddr] == nil {
		r.ribIn[pr.peerAddr] = newSeqMap()
	}
	r.seqCounter++
	r.ribIn[pr.peerAddr].Put(pr.routeKey, r.seqCounter, pr.route)
	r.noteValidationChange(pr.peerAddr, pr.routeKey)
}

// applyToInstalled applies a decision only to the matching UPDATE generation.
// A newer decision waits for its route; the previous path becomes ineligible
// immediately so a failed replacement cannot leave it active.
// Caller MUST hold mu and finish with unlockValidation.
func (r *AdjRIBInManager) applyToInstalled(peerAddr netip.Addr, routeKey compactRouteKey, d *rpc.ValidationDecision) bool {
	routes := r.ribIn[peerAddr]
	if routes == nil {
		return false
	}
	route, ok := routes.Get(routeKey)
	if !ok {
		return false
	}
	if d.MsgID != 0 && d.MsgID != route.MsgID {
		if d.MsgID < route.MsgID {
			return true
		}
		route.Ineligible = true
		r.noteValidationChange(peerAddr, routeKey)
		return false
	}
	if !d.Accept && !d.Ineligible {
		r.removeInstalled(peerAddr, routeKey)
		return true
	}
	wasIneligible := route.Ineligible
	route.ValidationState = d.ValState
	// draft-ietf-sidrops-aspa-verification-28 Section 5.7: an Invalid
	// route "MUST be kept in the Adj-RIB-In for potential future re-evaluation".
	route.Ineligible = d.Ineligible
	if wasIneligible && d.Accept {
		// Recovery must appear after an existing replay cursor.
		r.seqCounter++
		routes.Put(routeKey, r.seqCounter, route)
	}
	r.noteValidationChange(peerAddr, routeKey)
	return true
}

// applyDecision is shared by typed, batch-text and individual text commands.
// It returns true only when the decision is buffered ahead of its UPDATE.
// Caller MUST hold mu and finish with unlockValidation.
func (r *AdjRIBInManager) applyDecision(peer netip.Addr, key compactRouteKey, d *rpc.ValidationDecision) bool {
	if !r.validationEnabled || !isSimplePrefixFamily(key.Fam) {
		return false
	}
	pKey := pendingKey(peer, key)
	if newer := r.earlyDecisions[pKey]; newer != nil && d.MsgID != 0 && newer.msgID > d.MsgID {
		return false
	}
	if pr, ok := r.pending[pKey]; ok {
		if d.MsgID != 0 && d.MsgID != pr.route.MsgID {
			if d.MsgID < pr.route.MsgID {
				return false
			}
			r.storeEarlyDecision(peer, key, d)
			return true
		}
		delete(r.pending, pKey)
		if d.Accept || d.Ineligible {
			pr.route.Ineligible = d.Ineligible
			r.promoteToInstalled(pr, d.ValState)
		} else {
			r.removeInstalled(peer, key)
		}
		return false
	}
	if r.applyToInstalled(peer, key, d) {
		return false
	}
	r.storeEarlyDecision(peer, key, d)
	return true
}

// sweepExpiredPending promotes pending routes that have exceeded the validation timeout.
// Caller must hold r.mu write lock.
func (r *AdjRIBInManager) sweepExpiredPending() {
	now := time.Now()
	timeout := r.validationTimeout
	if timeout == 0 {
		timeout = defaultValidationTimeout
	}

	for key, pr := range r.pending {
		if now.Sub(pr.receivedAt) > timeout {
			if ed := r.earlyDecisions[key]; ed != nil && ed.msgID > pr.route.MsgID {
				continue
			}
			logger().Warn("validation timeout, promoting route (fail-open)",
				"peer", pr.peerAddr, "family", pr.family, "prefix", pr.prefix)
			r.promoteToInstalled(pr, ValidationNotValidated)
			delete(r.pending, key)
		}
	}
}

// clearPeerPending removes all pending routes and early decisions for a peer.
// Caller must hold r.mu write lock.
func (r *AdjRIBInManager) clearPeerPending(peerAddr netip.Addr) {
	for key := range r.pending {
		if key.PeerAddr == peerAddr {
			delete(r.pending, key)
		}
	}
	for key := range r.earlyDecisions {
		if key.PeerAddr == peerAddr {
			delete(r.earlyDecisions, key)
		}
	}
}

// removePending removes a specific pending route by routeKey.
// Caller must hold r.mu write lock.
func (r *AdjRIBInManager) removePending(peerAddr netip.Addr, routeKey compactRouteKey) {
	key := pendingKey(peerAddr, routeKey)
	delete(r.pending, key)
	delete(r.earlyDecisions, key)
	r.noteValidationChange(peerAddr, routeKey)
}

// parseValidationState converts a string state argument to an RFC 6811 state.
//
// RFC requirement: RFC6811-2-1 -- RFC 6811 Section 2 requires the route's
// validation state to reflect the lookup result. Valid, NotFound, and Invalid
// are all lookup results, so an operator policy that accepts Invalid must be
// able to retain state 3 on the route.
func parseValidationState(s string) (uint8, error) {
	switch s {
	case "1":
		return ValidationValid, nil
	case "2":
		return ValidationNotFound, nil
	case "3":
		return ValidationInvalid, nil
	default:
		return 0, fmt.Errorf("invalid validation state: %s (expected 1=Valid, 2=NotFound, or 3=Invalid)", s)
	}
}

// earlyDecision stores a validation decision that arrived before the route.
type earlyDecision struct {
	action     earlyAction
	state      uint8
	msgID      uint64
	receivedAt time.Time
}

type earlyAction uint8

const (
	earlyAccept     earlyAction = 1
	earlyReject     earlyAction = 2
	earlyIneligible earlyAction = 3
)

// earlyDecisionTimeout is how long an early decision stays buffered.
// Expiry means the route never arrived, which indicates a bug.
const earlyDecisionTimeout = 1 * time.Minute

// applyEarlyDecision checks for a buffered decision and applies it to a
// newly-pending route. Returns true if a decision was found and applied.
// Caller must hold r.mu write lock.
func (r *AdjRIBInManager) applyEarlyDecision(peerAddr netip.Addr, routeKey compactRouteKey, pr *pendingRoute) bool {
	key := pendingKey(peerAddr, routeKey)
	// This arrival supersedes any older pending route even when its decision
	// is already buffered and it bypasses the pending timeout queue entirely.
	delete(r.pending, key)
	ed, ok := r.earlyDecisions[key]
	if !ok {
		return false
	}
	if ed.msgID != 0 && ed.msgID != pr.route.MsgID {
		if ed.msgID < pr.route.MsgID {
			delete(r.earlyDecisions, key)
		}
		return false
	}
	delete(r.earlyDecisions, key)
	switch ed.action {
	case earlyAccept:
		r.promoteToInstalled(pr, ed.state)
	case earlyIneligible:
		pr.route.Ineligible = true
		r.promoteToInstalled(pr, ed.state)
	case earlyReject:
		logger().Debug("applied early reject", "peer", peerAddr)
		r.removeInstalled(peerAddr, routeKey)
	default:
		logger().Warn("early decision with unknown action, ignoring",
			"peer", peerAddr, "action", ed.action)
		return false
	}
	return true
}

// storeEarlyDecision buffers a validation decision for a route not yet pending.
// Caller must hold r.mu write lock.
func (r *AdjRIBInManager) storeEarlyDecision(peerAddr netip.Addr, routeKey compactRouteKey, d *rpc.ValidationDecision) {
	key := pendingKey(peerAddr, routeKey)
	if old := r.earlyDecisions[key]; old != nil && d.MsgID != 0 && old.msgID > d.MsgID {
		return
	}
	action := earlyReject
	if d.Accept {
		action = earlyAccept
	}
	if d.Ineligible {
		action = earlyIneligible
	}
	r.earlyDecisions[key] = &earlyDecision{
		action: action, state: d.ValState, msgID: d.MsgID, receivedAt: time.Now(),
	}
}

// sweepExpiredEarlyDecisions removes stale decisions without a predecessor.
// A known replacement must keep blocking its predecessor until receive,
// withdrawal, or session removal resolves it; expiration cannot revive it.
// Caller must hold r.mu write lock.
func (r *AdjRIBInManager) sweepExpiredEarlyDecisions() {
	now := time.Now()
	for key, ed := range r.earlyDecisions {
		if now.Sub(ed.receivedAt) > earlyDecisionTimeout {
			if pending := r.pending[key]; pending != nil && ed.msgID > pending.route.MsgID {
				continue
			}
			if routes := r.ribIn[key.PeerAddr]; routes != nil {
				if route, ok := routes.Get(key.Route); ok && ed.msgID > route.MsgID {
					continue
				}
			}
			logger().Warn("early validation decision expired without matching route",
				"key", key, "action", ed.action, "age", now.Sub(ed.receivedAt))
			delete(r.earlyDecisions, key)
		}
	}
}

// sweepInterval is the period between timeout scans.
const sweepInterval = 5 * time.Second

// startTimeoutScanner launches a long-lived goroutine that periodically
// promotes expired pending routes (fail-open). Stops when stopCh is closed.
func (r *AdjRIBInManager) startTimeoutScanner(stopCh <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(sweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				r.mu.Lock()
				r.sweepExpiredPending()
				r.sweepExpiredEarlyDecisions()
				r.unlockValidation()
			}
		}
	}()
}
