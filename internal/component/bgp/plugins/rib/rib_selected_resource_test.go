// Design: docs/architecture/plugin/rib-storage-design.md -- bounded selected ownership.
package rib

import (
	"context"
	"errors"
	"math"
	"net/netip"
	"runtime"
	"slices"
	"strings"
	"testing"
	"weak"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// selectedBatchWeakRoutes leaves the input frame before its caller collects.
// Keeping the returned batch alive must not keep an unchosen StoredRoute alive.
func selectedBatchWeakRoutes(t *testing.T) (rpc.SelectRoutesInput, []selectedDispatchItem, []weak.Pointer[rpc.StoredRoute]) {
	t.Helper()
	items := make([]selectedDispatchItem, 4)
	tracked := make([]weak.Pointer[rpc.StoredRoute], len(items))
	attributes := strings.Repeat("a", rpc.MaxMessageSize/2)
	for i := range items {
		stored := &rpc.StoredRoute{SourcePeer: "192.0.2.77", Family: family.IPv4Unicast.String(), AttrHex: attributes}
		tracked[i] = weak.Make(stored)
		items[i] = selectedDispatchItem{route: rpc.SelectedRoute{Kind: rpc.SelectedRouteReceived,
			Family: family.IPv4Unicast.String(), KeyNLRI: []byte{24, 198, 18, byte(i)}, Revision: uint64(i + 1), Route: stored}}
	}
	batch, chosen, err := selectedDispatchBatch(rpc.SelectRoutesInput{Destination: "192.0.2.101", DestinationSession: 1001}, slices.Values(items))
	require.NoError(t, err)
	return batch, chosen, tracked
}

// TestSelectedDispatchBatchReleasesUnchosenRoutes drives the real frame packer
// with an initially populated backlog, then observes ownership after that
// backlog leaves scope. Capacity clipping alone cannot satisfy the weak checks.
func TestSelectedDispatchBatchReleasesUnchosenRoutes(t *testing.T) {
	batch, chosen, tracked := selectedBatchWeakRoutes(t)
	require.NotEmpty(t, chosen)
	require.Less(t, len(chosen), len(tracked))
	runtime.GC()
	for i := range tracked {
		if i < len(chosen) {
			require.NotNil(t, tracked[i].Value(), "the admitted batch still owns its payload")
			continue
		}
		require.True(t, tracked[i].Value() == nil, "an unchosen payload must not survive in a batch backing-array tail")
	}
	require.Equal(t, len(chosen), cap(chosen), "the admitted item array has no historical capacity")
	require.Equal(t, len(batch.Routes), cap(batch.Routes), "the admitted route array has no historical capacity")
	runtime.KeepAlive(batch)
	runtime.KeepAlive(chosen)
}

// TestSelectedDispatchDownReleasesRoutesWhenWarningClearFails preserves a useful
// same-address sibling while a failed diagnostic clear outlives ended output.
func TestSelectedDispatchDownReleasesRoutesWhenWarningClearFails(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.102")
	raised := make(chan struct{}, 1)
	report := func(_ context.Context, cause error) error {
		if cause == nil {
			return errors.New("controlled warning clear failure")
		}
		select {
		case raised <- struct{}{}:
		default:
		}
		return nil
	}
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		if input.DestinationSession == 1002 {
			return nil, errors.New("controlled destination send failure")
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, report)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(102, 77)
	require.NoError(t, d.publish(&route))
	require.NoError(t, d.destinationUp(peer, 1002))
	require.NoError(t, d.destinationUp(peer, 1003))
	selectedDispatchAwait(t, ctx, raised)
	require.NoError(t, d.destinationDown(ctx, peer, 1002), "diagnostic failure does not retain ended route ownership")
	d.mu.Lock()
	require.NotContains(t, d.destinations, selectedDestinationID{peer: peer, session: 1002})
	require.Contains(t, d.destinations, selectedDestinationID{peer: peer, session: 1003})
	d.mu.Unlock()
	require.NoError(t, d.barrier(ctx, peer), "the healthy same-address Session remains useful")
}

// TestSelectedDispatchCaptureFailureInvalidatesPublishedAuthority exercises the
// current-selection callback and both empty and established completion barriers.
func TestSelectedDispatchCaptureFailureInvalidatesPublishedAuthority(t *testing.T) {
	for _, overflow := range []bool{false, true} {
		t.Run(map[bool]string{false: "extraction", true: "clock"}[overflow], func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			peer := netip.MustParseAddr("192.0.2.103")
			d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
			}, nil, nil)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
			r := newTestRIBManager(t)
			r.selectedDispatch = d
			route := selectedDispatchRoute(103, 77)
			r.commitAdvertise(&route)
			require.NoError(t, d.destinationUp(peer, 1004))
			require.NoError(t, d.barrier(ctx, peer))
			if overflow {
				r.advertise.mu.Lock()
				r.advertise.revision = math.MaxUint64
				r.advertise.mu.Unlock()
				r.commitAdvertise(&route)
			} else {
				r.failAdvertise(errors.New("controlled capture failure"))
			}
			_, err = d.currentSelection(ctx, rpc.CurrentSelectionInput{Destination: peer.String(), DestinationSession: 1004,
				Keys: []rpc.CurrentSelectionKey{{Family: route.key.family.String(), KeyNLRI: route.keyNLRI, Revision: 1}}})
			require.Error(t, err, "the callback must not authorize its older retained capture")
			require.Error(t, d.barrier(ctx))
			require.Error(t, d.barrier(ctx, peer))
			require.Error(t, d.barrierSource(ctx, nil), "an empty target set cannot hide failed publication")
		})
	}
}

// TestSelectedDispatchEmptySourceBarrierIgnoresUnrelatedUncertainty observes a
// genuinely lost write answer before asking to settle no source at all.
func TestSelectedDispatchEmptySourceBarrierIgnoresUnrelatedUncertainty(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.104")
	failed := make(chan struct{}, 1)
	report := func(_ context.Context, cause error) error {
		if cause != nil {
			select {
			case failed <- struct{}{}:
			default:
			}
		}
		return nil
	}
	d, err := newSelectedDispatch(ctx, func(_ context.Context, _ rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		return nil, errors.New("controlled lost selected answer")
	}, nil, report)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(104, 77)
	require.NoError(t, d.publish(&route))
	require.NoError(t, d.destinationUp(peer, 1005))
	selectedDispatchAwait(t, ctx, failed)
	bound, cancel := context.WithCancel(ctx)
	cancel()
	require.NoError(t, d.barrierSource(bound, nil), "no source has no affected output, even when another source is uncertain")
}
