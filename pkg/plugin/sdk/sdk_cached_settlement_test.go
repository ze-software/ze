// Design: docs/architecture/api/process-protocol.md -- positive writer evidence on both rails.
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"testing"
	"time"
)

// Exact positive failed completion is the only remote joined evidence. These
// actual SDK rails cannot promote absent/malformed receipts or transport errors.
func TestBoundCachedSDKFailureEvidenceOnBothTransports(t *testing.T) {
	for _, method := range []string{rpc.MethodForwardCached, rpc.MethodRelayStoredRoute} {
		for _, typed := range []bool{false, true} {
			for _, outcome := range []string{"joined", "missing", "false", "wrong-deadline", "transport"} {
				t.Run(method+map[bool]string{false: "/framed/", true: "/typed/"}[typed]+outcome, func(t *testing.T) {
					client, engine := newTestPair(t)
					work, cancel := context.WithTimeout(t.Context(), 5*time.Second)
					defer cancel()
					answer := func(deadline int64) (*rpc.CachedWriteOutput, error) {
						if outcome == "transport" {
							return nil, errors.New("target outcome unknown")
						}
						if outcome == "missing" {
							return nil, nil
						}
						output := rpc.CompletedCachedWrite(deadline, context.Canceled)
						if outcome == "false" {
							output.Completed = false
						}
						if outcome == "wrong-deadline" {
							output.DeadlineMS++
						}
						return output, nil
					}
					joined := make(chan struct{})
					if typed {
						client.bridge = rpc.NewDirectBridge()
						client.bridge.SetForwardCached(func(_ context.Context, input rpc.ForwardCachedInput) (*rpc.CachedWriteOutput, error) {
							return answer(input.DeadlineMS)
						})
						client.bridge.SetRelayStoredRoute(func(_ context.Context, input rpc.RelayStoredRouteInput) (*rpc.CachedWriteOutput, error) {
							return answer(input.DeadlineMS)
						})
						client.bridge.SetReady()
						close(joined)
					} else {
						go func() {
							defer close(joined)
							request := readEngineRequest(t, work, engine.mux)
							require.Equal(t, method, request.Method)
							var input struct {
								DeadlineMS int64 `json:"deadline-ms"`
							}
							require.NoError(t, json.Unmarshal(request.Params, &input))
							output, err := answer(input.DeadlineMS)
							if err != nil {
								require.NoError(t, engine.mux.SendError(work, request.ID, err.Error()))
								return
							}
							require.NoError(t, engine.mux.SendResult(work, request.ID, output))
						}()
					}
					var err error
					if method == rpc.MethodForwardCached {
						err = client.ForwardCached(work, rpc.ForwardCachedInput{IDs: []uint64{17}, Targets: []rpc.ForwardTarget{{Destination: "192.0.2.7", DestinationOwner: 91, DestinationSession: 71}}})
					} else {
						err = client.RelayStoredRoute(work, rpc.RelayStoredRouteInput{Destination: "192.0.2.7", DestinationOwner: 91, DestinationSession: 71})
					}
					<-joined
					require.Error(t, err)
					_, positive := errors.AsType[*rpc.CachedWriteJoinedError](err)
					require.Equal(t, outcome == "joined", positive, "only the exact positive receipt proves actual writer completion")
					_, local := errors.AsType[*rpc.CachedWriteNotDispatchedError](err)
					require.False(t, local)
					if positive {
						require.ErrorIs(t, err, context.Canceled)
					}
				})
			}
		}
	}
}
