// Design: docs/guide/graceful-restart.md -- the GR producer and owned verdict lifetime.

package gr

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRetentionProducerBusLifecycle runs the actual GR DOWN handler and SDK
// emitter through both upstream transports into the real bus and two independent
// process delivery workers. Only the state owner gets a DOWN; FIFO delivery must
// record its legitimate verdict first without creating unrelated ledger entries.
func TestRetentionProducerBusLifecycle(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(map[bool]string{false: "rpc", true: "bridge"}[direct], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			bus, err := pluginserver.NewServer(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			var owner, unrelated retention.Ledger
			owner.Begin(removalTestPeer)
			const down = "owner-down"
			decisions := make(chan map[family.Family]bool, 1)
			ownProc := retentionBusRecipient(t, bus, "owner", func(text string) error {
				if text == down {
					decisions <- owner.Take(removalTestPeer)
					return nil
				}
				envelope, ok := retention.Accepts(text)
				if !ok {
					return errors.New("owner received a non-retention event")
				}
				return owner.Record(envelope)
			})
			otherProc := retentionBusRecipient(t, bus, "unrelated", func(text string) error {
				envelope, ok := retention.Accepts(text)
				if !ok {
					return errors.New("unrelated recipient received a non-retention event")
				}
				return unrelated.Record(envelope)
			})
			gp := newGRPluginWithGRCap()
			gp.sdk = retentionBusEmitter(t, ctx, bus, direct)
			var commands []string
			gp.dispatchHook = func(command string, _ ...string) { commands = append(commands, command) }
			t.Cleanup(func() {
				gp.state.removePeer(removalTestPeer)
				gp.state.removePeer("192.0.2.99")
			})
			gp.handleStructuredState(removalTestPeer, rpc.SessionStateDown, rpc.ReasonTCPFailure)
			if len(commands) == 0 {
				t.Fatal("GR DOWN did not run its retention procedure")
			}
			if !ownProc.Deliver(process.EventDelivery{Output: down}) {
				t.Fatal("owning DOWN not enqueued")
			}
			if err := ownProc.DrainEventsApplied(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case kept := <-decisions:
				if !kept[family.IPv4Unicast] {
					t.Fatal("actual GR verdict did not precede owning DOWN")
				}
			case <-ctx.Done():
				t.Fatal("owning DOWN was not applied")
			}
			// GR also handles a peer outside both forwarders' state populations.
			const unowned = "192.0.2.99"
			gp.peerCaps[unowned] = gp.peerCaps[removalTestPeer]
			gp.handleStructuredState(unowned, rpc.SessionStateDown, rpc.ReasonTCPFailure)
			gp.handleStructuredState(unowned, rpc.SessionStateDown, rpc.ReasonPeerRemoved)
			for _, proc := range []*process.Process{ownProc, otherProc} {
				if err := proc.DrainEventsApplied(ctx); err != nil {
					t.Fatal(err)
				}
			}
			for _, ledger := range []*retention.Ledger{&owner, &unrelated} {
				for _, peer := range []string{removalTestPeer, unowned} {
					if kept := ledger.Take(peer); kept != nil {
						t.Errorf("unowned or consumed peer %s retained a verdict: %v", peer, kept)
					}
				}
			}
		})
	}
}

// retentionBusRecipient owns a real acknowledged deliver-batch connection and
// its joined callback reader; it makes no assumptions about batching boundaries.
func retentionBusRecipient(t *testing.T, bus *pluginserver.Server, name string, apply func(string) error) *process.Process {
	t.Helper()
	engine, consumer := net.Pipe()
	proc := process.NewProcess(plugin.PluginConfig{Name: name})
	proc.SetConn(ipc.NewPluginConn(engine, engine))
	proc.SetEnvelope(true)
	proc.SetRunning(true)
	proc.StartDelivery(t.Context())
	bus.Subscriptions().Add(proc, &pluginserver.Subscription{
		Namespace: events.LookupNamespaceID(retention.Namespace),
		EventType: events.LookupEventTypeID(retention.EventType),
	})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		wire := rpc.NewConn(consumer, consumer)
		// The process lifetime ends when cleanup closes the connection.
		for {
			request, err := wire.ReadRequest(t.Context())
			if err != nil {
				return
			}
			batch, err := rpc.ParseBatchEvents(request.Params)
			if err != nil {
				t.Error(err)
				return
			}
			for _, raw := range batch {
				var text string
				if err := json.Unmarshal(raw, &text); err != nil {
					t.Error(err)
					return
				}
				if err := apply(text); err != nil {
					t.Error(err)
					return
				}
			}
			if err := wire.SendResult(t.Context(), request.ID, nil); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		proc.Stop()
		joined, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proc.Wait(joined); err != nil {
			t.Error(err)
		}
		if err := consumer.Close(); err != nil {
			t.Error(err)
		}
		select {
		case <-finished:
		case <-joined.Done():
			t.Error("retention recipient reader did not join")
		}
	})
	return proc
}

// retentionBusEmitter attaches the actual SDK producer to the bus through its
// typed bridge or JSON emit-event RPC. Cleanup closes and joins the host reader.
func retentionBusEmitter(t *testing.T, ctx context.Context, bus *pluginserver.Server, direct bool) *sdk.Plugin {
	t.Helper()
	engine, producer := net.Pipe()
	emit := func(namespace, event, direction, peer, payload string) (int, error) {
		if direction != "" {
			return 0, errors.New("retention emitter unexpectedly selected a direction")
		}
		if peer != "" {
			return 0, errors.New("retention emitter unexpectedly required a peer namespace grant")
		}
		return bus.EmitEngineEvent(namespace, event, payload)
	}
	finished := make(chan struct{})
	var p *sdk.Plugin
	if direct {
		bridge := rpc.NewDirectBridge()
		bridge.SetEmitEvent(emit)
		bridge.SetReady()
		p = sdk.NewWithConn("gr-bus-producer", rpc.NewBridgedConn(producer, bridge))
		close(finished)
	} else {
		p = sdk.NewWithConn("gr-bus-producer", producer)
		go func() {
			defer close(finished)
			wire := rpc.NewConn(engine, engine)
			// The producer lifetime ends when cleanup closes its SDK connection.
			for {
				request, err := wire.ReadRequest(ctx)
				if err != nil {
					return
				}
				if request.Method != rpc.MethodEmitEvent {
					t.Errorf("unexpected GR bus RPC %s", request.Method)
					return
				}
				var input rpc.EmitEventInput
				if err := json.Unmarshal(request.Params, &input); err != nil {
					t.Error(err)
					return
				}
				delivered, err := emit(input.Namespace, input.EventType, input.Direction, input.PeerAddress, input.Event)
				if err != nil {
					t.Error(err)
					return
				}
				if err := wire.SendResult(ctx, request.ID, &rpc.EmitEventOutput{Delivered: delivered}); err != nil {
					return
				}
			}
		}()
	}
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("retention producer reader did not join")
		}
	})
	return p
}
