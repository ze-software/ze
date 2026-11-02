// Design: docs/architecture/plugin/rib-storage-design.md -- FlowSpec authorization across the process boundary
// Related: rib_eligibility.go -- the validation gate this store announces
// Related: rib_replay_path.go -- the FlowSpec replay that reads the RIB's lookup

package adj_rib_in

import (
	"context"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// gateEventTimeout bounds the gate-query this store emits at start.
const gateEventTimeout = 5 * time.Second

// handleGateEvent applies a delivered gate event and reports whether text was
// one. flowspec-gate tells this store which command answers the selecting
// RIB's FlowSpec lookup (peer-up replay reads it, rib_replay_path.go), so a
// forked store, or a store beside a forked RIB, replays FlowSpec rules rather
// than none. gate-query asks this store to announce its validation gate again,
// for a reader whose subscription went live after bgp-rpki enabled it.
func (r *AdjRIBInManager) handleGateEvent(text string) bool {
	matched, err := ribevents.ApplyFlowSpecGate(text)
	if err != nil {
		logger().Warn("bgp-adj-rib-in: flowspec-gate refused", "error", err)
		return true
	}
	if matched {
		return true
	}
	if !ribevents.IsGateQuery(text) {
		return false
	}
	// refuse-by-default: guards emitValidationGateEvent -- a nil announcer is a test manager with no engine, and no other process exists to tell
	if r.announceGate == nil {
		return true
	}
	r.mu.RLock()
	current := ribevents.ValidationGateState{Enabled: r.validationEnabled, Command: eligibilityCommand}
	r.mu.RUnlock()
	if err := r.announceGate(current); err != nil {
		logger().Error("bgp-adj-rib-in: validation-gate not announced again: a reader that subscribed late may select unvalidated routes", "error", err)
	}
	return true
}

// askGateOwners emits a gate-query once this store's subscriptions are live,
// so the selecting RIB announces its flowspec-gate again in case it did so
// before this store subscribed.
func (r *AdjRIBInManager) askGateOwners(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, gateEventTimeout)
	defer cancel()
	_, err := r.plugin.EmitEvent(ctx, ribevents.Namespace, ribevents.EventGateQuery, "", "", "{}")
	return err
}
