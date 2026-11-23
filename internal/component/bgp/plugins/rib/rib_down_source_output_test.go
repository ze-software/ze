// Design: docs/architecture/plugin/rib-storage-design.md -- source-DOWN lease lifetime.
// Related: rib_source_output_test.go -- GR purge and expiry source-output settlement.
// RFC: rfc/short/rfc4271.md -- Section 6 route invalidation and peer advertisement order.
package rib

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestPeerDownKeepsSourceLeaseUntilSelectedOutputSettles proves that a non-GR
// peer DOWN holds the departed source's lease until every affected ordinary
// selected-output destination finishes writing its withdrawal or replacement,
// then wakes and releases the lease without raising a spurious doctor failure.
//
// VALIDATES: both structured and JSON peer-DOWN handlers attach affected
// selected destinations to the source lease before releasing PeerRIB input.
// PREVENTS: releasing the last source lease in bgp-rib while a destination's
// withdrawal or alternate replacement is still queued or in flight.
// MUTATION: omit beginDownSourceHold/finishDownSourceHold in handleStructuredState
// or handleState: SourceLeaseRelease fires immediately while the selected writer
// is still paused.
//
// RFC 4271 Section 6: "Before the invalid routes are deleted from the system,
// it advertises, to its peers, either withdraws for the routes marked as
// invalid, or the new best routes before the invalid routes are deleted from
// the system."
func TestPeerDownKeepsSourceLeaseUntilSelectedOutputSettles(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		name := "structured"
		if jsonRail {
			name = "json"
		}
		t.Run(name, func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			client, remote := net.Pipe()
			bridge := rpc.NewDirectBridge()
			calls := make(chan rpc.SourceLeaseInput, 32)
			bridge.SetDispatchNative(func(_ context.Context, method string, raw any) (any, bool, error) {
				if method != rpc.MethodSourceLease {
					return nil, false, nil
				}
				input, valid := raw.(rpc.SourceLeaseInput)
				if !valid {
					return nil, true, errors.New("source lease request has the wrong type")
				}
				if err := input.Validate(time.Now()); err != nil {
					return nil, true, err
				}
				calls <- input
				return &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}, true, nil
			})
			bridge.SetDispatchRPC(func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
				if method != rpc.MethodReportIssue {
					return nil, errors.New("unexpected source worker RPC")
				}
				return json.RawMessage(`{}`), nil
			})
			bridge.SetReady()
			p := sdk.NewWithConn("bgp-rib", rpc.NewBridgedConn(client, bridge))
			t.Cleanup(func() { require.NoError(t, p.Close()); require.NoError(t, remote.Close()) })

			r := newRIBManager(p)
			r.sourceLeases = newRIBSourceLeases(r)
			electionFixtureLocRIB(t, r, netip.MustParseAddr("10.0.0.1"))

			source := netip.MustParseAddr("192.0.2.231")
			const (
				sourceOwner uint64 = 1231
				sourceConn  uint64 = 41
				receipt     uint64 = 8231
			)
			r.peerMu.Lock()
			r.peerUp[sourceOwner] = true
			r.recordPeerStateLocked(sourceOwner, source, sourceConn, sourceConn, true)
			r.peerMu.Unlock()
			sourceLeaseReceive(t, r, source, sourceOwner, receipt, jsonRail, false)

			routes, err := r.advertiseSnapshot()
			require.NoError(t, err)
			require.Len(t, routes, 1)

			entered, release := make(chan struct{}), make(chan struct{})
			var enterOnce, releaseOnce sync.Once
			d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				for _, route := range input.Routes {
					if route.Kind == rpc.SelectedRouteAbsent {
						enterOnce.Do(func() { close(entered) })
						<-release
					}
				}
				return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
			}, nil, nil)
			require.NoError(t, err)
			d.setOnSettled(r.sourceLeases.notify)
			r.selectedDispatch = d
			t.Cleanup(func() {
				releaseOnce.Do(func() { close(release) })
				cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
				defer stop()
				require.NoError(t, d.stop(cleanup))
				r.sourceLeases.stop()
			})
			require.NoError(t, d.publish(&routes[0]))
			destination := netip.MustParseAddr("192.0.2.232")
			require.NoError(t, d.destinationUp(destination, 2232))
			require.NoError(t, d.barrier(ctx, destination))

			// Drain the initial acquire call so later reads observe only releases.
			for {
				call := selectedDispatchAwait(t, ctx, calls)
				if call.Action == rpc.SourceLeaseAcquire && call.SourceReceipt == receipt {
					break
				}
			}
			r.sourceLeases.start(ctx)

			if jsonRail {
				r.handleState(&Event{
					PeerOwner:          sourceOwner,
					DestinationSession: sourceConn,
					ConnectionID:       sourceConn,
					Peer: mustMarshal(t, map[string]any{
						"state":  "down",
						"remote": map[string]any{"address": source.String()},
					}),
				})
			} else {
				r.handleStructuredState(&rpc.StructuredEvent{
					PeerAddress:        source.String(),
					PeerOwner:          sourceOwner,
					DestinationSession: sourceConn,
					ConnectionID:       sourceConn,
					State:              rpc.SessionStateDown,
					Reason:             rpc.ReasonTCPFailure,
				})
			}

			selectedDispatchAwait(t, ctx, entered)
			require.Nil(t, r.bgpPeers[sourceOwner], "departed peer input is cleared from Adj-RIB-In")
			require.Empty(t, r.sourceLeases.doctorFailures(), "normal convergence must not raise a source-lease doctor warning")

			// Release two fresh independent sources to prove the live background
			// lease worker completed a full scan while receipt stayed held.
			releaseIndependent := func(id, conn uint64) {
				ref, acquireErr := r.sourceLeases.acquire(ctx, netip.MustParseAddr("192.0.2.233"), 1233, id, conn)
				require.NoError(t, acquireErr)
				ref.Release()
				for {
					call := selectedDispatchAwait(t, ctx, calls)
					if call.Action != rpc.SourceLeaseRelease {
						continue
					}
					require.NotEqual(t, receipt, call.SourceReceipt, "source lease released while selected withdrawal was still in flight")
					if call.SourceReceipt == id {
						break
					}
				}
			}
			releaseIndependent(8234, 2234)
			releaseIndependent(8235, 2235)
			require.Empty(t, r.sourceLeases.doctorFailures(), "held normal convergence still raises no doctor failure")

			// Unblocking the destination write must complete absence delivery
			// and wake the lease worker automatically without manual notify.
			releaseOnce.Do(func() { close(release) })
			require.NoError(t, d.barrier(ctx, destination))
			for {
				call := selectedDispatchAwait(t, ctx, calls)
				if call.Action == rpc.SourceLeaseRelease && call.SourceReceipt == receipt {
					break
				}
			}
			require.Empty(t, r.sourceLeases.doctorFailures())
		})
	}
}
