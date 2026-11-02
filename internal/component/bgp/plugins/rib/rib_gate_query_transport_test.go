// Design: docs/architecture/plugin/rib-storage-design.md -- FlowSpec authorization across the process boundary
// Related: rib_flowspec_gate.go -- announceFlowSpecGate, askGateOwners
// Related: ../../../../test/pluginengine/pluginengine.go -- the engine side of the connection

package rib

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestGateQueryAnnouncesFlowSpecGate proves a reader whose subscription went
// live after bgp-rib announced its FlowSpec gate still learns it, on both
// transports, and that bgp-rib itself asks the receive store for its
// validation gate once its own subscriptions are live.
//
// VALIDATES: at start bgp-rib emits bgp-rib/flowspec-gate naming its FlowSpec
// command, then a gate-query; each delivered gate-query makes it announce
// the FlowSpec gate again with the same command.
// PREVENTS: a forked bgp-rr, bgp-rs or bgp-adj-rib-in that subscribed after
// the announcement never learning the command, so it forwards and replays no
// FlowSpec rule; and a forked bgp-rib that subscribed after bgp-rpki enabled
// validation selecting every path.
// Method: run the real entry point through the fake engine, deliver two
// enveloped gate-queries, and decode every bgp-rib emit-event the engine saw.
func TestGateQueryAnnouncesFlowSpecGate(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			engine := pluginengine.Start(t, transport, runRIBPlugin)
			require.NoError(t, engine.Deliver(ctx, transportStateUp(transportSource, 65001)))
			require.Equal(t, []string{ribevents.EventFlowSpecGate, ribevents.EventGateQuery}, ribEmitTypes(t, engine),
				"at start the RIB announces its FlowSpec gate, then asks the gate owners")

			query, err := json.Marshal(rpc.EventEnvelope{Namespace: ribevents.Namespace, Event: ribevents.EventGateQuery, Payload: json.RawMessage(`{}`)})
			require.NoError(t, err)
			require.NoError(t, engine.Deliver(ctx, string(query)))
			require.NoError(t, engine.Deliver(ctx, string(query)))

			var commands []string
			for _, call := range ribEmits(t, engine) {
				if call.EventType != ribevents.EventFlowSpecGate {
					continue
				}
				var state ribevents.FlowSpecGateState
				require.NoError(t, json.Unmarshal([]byte(call.Event), &state), call.Event)
				commands = append(commands, state.Command)
			}
			require.Equal(t, []string{flowSpecGateCommand, flowSpecGateCommand, flowSpecGateCommand}, commands,
				"the start announcement, then one per gate-query")
		})
	}
}

// ribEmits returns, in order, every bgp-rib emit-event the engine has seen.
func ribEmits(t *testing.T, engine *pluginengine.Engine) []rpc.EmitEventInput {
	t.Helper()
	var emits []rpc.EmitEventInput
	for _, call := range engine.RPCs() {
		if call.Method != rpc.MethodEmitEvent {
			continue
		}
		var input rpc.EmitEventInput
		require.NoError(t, json.Unmarshal(call.Params, &input))
		if input.Namespace != ribevents.Namespace {
			continue
		}
		emits = append(emits, input)
	}
	return emits
}

// ribEmitTypes returns the event type of every bgp-rib emit-event, in order.
func ribEmitTypes(t *testing.T, engine *pluginengine.Engine) []string {
	t.Helper()
	var types []string
	for _, emit := range ribEmits(t, engine) {
		types = append(types, emit.EventType)
	}
	return types
}
