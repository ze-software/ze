// Design: docs/architecture/plugin/rib-storage-design.md -- FlowSpec authorization across the process boundary
// Related: rib_event_sdk.go -- sdkEventBus, dispatchReplayRequest
// Related: ../../../../test/pluginengine/pluginengine.go -- the engine side of the connection

package rib

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

// TestBestChangeReachesTheEngineOnBothTransports proves bgp-rib's best-change
// batches leave the RIB whichever process it runs in, and that it answers a
// replay request delivered through its subscriptions.
//
// VALIDATES: in the daemon (bridge) the batches go on the daemon bus; forked
// (wire) they reach the engine as bgp-rib/best-change emit-event RPCs whose
// JSON decodes into ribevents.BestChangeBatch, the type the engine decodes
// them into for its own subscribers; an enveloped bgp-rib/replay-request
// replays the table as a broadcast replay batch naming the stored prefix.
// PREVENTS: a forked bgp-rib publishing no best path at all (its bus pointer
// nil, every emission dropped in silence), and a forked RIB never answering
// sysrib's replay request (the old subscription went through the daemon bus).
// Method: run the real entry point through the fake engine on each transport,
// with a recording bus standing in for the daemon's on the bridge, feed two
// sessions and their routes, deliver one replay request, and decode every
// best-change batch the RIB produced.
func TestBestChangeReachesTheEngineOnBothTransports(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			previous := eventBusPtr.Load()
			t.Cleanup(func() { eventBusPtr.Store(previous) })
			eventBusPtr.Store(nil)
			bus := newTestEventBus()
			if transport == pluginengine.TransportBridge {
				SetEventBus(bus)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			engine, _ := startRIBOverTransport(t, ctx, transport)
			require.NotEmpty(t, bestChangeBatches(t, engine, bus), "a stored best path is published")

			request, err := json.Marshal(&replay.Request{ReplayID: replay.Broadcast})
			require.NoError(t, err)
			envelope, err := json.Marshal(rpc.EventEnvelope{Namespace: ribevents.Namespace, Event: ribevents.EventReplayRequest, Payload: request})
			require.NoError(t, err)
			require.NoError(t, engine.Deliver(ctx, string(envelope)))

			shared := netip.MustParsePrefix(transportShared)
			replayed := false
			for _, batch := range bestChangeBatches(t, engine, bus) {
				if !batch.IsReplay() {
					continue
				}
				for i := range batch.Changes {
					if batch.Changes[i].Prefix == shared {
						replayed = true
					}
				}
			}
			require.True(t, replayed, "the replay request replays the stored prefix as a replay batch")
		})
	}
}

// bestChangeBatches returns every best-change batch the RIB produced: the
// recording bus's on the bridge, the decoded emit-event RPCs on the wire.
func bestChangeBatches(t *testing.T, engine *pluginengine.Engine, bus *testEventBus) []*ribevents.BestChangeBatch {
	t.Helper()
	var batches []*ribevents.BestChangeBatch
	bus.mu.Lock()
	for _, event := range bus.events {
		if batch, ok := event.Payload.(*ribevents.BestChangeBatch); ok {
			batches = append(batches, batch)
		}
	}
	bus.mu.Unlock()
	for _, call := range engine.RPCs() {
		if call.Method != rpc.MethodEmitEvent {
			continue
		}
		var input rpc.EmitEventInput
		require.NoError(t, json.Unmarshal(call.Params, &input))
		if input.Namespace != ribevents.Namespace {
			continue
		}
		if input.EventType != ribevents.EventBestChange {
			continue
		}
		var batch ribevents.BestChangeBatch
		require.NoError(t, json.Unmarshal([]byte(input.Event), &batch), input.Event)
		batches = append(batches, &batch)
	}
	return batches
}

var _ ze.EventBus = (*testEventBus)(nil)
