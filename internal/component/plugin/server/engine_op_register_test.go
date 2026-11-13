// Design: docs/architecture/api/process-protocol.md -- owner engine operation registration.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	coreipc "github.com/ze-software/ze/internal/core/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestOwnerEngineOperationRegistration rejects invalid declarations before any
// publication, then checks duplicate and sealed writes preserve the first op.
func TestOwnerEngineOperationRegistration(t *testing.T) {
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const method = "ze-plugin:registration-test"
	handler := func(*Server, context.Context, *process.Process, json.RawMessage) (any, error) { return "first", nil }
	valid := EngineOperation{Method: method, Handler: handler, Startup: EngineOperationConfigureSafe}
	for _, testcase := range []struct {
		name   string
		modify func(*EngineOperation)
	}{
		{"namespace missing", func(d *EngineOperation) { d.Method = "registration-test" }},
		{"namespace forged", func(d *EngineOperation) { d.Method = "ze-other:registration-test" }},
		{"method empty", func(d *EngineOperation) { d.Method = "ze-plugin:" }},
		{"method nested", func(d *EngineOperation) { d.Method += ":nested" }},
		{"method control", func(d *EngineOperation) { d.Method += "\x1b" }},
		{"method punctuation", func(d *EngineOperation) { d.Method += "/nested" }},
		{"method digit first", func(d *EngineOperation) { d.Method = "ze-plugin:1method" }},
		{"handler missing", func(d *EngineOperation) { d.Handler = nil }},
		{"policy missing", func(d *EngineOperation) { d.Startup = EngineOperationStartupUnspecified }},
		{"policy unknown", func(d *EngineOperation) { d.Startup = 255 }},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			declaration := valid
			testcase.modify(&declaration)
			if err := RegisterEngineOp(declaration); err == nil {
				t.Fatal("invalid declaration was accepted")
			}
			if lookupEngineOp(declaration.Method) != nil {
				t.Fatal("invalid declaration was published")
			}
		})
	}
	registerTestEngineOperation(t, valid)
	first := lookupEngineOp(method)
	if err := RegisterEngineOp(valid); err == nil {
		t.Fatal("duplicate operation was accepted")
	}
	if lookupEngineOp(method) != first {
		t.Fatal("duplicate replaced the first operation")
	}
	registry.Seal()
	late := valid
	late.Method = "ze-plugin:late-registration-test"
	if err := RegisterEngineOp(late); !errors.Is(err, registry.ErrRegistrySealed) {
		t.Fatalf("late registration = %v", err)
	}
	if lookupEngineOp(late.Method) != nil {
		t.Fatal("late operation was published")
	}
}

// TestOwnerEngineOperationMethodLength preserves the shared IPC boundary:
// the longest permitted method registers, while the next byte publishes nothing.
func TestOwnerEngineOperationMethodLength(t *testing.T) {
	const prefix = "ze-plugin:"
	method := prefix + strings.Repeat("a", coreipc.MaxMethodLength-len(prefix))
	valid := EngineOperation{
		Method: method, Startup: EngineOperationRuntimeOnly,
		Handler: func(*Server, context.Context, *process.Process, json.RawMessage) (any, error) { return "bounded", nil },
	}
	registerTestEngineOperation(t, valid)
	registered := lookupEngineOp(method)
	if registered == nil {
		t.Fatal("maximum-length method was not published")
	}
	tooLong := valid
	tooLong.Method += "a"
	t.Cleanup(func() {
		engineOperationMu.Lock()
		delete(engineOpTable, tooLong.Method)
		engineOperationMu.Unlock()
	})
	if err := RegisterEngineOp(tooLong); err == nil {
		t.Error("method beyond the shared IPC length limit was accepted")
	}
	if lookupEngineOp(tooLong.Method) != nil {
		t.Error("overlong method was published")
	}
	if lookupEngineOp(method) != registered {
		t.Fatal("refusal changed the valid boundary operation")
	}
}

