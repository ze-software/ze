// Design: docs/guide/graceful-restart.md -- verdict ownership follows state delivery.

package rs

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
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
			bus.Subscriptions().Add(proc, &pluginserver.Subscription{Namespace: events.LookupNamespaceID(retention.Namespace), EventType: events.LookupEventTypeID(retention.EventType)})
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
			emit := func(key workerKey, families []family.Family) {
				t.Helper()
				payload, err := retention.Encode(key.owner, key.connectionID, key.address.String(), families)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := bus.EmitEngineEvent(retention.Namespace, retention.EventType, payload); err != nil {
					t.Fatal(err)
				}
			}
			take := func(key workerKey) map[family.Family]bool {
				t.Helper()
				kept, err := rs.retained.Take(key.owner, key.connectionID)
				if err != nil {
					t.Fatal(err)
				}
				return kept
			}
			const owned = "192.0.2.1"
			original := rsFixtureKey(owned, 1, 101)
			deliver("peer 192.0.2.1 remote as 65001 state up reason attached peer-owner 1 connection-id 101 destination-session 201 session-generation 7 rs-client true")
			if err := rs.acquireSource(original, 301); err != nil {
				t.Fatal(err)
			}
			native := withdrawalKey{owner: 1, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
			entries := map[withdrawalKey]withdrawalEntry{native: {owner: 1, connectionID: 101, sourceReceipt: 301, msgID: 1}}
			rs.withdrawals[owned] = entries
			emit(original, []family.Family{family.IPv4Unicast})
			deliver("peer 192.0.2.1 remote as 65001 state down session-generation 7 peer-owner 1 connection-id 101")
			if err := rs.settlePendingDown(ctx, original); err != nil {
				t.Fatal(err)
			}
			entry, exists := rs.withdrawals[owned][native]
			if !exists || !entry.retained || entry.owner != 1 || entry.connectionID != 101 || entry.sourceReceipt != 301 {
				t.Fatal("owned verdict lost current exact retained family authority")
			}
			if kept := take(original); kept != nil {
				t.Fatal("DOWN left a verdict behind")
			}
			// The same display address's next physical lifetime owns no old verdict.
			successor := rsFixtureKey(owned, 1, 102)
			deliver("peer 192.0.2.1 remote as 65001 state up reason attached peer-owner 1 connection-id 102 destination-session 202 session-generation 8 rs-client true")
			emit(successor, nil)
			deliver("peer 192.0.2.1 remote as 65001 state down session-generation 8 peer-owner 1 connection-id 102")
			if err := rs.settlePendingDown(ctx, successor); err != nil {
				t.Fatal(err)
			}
			if kept := take(successor); kept != nil {
				t.Fatal("empty verdict survived its DOWN")
			}
			const churn = 32
			for i := range churn {
				peer := "198.51.100." + strconv.Itoa(i+1)
				unowned := rsFixtureKey(peer, uint64(i+1000), uint64(i+2000))
				if i%2 == 0 {
					emit(unowned, []family.Family{family.IPv4Unicast})
				} else {
					emit(unowned, nil)
				}
			}
			emit(successor, []family.Family{family.IPv4Unicast})
			if err := proc.DrainEventsApplied(ctx); err != nil {
				t.Fatal(err)
			}
			for i := range churn {
				peer := "198.51.100." + strconv.Itoa(i+1)
				if kept := take(rsFixtureKey(peer, uint64(i+1000), uint64(i+2000))); kept != nil {
					t.Errorf("unowned peer %s left a verdict: %v", peer, kept)
				}
			}
			if kept := take(successor); kept != nil {
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
	for index, reason := range []string{rpc.ReasonNotification, rpc.ReasonPeerRemoved} {
		physical := uint64(index + 101)
		original := rsFixtureKey(peer, 1, physical)
		rs.handleState(parseStructuredState(&rpc.StructuredEvent{PeerAddress: peer, PeerOwner: 1, ConnectionID: physical, DestinationSession: uint64(index + 201), SessionGeneration: uint64(index + 7), State: rpc.SessionStateUp, Reason: rpc.ReasonAttached, RSClient: true}))
		if err := rs.acquireSource(original, uint64(index+301)); err != nil {
			t.Fatal(err)
		}
		payload, err := retention.Encode(1, physical, peer, []family.Family{family.IPv4Unicast})
		if err != nil {
			t.Fatal(err)
		}
		envelope := `{"namespace":"` + retention.Namespace + `","event":"` + retention.EventType + `","payload":` + payload + `}`
		rs.dispatchText(envelope)
		native := withdrawalKey{owner: 1, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
		entries := map[withdrawalKey]withdrawalEntry{native: {owner: 1, connectionID: physical, sourceReceipt: uint64(index + 301), msgID: uint64(index + 1)}}
		rs.withdrawalMu.Lock()
		rs.withdrawals[peer] = entries
		rs.withdrawalMu.Unlock()
		rs.handleState(parseStructuredState(&rpc.StructuredEvent{PeerAddress: peer, PeerOwner: 1, ConnectionID: physical, State: rpc.SessionStateDown, Reason: reason, SessionGeneration: uint64(index + 7)}))
		if err := rs.settlePendingDown(t.Context(), original); err != nil {
			t.Fatal(err)
		}
		rs.withdrawalMu.Lock()
		entry, exists := rs.withdrawals[peer][native]
		rs.withdrawalMu.Unlock()
		if !exists || !entry.retained || entry.owner != 1 || entry.connectionID != physical || entry.sourceReceipt != uint64(index+301) {
			t.Fatal("structured lifetime lost exact GR family authority before DOWN")
		}
		rs.dispatchText(envelope)
		kept, err := rs.retained.Take(1, physical)
		if err != nil {
			t.Fatal(err)
		}
		if kept != nil {
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
