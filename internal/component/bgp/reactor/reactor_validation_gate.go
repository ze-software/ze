// Design: docs/architecture/plugin/rib-storage-design.md -- validation eligibility between receive stores
// Related: forward_validation.go -- the forward path that reads ribevents.RouteEligible

package reactor

import (
	"context"
	"encoding/json"
	"errors"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// subscribeValidationGate applies every bgp-rib/validation-gate announcement
// to the daemon's receive gate (ribevents.NoteValidationGate). The gate's
// owner may run forked, where its eligibility lookup cannot be registered in
// this process; the announcement is then what makes the forward path refuse
// routes instead of forwarding every one as eligible.
//
// It runs from StartWithContext beside subscribeDaemonSignals, before the
// plugins that depend on BGP (bgp-adj-rib-in, bgp-rpki) run their startup,
// because a subscription delivers future events only and bgp-rpki enables
// validation while plugins start. The unsubscribe joins eventBusUnsubs
// (releaseEventSubscriptions).
//
// It also registers dispatchGateCommand as this process's route to the
// command the announcement names, so the forward path reads a forked owner's
// verdicts (ribevents.RouteEligibleBatch) rather than refusing every route.
// The release joins eventBusUnsubs too.
//
// The selecting RIB's bgp-rib/flowspec-gate is applied the same way
// (ribevents.NoteFlowSpecGate): with bgp-rib forked, the forward path's
// FlowSpec authorization and redirect verdicts and policy reload's FlowSpec
// replay ask it through the command it names, rather than forwarding no rule.
func (r *Reactor) subscribeValidationGate() {
	if r.eventBus == nil {
		return
	}
	release := ribevents.RegisterGateDispatch(r.dispatchGateCommand)
	unsubscribe := ribevents.ValidationGate.Subscribe(r.eventBus, ribevents.NoteValidationGate)
	unsubscribeFlowSpec := ribevents.FlowSpecGate.Subscribe(r.eventBus, ribevents.NoteFlowSpecGate)
	r.eventBusUnsubs = append(r.eventBusUnsubs, unsubscribe, unsubscribeFlowSpec, release)
}

// dispatchGateCommand sends the gate owner's eligibility command through the
// daemon's command dispatcher, to whichever plugin declared it. The engine is
// the caller, so no operator authorization applies.
func (r *Reactor) dispatchGateCommand(ctx context.Context, command string, args ...string) (string, json.RawMessage, error) {
	dispatcher := r.Dispatcher()
	if dispatcher == nil {
		return "", nil, errors.New("the API server has not started, so no plugin command can be dispatched")
	}
	resp, err := dispatcher.ForwardToPlugin(&pluginserver.CommandContext{RequestContext: ctx}, command, args, "")
	if err != nil {
		return "", nil, err
	}
	if resp == nil {
		return "", nil, errors.New(command + " answered no response")
	}
	if resp.Error != "" {
		return resp.Status, nil, errors.New(resp.Error)
	}
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return "", nil, err
	}
	return resp.Status, data, nil
}