// TestOwnerEngineOperationTransportParity proves the registered raw handler
// gets the same authenticated process and error contract on JSON and Direct.
func TestOwnerEngineOperationTransportParity(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	conn, remote := newDriverPipe(t)
	proc := process.NewProcess(plugin.PluginConfig{Name: "parity-owner"})
	s := &Server{ctx: ctx}
	const method = "ze-plugin:registered-parity-test"
	registerTestEngineOperation(t, EngineOperation{
		Method: method, Startup: EngineOperationRuntimeOnly,
		Handler: func(_ *Server, _ context.Context, actual *process.Process, params json.RawMessage) (any, error) {
			if actual != proc {
				return nil, errors.New("wrong authenticated process")
			}
			if strings.Contains(string(params), "refuse") {
				return nil, &rpc.RPCCallError{Message: "owner refused this request"}
			}
			return map[string]string{"process": actual.Name()}, nil
		},
	})
	for _, params := range []json.RawMessage{json.RawMessage(`{}`), json.RawMessage(`{"refuse":true}`)} {
		direct, directErr := s.dispatchPluginRPCDirect(ctx, proc, method, params)
		served := make(chan struct{})
		go func() {
			request, err := conn.ReadRequest(ctx)
			if err == nil {
				s.dispatchPluginRPC(proc, conn, request)
			}
			close(served)
		}()
		wire, wireErr := remote.CallRPC(ctx, method, params)
		<-served
		if string(wire) != string(direct) {
			t.Errorf("JSON %s differs from Direct %s", wire, direct)
		}
		if (wireErr == nil) != (directErr == nil) {
			t.Fatalf("JSON error %v differs from Direct %v", wireErr, directErr)
		}
		if wireErr != nil {
			var wireCall, directCall *rpc.RPCCallError
			if !errors.As(wireErr, &wireCall) || !errors.As(directErr, &directCall) {
				t.Fatalf("JSON/Direct error types: %T / %T", wireErr, directErr)
			}
			if wireCall.Message != directCall.Message {
				t.Errorf("JSON/Direct error detail: %q / %q", wireCall.Message, directCall.Message)
			}
		}
	}
}

// TestProcessGenerationCleanupIdentity proves name reuse cannot replace the
// identity delivered to an owner's cleanup, and registration seals with ops.
func TestProcessGenerationCleanupIdentity(t *testing.T) {
	snapshot := registry.Snapshot()
	registry.Reset()
	engineOperationMu.Lock()
	before := len(processGenerationCleanups)
	engineOperationMu.Unlock()
	t.Cleanup(func() {
		engineOperationMu.Lock()
		processGenerationCleanups = processGenerationCleanups[:before]
		engineOperationMu.Unlock()
		registry.Restore(snapshot)
	})
	var cleaned *process.Process
	if err := RegisterProcessGenerationCleanup(func(proc *process.Process) { cleaned = proc }); err != nil {
		t.Fatal(err)
	}
	old := process.NewProcess(plugin.PluginConfig{Name: "reused-name"})
	replacement := process.NewProcess(plugin.PluginConfig{Name: "reused-name"})
	runProcessGenerationCleanups(old)
	if cleaned != old || cleaned == replacement {
		t.Fatal("cleanup lost process-generation identity")
	}
	registry.Seal()
	if err := RegisterProcessGenerationCleanup(func(*process.Process) {}); !errors.Is(err, registry.ErrRegistrySealed) {
		t.Fatalf("late cleanup registration = %v", err)
	}
}

// TestOwnerEngineOperationTypedDiscovery exercises production bridge wiring
// with an internal process, proving an owner's optional native descriptor joins
// the registry used by builtins and carries the same authenticated identity.
func TestOwnerEngineOperationTypedDiscovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	s := &Server{ctx: ctx}
	const method = "ze-plugin:typed-discovery-test"
	var wiredProcess *process.Process
	ownerError := errors.New("owner typed request reached")
	registerTestEngineOperation(t, EngineOperation{
		Method: method, Startup: EngineOperationRuntimeOnly,
		Handler: func(*Server, context.Context, *process.Process, json.RawMessage) (any, error) {
			return nil, nil //nolint:nilnil // no result payload; (nil,nil) is success-with-no-content
		},
		TypedWire: func(_ *Server, proc *process.Process, bridge *rpc.DirectBridge) {
			wiredProcess = proc
			bridge.SetForwardCached(func(context.Context, []uint64, []string) error { return ownerError })
		},
	})
	const pluginName = "typed-owner-test"
	if err := registry.Register(registry.Registration{
		Name:       pluginName,
		RunEngine:  func(net.Conn) int { <-ctx.Done(); return 0 },
		CLIHandler: func([]string) int { return 0 },
	}); err != nil {
		t.Fatal(err)
	}
	proc := process.NewProcess(plugin.PluginConfig{Name: pluginName, Internal: true})
	if err := proc.StartWithContext(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		proc.Stop()
		join, stopJoin := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopJoin()
		if err := proc.Wait(join); err != nil {
			t.Errorf("join typed owner process: %v", err)
		}
	})
	s.wireBridgeDispatch(proc)
	if wiredProcess != proc {
		t.Fatal("owner descriptor did not receive the internal process generation")
	}
	proc.Bridge().SetReady()
	if err := proc.Bridge().ForwardCached(ctx, nil, nil); !errors.Is(err, ownerError) {
		t.Fatalf("owner typed bridge call = %v", err)
	}
}
