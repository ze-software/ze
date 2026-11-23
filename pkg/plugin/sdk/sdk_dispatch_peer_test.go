// Design: docs/architecture/api/process-protocol.md -- physical control transport and join.
package sdk

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Real SDK native values and public frames preserve exact physical identity,
// refuse legacy acknowledgments, and continue useful work after a refusal.
func TestDispatchPhysicalPeerSDKTransportParity(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "wire", true: "native"}[native], func(t *testing.T) {
			p, engine := newTestPair(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if native {
				p.bridge = rpc.NewDirectBridge()
				p.bridge.SetReady()
			}
			for _, echo := range []bool{true, false, true} {
				reply := func(input rpc.DispatchCommandInput) *rpc.DispatchCommandOutput {
					require.NoError(t, input.ValidatePeer(time.Now()))
					require.Equal(t, uint64(18446744073709551614), input.PeerOwner)
					require.Equal(t, uint64(18446744073709551613), input.ConnectionID)
					output := &rpc.DispatchCommandOutput{Status: rpc.StatusDone}
					if echo {
						output.PeerOwner = input.PeerOwner
						output.ConnectionID = input.ConnectionID
						output.DeadlineMS = input.DeadlineMS
					}
					return output
				}
				joined := make(chan struct{})
				if native {
					p.bridge.SetDispatchNative(func(_ context.Context, method string, value any) (any, bool, error) {
						require.Equal(t, rpc.MethodDispatchCommand, method)
						input, ok := value.(rpc.DispatchCommandInput)
						require.True(t, ok)
						return reply(input), true, nil
					})
				} else {
					go func() {
						defer close(joined)
						request := readEngineRequest(t, ctx, engine.mux)
						require.Equal(t, rpc.MethodDispatchCommand, request.Method)
						var input rpc.DispatchCommandInput
						require.NoError(t, json.Unmarshal(request.Params, &input))
						require.NoError(t, engine.mux.SendResult(ctx, request.ID, reply(input)))
					}()
				}
				status, _, err := p.DispatchPeerCommand(ctx, rpc.DispatchCommandInput{Command: "request peer 192.0.2.90 pause", PeerOwner: 18446744073709551614, ConnectionID: 18446744073709551613})
				if echo {
					require.NoError(t, err)
					require.Equal(t, rpc.StatusDone, status)
				} else {
					require.Error(t, err)
				}
				if !native {
					select {
					case <-joined:
					case <-ctx.Done():
						t.Fatal("frame server did not finish")
					}
				}
			}
		})
	}
}

// The existing authenticated cancellation receipt must follow the remote
// physical action's join. Caller cancellation alone cannot release its worker.
func TestDispatchPhysicalPeerSDKCancellationJoinsRemote(t *testing.T) {
	p, engine := newTestPair(t)
	work, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	ctx, cancel := context.WithCancel(work)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := p.DispatchPeerCommand(ctx, rpc.DispatchCommandInput{Command: "request peer 192.0.2.90 resume", PeerOwner: 901, ConnectionID: 902})
		done <- err
	}()
	original := readEngineRequest(t, work, engine.mux)
	require.Equal(t, rpc.MethodDispatchCommand, original.Method)
	cancel()
	settlement := readEngineRequest(t, work, engine.mux)
	require.Equal(t, rpc.MethodCancelRequest, settlement.Method)
	var input rpc.CancelRequestInput
	require.NoError(t, json.Unmarshal(settlement.Params, &input))
	require.Equal(t, original.ID, input.RequestID)
	require.Equal(t, rpc.MethodDispatchCommand, input.Method)
	select {
	case err := <-done:
		t.Fatalf("physical command returned before remote settlement: %v", err)
	default:
	}
	require.NoError(t, engine.mux.SendResult(work, settlement.ID, &rpc.CancelRequestOutput{RequestID: original.ID, Outcome: rpc.CancelRequestSettled}))
	require.ErrorIs(t, <-done, context.Canceled)
}
