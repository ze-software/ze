// Design: docs/architecture/api/process-protocol.md -- native and wire refusal parity.
// Related: dispatch_native.go -- the production adapter normalizes live owner errors.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// These errors originate in a LIVE registered owner, after useful native output
// has been acknowledged. They pass the real production bridge/engine-op adapter,
// not a test SetDispatchNative closure which would preserve impossible identity.
func TestOwnerNativeClosedOutputFailuresRemainRPCRefusals(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	host := &Server{ctx: ctx}
	causes := []struct {
		name  string
		cause error
	}{
		{"ordinary", errors.New("controlled live output refusal")},
		{"net-closed", net.ErrClosed},
		{"closed-pipe", io.ErrClosedPipe},
		{"eof", io.EOF},
		{"bridge-closed", rpc.ErrBridgeClosed},
		{"mux-closed", rpc.ErrMuxConnClosed},
		{"bridge-failed", rpc.ErrBridgeFailed},
	}
	var owner *process.Process
	evaluate := func(proc *process.Process, input nativeOwnerInput) (any, error) {
		if proc != owner {
			return nil, errors.New("live output fixture lost its authenticated owner")
		}
		if input.Value == 0 {
			return nativeOwnerOutput{Value: 7, Process: proc.Name()}, nil
		}
		index := input.Value - 1
		if index < 0 {
			return nil, errors.New("live output fixture cause index is negative")
		}
		if index >= len(causes) {
			return nil, errors.New("live output fixture cause index exceeds its finite population")
		}
		if !proc.Bridge().Ready() {
			return nil, errors.New("live output evaluator was reached after transport stop")
		}
		return nil, fmt.Errorf("live forwarding evaluator: %w", causes[index].cause)
	}
	const method = "ze-plugin:closed-output-refusal-test"
	registerTestEngineOperation(t, EngineOperation{
		Method: method, Startup: EngineOperationRuntimeOnly,
		Handler: func(_ *Server, _ context.Context, proc *process.Process, params json.RawMessage) (any, error) {
			var input nativeOwnerInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			return evaluate(proc, input)
		},
		Native: func(_ *Server, _ context.Context, proc *process.Process, payload any) (any, error) {
			input, valid := payload.(nativeOwnerInput)
			if !valid {
				return nil, fmt.Errorf("live output fixture received %T", payload)
			}
			return evaluate(proc, input)
		},
	})
	owner = nativeOwnerProcess(t, host)
	bridge := owner.Bridge()
	bridge.SetReady()
	useful, handled, err := bridge.DispatchNative(ctx, method, nativeOwnerInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("registered useful native output was not handled")
	}
	if got, valid := useful.(nativeOwnerOutput); !valid || got.Value != 7 || got.Process != owner.Name() {
		t.Fatalf("useful native ACK lost its value/owner: %+v", useful)
	}
	for index, test := range causes {
		t.Run(test.name, func(t *testing.T) {
			value, handled, err := bridge.DispatchNative(ctx, method, nativeOwnerInput{Value: index + 1})
			if !handled {
				t.Fatal("live registered native refusal became an adapter miss")
			}
			if value != nil {
				t.Fatalf("live native refusal returned successful output: %+v", value)
			}
			var refusal *rpc.RPCCallError
			if !errors.As(err, &refusal) {
				t.Fatalf("live owner cause bypassed actual RPC normalization: %T %v", err, err)
			}
			want := "live forwarding evaluator: " + test.cause.Error()
			if refusal.Message != want {
				t.Fatalf("live owner refusal detail=%q, want %q", refusal.Message, want)
			}
			if errors.Is(err, test.cause) {
				t.Fatalf("live owner retained transport sentinel identity after the actual RPC adapter: %v", err)
			}
			if !bridge.Ready() {
				t.Fatal("live owner refusal stopped the actual bridge")
			}
		})
	}
}
