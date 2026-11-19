// Design: docs/architecture/plugin/rib-storage-design.md -- late source-output capture.
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

// MUTATION: discard the post-mutation source-worker hold before returning a
// refused foreground join. A late UP then loses the sole release obligation.
// This root starts the real lease worker and holds an actual old-source send.
// Independent useful releases and eventual original-source release are required.
func TestLateDestinationUPKeepsPurgedSourceUntilWriterJoins(t *testing.T) {
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
	source := netip.MustParseAddr("192.0.2.226")
	const receipt uint64 = 8226
	sourceLeaseReceive(t, r, source, 1226, receipt, false, false)
	r.peerUp[1226] = true
	_, _, err := r.markStaleCommand([]string{source.String(), "0"})
	require.NoError(t, err)
	routes, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, routes, 1)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		for _, route := range input.Routes {
			if route.SourceReceipt == receipt {
				close(entered)
				<-release
			}
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	r.selectedDispatch = d
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		require.NoError(t, d.stop(cleanup))
		r.sourceLeases.stop()
	})
	require.NoError(t, d.publish(&routes[0]))
	r.sourceLeases.start(ctx)

	// Hold only the existing mutation lock. Observing the actual operation
	// count proves purge has captured its empty destination set and is now
	// waiting to remove input; no scheduler timing or sleep establishes it.
	r.peerMu.Lock()
	var unlockOnce sync.Once
	unlock := func() { unlockOnce.Do(r.peerMu.Unlock) }
	t.Cleanup(unlock)
	result := make(chan error, 1)
	joined := false
	t.Cleanup(func() {
		unlock()
		d.cancel()
		releaseOnce.Do(func() { close(release) })
		if !joined {
			cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
			defer stop()
			selectedDispatchAwait(t, cleanup, result)
		}
	})
	go func() { _, _, err := r.purgeStaleCommand([]string{source.String()}); result <- err }()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		r.sourceLeases.mu.Lock()
		lease := r.sourceLeases.leases[receipt]
		captured := lease != nil && lease.outputHolds == 1 && len(lease.outputs) == 0
		r.sourceLeases.mu.Unlock()
		if captured {
			break
		}
		select {
		case err := <-result:
			joined = true
			t.Fatalf("purge completed before its late-UP setup: %v", err)
		case <-tick.C:
		case <-ctx.Done():
			t.Fatal("purge did not acquire its pre-mutation operation hold", ctx.Err())
		}
	}
	destination := netip.MustParseAddr("192.0.2.227")
	require.NoError(t, d.destinationUp(destination, 2227))
	selectedDispatchAwait(t, ctx, entered)
	unlock()
	d.cancel()
	require.ErrorIs(t, selectedDispatchAwait(t, ctx, result), context.Canceled)
	joined = true
	require.Zero(t, r.bgpPeers[1226].Len())
	releaseIndependent := func(id, connectionID uint64) {
		ref, err := r.sourceLeases.acquire(ctx, netip.MustParseAddr("192.0.2.228"), 1228, id, connectionID)
		require.NoError(t, err)
		ref.Release()
		for {
			call := selectedDispatchAwait(t, ctx, calls)
			if call.Action != rpc.SourceLeaseRelease {
				continue
			}
			require.NotEqual(t, receipt, call.SourceReceipt, "late-UP writer lost its source release obligation")
			if call.SourceReceipt == id {
				break
			}
		}
	}
	releaseIndependent(8228, 2228)
	// This new receipt cannot have been in the preceding scan. Its useful
	// release proves a complete real worker pass covered the held source.
	releaseIndependent(8229, 2229)
	releaseOnce.Do(func() { close(release) })
	require.NoError(t, d.stop(ctx))
	r.sourceLeases.notify()
	for {
		call := selectedDispatchAwait(t, ctx, calls)
		if call.Action == rpc.SourceLeaseRelease && call.SourceReceipt == receipt {
			break
		}
	}
}
