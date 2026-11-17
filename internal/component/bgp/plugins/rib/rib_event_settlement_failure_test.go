// Design: docs/architecture/api/process-protocol.md -- actionable quiesce failures.
package rib

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestQuiesceFailureNamesSelectedDestination requires the actual manager path
// to retain the dispatcher's actionable destination failure across both rails.
// A plain callback deadline is insufficient to identify the held final writer.
func TestQuiesceFailureNamesSelectedDestination(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("native=%t", native), func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			peers := [2]netip.Addr{netip.MustParseAddr("192.0.2.181"), netip.MustParseAddr("192.0.2.182")}
			entered := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			send := func(ctx context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				if input.Destination == peers[0].String() {
					close(entered)
					select {
					case <-release:
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				}
				return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
			}
			manager, r := startQuiesceRIB(t, ctx, native, peers, send)
			t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
			route := selectedDispatchRoute(89, 0x7789)
			require.NoError(t, r.selectedDispatch.publish(&route))
			require.NoError(t, r.selectedDispatch.destinationUp(peers[0], 181))
			selectedDispatchAwait(t, ctx, entered)
			require.NoError(t, r.selectedDispatch.destinationUp(peers[1], 182))
			require.NoError(t, r.selectedDispatch.barrier(ctx, peers[1]))

			blocked, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			err := manager.DrainEventDelivery(blocked)
			cancel()
			require.Error(t, err)
			require.ErrorContains(t, err, peers[0].String(), "quiesce must name the held destination before cleanup releases it")

			releaseOnce.Do(func() { close(release) })
			require.NoError(t, manager.DrainEventDelivery(ctx))
		})
	}
}
