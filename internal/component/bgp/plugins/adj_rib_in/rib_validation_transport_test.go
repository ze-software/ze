// Design: docs/architecture/plugin/rib-storage-design.md -- receive validation gate
// Related: rib_eligibility.go -- noteValidationChange, emitValidationEvent
// Related: ../../../../test/pluginengine/pluginengine.go -- the engine side of the connection

package adj_rib_in

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestValidationChangeEmittedOverTransport proves bgp-adj-rib-in hands its
// validation-change batch to the engine as an emit-event RPC through the SDK,
// with no EventBus handed to it, so every subscriber (bgp-rib, bgp-rs, bgp-rr,
// in the daemon or forked) receives it the same way.
//
// VALIDATES: a received route under a registered eligibility gate reaches the
// engine as bgp-rib/validation-change naming the route.
// PREVENTS: the emission depending on Registration.ConfigureEventBus, which
// runs only in the daemon process.
// Method: run the real entry point through the fake engine on the bridge,
// bring the peer up, register the gate with enable then disable validation (as
// TestValidationChangeNeedsRegisteredGate does), receive one route, and decode
// every emit-event RPC the engine saw. Only the bridge registers the gate: the
// eligibility lookup lives in the daemon process (enableValidationCommand), so
// a forked store has no gate and no change to announce until that lookup
// crosses the boundary (journal: process-global-read-across-the-plugin-boundary).
func TestValidationChangeEmittedOverTransport(t *testing.T) {
	for _, transport := range []pluginengine.Transport{pluginengine.TransportBridge} {
		t.Run(transport.String(), func(t *testing.T) {
			t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			engine := pluginengine.Start(t, transport, runAdjRIBInPlugin)
			// The first delivery returns once the callback loop runs, so the
			// command handler is installed on the bridge before Execute.
			require.NoError(t, engine.Deliver(ctx, adjTransportStateUp(adjTransportSource)))
			_, err := engine.Execute(ctx, "request bgp adj-rib-in enable-validation", nil)
			require.NoError(t, err)
			_, err = engine.Execute(ctx, "request bgp adj-rib-in disable-validation", nil)
			require.NoError(t, err)
			require.NoError(t, engine.Deliver(ctx, adjTransportReceived(adjTransportSource, 100, "10.1.0.0/24", "180a0100")))

			var routes []ribevents.ValidationRoute
			for _, call := range engine.RPCs() {
				if call.Method != rpc.MethodEmitEvent {
					continue
				}
				var input rpc.EmitEventInput
				require.NoError(t, json.Unmarshal(call.Params, &input))
				if input.Namespace != ribevents.Namespace {
					continue
				}
				if input.EventType != ribevents.EventValidationChange {
					continue
				}
				var batch []ribevents.ValidationRoute
				require.NoError(t, json.Unmarshal([]byte(input.Event), &batch), input.Event)
				routes = append(routes, batch...)
			}
			require.Len(t, routes, 1, "the install under the registered gate is one emitted change")
			require.Equal(t, netip.MustParseAddr(adjTransportSource), routes[0].Peer)
			require.Equal(t, netip.MustParsePrefix("10.1.0.0/24"), routes[0].Prefix)
		})
	}
}
