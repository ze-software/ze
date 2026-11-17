// Design: docs/architecture/plugin/rib-storage-design.md -- bounded ordinary output.
package rib

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func selectedDispatchRoute(last byte, receipt uint64) advertiseRoute {
	raw := []byte{24, 198, 18, last}
	return advertiseRoute{key: advertiseKey{family: family.IPv4Unicast,
		prefix: netip.PrefixFrom(netip.AddrFrom4([4]byte{198, 18, last, 0}), 24)},
		keyNLRI: raw, revision: 900,
		route: ribevents.RecoveryRoute{Source: netip.MustParseAddr("192.0.2.77"),
			NLRI: raw, Attributes: []byte{0x40, 1, 1, 0}, NextHop: []byte{192, 0, 2, 77},
			MessageID: 34, SourcePolicyEpoch: 32, SourceReceipt: receipt}}
}

func selectedDispatchAnswer(input rpc.SelectRoutesInput, outcome rpc.SelectedRouteOutcome) *rpc.SelectRoutesOutput {
	output := &rpc.SelectRoutesOutput{Results: make([]rpc.SelectedRouteResult, len(input.Routes))}
	for i, route := range input.Routes {
		result := outcome
		if route.Kind == rpc.SelectedRouteAbsent {
			result = rpc.SelectedAbsent
		}
		output.Results[i] = rpc.SelectedRouteResult{Family: route.Family, KeyNLRI: route.KeyNLRI,
			Revision: route.Revision, CurrentRevision: route.Revision, Outcome: result}
	}
	return output
}

func selectedDispatchContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func selectedDispatchAwait[T any](t *testing.T, ctx context.Context, input <-chan T) T {
	t.Helper()
	select {
	case value := <-input:
		return value
	case <-ctx.Done():
		t.Fatal("selected dispatcher did not reach the asserted state", ctx.Err())
		var zero T
		return zero
	}
}

