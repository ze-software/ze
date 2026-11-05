// Design: docs/architecture/plugin/rib-storage-design.md -- validation eligibility.
// Related: rib_event_burst_test.go -- real emit and enveloped delivery frames.
package rib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestDisableValidationBurstRecoversActualSelection drives the registered
// receive plugin, its command callback, real SDK/RPC emission and the selecting
// RIB's registered typed subscriber. A full pending table must become selected
// without another UPDATE; command success cannot conceal a discarded event.
func TestDisableValidationBurstRecoversActualSelection(t *testing.T) {
	previous := eventBusPtr.Load()
	t.Cleanup(func() { eventBusPtr.Store(previous); ribevents.RegisterValidationLookup(nil, nil) })
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	t.Cleanup(ribevents.ValidationChange.Subscribe(bus, r.validationChanged))
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	t.Cleanup(func() {
		r.SetLocRIB(nil)
		for _, routes := range r.bgpPeers {
			routes.Release()
		}
	})
	bridge := startBurstValidationReceiver(t, bus)
	const routes = 180000
	if err := bridge.DeliverStructured([]any{&rpc.StructuredEvent{
		EventType: rpc.EventKindState, State: rpc.SessionStateUp, PeerAddress: "192.0.2.1",
		PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000,
	}}); err != nil {
		t.Fatal(err)
	}
	setupCtx, setupCancel := context.WithTimeout(t.Context(), 5*time.Second)
	_, err := bridge.ExecuteCommand(setupCtx, "fixture-timeout", "request bgp adj-rib-in enable-validation",
		[]string{"timeout", "65535"}, "")
	setupCancel()
	if err != nil {
		t.Fatal(err)
	}
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	if err != nil {
		t.Fatal(err)
	}
	for start := 0; start < routes; start += 4096 {
		end := min(start+4096, routes)
		body := []byte{0, 0, 0, 14, 0x40, 1, 1, 0, 0x40, 2, 0,
			0x40, 3, 4, 192, 0, 2, 9}
		for index := start; index < end; index++ {
			body = append(body, 32, 10, byte(index>>16), byte(index>>8), byte(index))
		}
		update := wireu.NewWireUpdate(body, ctxID)
		attributes, err := update.Attrs()
		if err != nil {
			t.Fatal(err)
		}
		event := &rpc.StructuredEvent{EventType: rpc.EventKindUpdate,
			PeerAddress: "192.0.2.1", PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000,
			SourcePolicyEpoch: 5,
			RawMessage: &bgptypes.RawMessage{MessageID: uint64(start + 1), RawBytes: body,
				WireUpdate: update, AttrsWire: attributes}}
		r.handleReceivedStructured(event)
		if err := bridge.DeliverStructured([]any{event}); err != nil {
			t.Fatal(err)
		}
	}
	if selected := r.collectBestPaths()[family.IPv4Unicast]; len(selected) != 0 {
		t.Fatalf("pending routes selected before disable: %d", len(selected))
	}
	for index := range routes {
		key := ribevents.ValidationRoute{Peer: netip.MustParseAddr("192.0.2.1"),
			Family: family.IPv4Unicast, Prefix: burstPrefix(index)}
		if !ribevents.RoutePresent(key) || ribevents.RouteEligible(key, 0) {
			t.Fatalf("admitted route %d is not still pending before disable", index)
		}
	}
	bus.mu.Lock()
	admitted := 0
	for _, event := range bus.events {
		if event.Namespace == ribevents.Namespace && event.EventType == ribevents.EventValidationChange {
			changes, ok := event.Payload.([]ribevents.ValidationRoute)
			if !ok {
				t.Error("admission event lost its registered payload type")
				continue
			}
			admitted += len(changes)
		}
	}
	bus.events = nil
	bus.mu.Unlock()
	if admitted != routes {
		t.Fatalf("admission delivered %d/%d pending-path notifications", admitted, routes)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	seen := make([]bool, routes)
	delivered := 0
	frames := 0
	wire := burstSDKBus(t, func(input rpc.EmitEventInput) error {
		payload, err := burstDeliveredPayload(input)
		if err != nil {
			return err
		}
		if input.EventType != ribevents.EventValidationChange {
			return nil
		}
		var changes []ribevents.ValidationRoute
		if err := json.Unmarshal(payload, &changes); err != nil {
			return err
		}
		for _, change := range changes {
			address := change.Prefix.Addr().As4()
			index := int(address[1])<<16 | int(address[2])<<8 | int(address[3])
			if index >= routes || seen[index] || change.Peer.String() != "192.0.2.1" || change.Family != family.IPv4Unicast {
				return fmt.Errorf("recovery change is missing, duplicated or names another path: %+v", change)
			}
			seen[index] = true
			delivered++
		}
		_, err = ribevents.ValidationChange.Emit(bus, changes)
		frames++
		return err
	})
	// The registered internal receiver still owns the real lookup. Its event
	// hand-off traverses the forked SDK framing before the selecting consumer.
	bridge.SetEmitEvent(func(namespace, eventType, direction, peer, event string) (int, error) {
		return wire.plugin.EmitEvent(ctx, namespace, eventType, direction, peer, event)
	})
	if err := ctx.Err(); err != nil {
		t.Fatalf("disable deadline expired before its command started: %v", err)
	}
	answer, err := bridge.ExecuteCommand(ctx, "disable-burst", "request bgp adj-rib-in disable-validation", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if answer.Status != rpc.StatusDone {
		t.Fatalf("disable status=%q", answer.Status)
	}
	if delivered != routes || frames < 2 {
		t.Fatalf("successful disable delivered %d/%d recovery changes in %d frames", delivered, routes, frames)
	}
	selected := r.collectBestPaths()[family.IPv4Unicast]
	if len(selected) != routes {
		t.Fatalf("selected %d/%d retained routes without another UPDATE", len(selected), routes)
	}
	for _, index := range []int{0, routes / 2, routes - 1} {
		path, ok := loc.Best(family.IPv4Unicast, burstPrefix(index))
		if !ok || path.NextHop != netip.MustParseAddr("192.0.2.9") {
			t.Fatalf("retained route %s did not reach the forwarding Loc-RIB: %+v present=%v", burstPrefix(index), path, ok)
		}
	}
}

// TestValidationSourcePolicyDuplicateFlushesOwedChanges exercises the actual
// lifecycle callback after a failed emission. Same-epoch retries must remain
// errors until the owed keys reach the consumer, then become no-op receipts.
func TestValidationSourcePolicyDuplicateFlushesOwedChanges(t *testing.T) {
	previous := eventBusPtr.Load()
	t.Cleanup(func() { eventBusPtr.Store(previous); ribevents.RegisterValidationLookup(nil, nil) })
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	bridge, ctx := startValidationReceiver(t, bus)
	if err := bridge.DeliverStructured([]any{&rpc.StructuredEvent{
		EventType: rpc.EventKindState, State: rpc.SessionStateUp, PeerAddress: "192.0.2.1",
		PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000,
	}}); err != nil {
		t.Fatal(err)
	}
	validationReceive(t, bridge, r, 100, 9)
	if _, err := bridge.ExecuteCommand(ctx, "disable", "request bgp adj-rib-in disable-validation", nil, ""); err != nil {
		t.Fatal(err)
	}
	attempts := 0
	accepted := 0
	bridge.SetEmitEvent(func(_, eventType, _, _, event string) (int, error) {
		if eventType != ribevents.EventValidationChange {
			return 0, nil
		}
		attempts++
		if attempts <= 2 {
			return 0, errors.New("validation notification refused")
		}
		var changes []ribevents.ValidationRoute
		if err := json.Unmarshal([]byte(event), &changes); err != nil {
			return 0, err
		}
		accepted += len(changes)
		return 1, nil
	})
	input := rpc.SourcePolicyInput{TransactionID: "validation-burst-owed",
		Operation: rpc.SourcePolicyPrepare,
		Source: rpc.SourcePolicyIdentity{Peer: "192.0.2.1", PeerOwner: 1,
			SessionGeneration: 1, PolicyEpoch: 5, ReceiveFrontier: 100, TemplateGeneration: 1},
		DeadlineMS: time.Now().Add(time.Minute).UnixMilli()}
	call := func() rpc.SourcePolicyOutput {
		t.Helper()
		encoded, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		answer, err := bridge.SendCallback(ctx, "ze-plugin-callback:source-policy", encoded)
		if err != nil {
			t.Fatal(err)
		}
		var output rpc.SourcePolicyOutput
		if err := json.Unmarshal(answer, &output); err != nil {
			t.Fatal(err)
		}
		return output
	}
	if output := call(); output.Status != rpc.StatusOK {
		t.Fatalf("prepare refused: %+v", output)
	}
	input.Operation = rpc.SourcePolicyApply
	input.NextPolicyEpoch = 6
	for attempt := range 2 {
		if output := call(); output.Status != rpc.StatusError {
			t.Fatalf("failed flush attempt %d reported successful lifecycle receipt: %+v", attempt, output)
		}
	}
	if output := call(); output.Status != rpc.StatusOK {
		t.Fatalf("same-epoch retry did not flush owed changes: %+v", output)
	}
	if attempts != 3 || accepted != 1 {
		t.Fatalf("retry attempts=%d delivered=%d, want 3 and 1", attempts, accepted)
	}
	if output := call(); output.Status != rpc.StatusOK {
		t.Fatalf("settled duplicate failed: %+v", output)
	}
	if attempts != 3 || accepted != 1 {
		t.Fatalf("acknowledged changes replayed: attempts=%d delivered=%d", attempts, accepted)
	}
}

// startBurstValidationReceiver gives startup and shutdown their own five-second
// bounds. Admission does not consume the later disable command's deadline, and
// an old startup deadline cannot falsely report a failed shutdown.
func startBurstValidationReceiver(t *testing.T, bus *testEventBus) *rpc.DirectBridge {
	t.Helper()
	registration := registry.Lookup("bgp-adj-rib-in")
	if registration == nil {
		t.Fatal("Adj-RIB-In registration missing")
	}
	previousValidator := rpc.GetBatchValidator()
	bridge := rpc.NewDirectBridge()
	bridge.SetEmitEvent(func(namespace, eventType, _, _, event string) (int, error) {
		if namespace != ribevents.Namespace {
			return 0, errors.New("unexpected event namespace")
		}
		switch eventType {
		case ribevents.EventValidationGate:
			var state ribevents.ValidationGateState
			if err := json.Unmarshal([]byte(event), &state); err != nil {
				return 0, err
			}
			return ribevents.ValidationGate.Emit(bus, state)
		case ribevents.EventValidationChange:
			var changes []ribevents.ValidationRoute
			if err := json.Unmarshal([]byte(event), &changes); err != nil {
				return 0, err
			}
			return ribevents.ValidationChange.Emit(bus, changes)
		case ribevents.EventGateQuery:
			return 0, nil
		}
		return 0, errors.New("unexpected receive-store event")
	})
	pluginEnd, engineEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	exited := make(chan int, 1)
	go func() { exited <- registration.RunEngine(rpc.NewBridgedConn(pluginEnd, bridge)) }()
	t.Cleanup(func() {
		bridge.CloseCallbacks()
		_ = mux.Close()
		_ = pluginEnd.Close()
		_ = engineEnd.Close()
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		select {
		case <-exited:
		case <-deadline.C:
			t.Error("Adj-RIB-In did not stop after closing its connection")
		}
		rpc.RegisterBatchValidator(previousValidator)
	})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for stage := range 3 {
		select {
		case request, ok := <-mux.Requests():
			if !ok {
				t.Fatal("receive-store startup connection closed")
			}
			if err := mux.SendResult(ctx, request.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}); err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		switch stage {
		case 0:
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", &rpc.ConfigureInput{}); err != nil {
				t.Fatal(err)
			}
		case 1:
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:share-registry", &rpc.ShareRegistryInput{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := bridge.SendCallback(ctx, "ze-plugin-callback:post-startup", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.ExecuteCommand(ctx, "enable", "request bgp adj-rib-in enable-validation", nil, ""); err != nil {
		t.Fatal(err)
	}
	return bridge
}
