// Design: docs/architecture/plugin/rib-storage-design.md -- scoped origin admission.
package rib

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRIBOriginRetainedLoserBecomesUseful keeps an accepted distance-zero
// learned winner while the losing origin is admitted, then removes that winner.
func TestRIBOriginRetainedLoserBecomesUseful(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.213")
	writes := make(chan rpc.SelectRoutesInput, 8)
	d, err := newSelectedDispatch(ctx, func(_ context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		writes <- in
		return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	learned := selectedDispatchRoute(213, 813)
	require.NoError(t, d.publish(&learned))
	require.NoError(t, d.destinationOwnerUp(peer, 313, 413))
	require.NoError(t, d.barrier(ctx))
	selectedDispatchAwait(t, ctx, writes)
	table := originDefaultDistances()
	table["ibgp"] = 0
	require.NoError(t, d.installOriginPreferences(ctx, table))
	input := originIntakeFixture(peer, 313, 903, 213)
	output, err := d.originIntake(ctx, input)
	require.NoError(t, err)
	require.Zero(t, output.Announced)
	require.Equal(t, rpc.RouteAdmissionSuppressedSelection, output.Admissions[0].Disposition)
	require.Empty(t, writes, "retaining a loser cannot replace useful learned output")
	require.NoError(t, d.remove(learned.key))
	require.NoError(t, d.barrier(ctx))
	selected := selectedDispatchAwait(t, ctx, writes)
	require.Equal(t, rpc.SelectedRouteOrigin, selected.Routes[0].Kind)
	require.Equal(t, uint64(903), selected.Routes[0].OriginReceipt)
	input.Operation = rpc.OriginIntakeWithdraw
	_, err = d.originIntake(ctx, input)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedRouteAbsent, selectedDispatchAwait(t, ctx, writes).Routes[0].Kind)
}

// TestRIBOriginPreferenceRollbackKeepsExactSurvivor changes configured distance
// without any received UPDATE, rolls it back, then removes the losing origin.
func TestRIBOriginPreferenceRollbackKeepsExactSurvivor(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.214")
	writes := make(chan rpc.SelectRoutesInput, 12)
	d, err := newSelectedDispatch(ctx, func(_ context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		writes <- in
		return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	learned := selectedDispatchRoute(214, 814)
	require.NoError(t, d.publish(&learned))
	require.NoError(t, d.destinationOwnerUp(peer, 314, 414))
	require.NoError(t, d.barrier(ctx))
	selectedDispatchAwait(t, ctx, writes)
	input := originIntakeFixture(peer, 314, 904, 214)
	_, err = d.originIntake(ctx, input)
	require.NoError(t, err)
	selectedDispatchAwait(t, ctx, writes)
	table := originDefaultDistances()
	table["static"] = 250
	require.NoError(t, d.installOriginPreferences(ctx, table))
	learnedWrite := selectedDispatchAwait(t, ctx, writes)
	require.Equal(t, rpc.SelectedRouteReceived, learnedWrite.Routes[0].Kind)
	require.NoError(t, d.installOriginPreferences(ctx, originDefaultDistances()))
	localWrite := selectedDispatchAwait(t, ctx, writes)
	require.Equal(t, rpc.SelectedRouteOrigin, localWrite.Routes[0].Kind)
	require.Greater(t, localWrite.Routes[0].Revision, learnedWrite.Routes[0].Revision)
	require.NoError(t, d.installOriginPreferences(ctx, table))
	selectedDispatchAwait(t, ctx, writes)
	input.Operation = rpc.OriginIntakeWithdraw
	_, err = d.originIntake(ctx, input)
	require.NoError(t, err)
	require.Empty(t, writes, "losing withdrawal cannot erase or resend another winner")
	query := rpc.CurrentSelectionInput{Destination: peer.String(), DestinationSession: 414, Keys: []rpc.CurrentSelectionKey{{Family: "ipv4/unicast", KeyNLRI: input.Route.KeyNLRI, Revision: learnedWrite.Routes[0].Revision}}}
	answer, err := d.currentSelection(ctx, query)
	require.NoError(t, err)
	require.Equal(t, rpc.CurrentSelectionPresent, answer.Results[0].Presence)
}

// TestRIBOriginSameAddressScopesAndReconnect preserves an unrelated exact owner
// at the same address and replays only current admitted origins to its owner.
func TestRIBOriginSameAddressScopesAndReconnect(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.215")
	writes := make(chan rpc.SelectRoutesInput, 12)
	d, err := newSelectedDispatch(ctx, func(_ context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		writes <- in
		return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	learned := selectedDispatchRoute(215, 815)
	require.NoError(t, d.publish(&learned))
	require.NoError(t, d.destinationOwnerUp(peer, 315, 415))
	require.NoError(t, d.destinationOwnerUp(peer, 316, 416))
	require.NoError(t, d.barrier(ctx))
	require.ErrorContains(t, d.destinationOwnerUp(peer, 316, 415), "another original Peer owner")
	selectedDispatchAwait(t, ctx, writes)
	selectedDispatchAwait(t, ctx, writes)
	input := originIntakeFixture(peer, 315, 905, 215)
	_, err = d.originIntake(ctx, input)
	require.NoError(t, err)
	local := selectedDispatchAwait(t, ctx, writes)
	require.Equal(t, uint64(415), local.DestinationSession)
	require.NoError(t, d.destinationDown(ctx, peer, 415))
	require.NoError(t, d.destinationOwnerUp(peer, 315, 417))
	require.NoError(t, d.barrier(ctx))
	replay := selectedDispatchAwait(t, ctx, writes)
	require.Equal(t, uint64(417), replay.DestinationSession)
	require.Equal(t, uint64(905), replay.Routes[0].OriginReceipt)
	query := rpc.CurrentSelectionInput{Destination: peer.String(), DestinationSession: 416, Keys: []rpc.CurrentSelectionKey{{Family: family.IPv4Unicast.String(), KeyNLRI: input.Route.KeyNLRI, Revision: local.Routes[0].Revision}}}
	answer, err := d.currentSelection(ctx, query)
	require.NoError(t, err)
	require.NotEqual(t, local.Routes[0].Revision, answer.Results[0].CurrentRevision)
	require.NoError(t, d.destinationDown(ctx, peer, 415))
	require.Empty(t, writes)
}

// TestRIBOriginReplacementRejectsStaleRemoval retains the latest generation and
// legal path-zero presence while old removal and caller mutations cannot alter it.
func TestRIBOriginReplacementRejectsStaleRemoval(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.216")
	d, err := newSelectedDispatch(ctx, func(_ context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	first := originIntakeFixture(peer, 317, 906, 216)
	_, err = d.originIntake(ctx, first)
	require.NoError(t, err)
	next := rpc.CloneOriginIntake(first)
	next.OriginReceipt = 907
	_, err = d.originIntake(ctx, next)
	require.NoError(t, err)
	first.Operation = rpc.OriginIntakeWithdraw
	_, err = d.originIntake(ctx, first)
	require.NoError(t, err)
	next.Route.KeyNLRI[1] = 0
	next.Route.Attributes[0] = 0
	require.NoError(t, d.destinationOwnerUp(peer, 317, 418))
	require.NoError(t, d.barrier(ctx))
	d.mu.Lock()
	current := d.currentForLocked(d.destinations[selectedDestinationID{peer: peer, session: 418}], advertiseKey{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("198.18.216.0/24")})
	require.Equal(t, uint64(907), current.wire.OriginReceipt)
	require.Equal(t, []byte{24, 198, 18, 216}, current.wire.KeyNLRI)
	d.mu.Unlock()
}

// TestRIBOriginCancellationJoinsActualWrite holds a real destination-worker
// request after its cancellation is observed. The provider cannot return until
// it joins that request and restores the useful learned winner; B stays useful.
func TestRIBOriginCancellationJoinsActualWrite(t *testing.T) {
	ctx := selectedDispatchContext(t)
	a := netip.MustParseAddr("192.0.2.217")
	b := netip.MustParseAddr("192.0.2.218")
	entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	writes := make(chan rpc.SelectRoutesInput, 16)
	d, err := newSelectedDispatch(ctx, func(work context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		for _, route := range in.Routes {
			if route.OriginReceipt == 908 {
				close(entered)
				<-work.Done()
				close(canceled)
				<-release
				return nil, work.Err()
			}
		}
		writes <- in
		return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); require.NoError(t, d.stop(ctx)) })
	learned := selectedDispatchRoute(217, 817)
	require.NoError(t, d.publish(&learned))
	require.NoError(t, d.destinationOwnerUp(a, 318, 419))
	require.NoError(t, d.destinationOwnerUp(b, 319, 420))
	require.NoError(t, d.barrier(ctx))
	selectedDispatchAwait(t, ctx, writes)
	selectedDispatchAwait(t, ctx, writes)
	intake, stop := context.WithCancel(ctx)
	defer stop()
	result := make(chan error, 1)
	input := originIntakeFixture(a, 318, 908, 217)
	go func() { _, failure := d.originIntake(intake, input); result <- failure }()
	selectedDispatchAwait(t, ctx, entered)
	stop()
	selectedDispatchAwait(t, ctx, canceled)
	marker := selectedDispatchRoute(218, 818)
	require.NoError(t, d.publish(&marker))
	require.NoError(t, d.barrier(ctx, b))
	useful := selectedDispatchAwait(t, ctx, writes)
	require.Equal(t, b.String(), useful.Destination)
	select {
	case err := <-result:
		t.Fatalf("origin returned before its actual canceled writer joined: %v", err)
	default:
	}
	releaseOnce.Do(func() { close(release) })
	require.ErrorIs(t, selectedDispatchAwait(t, ctx, result), context.Canceled)
	require.NoError(t, d.barrier(ctx))
	d.mu.Lock()
	require.Empty(t, d.origins)
	current := d.currentForLocked(d.destinations[selectedDestinationID{peer: a, session: 419}], learned.key)
	require.Equal(t, rpc.SelectedRouteReceived, current.wire.Kind)
	d.mu.Unlock()
}

// TestRIBOriginNeverIssuedChurnHasNoHistory retains no scope/key tombstones
// for origins announced then withdrawn before any destination Session exists.
func TestRIBOriginNeverIssuedChurnHasNoHistory(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.219")
	d, err := newSelectedDispatch(ctx, func(_ context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	for n := range 128 {
		input := originIntakeFixture(peer, 320, uint64(1000+n), byte(n))
		output, err := d.originIntake(ctx, input)
		require.NoError(t, err)
		require.Zero(t, output.Announced)
		input.Operation = rpc.OriginIntakeWithdraw
		output, err = d.originIntake(ctx, input)
		require.NoError(t, err)
		require.Equal(t, uint32(1), output.Withdrawn)
	}
	d.mu.Lock()
	require.Empty(t, d.origins)
	require.Empty(t, d.scoped)
	require.Empty(t, d.current)
	require.Empty(t, d.destinations)
	d.mu.Unlock()
}

// TestRIBOriginConfigCallbacksReelectAndRollback drives the two-root config
// transaction over both transports. The AC17 tests cover partial root deliveries.
func TestRIBOriginConfigCallbacksReelectAndRollback(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("native=%t", native), func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			peers := [2]netip.Addr{netip.MustParseAddr("192.0.2.220"), netip.MustParseAddr("192.0.2.221")}
			writes := make(chan rpc.SelectRoutesInput, 12)
			manager, r := startQuiesceRIB(t, ctx, native, peers, func(_ context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				writes <- in
				return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
			})
			learned := selectedDispatchRoute(220, 820)
			require.NoError(t, r.selectedDispatch.publish(&learned))
			r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peers[0].String(), PeerOwner: 321, DestinationSession: 421, SelectedDestination: true, State: rpc.SessionStateUp, Reason: rpc.ReasonAttached})
			require.NoError(t, r.selectedDispatch.barrier(ctx))
			selectedDispatchAwait(t, ctx, writes)
			connection := manager.GetProcess("bgp-rib").Conn()
			input := originIntakeFixture(peers[0], 321, 909, 220)
			output, err := connection.SendOriginIntake(ctx, input)
			require.NoError(t, err)
			require.Equal(t, uint32(1), output.Announced)
			selectedDispatchAwait(t, ctx, writes)
			verified, err := connection.SendConfigVerify(ctx, []rpc.ConfigSection{{Root: "bgp", Data: `{"bgp":{}}`}, {Root: "rib", Data: `{"rib":{"distance":{"static":"250"}}}`}})
			require.NoError(t, err)
			require.Equal(t, rpc.StatusOK, verified.Status)
			applied, err := connection.SendConfigApply(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, rpc.StatusOK, applied.Status)
			changed := selectedDispatchAwait(t, ctx, writes)
			require.Equal(t, rpc.SelectedRouteReceived, changed.Routes[0].Kind)
			require.NoError(t, connection.SendConfigRollback(ctx, "origin-preference-rollback"))
			restored := selectedDispatchAwait(t, ctx, writes)
			require.Equal(t, rpc.SelectedRouteOrigin, restored.Routes[0].Kind)
			require.Equal(t, uint64(909), restored.Routes[0].OriginReceipt)
			require.Greater(t, restored.Routes[0].Revision, changed.Routes[0].Revision)
		})
	}
}

// TestRIBOriginPreferenceOverrideMetricAndStableAdmission uses the same actual
// preference comparison as Loc-RIB: explicit zero, then metric, then order.
func TestRIBOriginPreferenceOverrideMetricAndStableAdmission(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.222")
	writes := make(chan rpc.SelectRoutesInput, 12)
	d, err := newSelectedDispatch(ctx, func(_ context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		writes <- in
		return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	require.NoError(t, d.destinationOwnerUp(peer, 322, 422))
	require.NoError(t, d.barrier(ctx))
	first := originIntakeFixture(peer, 322, 910, 222)
	first.Source.HasDistance = true
	first.Source.Distance = 0
	first.Source.Metric = 30
	_, err = d.originIntake(ctx, first)
	require.NoError(t, err)
	selectedDispatchAwait(t, ctx, writes)
	second := rpc.CloneOriginIntake(first)
	second.Producer.Name = "second-producer"
	second.OriginReceipt = 911
	output, err := d.originIntake(ctx, second)
	require.NoError(t, err)
	require.Equal(t, rpc.RouteAdmissionSuppressedSelection, output.Admissions[0].Disposition)
	require.Empty(t, writes)
	second.OriginReceipt = 912
	second.Source.Metric = 10
	output, err = d.originIntake(ctx, second)
	require.NoError(t, err)
	require.Equal(t, uint32(1), output.Announced)
	require.Equal(t, uint64(912), selectedDispatchAwait(t, ctx, writes).Routes[0].OriginReceipt)
	first.Operation = rpc.OriginIntakeWithdraw
	_, err = d.originIntake(ctx, first)
	require.NoError(t, err)
	require.Empty(t, writes)
}

// TestRIBOriginPathZeroAndAbsentAreIndependent removes only the exact source
// path-presence identity while a separately admitted absent-ID winner survives.
func TestRIBOriginPathZeroAndAbsentAreIndependent(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.223")
	writes := make(chan rpc.SelectRoutesInput, 8)
	d, err := newSelectedDispatch(ctx, func(_ context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		writes <- in
		return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	require.NoError(t, d.destinationOwnerUp(peer, 323, 423))
	require.NoError(t, d.barrier(ctx))
	zero := originIntakeFixture(peer, 323, 913, 223)
	zero.Source.Metric = 20
	_, err = d.originIntake(ctx, zero)
	require.NoError(t, err)
	selectedDispatchAwait(t, ctx, writes)
	absent := rpc.CloneOriginIntake(zero)
	absent.OriginReceipt = 914
	absent.Route.SourceAddPath = false
	absent.Source.Metric = 10
	_, err = d.originIntake(ctx, absent)
	require.NoError(t, err)
	selectedDispatchAwait(t, ctx, writes)
	zero.Operation = rpc.OriginIntakeWithdraw
	_, err = d.originIntake(ctx, zero)
	require.NoError(t, err)
	require.Empty(t, writes)
	absent.Operation = rpc.OriginIntakeWithdraw
	_, err = d.originIntake(ctx, absent)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedRouteAbsent, selectedDispatchAwait(t, ctx, writes).Routes[0].Kind)
}

// TestRIBOriginNativeBuildFactsStayOpaque retains a non-prefix FlowSpec identity
// and a deliberately process-local engine context without consulting the fork's
// context registry. Caller mutations cannot replace retained original facts.
func TestRIBOriginNativeBuildFactsStayOpaque(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.224")
	input := originIntakeFixture(peer, 324, 915, 224)
	input.Route.Family = flowspecFamily.String()
	input.Route.KeyNLRI = flowspecNLRI(16, 10, 20)
	input.Route.SourceContext = 65535
	input.Route.ASPathKnown = true
	input.Route.ASPathASN4 = false
	original := rpc.CloneOriginIntake(input)
	value, key, _, err := newOriginContribution(input)
	require.NoError(t, err)
	require.False(t, key.prefix.IsValid())
	require.NotEmpty(t, key.native)
	input.Route.KeyNLRI[0] = 0
	input.Route.Attributes[0] = 0
	require.Equal(t, original.Route.KeyNLRI, value.input.Route.KeyNLRI)
	require.Equal(t, original.Route.Attributes, value.input.Route.Attributes)
	require.Equal(t, uint16(65535), value.input.Route.SourceContext)
	require.True(t, value.input.Route.ASPathKnown)
	require.False(t, value.input.Route.ASPathASN4)
	require.Equal(t, original.Route.KeyNLRI, value.wire.wire.KeyNLRI)
}

// TestRIBOriginRedistributedInstanceRemovalKeepsAdmittedSurvivor admits exact
// OSPF instances with the existing class and keeps the second after removing
// only the first. A registered but unadmitted system contribution never enters.
func TestRIBOriginRedistributedInstanceRemovalKeepsAdmittedSurvivor(t *testing.T) {
	restore := redistevents.PreserveForTest()
	t.Cleanup(restore)
	redistevents.RegisterProtocol("ospf")
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.225")
	writes := make(chan rpc.SelectRoutesInput, 12)
	d, err := newSelectedDispatch(ctx, func(_ context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		writes <- in
		return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	learned := selectedDispatchRoute(225, 825)
	require.NoError(t, d.publish(&learned))
	require.NoError(t, d.destinationOwnerUp(peer, 325, 425))
	require.NoError(t, d.barrier(ctx))
	selectedDispatchAwait(t, ctx, writes)
	first := originIntakeFixture(peer, 325, 916, 225)
	first.Source = rpc.OriginSource{Kind: rpc.OriginSourceRedistributed, Protocol: "ospf", Instance: 11, Metric: 10}
	_, err = d.originIntake(ctx, first)
	require.NoError(t, err)
	selectedDispatchAwait(t, ctx, writes)
	second := rpc.CloneOriginIntake(first)
	second.OriginReceipt = 917
	second.Source.Instance = 12
	second.Source.Metric = 20
	output, err := d.originIntake(ctx, second)
	require.NoError(t, err)
	require.Zero(t, output.Announced)
	require.Empty(t, writes)
	first.Operation = rpc.OriginIntakeWithdraw
	_, err = d.originIntake(ctx, first)
	require.NoError(t, err)
	require.Equal(t, uint64(917), selectedDispatchAwait(t, ctx, writes).Routes[0].OriginReceipt)
	d.mu.Lock()
	require.Len(t, d.origins[learned.key], 1)
	d.mu.Unlock()
}

// TestRIBOriginAdmissionFailurePreservesCause refuses an unknown original
// protocol before mutation while a useful admitted local contribution survives.
func TestRIBOriginAdmissionFailurePreservesCause(t *testing.T) {
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.226")
	d, err := newSelectedDispatch(ctx, func(_ context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	good := originIntakeFixture(peer, 326, 918, 226)
	output, err := d.originIntake(ctx, good)
	require.NoError(t, err)
	require.Len(t, output.Admissions, 1)
	bad := rpc.CloneOriginIntake(good)
	bad.OriginReceipt = 919
	bad.Source = rpc.OriginSource{Kind: rpc.OriginSourceRedistributed, Protocol: "missing-origin-source", Instance: 99}
	output, err = d.originIntake(ctx, bad)
	require.Nil(t, output)
	require.ErrorContains(t, err, "missing-origin-source")
	key, err := selectedDispatchKey(good.Route.Family, good.Route.KeyNLRI)
	require.NoError(t, err)
	d.mu.Lock()
	require.Len(t, d.origins[key], 1)
	require.Equal(t, uint64(918), d.currentScopeLocked(originScope{peer: peer, owner: 326}, key).wire.OriginReceipt)
	d.mu.Unlock()
}

// TestRIBOriginPermanentDownDropsOnlyExactOwner uses the actual structured state
// handler and real startup callback on both transports. Permanent removal drops
// A's membership, while same-address B survives transient DOWN and reconnect.
func TestRIBOriginPermanentDownDropsOnlyExactOwner(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("native=%t", native), func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			peer := netip.MustParseAddr("192.0.2.227")
			peers := [2]netip.Addr{peer, netip.MustParseAddr("192.0.2.228")}
			writes := make(chan rpc.SelectRoutesInput, 12)
			manager, r := startQuiesceRIB(t, ctx, native, peers, func(_ context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				writes <- in
				return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
			})
			for _, pair := range [][2]uint64{{331, 431}, {332, 432}} {
				r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), PeerOwner: pair[0], DestinationSession: pair[1], SelectedDestination: true, State: rpc.SessionStateUp, Reason: rpc.ReasonAttached})
			}
			require.NoError(t, r.selectedDispatch.barrier(ctx))
			connection := manager.GetProcess("bgp-rib").Conn()
			a := originIntakeFixture(peer, 331, 920, 227)
			_, err := connection.SendOriginIntake(ctx, a)
			require.NoError(t, err)
			selectedDispatchAwait(t, ctx, writes)
			b := originIntakeFixture(peer, 332, 921, 227)
			_, err = connection.SendOriginIntake(ctx, b)
			require.NoError(t, err)
			selectedDispatchAwait(t, ctx, writes)
			r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), PeerOwner: 331, DestinationSession: 431, SelectedDestination: true, State: rpc.SessionStateDown, Reason: rpc.ReasonPeerRemoved})
			_, key, id, err := newOriginContribution(b)
			require.NoError(t, err)
			d := r.selectedDispatch
			d.mu.Lock()
			require.Nil(t, d.originLocked(key, id, originScope{peer: peer, owner: 331}))
			require.Equal(t, uint64(921), d.originLocked(key, id, originScope{peer: peer, owner: 332}).input.OriginReceipt)
			d.mu.Unlock()
			require.Empty(t, writes, "removing A cannot withdraw or resend B")
			r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), PeerOwner: 332, DestinationSession: 432, SelectedDestination: true, State: rpc.SessionStateDown, Reason: rpc.ReasonTCPFailure})
			d.mu.Lock()
			require.NotNil(t, d.originLocked(key, id, originScope{peer: peer, owner: 332}))
			d.mu.Unlock()
			r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), PeerOwner: 332, DestinationSession: 433, SelectedDestination: true, State: rpc.SessionStateUp, Reason: rpc.ReasonAttached})
			require.NoError(t, d.barrier(ctx))
			replay := selectedDispatchAwait(t, ctx, writes)
			require.Equal(t, uint64(433), replay.DestinationSession)
			require.Equal(t, uint64(921), replay.Routes[0].OriginReceipt)
		})
	}
}
