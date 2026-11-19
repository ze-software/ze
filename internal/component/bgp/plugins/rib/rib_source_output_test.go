// Design: docs/architecture/plugin/rib-storage-design.md -- source mutation completion.
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

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// MUTATION: discard the before-phase destination capture. Unsent A vanishes
// while an unrelated initial write holds the worker, so only B remains owed.
func TestSourcePolicyJoinsReplacementOfNeverWrittenWinner(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.219")
	source := netip.MustParseAddr("192.0.2.77")
	initialEntered, initialRelease := make(chan struct{}), make(chan struct{})
	replacementEntered, replacementRelease := make(chan struct{}), make(chan struct{})
	var initialOnce, replacementOnce sync.Once
	blocked := selectedDispatchRoute(217, 8217)
	previous := selectedDispatchRoute(218, 8218)
	next := previous
	next.route.SourceReceipt = 8219
	d, err := newSelectedDispatch(ctx, func(work context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		for _, route := range input.Routes {
			var entered, release chan struct{}
			switch route.SourceReceipt {
			case blocked.route.SourceReceipt:
				entered, release = initialEntered, initialRelease
			case next.route.SourceReceipt:
				entered, release = replacementEntered, replacementRelease
			}
			if entered != nil {
				close(entered)
				select {
				case <-release:
				case <-work.Done():
					return nil, work.Err()
				}
			}
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		initialOnce.Do(func() { close(initialRelease) })
		replacementOnce.Do(func() { close(replacementRelease) })
		require.NoError(t, d.stop(ctx))
	})
	require.NoError(t, d.publish(&blocked))
	require.NoError(t, d.destinationUp(peer, 2219))
	selectedDispatchAwait(t, ctx, initialEntered)
	require.NoError(t, d.publish(&previous))
	r := &RIBManager{selectedDispatch: d}
	r.sourceLeases = newRIBSourceLeases(r)
	ref := storage.NewSourceReference(1218, 2218, nil)
	t.Cleanup(ref.Release)
	r.sourceLeases.leases[8218] = &ribSourceLease{peer: source, peerOwner: 1218, connectionID: 2218, ref: ref}
	phaseContext, cancelPhase := context.WithCancel(ctx)
	t.Cleanup(cancelPhase)
	applied := make(chan struct{})
	result := make(chan error, 1)
	phase := r.settleSourcePolicyPhase(phaseContext, func(_ netip.Addr, _ rpc.SourcePolicyInput) error {
		if err := d.remove(previous.key); err != nil {
			return err
		}
		if err := d.publish(&next); err != nil {
			return err
		}
		close(applied)
		return nil
	})
	// The helper MUST join on every assertion path, including the marked cut.
	joined := false
	t.Cleanup(func() {
		cancelPhase()
		initialOnce.Do(func() { close(initialRelease) })
		replacementOnce.Do(func() { close(replacementRelease) })
		if !joined {
			selectedDispatchAwait(t, ctx, result)
		}
	})
	input := rpc.SourcePolicyInput{}
	input.Source.PeerOwner = 1218
	go func() { result <- phase(source, input) }()
	selectedDispatchAwait(t, ctx, applied)
	initialOnce.Do(func() { close(initialRelease) })
	selectedDispatchAwait(t, ctx, replacementEntered)
	cancelPhase()
	phaseErr := selectedDispatchAwait(t, ctx, result)
	joined = true
	require.ErrorIs(t, phaseErr, context.Canceled, "source completion must join the captured replacement destination")
	require.NoError(t, ctx.Err(), "only the transaction caller was canceled")
	replacementOnce.Do(func() { close(replacementRelease) })
	require.NoError(t, d.barrier(ctx, peer))
	d.mu.Lock()
	acknowledged := d.destinations[selectedDestinationID{peer: peer, session: 2219}].acknowledged[next.key]
	d.mu.Unlock()
	require.Equal(t, uint64(8219), acknowledged.source, "the genuine replacement remains useful")
}

// MUTATION: release source leases without joining selected output. The real
// source lease must remain unreleased when purge's final absence cannot settle.
func TestStalePurgeRefusesBeforeSelectedAbsenceSettles(t *testing.T) {
	staleSourceSettlement(t, false)
}

// Safety-net expiry has no command receipt, but MUST retain the same source
// rights while final selected output remains unsettled. It still purges input.
func TestStaleExpiryKeepsRightsUntilSelectedAbsenceSettles(t *testing.T) {
	staleSourceSettlement(t, true)
}

func staleSourceSettlement(t *testing.T, expiry bool) {
	t.Helper()
	ctx := selectedDispatchContext(t)
	r, calls := sourceLeaseTestRIB(t, true)
	source := netip.MustParseAddr("192.0.2.220")
	sourceLeaseReceive(t, r, source, 1220, 8220, false, false)
	r.peerUp[1220] = true
	_, _, err := r.markStaleCommand([]string{source.String(), "0"})
	require.NoError(t, err)
	route, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, route, 1)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		for _, item := range input.Routes {
			if item.Kind == rpc.SelectedRouteAbsent {
				close(entered)
				<-release
			}
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	r.selectedDispatch = d
	t.Cleanup(func() { once.Do(func() { close(release) }); require.NoError(t, d.stop(ctx)) })
	require.NoError(t, d.publish(&route[0]))
	recipient := netip.MustParseAddr("192.0.2.221")
	require.NoError(t, d.destinationUp(recipient, 2221))
	require.NoError(t, d.barrier(ctx, recipient))
	result := make(chan error, 1)
	joined := false
	t.Cleanup(func() {
		d.cancel()
		once.Do(func() { close(release) })
		if !joined {
			selectedDispatchAwait(t, ctx, result)
		}
	})
	grState := r.grState[1220]
	go func() {
		if expiry {
			r.autoExpireStale(1220, grState)
			result <- nil
			return
		}
		_, _, err := r.purgeStaleCommand([]string{source.String()})
		result <- err
	}()
	selectedDispatchAwait(t, ctx, entered)
	// Ending only the selecting lifetime makes settlement refuse immediately;
	// the held writer still has not completed, even though input is now absent.
	d.cancel()
	purgeErr := selectedDispatchAwait(t, ctx, result)
	joined = true
	if !expiry {
		require.ErrorIs(t, purgeErr, context.Canceled, "purge must not acknowledge a still-held final absence")
	}
	require.Zero(t, calls.count(rpc.SourceLeaseRelease, 8220), "source rights remain until final selected output joins")
	require.Zero(t, r.bgpPeers[1220].Len(), "refused settlement does not restore expired input")
	once.Do(func() { close(release) })
	require.NoError(t, d.stop(ctx))
	require.NoError(t, r.sourceLeases.settle(ctx))
	require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, 8220))
}

