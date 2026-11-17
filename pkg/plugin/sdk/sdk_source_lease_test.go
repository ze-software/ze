// Design: docs/architecture/api/process-protocol.md -- source lease transport parity.
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSourceLeaseSDKTransportParity exercises the actual SDK's native value
// dispatcher and external Mux frames, preserving every action and receipt.
func TestSourceLeaseSDKTransportParity(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "wire", true: "native"}[native], func(t *testing.T) {
			p, engine := newTestPair(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			calls := 0
			if native {
				p.bridge = rpc.NewDirectBridge()
				p.bridge.SetDispatchNative(func(ctx context.Context, method string, value any) (any, bool, error) {
					require.Equal(t, rpc.MethodSourceLease, method)
					input, valid := value.(rpc.SourceLeaseInput)
					require.True(t, valid)
					require.NoError(t, input.Validate(time.Now()))
					require.NoError(t, ctx.Err())
					calls++
					return &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}, true, nil
				})
				p.bridge.SetReady()
			}
			for _, action := range []rpc.SourceLeaseAction{rpc.SourceLeaseAcquire, rpc.SourceLeaseRetain, rpc.SourceLeaseRelease} {
				joined := make(chan struct{})
				if !native {
					go func() {
						defer close(joined)
						request := readEngineRequest(t, ctx, engine.mux)
						require.Equal(t, rpc.MethodSourceLease, request.Method)
						var input rpc.SourceLeaseInput
						require.NoError(t, json.Unmarshal(request.Params, &input))
						require.NoError(t, input.Validate(time.Now()))
						require.NoError(t, engine.mux.SendResult(ctx, request.ID, &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}))
					}()
				}
				input := rpc.SourceLeaseInput{Action: action, SourceReceipt: 79, PeerOwner: 701}
				output, err := p.SourceLease(ctx, input)
				require.NoError(t, err)
				require.NoError(t, output.Validate(input))
				if !native {
					<-joined
				}
			}
			if native {
				require.Equal(t, 3, calls)
			}
		})
	}
}

// TestSourceLeaseSDKCancellationAwaitsSettlement proves the external SDK uses
// the shared acknowledged cancellation method for its original source lease.
func TestSourceLeaseSDKCancellationAwaitsSettlement(t *testing.T) {
	p, engine := newTestPair(t)
	work, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	ctx, cancel := context.WithCancel(work)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := p.SourceLease(ctx, rpc.SourceLeaseInput{Action: rpc.SourceLeaseRelease, SourceReceipt: 97, PeerOwner: 701})
		done <- err
	}()
	original := readEngineRequest(t, work, engine.mux)
	require.Equal(t, rpc.MethodSourceLease, original.Method)
	cancel()
	settlement := readEngineRequest(t, work, engine.mux)
	require.Equal(t, rpc.MethodCancelRequest, settlement.Method)
	var input rpc.CancelRequestInput
	require.NoError(t, json.Unmarshal(settlement.Params, &input))
	require.Equal(t, original.ID, input.RequestID)
	require.Equal(t, rpc.MethodSourceLease, input.Method)
	select {
	case err := <-done:
		t.Fatalf("source release returned before remote settlement: %v", err)
	default:
	}
	require.NoError(t, engine.mux.SendResult(work, settlement.ID, &rpc.CancelRequestOutput{RequestID: original.ID, Outcome: rpc.CancelRequestSettled}))
	require.ErrorIs(t, <-done, context.Canceled)
}

// TestSourceLeaseSDKRefusesReboundNativeAnswer keeps transport success separate
// from an answer which settled another source's lease.
func TestSourceLeaseSDKRefusesReboundNativeAnswer(t *testing.T) {
	p, _ := newTestPair(t)
	p.bridge = rpc.NewDirectBridge()
	p.bridge.SetDispatchNative(func(context.Context, string, any) (any, bool, error) {
		return &rpc.SourceLeaseOutput{Action: rpc.SourceLeaseAcquire, SourceReceipt: 80, PeerOwner: 701}, true, nil
	})
	p.bridge.SetReady()
	_, err := p.SourceLease(t.Context(), rpc.SourceLeaseInput{Action: rpc.SourceLeaseAcquire, SourceReceipt: 79, PeerOwner: 701})
	require.ErrorContains(t, err, "another source")
	p.bridge.StopDispatch()
	_, err = p.SourceLease(t.Context(), rpc.SourceLeaseInput{Action: rpc.SourceLeaseAcquire, SourceReceipt: 79, PeerOwner: 701})
	require.True(t, errors.Is(err, rpc.ErrBridgeClosed))
}
