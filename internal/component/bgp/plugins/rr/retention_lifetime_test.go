// Design: docs/guide/graceful-restart.md -- verdict ownership follows state delivery.

package rr

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
			rr, _ := newPeerDownReflector(t, rrOriginalFixture(t, "192.0.2.1", 701, 41))
			t.Cleanup(rr.stopDepartures)
			bus, err := pluginserver.NewServer(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			proc := retentionLifetimeProcess(t, direct, rr.dispatchText)
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
			emit := func(peer string, owner, connection uint64, families []family.Family) {
				t.Helper()
				payload, err := retention.Encode(owner, connection, peer, families)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := bus.EmitEngineEvent(retention.Namespace, retention.EventType, payload); err != nil {
					t.Fatal(err)
				}
			}
			const owned = "192.0.2.1"
			up := "peer " + owned + " remote as 65001 state up peer-owner 701 connection-id 41 session-generation 7 destination-session 141 reason attached is-ibgp true rr-client true"
			down := "peer " + owned + " remote as 65001 state down peer-owner 701 connection-id 41 session-generation 7"
			deliver(up)
			entries := map[withdrawalKey]withdrawalInfo{{owner: 701, connection: 41, route: "retained"}: {SourceOwner: 701, ConnectionID: 41, SourceReceipt: 70, Family: family.IPv4Unicast}}
			rr.withdrawals[owned] = entries
			emit(owned, 701, 41, []family.Family{family.IPv4Unicast})
			deliver(down)
			if len(entries) != 1 {
				t.Fatal("owned verdict lost its retained family before DOWN")
			}
			for _, info := range entries {
				if !info.Retained || info.Closing {
					t.Fatal("owned family became a departure instead of retained authority")
				}
			}
			if kept, err := rr.retained.Take(701, 41); err != nil || kept != nil {
				t.Fatal("DOWN left a verdict behind")
			}
			// The same address's next physical lifetime starts without old retention.
			up = "peer " + owned + " remote as 65001 state up peer-owner 701 connection-id 44 session-generation 8 destination-session 144 reason attached is-ibgp true rr-client true"
			down = "peer " + owned + " remote as 65001 state down peer-owner 701 connection-id 44 session-generation 8"
			deliver(up)
			emit(owned, 701, 44, nil)
			deliver(down)
			if kept, err := rr.retained.Take(701, 44); err != nil || kept != nil {
				t.Fatal("empty verdict survived its DOWN")
			}
			const churn = 32
			for i := range churn {
				peer := "198.51.100." + strconv.Itoa(i+1)
				if i%2 == 0 {
					emit(peer, uint64(1000+i), uint64(2000+i), []family.Family{family.IPv4Unicast})
				} else {
					emit(peer, uint64(1000+i), uint64(2000+i), nil)
				}
			}
			// A detached former owner is as unrelated as a never-attached peer.
			emit(owned, 701, 41, []family.Family{family.IPv4Unicast})
			if err := proc.DrainEventsApplied(ctx); err != nil {
				t.Fatal(err)
			}
			for i := range churn {
				peer := "198.51.100." + strconv.Itoa(i+1)
				if kept, err := rr.retained.Take(uint64(1000+i), uint64(2000+i)); err != nil || kept != nil {
					t.Errorf("unowned peer %s left a verdict: %v", peer, kept)
				}
			}
			if kept, err := rr.retained.Take(701, 41); err != nil || kept != nil {
				t.Error("detached former owner reinstalled its original physical verdict")
			}
			if kept, err := rr.retained.Take(701, 44); err != nil || kept != nil {
				t.Error("former owner's terminal DOWN did not close admission")
			}
		})
	}
}

// TestRetentionStructuredOwnershipLifetime checks the structured UP/DOWN
// callbacks, including an attached session, against the same verdict consumer.
// Recreating an address must not inherit the previous lifetime's verdict.
func TestRetentionStructuredOwnershipLifetime(t *testing.T) {
	rr, _ := newPeerDownReflector(t, rrOriginalFixture(t, "192.0.2.1", 701, 41))
	t.Cleanup(rr.stopDepartures)
	const peer = "192.0.2.1"
	for index, reason := range []string{rpc.ReasonNotification, rpc.ReasonPeerRemoved} {
		connection := uint64(41 + index)
		rr.handleStructuredState(&rpc.StructuredEvent{
			PeerAddress: peer, PeerOwner: 701, ConnectionID: connection, SessionGeneration: 7, DestinationSession: 141 + uint64(index), State: rpc.SessionStateUp, Reason: rpc.ReasonAttached, IsIBGP: true, RouteReflectorClient: true,
		})
		rrFixtureStart(t, rr, rrOriginalFixture(t, peer, 701, connection), 70+uint64(index), 7, 141+uint64(index))
		rr.dispatchText(retentionEvent(t, peer, 701, connection, family.IPv4Unicast))
		entries := map[withdrawalKey]withdrawalInfo{{owner: 701, connection: connection, route: "retained"}: {SourceOwner: 701, ConnectionID: connection, SourceReceipt: 70 + uint64(index), Family: family.IPv4Unicast}}
		rr.withdrawals[peer] = entries
		rr.handleStructuredState(&rpc.StructuredEvent{
			PeerAddress: peer, PeerOwner: 701, ConnectionID: connection, State: rpc.SessionStateDown, Reason: reason,
			SessionGeneration: 7,
		})
		if len(entries) != 1 {
			t.Fatal("structured lifetime lost its GR verdict before DOWN")
		}
		for _, info := range entries {
			if !info.Retained || info.Closing {
				t.Fatal("structured retained family became a departure")
			}
		}
		rr.dispatchText(retentionEvent(t, peer, 701, connection, family.IPv4Unicast))
		if kept, err := rr.retained.Take(701, connection); err != nil || kept != nil {
			t.Fatal("structured DOWN left admission open")
		}
	}
}

// retentionLifetimeProcess owns one process delivery worker and, for RPC, one
// callback reader. Cleanup stops and joins both, including on failed assertions.
func retentionLifetimeProcess(t *testing.T, direct bool, apply func(string) error) *process.Process {
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
			if err := apply(text); err != nil {
				return err
			}
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
