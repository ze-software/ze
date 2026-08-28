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
}

// applyToInstalled applies a validation decision to a route the RIB already holds.
//
// RFC 6811 Section 4: when a VRP is added or deleted, the RPKI plugin re-validates every tracked
// route and re-dispatches an accept or a reject for each one whose state changed. Those routes
// are installed, not pending, so the pending map cannot carry them. Without this, a re-dispatched
// decision fell through to storeEarlyDecision -- the slot for a decision that arrives BEFORE its
// route -- and the installed route kept the state it was given on arrival for the life of the
// session. A route validated against an empty cache stayed NotFound after the cache synced, and
// a route that became Invalid stayed in the Adj-RIB-In under `invalid reject`.
//
// An accept rewrites the state in place and keeps the route's sequence number. The wire bytes did
// not change, and a new sequence number re-sends the same route to every peer that replays from a
// cursor (buildReplayRoutes).
//
// A reject removes the route. RFC 6811 Section 2 forbids excluding a route from the Adj-RIB-In as
// a side effect of its validation state, so a reject reaches here only when the operator
// configured `invalid reject` or `not-found reject` (the rpki plugin's buildDecisions).
//
// Returns false when the RIB does not hold the route, which leaves the caller's early-decision
// path in charge.
// Caller must hold r.mu write lock.
func (r *AdjRIBInManager) applyToInstalled(peerAddr netip.Addr, routeKey compactRouteKey, accept bool, validationState uint8) bool {
	routes := r.ribIn[peerAddr]
	if routes == nil {
		return false
	}
	route, ok := routes.Get(routeKey)
	if !ok {
		return false
	}
	if !accept {
		routes.Delete(routeKey)
		logger().Debug("re-validation removed an installed route",
			"peer", peerAddr, "family", routeKey.Fam, "prefix", routeKey.Prefix)
		return true
	}
	route.ValidationState = validationState
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
	action     earlyAction // accept or reject
	state      uint8       // validation state (only meaningful for accept)
	receivedAt time.Time
}

type earlyAction uint8

const (
	earlyAccept earlyAction = 1
	earlyReject earlyAction = 2
)

// earlyDecisionTimeout is how long an early decision stays buffered.
// Expiry means the route never arrived, which indicates a bug.
const earlyDecisionTimeout = 1 * time.Minute

// applyEarlyDecision checks for a buffered decision and applies it to a
// newly-pending route. Returns true if a decision was found and applied.
// Caller must hold r.mu write lock.
func (r *AdjRIBInManager) applyEarlyDecision(peerAddr netip.Addr, routeKey compactRouteKey, pr *pendingRoute) bool {
	key := pendingKey(peerAddr, routeKey)
	ed, ok := r.earlyDecisions[key]
	if !ok {
		return false
	}
	delete(r.earlyDecisions, key)
	switch ed.action {
	case earlyAccept:
		r.promoteToInstalled(pr, ed.state)
	case earlyReject:
		logger().Debug("applied early reject", "peer", peerAddr)
	default:
		logger().Warn("early decision with unknown action, ignoring",
			"peer", peerAddr, "action", ed.action)
		return false
	}
	return true
}

// storeEarlyDecision buffers a validation decision for a route not yet pending.
// Caller must hold r.mu write lock.
func (r *AdjRIBInManager) storeEarlyDecision(peerAddr netip.Addr, routeKey compactRouteKey, action earlyAction, state uint8) {
	key := pendingKey(peerAddr, routeKey)
	r.earlyDecisions[key] = &earlyDecision{
		action:     action,
		state:      state,
		receivedAt: time.Now(),
	}
}

// sweepExpiredEarlyDecisions removes stale early decisions.
// Caller must hold r.mu write lock.
func (r *AdjRIBInManager) sweepExpiredEarlyDecisions() {
	now := time.Now()
	for key, ed := range r.earlyDecisions {
		if now.Sub(ed.receivedAt) > earlyDecisionTimeout {
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
				r.mu.Unlock()
			}
		}
	}()
}
