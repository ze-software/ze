// Design: docs/architecture/api/process-protocol.md -- source lease answer binding.
package sdk

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSourceLeaseSDKRefusesReboundPeerOwner returns another owner through the
// actual native and framed SDK transports; transport success cannot settle it.
func TestSourceLeaseSDKRefusesReboundPeerOwner(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "wire", true: "native"}[native], func(t *testing.T) {
			p, engine := newTestPair(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			input := rpc.SourceLeaseInput{Action: rpc.SourceLeaseAcquire, SourceReceipt: 7, PeerOwner: 701}
			answer := &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: 702}
			joined := make(chan struct{})
			if native {
				p.bridge = rpc.NewDirectBridge()
				p.bridge.SetDispatchNative(func(context.Context, string, any) (any, bool, error) {
					return answer, true, nil
				})
				p.bridge.SetReady()
				close(joined)
			} else {
				go func() {
					defer close(joined)
					request := readEngineRequest(t, ctx, engine.mux)
					require.Equal(t, rpc.MethodSourceLease, request.Method)
					var observed rpc.SourceLeaseInput
					require.NoError(t, json.Unmarshal(request.Params, &observed))
					require.Equal(t, input.PeerOwner, observed.PeerOwner)
					require.NoError(t, engine.mux.SendResult(ctx, request.ID, answer))
				}()
			}
			_, err := p.SourceLease(ctx, input)
			require.ErrorContains(t, err, "another peer owner")
			<-joined
		})
	}
}