func TestSelectedDispatchBlockedRecipientDoesNotRetainChurn(t *testing.T) {
	ctx := selectedDispatchContext(t)
	a, b := netip.MustParseAddr("192.0.2.81"), netip.MustParseAddr("192.0.2.82")
	entered := make(chan struct{}, 1)
	send := func(ctx context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		if input.Destination == a.String() {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}
	d, err := newSelectedDispatch(ctx, send, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(1, 77)
	require.NoError(t, d.publish(route))
	require.NoError(t, d.destinationUp(a, 801))
	selectedDispatchAwait(t, ctx, entered)
	require.NoError(t, d.destinationUp(b, 802))
	for n := byte(2); n < 200; n++ {
		next := selectedDispatchRoute(n, 77)
		require.NoError(t, d.publish(next))
		require.NoError(t, d.remove(next.key))
	}
	require.NoError(t, d.barrier(ctx, b))
	d.mu.Lock()
	require.Len(t, d.current, 1)
	require.Len(t, d.destinations[selectedDestinationID{peer: a, session: 801}].acknowledged, 1, "only the one possible in-flight install remains")
	require.Len(t, d.destinations[selectedDestinationID{peer: a, session: 801}].inFlight, 1)
	require.Len(t, d.destinations[selectedDestinationID{peer: a, session: 801}].pending, 1)
	require.Len(t, d.destinations[selectedDestinationID{peer: b, session: 802}].acknowledged, 1)
	require.Empty(t, d.destinations[selectedDestinationID{peer: b, session: 802}].pending)
	d.mu.Unlock()
	require.NoError(t, d.destinationDown(ctx, a, 801))
	d.mu.Lock()
	require.NotContains(t, d.destinations, selectedDestinationID{peer: a, session: 801})
	d.mu.Unlock()
}

func TestSelectedDispatchInflightInstallStillOwesAbsence(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.83")
	entered, release := make(chan struct{}), make(chan struct{})
	requests := make(chan rpc.SelectRoutesInput, 4)
	var once sync.Once
	send := func(ctx context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		requests <- input
		if input.Routes[0].Kind == rpc.SelectedRouteReceived {
			once.Do(func() { close(entered) })
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}
	d, err := newSelectedDispatch(ctx, send, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(3, 77)
	require.NoError(t, d.publish(route))
	require.NoError(t, d.destinationUp(peer, 803))
	selectedDispatchAwait(t, ctx, entered)
	require.NoError(t, d.remove(route.key))
	close(release)
	require.NoError(t, d.barrier(ctx, peer))
	announce := selectedDispatchAwait(t, ctx, requests)
	absent := selectedDispatchAwait(t, ctx, requests)
	require.Equal(t, rpc.SelectedRouteReceived, announce.Routes[0].Kind)
	require.Equal(t, rpc.SelectedRouteAbsent, absent.Routes[0].Kind)
	require.Greater(t, absent.Routes[0].Revision, announce.Routes[0].Revision)
	d.mu.Lock()
	require.Empty(t, d.current)
	require.Empty(t, d.destinations[selectedDestinationID{peer: peer, session: 803}].acknowledged)
	require.Empty(t, d.destinations[selectedDestinationID{peer: peer, session: 803}].inFlight)
	d.mu.Unlock()
}

func TestSelectedDispatchSuppressedRemovalHasNoTombstone(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.84")
	requests := make(chan rpc.SelectRoutesInput, 4)
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		requests <- input
		return selectedDispatchAnswer(input, rpc.SelectedSuppressed), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(4, 77)
	require.NoError(t, d.publish(route))
	require.NoError(t, d.destinationUp(peer, 804))
	require.NoError(t, d.barrier(ctx, peer))
	selectedDispatchAwait(t, ctx, requests)
	require.NoError(t, d.remove(route.key))
	require.NoError(t, d.barrier(ctx, peer))
	require.Empty(t, requests, "suppression already settled actual output removal")
	d.mu.Lock()
	require.Empty(t, d.destinations[selectedDestinationID{peer: peer, session: 804}].acknowledged)
	d.mu.Unlock()
}

func TestSelectedDispatchCurrentScopeRenewsRestoredSource(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.85")
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(5, 77)
	require.NoError(t, d.publish(route))
	require.NoError(t, d.destinationUp(peer, 805))
	query := rpc.CurrentSelectionInput{Destination: peer.String(), DestinationSession: 805,
		Keys: []rpc.CurrentSelectionKey{{Family: family.IPv4Unicast.String(), KeyNLRI: route.keyNLRI, Revision: 1}}}
	first, err := d.currentSelection(ctx, query)
	require.NoError(t, err)
	require.Equal(t, uint64(1), first.Results[0].CurrentRevision, "capture revision 900 is not the scope clock")
	require.NoError(t, d.publish(selectedDispatchRoute(6, 88)))
	second, err := d.currentSelection(ctx, query)
	require.NoError(t, err)
	require.Greater(t, second.Cut, first.Cut)
	require.Equal(t, first.Results[0].CurrentRevision, second.Results[0].CurrentRevision)
	require.NoError(t, d.renewSource([]uint64{77}))
	restored, err := d.currentSelection(ctx, query)
	require.NoError(t, err)
	require.Greater(t, restored.Results[0].CurrentRevision, second.Results[0].CurrentRevision)
	require.NoError(t, d.remove(route.key))
	absent, err := d.currentSelection(ctx, query)
	require.NoError(t, err)
	require.Equal(t, rpc.CurrentSelectionAbsent, absent.Results[0].Presence)
	require.Zero(t, absent.Results[0].CurrentRevision)
	query.DestinationSession++
	_, err = d.currentSelection(ctx, query)
	require.Error(t, err)
}

func TestSelectedDispatchFailureDoesNotCompleteBarrier(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.86")
	failed := make(chan struct{}, 1)
	d, err := newSelectedDispatch(ctx, func(_ context.Context, _ rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		select {
		case failed <- struct{}{}:
		default:
		}
		return nil, errors.New("destination explicitly refuses the selecting process")
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	require.NoError(t, d.publish(selectedDispatchRoute(7, 77)))
	require.NoError(t, d.destinationUp(peer, 806))
	selectedDispatchAwait(t, ctx, failed)
	bound, cancel := context.WithCancel(ctx)
	cancel()
	err = d.barrier(bound, peer)
	require.Error(t, err)
	require.Contains(t, err.Error(), peer.String())
	d.mu.Lock()
	require.Len(t, d.destinations[selectedDestinationID{peer: peer, session: 806}].acknowledged, 1, "a possibly applied output remains owed after failure")
	d.mu.Unlock()
}
