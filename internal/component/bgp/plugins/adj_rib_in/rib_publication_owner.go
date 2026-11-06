// Design: docs/architecture/plugin/rib-storage-design.md -- validation publication ownership.
package adj_rib_in

import (
	"errors"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

var errReceiveStoreRemoved = errors.New("receive store was removed; configure it again before changing routes")
var errReceiveStoreStopped = errors.New("receive store stopped; restart ze before changing routes")

// admitMutation MUST precede a mutation that can publish validation changes.
// Its successful caller MUST release mutationMu after publication finishes.
// Explicit commands settle earlier debt before changing another key. Ordinary
// intake cannot retry or skip a delivered state change: paused debt ends its
// owner before that delivery mutates the store. Readers use mu alone, so
// publication consumers can query the store meanwhile.
func (r *AdjRIBInManager) admitMutation(retry bool) error {
	r.mutationMu.Lock()
	r.mu.RLock()
	err := r.terminal
	r.mu.RUnlock()
	if err != nil {
		r.mutationMu.Unlock()
		return err
	}
	if err := r.flushValidationChanges(retry); err != nil {
		if !retry {
			r.failValidationOwner(err)
		}
		r.mutationMu.Unlock()
		return err
	}
	return nil
}

// failValidationOwner runs while the mutation owner still excludes the next
// delivery. The static lookup MUST survive teardown, until daemon shutdown;
// clearing it would make ordinary routes eligible again.
func (r *AdjRIBInManager) failValidationOwner(cause error) {
	r.mu.Lock()
	if r.terminal == nil {
		r.terminal = cause
	}
	r.mu.Unlock()
	ribevents.RegisterValidationLookup(refuseValidationRoute, refusePresentRoute)
	if r.plugin != nil {
		if err := r.plugin.Close(); err != nil {
			logger().Error("receive store could not close after validation publication failed; restart ze", "error", err)
		}
	}
}

func refuseValidationRoute(ribevents.ValidationRoute, uint64) bool { return false }
func refusePresentRoute(ribevents.ValidationRoute) bool            { return false }

func (r *AdjRIBInManager) validationFailure() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if errors.Is(r.terminal, errReceiveStoreRemoved) {
		return nil
	}
	return r.terminal
}

// removeValidationOwner MUST settle publication while mutations are excluded.
// An acknowledged removal admits no later mutation or publication failure.
// A refused cleanup leaves the owner live for the existing recovery protocol.
func (r *AdjRIBInManager) removeValidationOwner(string) error {
	if err := r.admitMutation(true); err != nil {
		return err
	}
	defer r.mutationMu.Unlock()
	if err := r.flushValidationChanges(true); err != nil {
		return err
	}
	r.mu.Lock()
	r.terminal = errReceiveStoreRemoved
	r.mu.Unlock()
	return nil
}

// releaseValidationOwner runs after SDK Run has joined admitted handlers and
// the runtime owner has stopped and joined its timeout scanner.
// Successful removal releases the lookup. An exit with no removal, which a
// daemon stop also is (it closes the connection and sends no bye), leaves
// selection closed while the declared fatal policy stops the daemon and its
// peers. That holds only for an owner that ever gated a route: one whose
// validation was never enabled registered no lookup, so its exit has no verdict
// to keep, and a refusing lookup would switch on a gate nobody asked for
// (ribevents.ValidationEnabled) in this process.
func (r *AdjRIBInManager) releaseValidationOwner() {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	r.mu.Lock()
	removed := errors.Is(r.terminal, errReceiveStoreRemoved)
	gated := r.validationGate || r.terminal != nil
	if r.terminal == nil {
		r.terminal = errReceiveStoreStopped
	}
	r.mu.Unlock()
	if removed {
		ribevents.RegisterValidationLookup(nil, nil)
		return
	}
	if !gated {
		return
	}
	ribevents.RegisterValidationLookup(refuseValidationRoute, refusePresentRoute)
}
