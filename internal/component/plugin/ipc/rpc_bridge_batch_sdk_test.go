// Design: docs/architecture/api/process-protocol.md -- batched events survive negotiated bridge cutover.
package ipc_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestPluginConnBridgeBatchCutover holds the final startup acknowledgement while
// SendDeliverBatch queues both OPEN directions. Only the real SDK may consume
// the callback, activate its bridge, close IPC and acknowledge handler application.
// The pre-activation queue fence makes the old IPC-only producer deterministically
// fail, rather than depending on whether activation wins a scheduling race.
func TestPluginConnBridgeBatchCutover(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	engineEnd, pluginEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	engine := ipc.NewMuxPluginConn(mux)
	bridge := rpc.NewDirectBridge()
	consumer := sdk.NewWithConn("batch-cutover-consumer", rpc.NewBridgedConn(pluginEnd, bridge))
	var mu sync.Mutex
	var applied []string
	capabilities := make(map[string]string)
	refused := errors.New("consumer refuses non-OPEN event")
	consumer.OnEvent(func(event string) error {
		var input struct {
			BGP struct {
				Message struct {
					Type      string `json:"type"`
					Direction string `json:"direction"`
				} `json:"message"`
				Open struct {
					Capabilities []struct {
						Code  uint8  `json:"code"`
						Value string `json:"value"`
					} `json:"capabilities"`
				} `json:"open"`
			} `json:"bgp"`
		}
		if err := json.Unmarshal([]byte(event), &input); err != nil {
			return fmt.Errorf("decode consumer event: %w", err)
		}
		if input.BGP.Message.Type != "open" {
			return refused
		}
		mu.Lock()
		defer mu.Unlock()
		for _, capability := range input.BGP.Open.Capabilities {
			if capability.Code == 71 {
				capabilities[input.BGP.Message.Direction] = capability.Value
			}
		}
		applied = append(applied, event)
		return nil
	})
	runDone := make(chan error, 1)
	go func() { runDone <- consumer.Run(ctx, sdk.Registration{}) }()
	t.Cleanup(func() {
		cancel()
		if err := consumer.Close(); err != nil {
			t.Errorf("close SDK: %v", err)
		}
		if err := engine.Close(); err != nil {
			t.Errorf("close engine: %v", err)
		}
		select {
		case <-runDone:
		case <-time.After(5 * time.Second):
			t.Error("SDK lifecycle did not stop")
		}
	})

	// Drive the real SDK startup, stopping just before the final OK. No test
	// installs bridge event handlers or calls SetReady on the SDK's behalf.
	for _, stage := range []struct {
		request  string
		callback string
	}{
		{rpc.MethodDeclareRegistration, "ze-plugin-callback:configure"},
		{"ze-plugin-engine:declare-capabilities", "ze-plugin-callback:share-registry"},
	} {
		request, err := engine.ReadRequest(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if request.Method != stage.request {
			t.Fatalf("startup request = %q, want %q", request.Method, stage.request)
		}
		if err := engine.SendOK(ctx, request.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := engine.CallRPC(ctx, stage.callback, struct{}{}); err != nil {
			t.Fatal(err)
		}
	}
	ready, err := engine.ReadRequest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ready.Method != "ze-plugin-engine:ready" {
		t.Fatalf("final startup request = %q", ready.Method)
	}
	var negotiated rpc.ReadyInput
	if err := json.Unmarshal(ready.Params, &negotiated); err != nil {
		t.Fatal(err)
	}
	if negotiated.Transport != "bridge" {
		t.Fatalf("SDK transport = %q, want bridge", negotiated.Transport)
	}
	engine.SetBridge(bridge)
	events := []string{
		`{"type":"bgp","bgp":{"message":{"type":"open","direction":"sent"},"peer":{"remote":{"address":"::1"}},"open":{"capabilities":[{"code":71,"value":"00010180000e10"}]}}}`,
		`{"type":"bgp","bgp":{"message":{"type":"open","direction":"received"},"peer":{"remote":{"address":"::1"}},"open":{"capabilities":[{"code":71,"value":"0001018000003c"}]}}}`,
	}
	delivered := make(chan error, 1)
	go func() { delivered <- engine.SendDeliverBatch(ctx, events) }()
	// Observe queue occupancy without removing a callback or manufacturing its
	// result. The SDK is still blocked on ready, so this batch cannot drain yet.
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for len(bridge.CallbackCh()) == 0 {
		select {
		case err := <-delivered:
			t.Fatalf("batch ended before SDK activation: %v", err)
		case <-ctx.Done():
			t.Fatal("negotiated batch did not reach bridge before activation")
		case <-tick.C:
		}
	}
	if bridge.Ready() {
		t.Fatal("SDK activated before final ready acknowledgement")
	}
	mu.Lock()
	appliedBeforeReady := len(applied)
	mu.Unlock()
	if appliedBeforeReady != 0 {
		t.Fatal("consumer applied the batch before SDK startup completed")
	}
	if err := engine.SendOK(ctx, ready.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-delivered:
		if err != nil {
			t.Fatalf("SDK batch application: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("SDK did not apply queued batch after activation")
	}
	select {
	case <-mux.Done():
	case <-ctx.Done():
		t.Fatal("SDK did not retire startup IPC")
	}
	// A handler refusal must cross the real SDK batch decoder and callback
	// response path too; transport success alone is not application success.
	if err := engine.SendDeliverBatch(ctx, []string{`{"bgp":{"message":{"type":"state"}}}`}); !errors.Is(err, refused) {
		t.Fatalf("SDK batch consumer refusal = %v, want %v", err, refused)
	}
	mu.Lock()
	gotEvents := append([]string(nil), applied...)
	gotSent, gotReceived := capabilities["sent"], capabilities["received"]
	mu.Unlock()
	if !reflect.DeepEqual(gotEvents, events) {
		t.Fatalf("SDK-applied event sequence = %q, want %q exactly once", gotEvents, events)
	}
	if gotSent != "00010180000e10" || gotReceived != "0001018000003c" {
		t.Fatalf("consumer capability state: sent=%q received=%q", gotSent, gotReceived)
	}
	if err := consumer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendDeliverBatch(ctx, events); !errors.Is(err, rpc.ErrBridgeClosed) {
		t.Fatalf("batch after SDK shutdown = %v, want ErrBridgeClosed", err)
	}
}
