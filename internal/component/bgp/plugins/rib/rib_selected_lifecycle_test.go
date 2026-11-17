// Design: docs/architecture/plugin/rib-storage-design.md -- selected lifecycle joins.
package rib

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestSelectedDispatchSourceBarrierKeepsOldInflightOwner(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.90")
	first, second := make(chan struct{}), make(chan struct{})
	firstRelease, secondRelease := make(chan struct{}), make(chan struct{})
	var calls atomic.Uint32
	d, err := newSelectedDispatch(ctx, func(ctx context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		var release <-chan struct{}
		switch calls.Add(1) {
		case 1:
			close(first)
			release = firstRelease
		case 2:
			close(second)
			release = secondRelease
		default:
			return nil, errors.New("unexpected selected request")
		}
		select {
		case <-release:
			return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(90, 77)
	require.NoError(t, d.publish(&route))
	require.NoError(t, d.destinationUp(peer, 890))
	selectedDispatchAwait(t, ctx, first)
	close(firstRelease)
	require.NoError(t, d.barrier(ctx, peer))
	route.route.SourceReceipt = 88
	require.NoError(t, d.publish(&route))
	selectedDispatchAwait(t, ctx, second)
	bound, cancel := context.WithCancel(ctx)
	cancel()
	err = d.barrierSource(bound, []uint64{77})
	require.Error(t, err, "replacing desired source must not hide the old output while its replacement is in flight")
	require.Contains(t, err.Error(), peer.String())
	close(secondRelease)
	require.NoError(t, d.barrier(ctx, peer))
	require.NoError(t, d.barrierSource(ctx, []uint64{77}))
}

func TestSelectedDispatchDestinationFailureClearsAfterUsefulOutput(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.91")
	var restored atomic.Bool
	reports := make(chan bool, 4)
	report := func(_ context.Context, cause error) error {
		if cause != nil {
			if !strings.Contains(cause.Error(), peer.String()) {
				return errors.New("destination diagnostic has another owner")
			}
		}
		reports <- cause != nil
		return nil
	}
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		if !restored.Load() {
			return nil, errors.New("selecting process has no destination send grant")
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, report)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(91, 77)
	require.NoError(t, d.publish(&route))
	require.NoError(t, d.destinationUp(peer, 891))
	require.True(t, selectedDispatchAwait(t, ctx, reports))
	findings := d.doctorFailures()
	require.Len(t, findings, 1)
	require.Contains(t, findings[0].Message, peer.String())
	require.Contains(t, findings[0].Message, "send grant")
	restored.Store(true)
	require.NoError(t, d.renewSource([]uint64{77}))
	require.NoError(t, d.barrier(ctx, peer))
	require.False(t, selectedDispatchAwait(t, ctx, reports))
	require.Empty(t, d.doctorFailures())
}

func TestSelectedDispatchReadyFollowsFinalWriterReceipt(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.92")
	entered, release := make(chan struct{}), make(chan struct{})
	ready := make(chan uint64, 1)
	d, err := newSelectedDispatch(ctx, func(ctx context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		close(entered)
		select {
		case <-release:
			return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, func(_ context.Context, destination string, session uint64) error {
		if destination != peer.String() {
			return errors.New("readiness reached another destination")
		}
		ready <- session
		return nil
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(92, 77)
	require.NoError(t, d.publish(&route))
	require.NoError(t, d.destinationUp(peer, 892))
	selectedDispatchAwait(t, ctx, entered)
	require.Empty(t, ready)
	close(release)
	require.Equal(t, uint64(892), selectedDispatchAwait(t, ctx, ready))
	require.NoError(t, d.barrier(ctx, peer))
}

func TestSelectedDispatchOpaqueKeysRemainIndependent(t *testing.T) {
	for _, tc := range opaqueFamilyCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			peer := netip.MustParseAddr("192.0.2.93")
			d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
			}, nil, nil)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
			first := selectedDispatchRoute(93, 77)
			first.keyNLRI = append([]byte(nil), tc.key...)
			first.key, err = selectedDispatchKey(tc.fam.String(), first.keyNLRI)
			require.NoError(t, err)
			first.route.NLRI = framedKey(0, first.keyNLRI)
			first.route.AddPath, first.route.PathID = true, 0
			second := first
			second.keyNLRI = append([]byte(nil), first.keyNLRI...)
			if tc.fam == vpnv4Family {
				second.keyNLRI[11] = 2
			} else {
				second.keyNLRI[9] = 2
			}
			second.key, err = selectedDispatchKey(tc.fam.String(), second.keyNLRI)
			require.NoError(t, err)
			second.route.NLRI = framedKey(0, second.keyNLRI)
			require.NoError(t, d.publish(&first))
			require.NoError(t, d.publish(&second))
			require.NoError(t, d.destinationUp(peer, 893))
			require.NoError(t, d.barrier(ctx, peer))
			require.NoError(t, d.remove(first.key))
			require.NoError(t, d.barrier(ctx, peer))
			d.mu.Lock()
			require.Len(t, d.current, 1)
			require.Contains(t, d.current, second.key)
			require.Len(t, d.destinations[selectedDestinationID{peer: peer, session: 893}].acknowledged, 1)
			require.Empty(t, d.destinations[selectedDestinationID{peer: peer, session: 893}].pending)
			d.mu.Unlock()
		})
	}
}
