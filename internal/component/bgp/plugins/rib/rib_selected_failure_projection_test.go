// Design: docs/architecture/plugin/rib-storage-design.md -- bounded settlement failures.
// Related: rib_selected_barrier.go -- timeout projection and original causes.
package rib

import (
	"context"
	"errors"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedBarrierFailurePreservesCauses pairs a small diagnostic with one
// whose single underlying transport cause exceeds the settlement field. Both
// must preserve the original errors locally while producing a usable receipt.
func TestSelectedBarrierFailurePreservesCauses(t *testing.T) {
	for _, width := range []int{16, rpc.EventSettlementErrorBytesMax + 1} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			cause := errors.New("transport refused: " + strings.Repeat("x", width))
			reported := make(chan struct{}, 1)
			d, err := newSelectedDispatch(ctx,
				func(context.Context, rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) { return nil, cause },
				nil, func(context.Context, error) error {
					reported <- struct{}{}
					return nil
				})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
			peer := netip.MustParseAddr("2001:db8::4096")
			require.NoError(t, d.publish(selectedDispatchRoute(91, 0x7791)))
			require.NoError(t, d.destinationUp(peer, 4096))
			selectedDispatchAwait(t, ctx, reported)
			failed, cancel := context.WithCancel(ctx)
			cancel()
			err = d.barrier(failed)
			require.Error(t, err)
			require.True(t, errors.Is(err, cause), "original transport cause was lost: %v", err)
			require.ErrorIs(t, err, context.Canceled)
			require.LessOrEqual(t, len(err.Error()), rpc.EventSettlementErrorBytesMax)
			require.ErrorContains(t, err, "destination "+peer.String()+" Session 4096")
			if width < rpc.EventSettlementErrorBytesMax {
				require.ErrorContains(t, err, cause.Error())
			} else {
				require.ErrorContains(t, err, "failure details omitted")
			}
			findings := d.doctorFailures()
			require.Len(t, findings, 1)
			require.Contains(t, findings[0].Message, cause.Error())
		})
	}
}
