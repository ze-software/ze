// Design: docs/architecture/plugin/rib-storage-design.md -- FlowSpec authorization across the process boundary
// Related: rib_gate_events.go -- handleGateEvent, askGateOwners
// Related: ../../../../test/pluginengine/pluginengine.go -- the engine side of the connection

package adj_rib_in

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

// TestGateQueryAnnouncesCurrentValidationGate proves a reader whose
// subscription went live after the receive store announced its validation
// gate still learns the gate's current state, on both transports: the store
// answers a gate-query by announcing the gate again.
//
// VALIDATES: the store emits a gate-query once its subscriptions are live; a
// delivered gate-query makes it announce bgp-rib/validation-gate with the
// state it holds now (enabled after enable-validation, disabled after
// disable-validation), naming its eligibility command.
// PREVENTS: a forked bgp-rib that subscribed after bgp-rpki enabled validation
// never hearing the gate and selecting every path as if validation were off.
// Method: run the real entry point through the fake engine, enable then
// disable validation, deliver an enveloped gate-query after each, and decode
// every validation-gate emit-event the engine saw.
func TestGateQueryAnnouncesCurrentValidationGate(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
			ribevents.RegisterValidationLookup(nil, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			engine := pluginengine.Start(t, transport, runAdjRIBInPlugin)
			require.NoError(t, engine.Deliver(ctx, adjTransportStateUp(adjTransportSource)))
			require.NotEmpty(t, gateEmits(t, engine, ribevents.EventGateQuery),
				"the store asks the gate owners once its subscriptions are live")

			_, err := engine.Execute(ctx, "request bgp adj-rib-in enable-validation", nil)
			require.NoError(t, err)
			require.NoError(t, engine.Deliver(ctx, gateQueryEnvelope(t)))
			_, err = engine.Execute(ctx, "request bgp adj-rib-in disable-validation", nil)
			require.NoError(t, err)
			require.NoError(t, engine.Deliver(ctx, gateQueryEnvelope(t)))

			var states []bool
			for _, payload := range gateEmits(t, engine, ribevents.EventValidationGate) {
				var state ribevents.ValidationGateState
				require.NoError(t, json.Unmarshal([]byte(payload), &state), payload)
				require.Equal(t, eligibilityCommand, state.Command, payload)
				states = append(states, state.Enabled)
			}
			// enable, its query, disable, its query: each query repeats the
			// state the store held when it arrived.
			require.Equal(t, []bool{true, true, false, false}, states)
		})
	}
}

// gateQueryEnvelope is the enveloped bgp-rib/gate-query a reader emits, as
// the engine delivers it to an envelope subscriber.
func gateQueryEnvelope(t *testing.T) string {
	t.Helper()
	b, err := json.Marshal(rpc.EventEnvelope{Namespace: ribevents.Namespace, Event: ribevents.EventGateQuery, Payload: json.RawMessage(`{}`)})
	require.NoError(t, err)
	return string(b)
}

// gateEmits returns, in order, the payload of every bgp-rib emit-event of
// eventType the engine has seen.
func gateEmits(t *testing.T, engine *pluginengine.Engine, eventType string) []string {
	t.Helper()
	var payloads []string
	for _, call := range engine.RPCs() {
		if call.Method != rpc.MethodEmitEvent {
			continue
		}
		var input rpc.EmitEventInput
		require.NoError(t, json.Unmarshal(call.Params, &input))
		if input.Namespace != ribevents.Namespace {
			continue
		}
		if input.EventType != eventType {
			continue
		}
		payloads = append(payloads, input.Event)
	}
	return payloads
}
