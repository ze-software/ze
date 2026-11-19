// Design: docs/architecture/plugin/rib-storage-design.md -- physical source completion.
package rib

import (
	"context"
	"net/netip"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The actual registered source commands must not acknowledge cleanup or release
// the real SDK source lease while the selected writer still owns its absence.
// Cancellation gives an explicit refusal while that writer remains held; after
// its independent release, the original source right can finally be released.
// MUTATION: bypass settlePurgedSource in removePhysicalSource. Purge/release then
// falsely answer success and expiry can release before the held absence joins.
func TestGRSourceCleanupJoinsHeldSelectedOutput(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, operation := range []string{"purge-stale", "release-routes", "expiry"} {
			t.Run(strconv.FormatBool(native)+"/"+operation, func(t *testing.T) {
				ctx := selectedDispatchContext(t)
				r, calls := sourceLeaseTestRIB(t, native)
				source := netip.MustParseAddr("192.0.2.248")
				sourceLeaseReceive(t, r, source, 1230, 8230, false, false)
				r.peerUp[1230] = true
				grSourceCommandRun(t, r, "retain-routes", 1230, 41, "on-down", "families", "ipv4/unicast")
				grSourceCommandRun(t, r, "mark-stale", 1230, 41, "restart-time", "0")
				routes, err := r.advertiseSnapshot()
				require.NoError(t, err)
				require.Len(t, routes, 1)
				entered, release := make(chan struct{}), make(chan struct{})
				var once sync.Once
				dispatcher, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
					for _, route := range input.Routes {
						if route.Kind == rpc.SelectedRouteAbsent {
							close(entered)
							<-release
						}
					}
					return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
				}, nil, nil)
				require.NoError(t, err)
				r.selectedDispatch = dispatcher
				t.Cleanup(func() { once.Do(func() { close(release) }); require.NoError(t, dispatcher.stop(ctx)) })
				require.NoError(t, dispatcher.publish(&routes[0]))
				destination := netip.MustParseAddr("192.0.2.249")
				require.NoError(t, dispatcher.destinationUp(destination, 2230))
				require.NoError(t, dispatcher.barrier(ctx, destination))
				result := make(chan error, 1)
				joined := false
				t.Cleanup(func() {
					dispatcher.cancel()
					once.Do(func() { close(release) })
					if !joined {
						selectedDispatchAwait(t, ctx, result)
					}
				})
				state := r.grState[1230]
				go func() {
					if operation == "expiry" {
						r.autoExpireStale(1230, state)
						result <- nil
						return
					}
					_, _, err := r.handleCommand("request bgp rib source "+operation, "*", []string{"owner", "1230", "connection", "41"})
					result <- err
				}()
				selectedDispatchAwait(t, ctx, entered)
				dispatcher.cancel()
				cleanupErr := selectedDispatchAwait(t, ctx, result)
				joined = true
				if operation != "expiry" {
					require.ErrorIs(t, cleanupErr, context.Canceled, "a held final absence cannot be acknowledged")
				}
				require.Zero(t, calls.count(rpc.SourceLeaseRelease, 8230), "the held writer still owns the original source right")
				require.Nil(t, r.bgpPeers[1230], "refused output join does not restore removed input")
				once.Do(func() { close(release) })
				require.NoError(t, dispatcher.stop(ctx))
				require.NoError(t, r.sourceLeases.settle(ctx))
				require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, 8230))
			})
		}
	}
}
