// Design: docs/architecture/plugin/rib-storage-design.md — RPKI validation gate
// Overview: rib.go — core types, event handlers, and raw hex storage
// Related: rib_commands.go — command handlers including validation commands
package adj_rib_in

import (
	"fmt"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
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

// PendingRoute stores a route awaiting validation.
type PendingRoute struct {
	peerAddr   string
	family     family.Family
	prefix     string
	routeKey   string    // Key for insertion into installed seqmap
	route      *RawRoute // The raw route data
	receivedAt time.Time
	state      uint8
}

// pendingKey builds a lookup key for the pending routes map.
// Uses peerAddr + routeKey (which includes pathID for ADD-PATH).
func pendingKey(peerAddr, routeKey string) string {
	return peerAddr + "|" + routeKey
}

// promoteToInstalled moves a pending route to the installed ribIn map.
// Caller must hold r.mu write lock.
func (r *AdjRIBInManager) promoteToInstalled(pr *PendingRoute, validationState uint8) {
	pr.route.ValidationState = validationState

	if r.ribIn[pr.peerAddr] == nil {
		r.ribIn[pr.peerAddr] = newSeqMap()
	}
	r.seqCounter++
	r.ribIn[pr.peerAddr].Put(pr.routeKey, r.seqCounter, pr.route)
}

// sweepExpiredPending promotes pending routes that have exceeded the validation timeout.
// Called periodically by the timeout scanner goroutine.
func (r *AdjRIBInManager) sweepExpiredPending() {
	r.mu.Lock()
	defer r.mu.Unlock()

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

// clearPeerPending removes all pending routes for a given peer.
// Caller must hold r.mu write lock.
func (r *AdjRIBInManager) clearPeerPending(peerAddr string) {
	for key, pr := range r.pending {
		if pr.peerAddr == peerAddr {
			delete(r.pending, key)
		}
	}
}

// removePending removes a specific pending route by routeKey.
// Caller must hold r.mu write lock.
func (r *AdjRIBInManager) removePending(peerAddr, routeKey string) {
	key := pendingKey(peerAddr, routeKey)
	delete(r.pending, key)
}

// parseValidationState converts a string state argument to uint8.
// Valid values: "1" (Valid), "2" (NotFound).
func parseValidationState(s string) (uint8, error) {
	if s == "1" {
		return ValidationValid, nil
	}
	if s == "2" {
		return ValidationNotFound, nil
	}
	return 0, fmt.Errorf("invalid validation state: %s (expected 1=Valid or 2=NotFound)", s)
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
				r.sweepExpiredPending()
			}
		}
	}()
}
