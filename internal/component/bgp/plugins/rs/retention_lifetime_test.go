// Design: docs/guide/graceful-restart.md -- verdict ownership follows state delivery.

package rs

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
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
)

// TestRetentionBusOwnershipLifetime delivers global GR verdicts through the real
// bus and process FIFO, with both bridge and acknowledged RPC delivery. Only the
// forwarder receiving a peer's state lifecycle may keep its verdict. Churn and
// empty verdicts must leave no records for peers this forwarder never owned.
func TestRetentionBusOwnershipLifetime(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(map[bool]string{false: "rpc", true: "bridge"}[direct], func(t *testing.T) {
			rs := newTestRouteServer(t)
			bus, err := pluginserver.NewServer(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			proc := retentionLifetimeProcess(t, direct, rs.dispatchText)
			proc.SetEnvelope(true)
			bus.Subscriptions().Add(proc, &pluginserver.Subscription{
				Namespace: events.LookupNamespaceID(retention.Namespace),
				EventType: events.LookupEventTypeID(retention.EventType),
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			deliver := func(text string) {
				t.Helper()
				if !proc.Deliver(process.EventDelivery{Output: text}) {
					t.Fatal("state event not enqueued")
				}
				if err := proc.DrainEventsApplied(ctx); err != nil {
					t.Fatal(err)
				}
			}
			emit := func(peer string, families []family.Family) {
				t.Helper()
				payload, err := retention.Encode(peer, families)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := bus.EmitEngineEvent(retention.Namespace, retention.EventType, payload); err != nil {
					t.Fatal(err)
				}
			}
			const owned = "192.0.2.1"
			up := "peer " + owned + " remote as 65001 state up reason attached"
			down := "peer " + owned + " remote as 65001 state down session-generation 7"
			deliver(up)
			entries := map[withdrawalKey]withdrawalEntry{{fam: family.IPv4Unicast}: {}}
			rs.withdrawals[owned] = entries
			emit(owned, []family.Family{family.IPv4Unicast})
			deliver(down)
			if len(entries) != 0 {
				t.Fatal("owned verdict did not retain its family before DOWN")
			}
			if kept := rs.retained.Take(owned); kept != nil {
				t.Fatal("DOWN left a verdict behind")
			}
			// The same address's next state lifetime starts without old retention.
			deliver(up)
			emit(owned, nil)
			deliver(down)
			if kept := rs.retained.Take(owned); kept != nil {
				t.Fatal("empty verdict survived its DOWN")
			}
			const churn = 32
			for i := range churn {
				peer := "198.51.100." + strconv.Itoa(i+1)
				if i%2 == 0 {
					emit(peer, []family.Family{family.IPv4Unicast})
				} else {
					emit(peer, nil)
				}
			}
			// A detached former owner is as unrelated as a never-attached peer.
			emit(owned, []family.Family{family.IPv4Unicast})
			if err := proc.DrainEventsApplied(ctx); err != nil {
				t.Fatal(err)
			}
			for i := range churn {
				peer := "198.51.100." + strconv.Itoa(i+1)
				if kept := rs.retained.Take(peer); kept != nil {
					t.Errorf("unowned peer %s left a verdict: %v", peer, kept)
				}
			}
			if kept := rs.retained.Take(owned); kept != nil {
				t.Error("former owner's terminal DOWN did not close admission")
			}
		})
	}
}

// TestRetentionStructuredOwnershipLifetime checks the structured UP/DOWN
// callbacks, including an attached session, against the same verdict consumer.
// Recreating an address must not inherit the previous lifetime's verdict.
func TestRetentionStructuredOwnershipLifetime(t *testing.T) {
	rs := newTestRouteServer(t)
	const peer = "192.0.2.1"
	for _, reason := range []string{rpc.ReasonNotification, rpc.ReasonPeerRemoved} {
		rs.handleState(parseStructuredState(&rpc.StructuredEvent{
			PeerAddress: peer, State: rpc.SessionStateUp, Reason: rpc.ReasonAttached,
		}))
		payload, err := retention.Encode(peer, []family.Family{family.IPv4Unicast})
		if err != nil {
			t.Fatal(err)
		}
		envelope := `{"namespace":"` + retention.Namespace + `","event":"` +
			retention.EventType + `","payload":` + payload + `}`
		rs.dispatchText(envelope)
		entries := map[withdrawalKey]withdrawalEntry{{fam: family.IPv4Unicast}: {}}
		rs.withdrawals[peer] = entries
		rs.handleState(parseStructuredState(&rpc.StructuredEvent{
			PeerAddress: peer, State: rpc.SessionStateDown, Reason: reason,
			SessionGeneration: 7,
		}))
		if len(entries) != 0 {
			t.Fatal("structured lifetime lost its GR verdict before DOWN")
		}
		rs.dispatchText(envelope)
		if kept := rs.retained.Take(peer); kept != nil {
			t.Fatal("structured DOWN left admission open")
		}
	}
}

// retentionLifetimeProcess owns one process delivery worker and, for RPC, one
// callback reader. Cleanup stops and joins both, including on failed assertions.
func retentionLifetimeProcess(t *testing.T, direct bool, apply func(string)) *process.Process {
	t.Helper()
	engine, consumer := net.Pipe()
	conn := ipc.NewPluginConn(engine, engine)
	proc := process.NewProcess(plugin.PluginConfig{Name: "retention-lifetime-probe"})
	proc.SetConn(conn)
	proc.SetRunning(true)
	proc.StartDelivery(t.Context())
	readerDone := make(chan struct{})
	applyBatch := func(params json.RawMessage) error {
		batch, err := rpc.ParseBatchEvents(params)
		if err != nil {
			return err
		}
		for _, raw := range batch {
			var text string
			if err := json.Unmarshal(raw, &text); err != nil {
				return err
			}
			apply(text)
		}
		return nil
	}
	var bridge *rpc.DirectBridge
	if direct {
		bridge = rpc.NewDirectBridge()
		bridge.SetReady()
		conn.SetBridge(bridge)
		go func() {
			defer close(readerDone)
			for callback := range bridge.CallbackCh() {
				callback.Result <- rpc.BridgeCallbackResult{Err: applyBatch(callback.Params)}
			}
		}()
	} else {
		go func() {
			defer close(readerDone)
			wire := rpc.NewConn(consumer, consumer)
			for {
				request, err := wire.ReadRequest(t.Context())
				if err != nil {
					return
				}
				if err := applyBatch(request.Params); err != nil {
					t.Error(err)
					return
				}
				if err := wire.SendResult(t.Context(), request.ID, nil); err != nil {
					return
				}
			}
		}()
	}
	t.Cleanup(func() {
		proc.Stop()
		if bridge != nil {
			bridge.CloseCallbacks()
		}
		joined, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proc.Wait(joined); err != nil {
			t.Error(err)
		}
		if err := consumer.Close(); err != nil {
			t.Error(err)
		}
		select {
		case <-readerDone:
		case <-time.After(5 * time.Second):
			t.Error("retention callback reader did not stop")
		}
	})
	return proc
}
