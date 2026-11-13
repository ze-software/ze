// Design: docs/architecture/api/process-protocol.md -- native owner operation parity.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"testing"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type nativeOwnerInput struct {
	Value int `json:"value"`
}

type nativeOwnerOutput struct {
	Value   int    `json:"value"`
	Process string `json:"process"`
}

func TestOwnerNativeOperationErrorNormalization(t *testing.T) {
	plain := errors.New("owner rejected input")
	call := &rpc.RPCCallError{Message: "owner rejected input"}
	for _, testcase := range []struct {
		name     string
		input    error
		identity error
	}{
		{"plain", plain, nil},
		{"RPC", call, call},
		{"wrapped RPC", fmt.Errorf("adapter: %w", call), call},
		{"canceled", context.Canceled, context.Canceled},
		{"deadline", context.DeadlineExceeded, context.DeadlineExceeded},
		{"wrapped canceled", fmt.Errorf("adapter: %w", context.Canceled), nil},
		{"wrapped deadline", fmt.Errorf("adapter: %w", context.DeadlineExceeded), nil},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			result := engineOpCallError(testcase.input)
			if testcase.identity != nil {
				if result != testcase.identity {
					t.Fatalf("error identity changed: %T %v", result, result)
				}
				return
			}
			if errors.Is(testcase.input, context.Canceled) || errors.Is(testcase.input, context.DeadlineExceeded) {
				if result != testcase.input {
					t.Fatal("wrapped context error identity changed")
				}
				return
			}
			var normalized *rpc.RPCCallError
			if !errors.As(result, &normalized) || normalized.Message != plain.Error() {
				t.Fatalf("plain refusal was not normalized once: %T %v", result, result)
			}
		})
	}
}

// nativeOwnerProcess starts a real internal process so production bridge wiring
// binds the authenticated process rather than a test-injected dispatcher closure.
func nativeOwnerProcess(t *testing.T, s *Server) *process.Process {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const name = "native-owner-engine-test"
	if err := registry.Register(registry.Registration{
		Name:       name,
		RunEngine:  func(net.Conn) int { <-ctx.Done(); return 0 },
		CLIHandler: func([]string) int { return 0 },
	}); err != nil {
		t.Fatal(err)
	}
	proc := process.NewProcess(plugin.PluginConfig{Name: name, Internal: true})
	if err := proc.StartWithContext(ctx); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		proc.Stop()
		join, stopJoin := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopJoin()
		if err := proc.Wait(join); err != nil {
			t.Errorf("join native owner: %v", err)
		}
	})
	s.wireBridgeDispatch(proc)
	return proc
}

func TestOwnerNativeOperationRegistrationRequiresPublicHandler(t *testing.T) {
	const method = "ze-plugin:native-without-public-test"
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() {
		engineOperationMu.Lock()
		delete(engineOpTable, method)
		engineOperationMu.Unlock()
		registry.Restore(snapshot)
	})
	if err := RegisterEngineOp(EngineOperation{
		Method: method, Startup: EngineOperationRuntimeOnly,
		Native: func(*Server, context.Context, *process.Process, any) (any, error) { return 7, nil },
	}); err == nil {
		t.Error("native-only declaration discarded the public process protocol")
	}
	if lookupEngineOp(method) != nil {
		t.Fatal("refused native-only declaration was published")
	}
}

