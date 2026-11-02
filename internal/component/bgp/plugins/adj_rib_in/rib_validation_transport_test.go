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
	"github.com/ze-software/ze/internal/core/family"
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

// TestValidationGateAnnouncedOverTransport proves bgp-adj-rib-in announces
// its receive gate to the engine on both transports, and that a process
// which cannot read the store's lookup then refuses the store's routes: RPKI
// and ASPA gating do not stop when the store runs forked.
//
// VALIDATES: enable-validation emits bgp-rib/validation-gate enabled, and
// disable-validation emits it disabled; applied as the daemon applies it
// (ribevents.NoteValidationGate, Reactor.subscribeValidationGate), the
// announcement makes ribevents.RouteEligible refuse a route the store
// received, where before it answered eligible for every path.
// PREVENTS: a forked store, whose lookup is never registered in the daemon,
// leaving the forward path and selection to forward every route unvalidated
// with no error.
// Method: run the real entry point through the fake engine on each transport,
// enable validation, apply every validation-gate emit-event to this process's
// gate in order, receive one route, and ask the gate about it; then disable.
func TestValidationGateAnnouncedOverTransport(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
			ribevents.RegisterValidationLookup(nil, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			engine := pluginengine.Start(t, transport, runAdjRIBInPlugin)
			require.NoError(t, engine.Deliver(ctx, adjTransportStateUp(adjTransportSource)))
			_, err := engine.Execute(ctx, "request bgp adj-rib-in enable-validation", nil)
			require.NoError(t, err)
			require.Equal(t, []bool{true}, applyGateAnnouncements(t, engine))

			require.NoError(t, engine.Deliver(ctx, adjTransportReceived(adjTransportSource, 100, "10.1.0.0/24", "180a0100")))
			route := ribevents.ValidationRoute{
				Peer:   netip.MustParseAddr(adjTransportSource),
				Family: family.IPv4Unicast,
				Prefix: netip.MustParsePrefix("10.1.0.0/24"),
			}
			require.True(t, ribevents.ValidationEnabled(), "the enabled gate must count in this process")
			require.False(t, ribevents.RouteEligible(route, 0), "a route awaiting validation must not be eligible")

			_, err = engine.Execute(ctx, "request bgp adj-rib-in disable-validation", nil)
			require.NoError(t, err)
			require.Equal(t, []bool{true, false}, applyGateAnnouncements(t, engine))
		})
	}
}

// applyGateAnnouncements applies every bgp-rib/validation-gate emit-event the
// engine has seen to this process's gate, in order, as the daemon's
// subscription does, and returns the announced states.
func applyGateAnnouncements(t *testing.T, engine *pluginengine.Engine) []bool {
	t.Helper()
	var states []bool
	for _, call := range engine.RPCs() {
		if call.Method != rpc.MethodEmitEvent {
			continue
		}
		var input rpc.EmitEventInput
		require.NoError(t, json.Unmarshal(call.Params, &input))
		if input.Namespace != ribevents.Namespace {
			continue
		}
		if input.EventType != ribevents.EventValidationGate {
			continue
		}
		var state ribevents.ValidationGateState
		require.NoError(t, json.Unmarshal([]byte(input.Event), &state), input.Event)
		ribevents.NoteValidationGate(state)
		states = append(states, state.Enabled)
	}
	return states
}

// TestForkedGateServesValidatedVerdicts proves a process that cannot read the
// receive store's lookup in memory reads the store's verdicts through the
// eligibility command its announcement names, on both transports: a forked
// store with validation enabled advertises its validated routes, not nothing.
//
// VALIDATES: the validation-gate announcement names the store's eligibility
// command; through it, ribevents.RouteEligible answers eligible for a route
// the store accepted (its received generation too) and refuses a route still
// pending and a generation the store does not hold; RoutePresent answers the
// pending route present.
// PREVENTS: a forked bgp-adj-rib-in refusing every route in the daemon (the
// fail-closed stand-in for a lookup that did not cross the boundary).
// Method: run the real entry point through the fake engine, apply the
// announcements, route this process's gate dispatch to the plugin's own
// command handler through the engine, receive two routes, accept one.
func TestForkedGateServesValidatedVerdicts(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
			ribevents.RegisterValidationLookup(nil, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			// Registered before the plugin starts: the first registration
			// wins, and the plugin registers its own (for the FlowSpec gate)
			// as it runs, which here shares this test's process.
			var engine *pluginengine.Engine
			release := ribevents.RegisterGateDispatch(func(ctx context.Context, command string, args ...string) (string, json.RawMessage, error) {
				data, err := engine.Execute(ctx, command, args)
				if err != nil {
					return "", nil, err
				}
				return "done", data, nil
			})
			t.Cleanup(release)
			engine = pluginengine.Start(t, transport, runAdjRIBInPlugin)
			// The bridge transport registers the store's lookup in memory
			// (the daemon case); drop it so this process reads the store only
			// through the command, as the daemon does while the store is forked.
			require.NoError(t, engine.Deliver(ctx, adjTransportStateUp(adjTransportSource)))
			_, err := engine.Execute(ctx, "request bgp adj-rib-in enable-validation", nil)
			require.NoError(t, err)
			ribevents.RegisterValidationLookup(nil, nil)
			require.Equal(t, []bool{true}, applyGateAnnouncements(t, engine))

			require.NoError(t, engine.Deliver(ctx, adjTransportReceived(adjTransportSource, 100, "10.1.0.0/24", "180a0100")))
			require.NoError(t, engine.Deliver(ctx, adjTransportReceived(adjTransportSource, 101, "10.2.0.0/24", "180a0200")))
			_, err = engine.Execute(ctx, "request bgp adj-rib-in accept-routes", []string{adjTransportSource, "ipv4/unicast", "10.1.0.0/24", "0", "1"})
			require.NoError(t, err)

			peer := netip.MustParseAddr(adjTransportSource)
			accepted := ribevents.ValidationRoute{Peer: peer, Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.1.0.0/24")}
			pending := ribevents.ValidationRoute{Peer: peer, Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.2.0.0/24")}
			require.True(t, ribevents.RouteEligible(accepted, 0), "an accepted route must be eligible through the command")
			require.True(t, ribevents.RouteEligible(accepted, 100), "the accepted route's received generation must be eligible")
			require.False(t, ribevents.RouteEligible(accepted, 99), "a generation the store does not hold must be refused")
			require.False(t, ribevents.RouteEligible(pending, 0), "a route awaiting validation must be refused")
			require.True(t, ribevents.RoutePresent(pending), "a pending route is still owned by the store")
		})
	}
}
