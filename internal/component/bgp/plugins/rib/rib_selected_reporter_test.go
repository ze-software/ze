// Design: docs/architecture/plugin/rib-storage-design.md -- independent report ownership.
package rib

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedDispatchWarningClearRetriesWithoutEndedRoutes drives a failed
// clear beyond DOWN, then observes retry with useful same-address output and
// a valid same-token worker replacement instead of a dead retained frontier.
func TestSelectedDispatchWarningClearRetriesWithoutEndedRoutes(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.105")
	raised, firstClear, cleared := make(chan struct{}, 1), make(chan struct{}, 1), make(chan struct{}, 1)
	var clearCalls atomic.Uint32
	var restored atomic.Bool
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		if input.DestinationSession == 1051 {
			if !restored.Load() {
				return nil, errors.New("controlled selected output failure")
			}
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, func(_ context.Context, cause error) error {
		if cause != nil {
			select {
			case raised <- struct{}{}:
			default:
			}
			return nil
		}
		if clearCalls.Add(1) == 1 {
			firstClear <- struct{}{}
			return errors.New("controlled lost warning-clear answer")
		}
		select {
		case cleared <- struct{}{}:
		default:
		}
		return nil
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	require.NoError(t, d.publish(selectedDispatchRoute(105, 77)))
	require.NoError(t, d.destinationUp(peer, 1051))
	require.NoError(t, d.destinationUp(peer, 1052))
	selectedDispatchAwait(t, ctx, raised)
	require.NoError(t, d.destinationDown(ctx, peer, 1051))
	selectedDispatchAwait(t, ctx, firstClear)
	d.mu.Lock()
	require.Len(t, d.destinations, 1)
	require.Contains(t, d.destinations, selectedDestinationID{peer: peer, session: 1052})
	d.mu.Unlock()
	require.NoError(t, d.barrier(ctx, peer), "warning delivery cannot block the surviving destination")
	restored.Store(true)
	require.NoError(t, d.destinationUp(peer, 1051))
	require.NoError(t, d.barrier(ctx, peer), "a rollback UP starts a live replacement worker")
	selectedDispatchAwait(t, ctx, cleared)
	require.GreaterOrEqual(t, clearCalls.Load(), uint32(2))
	d.mu.Lock()
	require.Len(t, d.destinations, 2, "only the two current Sessions remain")
	d.mu.Unlock()
}

// TestSelectedDispatchLostRaiseStillClearsUncertainIssue proves that a lost
// answer is not mistaken for proof that the daemon never raised the warning.
func TestSelectedDispatchLostRaiseStillClearsUncertainIssue(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.106")
	raised, cleared := make(chan struct{}, 1), make(chan struct{}, 1)
	d, err := newSelectedDispatch(ctx, func(_ context.Context, _ rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		return nil, errors.New("controlled selected output failure")
	}, nil, func(_ context.Context, cause error) error {
		if cause != nil {
			select {
			case raised <- struct{}{}:
			default:
			}
			return errors.New("controlled lost warning-raise answer")
		}
		select {
		case cleared <- struct{}{}:
		default:
		}
		return nil
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	require.NoError(t, d.publish(selectedDispatchRoute(106, 77)))
	require.NoError(t, d.destinationUp(peer, 1061))
	selectedDispatchAwait(t, ctx, raised)
	require.NoError(t, d.destinationDown(ctx, peer, 1061))
	selectedDispatchAwait(t, ctx, cleared)
	d.mu.Lock()
	require.Empty(t, d.destinations)
	d.mu.Unlock()
}

// TestSelectedDispatchSuccessfulRaisesRespectOwnerWindow changes the current
// failure past the daemon's shared raise allowance, then clears and fails again.
// The bubble waits for each reconciliation before advancing the actual timer:
// one window sends one raise, clears promptly, and publishes only its latest
// still-current failure when another raise becomes admissible.
func TestSelectedDispatchSuccessfulRaisesRespectOwnerWindow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		var reports []error
		d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
			return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
		}, nil, func(_ context.Context, cause error) error {
			reports = append(reports, cause)
			return nil
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, d.stop(ctx)) }()
		peer := netip.MustParseAddr("192.0.2.107")
		require.NoError(t, d.destinationUp(peer, 1071))
		synctest.Wait()
		change := func(cause error) {
			d.mu.Lock()
			d.destinations[selectedDestinationID{peer: peer, session: 1071}].failure = cause
			d.signalReportLocked()
			d.mu.Unlock()
			synctest.Wait()
		}
		for attempt := range rpc.ReportRaisesPerWindow + 1 {
			change(fmt.Errorf("controlled changing failure %d", attempt))
		}
		require.Len(t, reports, 1, "one changing aggregate must not spend the owner's shared raise allowance")
		require.ErrorContains(t, reports[0], "controlled changing failure 0")
		change(nil)
		require.Len(t, reports, 2, "clearing a recovered issue must not wait for the raise window")
		require.NoError(t, reports[1])
		change(errors.New("controlled replaced failure"))
		latest := errors.New("controlled latest failure")
		change(latest)
		require.Len(t, reports, 2, "a clear cannot renew the owner's raise allowance")
		// The timer under test runs on synctest's clock; no wall-clock sleep or
		// polling determines when this report becomes eligible.
		<-time.After(rpc.ReportRaiseWindow)
		synctest.Wait()
		require.Len(t, reports, 3)
		require.ErrorIs(t, reports[2], latest, "only the latest current aggregate is raised after the window")
		change(nil)
		require.Len(t, reports, 4)
		require.NoError(t, reports[3])
	})
}
