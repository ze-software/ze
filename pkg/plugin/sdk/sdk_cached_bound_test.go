// Design: docs/architecture/api/process-protocol.md -- typed and framed bound cached replay.
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestBoundCachedSDKTypedAndFramedCompletion drives both actual SDK operations
// through their typed bridge and real Mux framing. Missing, false and mismatched
// final acknowledgments cannot become successful cleanup or replay readiness.
func TestBoundCachedSDKTypedAndFramedCompletion(t *testing.T) {
	for _, method := range []string{rpc.MethodForwardCached, rpc.MethodRelayStoredRoute} {
		for _, typed := range []bool{false, true} {
			for _, answer := range []string{"complete", "missing", "not-complete", "wrong-deadline"} {
				t.Run(method+map[bool]string{false: "/framed/", true: "/typed/"}[typed]+answer, func(t *testing.T) {
					p, engine := newTestPair(t)
					ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
					defer cancel()
					response := func(owner, session uint64, deadline int64) *rpc.CachedWriteOutput {
						require.Equal(t, uint64(math.MaxUint64), owner)
						require.Equal(t, uint64(math.MaxUint64-1), session)
						require.Positive(t, deadline)
						if answer == "missing" {
							return nil
						}
						result := &rpc.CachedWriteOutput{Completed: answer != "not-complete", DeadlineMS: deadline}
						if answer == "wrong-deadline" {
							result.DeadlineMS++
						}
						return result
					}
					joined := make(chan struct{})
					if typed {
						p.bridge = rpc.NewDirectBridge()
						p.bridge.SetForwardCached(func(_ context.Context, input rpc.ForwardCachedInput) (*rpc.CachedWriteOutput, error) {
							require.Equal(t, []uint64{17}, input.IDs)
							require.Len(t, input.Targets, 1)
							return response(input.Targets[0].DestinationOwner, input.Targets[0].DestinationSession, input.DeadlineMS), nil
						})
						p.bridge.SetRelayStoredRoute(func(_ context.Context, input rpc.RelayStoredRouteInput) (*rpc.CachedWriteOutput, error) {
							require.Equal(t, "192.0.2.7", input.Destination)
							return response(input.DestinationOwner, input.DestinationSession, input.DeadlineMS), nil
						})
						p.bridge.SetReady()
						close(joined)
					} else {
						go func() {
							defer close(joined)
							request := readEngineRequest(t, ctx, engine.mux)
							require.Equal(t, method, request.Method)
							var header struct {
								Targets  []rpc.ForwardTarget `json:"targets"`
								Owner    uint64              `json:"destination-owner,string"`
								Session  uint64              `json:"destination-session,string"`
								Deadline int64               `json:"deadline-ms"`
							}
							require.NoError(t, json.Unmarshal(request.Params, &header))
							if method == rpc.MethodForwardCached {
								require.Len(t, header.Targets, 1)
								header.Owner = header.Targets[0].DestinationOwner
								header.Session = header.Targets[0].DestinationSession
							}
							require.NoError(t, engine.mux.SendResult(ctx, request.ID, response(header.Owner, header.Session, header.Deadline)))
						}()
					}
					var err error
					if method == rpc.MethodForwardCached {
						err = p.ForwardCached(ctx, rpc.ForwardCachedInput{IDs: []uint64{17}, Targets: []rpc.ForwardTarget{{Destination: "192.0.2.7", DestinationOwner: math.MaxUint64, DestinationSession: math.MaxUint64 - 1}}})
					} else {
						err = p.RelayStoredRoute(ctx, rpc.RelayStoredRouteInput{Destination: "192.0.2.7", DestinationOwner: math.MaxUint64, DestinationSession: math.MaxUint64 - 1})
					}
					<-joined
					if answer == "complete" {
						require.NoError(t, err)
					} else {
						require.Error(t, err)
					}
				})
			}
		}
	}
}

// TestBoundCachedSDKCancellationJoinsRemoteWriter exercises the existing framed
// cancel-request settlement. That acknowledgment is not target-writer evidence.
func TestBoundCachedSDKCancellationJoinsRemoteWriter(t *testing.T) {
	for _, method := range []string{rpc.MethodForwardCached, rpc.MethodRelayStoredRoute} {
		t.Run(method, func(t *testing.T) {
			p, engine := newTestPair(t)
			work, stop := context.WithTimeout(t.Context(), 5*time.Second)
			defer stop()
			ctx, cancel := context.WithCancel(work)
			defer cancel()
			result := make(chan error, 1)
			go func() {
				if method == rpc.MethodForwardCached {
					result <- p.ForwardCached(ctx, rpc.ForwardCachedInput{IDs: []uint64{17}, Targets: []rpc.ForwardTarget{{Destination: "192.0.2.7", DestinationOwner: 91, DestinationSession: 71}}})
				} else {
					result <- p.RelayStoredRoute(ctx, rpc.RelayStoredRouteInput{Destination: "192.0.2.7", DestinationOwner: 91, DestinationSession: 71})
				}
			}()
			original := readEngineRequest(t, work, engine.mux)
			require.Equal(t, method, original.Method)
			cancel()
			settlement := readEngineRequest(t, work, engine.mux)
			require.Equal(t, rpc.MethodCancelRequest, settlement.Method)
			select {
			case err := <-result:
				t.Fatalf("bound cached write returned before final settlement: %v", err)
			default:
			}
			require.NoError(t, engine.mux.SendResult(work, settlement.ID, &rpc.CancelRequestOutput{RequestID: original.ID, Outcome: rpc.CancelRequestSettled}))
			err := <-result
			require.ErrorIs(t, err, context.Canceled)
			_, joined := errors.AsType[*rpc.CachedWriteJoinedError](err)
			require.False(t, joined, "generic request settlement remains unknown")
		})
	}
}
