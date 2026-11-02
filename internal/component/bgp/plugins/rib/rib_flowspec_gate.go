// Design: docs/architecture/plugin/rib-storage-design.md -- FlowSpec authorization across the process boundary
// Related: rib_flowspec_validation.go -- the FlowSpec lookup this command serves
// Related: rib.go -- runRIBPlugin registers the lookup, subscribes gate-query and announces

package rib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// flowSpecGateCommand is the command a process that does not hold this RIB's
// FlowSpec lookup in memory asks (ribevents.FlowSpecRequest in,
// ribevents.FlowSpecAnswer out): the daemon's forward path and policy reload,
// and bgp-rr, bgp-rs and bgp-adj-rib-in, when either side runs forked. The
// flowspec-gate announcement names it, so no reader names this plugin.
const flowSpecGateCommand = "request bgp rib flowspec-gate"

// gateEmitTimeout bounds one announcement or query emitted to the engine.
const gateEmitTimeout = 5 * time.Second

// flowSpecGateAnswerCommand answers flowSpecGateCommand from the lookup
// runRIBPlugin registered in this process. Its one argument is the compact
// JSON request, bounded by ribevents.EligibilityBatchMax queries.
func (r *RIBManager) flowSpecGateAnswerCommand(_ string, args []string) (string, any, error) {
	if len(args) != 1 {
		return statusError, nil, errors.New(flowSpecGateCommand + " requires one argument: the JSON FlowSpec request")
	}
	answer, err := ribevents.AnswerFlowSpec(args[0])
	if err != nil {
		return statusError, nil, fmt.Errorf("%s: %w", flowSpecGateCommand, err)
	}
	return statusDone, answer, nil
}

// announceFlowSpecGate tells every process that reads the FlowSpec lookup
// which command answers it, through the SDK so it reaches the daemon and every
// forked subscriber alike. It runs once this RIB's subscriptions are live
// (OnStarted) and again on every gate-query.
func (r *RIBManager) announceFlowSpecGate() error {
	payload, err := json.Marshal(ribevents.FlowSpecGateState{Command: flowSpecGateCommand})
	if err != nil {
		return fmt.Errorf("encode flowspec-gate: %w", err)
	}
	return r.emitGateEvent(ribevents.EventFlowSpecGate, string(payload))
}

// askGateOwners emits a gate-query, so the receive store announces its
// validation gate again: this RIB's subscription may have gone live after
// bgp-rpki enabled validation, and a forked RIB that missed it would select
// every path.
func (r *RIBManager) askGateOwners() error {
	return r.emitGateEvent(ribevents.EventGateQuery, "{}")
}

func (r *RIBManager) emitGateEvent(event, payload string) error {
	// refuse-by-default: guards announceFlowSpecGate -- a nil plugin is a test manager with no engine, and no other process exists to tell
	if r.plugin == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), gateEmitTimeout)
	defer cancel()
	if _, err := r.plugin.EmitEvent(ctx, ribevents.Namespace, event, "", "", payload); err != nil {
		return fmt.Errorf("emit %s: %w", event, err)
	}
	return nil
}