// MUTATION: bypass settleReceipts' selected-output guard. The real background
// worker then releases the purged source while its actual absence is held.
// Independent releases prove that worker ran and remains useful while held.
func TestLiveLeaseWorkerKeepsPurgedSourceUntilWriterJoins(t *testing.T) {
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
	source := netip.MustParseAddr("192.0.2.222")
	const receipt uint64 = 8222
	sourceLeaseReceive(t, r, source, 1222, receipt, false, false)
	r.peerUp[1222] = true
	_, _, err := r.markStaleCommand([]string{source.String(), "0"})
	require.NoError(t, err)
	routes, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, routes, 1)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		for _, route := range input.Routes {
			if route.Kind == rpc.SelectedRouteAbsent {
				close(entered)
				<-release
			}
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	r.selectedDispatch = d
	t.Cleanup(func() {
		once.Do(func() { close(release) })
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		require.NoError(t, d.stop(cleanup))
		r.sourceLeases.stop()
	})
	require.NoError(t, d.publish(&routes[0]))
	destination := netip.MustParseAddr("192.0.2.223")
	require.NoError(t, d.destinationUp(destination, 2223))
	require.NoError(t, d.barrier(ctx, destination))
	r.sourceLeases.start(ctx)
	result := make(chan error, 1)
	joined := false
	t.Cleanup(func() {
		d.cancel()
		once.Do(func() { close(release) })
		if !joined {
			cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
			defer stop()
			selectedDispatchAwait(t, cleanup, result)
		}
	})
	go func() { _, _, err := r.purgeStaleCommand([]string{source.String()}); result <- err }()
	selectedDispatchAwait(t, ctx, entered)
	d.cancel()
	require.ErrorIs(t, selectedDispatchAwait(t, ctx, result), context.Canceled)
	joined = true
	require.Zero(t, r.bgpPeers[1222].Len())
	releaseIndependent := func(id, connectionID uint64) {
		ref, err := r.sourceLeases.acquire(ctx, netip.MustParseAddr("192.0.2.224"), 1224, id, connectionID)
		require.NoError(t, err)
		ref.Release()
		for {
			call := selectedDispatchAwait(t, ctx, calls)
			if call.Action != rpc.SourceLeaseRelease {
				continue
			}
			require.NotEqual(t, receipt, call.SourceReceipt, "background release overtook the held absence")
			if call.SourceReceipt == id {
				break
			}
		}
	}
	releaseIndependent(8224, 2224)
	// 8225 was absent from the pass that released 8224, so its receipt proves
	// that the running worker also completed its scan of the held source.
	releaseIndependent(8225, 2225)
	once.Do(func() { close(release) })
	require.NoError(t, d.stop(ctx))
	r.sourceLeases.notify()
	for {
		call := selectedDispatchAwait(t, ctx, calls)
		if call.Action == rpc.SourceLeaseRelease && call.SourceReceipt == receipt {
			break
		}
	}
}