// Both handlers adapt into one evaluator. The native registration must survive
// mutation of the caller's declaration and retain real process/context identity.
func TestOwnerNativeOperationTransportParity(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	s := &Server{ctx: ctx}
	var expected *process.Process
	evaluate := func(ctx context.Context, proc *process.Process, input nativeOwnerInput) (any, error) {
		if proc != expected {
			return nil, errors.New("different process generation")
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if ctx != s.ctx {
			return nil, errors.New("owner received a different active caller context")
		}
		if input.Value == -1 {
			return nil, errors.New("owner refused negative value; send a nonnegative value")
		}
		if input.Value == -2 {
			return nil, &rpc.RPCCallError{Message: "owner refused reserved value; send a nonnegative value"}
		}
		return nativeOwnerOutput{Value: input.Value + 1, Process: proc.Name()}, nil
	}
	const method = "ze-plugin:native-parity-test"
	declaration := EngineOperation{
		Method: method, Startup: EngineOperationRuntimeOnly,
		Handler: func(_ *Server, ctx context.Context, proc *process.Process, params json.RawMessage) (any, error) {
			var input nativeOwnerInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			return evaluate(ctx, proc, input)
		},
		Native: func(_ *Server, ctx context.Context, proc *process.Process, payload any) (any, error) {
			input, ok := payload.(nativeOwnerInput)
			if !ok {
				return nil, errors.New("native input must be nativeOwnerInput; send a valid owner input")
			}
			return evaluate(ctx, proc, input)
		},
	}
	registerTestEngineOperation(t, declaration)
	declaration.Native = nil
	expected = nativeOwnerProcess(t, s)
	bridge := expected.Bridge()
	if _, _, err := bridge.DispatchNative(ctx, method, nativeOwnerInput{7}); err == nil {
		t.Fatal("native call bypassed startup readiness")
	}
	bridge.SetReady()
	conn, remote := newDriverPipe(t)
	for _, input := range []nativeOwnerInput{{7}, {-1}, {-2}} {
		native, handled, nativeErr := bridge.DispatchNative(ctx, method, input)
		if !handled {
			t.Fatal("registered native operation was unavailable")
		}
		served := make(chan struct{})
		go func() {
			request, err := conn.ReadRequest(ctx)
			if err == nil {
				s.dispatchPluginRPC(expected, conn, request)
			}
			close(served)
		}()
		wire, wireErr := remote.CallRPC(ctx, method, input)
		<-served
		if (nativeErr == nil) != (wireErr == nil) {
			t.Fatalf("native error %v differs from public %v", nativeErr, wireErr)
		}
		if nativeErr != nil {
			var nativeCall, wireCall *rpc.RPCCallError
			if !errors.As(nativeErr, &nativeCall) || !errors.As(wireErr, &wireCall) {
				t.Fatalf("native/public refusal types = %T / %T", nativeErr, wireErr)
			}
			if nativeCall.Message != wireCall.Message {
				t.Errorf("native/public refusal detail = %q / %q", nativeCall.Message, wireCall.Message)
			}
			continue
		}
		var decoded nativeOwnerOutput
		if err := json.Unmarshal(wire, &decoded); err != nil {
			t.Fatal(err)
		}
		if native != decoded {
			t.Errorf("native result %#v differs from public %#v", native, decoded)
		}
	}
	before, _ := engineOpPanics().guard.Panics(taskEngineOp)
	for _, malformed := range []any{"wrong type", (*nativeOwnerInput)(nil), nil} {
		if _, handled, err := bridge.DispatchNative(ctx, method, malformed); !handled || err == nil {
			t.Errorf("malformed native payload %T = %v, %v", malformed, handled, err)
		}
	}
	after, _ := engineOpPanics().guard.Panics(taskEngineOp)
	if after != before {
		t.Fatal("malformed owner payload reached the BUG panic boundary")
	}
	if _, handled, err := bridge.DispatchNative(ctx, "ze-plugin:unknown-native-test", nativeOwnerInput{7}); handled || err != nil {
		t.Fatalf("unknown native method = %v, %v", handled, err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, handled, err := bridge.DispatchNative(canceled, method, nativeOwnerInput{7}); !handled || !errors.Is(err, context.Canceled) {
		t.Errorf("native cancellation lost caller context: %v, %v", handled, err)
	}
	params := json.RawMessage(`{"value":7}`)
	if _, err := s.dispatchPluginRPCDirect(canceled, expected, method, params); !errors.Is(err, context.Canceled) {
		t.Errorf("public Direct cancellation = %v", err)
	}
}

func TestOwnerNativeOperationNoJSON(t *testing.T) {
	s := &Server{ctx: t.Context()}
	const method = "ze-plugin:native-no-json-test"
	registerTestEngineOperation(t, EngineOperation{
		Method: method, Startup: EngineOperationRuntimeOnly,
		Handler: func(*Server, context.Context, *process.Process, json.RawMessage) (any, error) {
			t.Error("native call reached JSON owner adapter")
			return nil, errors.New("JSON adapter called")
		},
		Native: func(_ *Server, _ context.Context, _ *process.Process, input any) (any, error) {
			return input, nil
		},
	})
	proc := nativeOwnerProcess(t, s)
	proc.Bridge().SetReady()
	input := math.NaN()
	if _, err := json.Marshal(input); err == nil {
		t.Fatal("JSON-free control unexpectedly has a JSON representation")
	}
	result, handled, err := proc.Bridge().DispatchNative(t.Context(), method, input)
	if !handled || err != nil {
		t.Fatalf("native non-JSON value = %v, %v", handled, err)
	}
	value, ok := result.(float64)
	if !ok || !math.IsNaN(value) {
		t.Fatalf("non-JSON result changed: %#v", result)
	}
}

func TestOwnerNativeOperationPublicOnlyFallback(t *testing.T) {
	s := &Server{ctx: t.Context()}
	const method = "ze-plugin:native-public-only-test"
	registerTestEngineOperation(t, EngineOperation{
		Method: method, Startup: EngineOperationRuntimeOnly,
		Handler: func(_ *Server, _ context.Context, proc *process.Process, params json.RawMessage) (any, error) {
			if string(params) != `{"value":7}` {
				return nil, errors.New("public adapter received different input")
			}
			return nativeOwnerOutput{Value: 8, Process: proc.Name()}, nil
		},
	})
	proc := nativeOwnerProcess(t, s)
	bridge := proc.Bridge()
	bridge.SetReady()
	if value, handled, err := bridge.DispatchNative(t.Context(), method, nativeOwnerInput{7}); value != nil || handled || err != nil {
		t.Fatalf("public-only native miss = %v, %v, %v", value, handled, err)
	}
	value, err := bridge.DispatchRPC(t.Context(), method, json.RawMessage(`{"value":7}`))
	if err != nil {
		t.Fatal(err)
	}
	var output nativeOwnerOutput
	if err := json.Unmarshal(value, &output); err != nil {
		t.Fatal(err)
	}
	if output.Value != 8 || output.Process != proc.Name() {
		t.Fatalf("public-only fallback output = %#v", output)
	}
}

func TestOwnerNativeOperationPanicContained(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	s := &Server{ctx: t.Context()}
	const method = "ze-plugin:native-panic-test"
	calls := 0
	registerTestEngineOperation(t, EngineOperation{
		Method: method, Startup: EngineOperationRuntimeOnly,
		Handler: func(*Server, context.Context, *process.Process, json.RawMessage) (any, error) { return 7, nil },
		Native: func(*Server, context.Context, *process.Process, any) (any, error) {
			calls++
			if calls == 1 {
				panic("native owner defect secret")
			}
			return 7, nil
		},
	})
	proc := nativeOwnerProcess(t, s)
	proc.Bridge().SetReady()
	before, _ := engineOpPanics().guard.Panics(taskEngineOp)
	_, handled, err := proc.Bridge().DispatchNative(t.Context(), method, nil)
	if !handled || err == nil || !strings.Contains(err.Error(), "internal Ze defect while handling "+method) {
		t.Fatalf("native panic was not contained: %v, %v", handled, err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Error("native panic leaked handler data")
	}
	after, _ := engineOpPanics().guard.Panics(taskEngineOp)
	if after != before+1 {
		t.Errorf("native panic guard count = %d, want %d", after, before+1)
	}
	if value, handled, err := proc.Bridge().DispatchNative(t.Context(), method, nil); !handled || err != nil || value != 7 {
		t.Fatalf("healthy native control after panic = %v, %v, %v", value, handled, err)
	}
}
