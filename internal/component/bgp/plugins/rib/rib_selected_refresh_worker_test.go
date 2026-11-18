// Design: docs/architecture/plugin/rib-storage-design.md -- joined selected replay.
package rib

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func selectedRefreshReceipt(input rpc.SelectRefreshInput) *rpc.SelectRefreshOutput {
	return &rpc.SelectRefreshOutput{Destination: input.Destination, PeerOwner: input.PeerOwner,
		DestinationSession: input.DestinationSession, Family: input.Family, Action: input.Action,
		Outcome: rpc.SelectedRefreshWritten}
}

func selectedRefreshState(t *testing.T, r *RIBManager, native bool, peer netip.Addr, owner, session uint64) {
	t.Helper()
	if native {
		r.dispatchStructured(&rpc.StructuredEvent{EventType: rpc.EventKindState, PeerAddress: peer.String(),
			PeerOwner: owner, State: rpc.SessionStateUp, SelectedDestination: true,
			DestinationSession: session, InitialReplay: session, Reason: rpc.ReasonAttached})
		return
	}
	event := Event{Type: "state", PeerOwner: owner, State: "up", SelectedDestination: true,
		DestinationSession: session, InitialReplay: session, Reason: rpc.ReasonAttached}
	event.Peer = []byte(fmt.Sprintf(`{"remote":{"address":%q}}`, peer.String()))
	payload, err := json.Marshal(event)
	require.NoError(t, err)
	r.dispatchText(string(payload))
}

// Both SDK transports hold each of Begin, the route receipt and End in turn.
// The actual RIB barrier must remain refused, while B still receives useful
// output. Releasing the final receipt permits the exact original Session/cut.
func TestOrdinaryRefreshSDKWaitsForEveryReceipt(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, held := range []string{"begin", "routes", "end"} {
			t.Run(fmt.Sprintf("native=%t/%s", native, held), func(t *testing.T) {
				ctx := selectedDispatchContext(t)
				peers := [2]netip.Addr{netip.MustParseAddr("192.0.2.201"), netip.MustParseAddr("192.0.2.202")}
				entered, release := make(chan struct{}), make(chan struct{})
				var once, holdOnce sync.Once
				t.Cleanup(func() { once.Do(func() { close(release) }) })
				var refreshing atomic.Bool
				order := make(chan string, 12)
				attempts := make(chan rpc.SelectRoutesInput, 12)
				gate := func(work context.Context, phase string) error {
					if phase != held {
						return nil
					}
					hold := false
					holdOnce.Do(func() { close(entered); hold = true })
					if !hold {
						return nil
					}
					select {
					case <-release:
						return nil
					case <-work.Done():
						return work.Err()
					}
				}
				send := func(work context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
					if refreshing.Load() && input.Destination == peers[0].String() {
						if err := gate(work, "routes"); err != nil {
							return nil, err
						}
						order <- "routes"
					}
					attempts <- input
					return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
				}
				marker := func(work context.Context, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
					if input.PeerOwner != 201 || input.DestinationSession != 201 ||
						input.Destination != peers[0].String() || input.Family != "ipv4/unicast" {
						return nil, errors.New("refresh marker lost the original owner, Session or family")
					}
					phase := "begin"
					if input.Action == rpc.SelectedRefreshEnd {
						phase = "end"
					}
					if err := gate(work, phase); err != nil {
						return nil, err
					}
					order <- phase
					return selectedRefreshReceipt(input), nil
				}
				manager, r := startQuiesceRIB(t, ctx, native, peers, send, marker)
				route := selectedDispatchRoute(101, 0x1010)
				require.NoError(t, r.selectedDispatch.publish(&route))
				selectedRefreshState(t, r, native, peers[0], 201, 201)
				selectedRefreshState(t, r, native, peers[1], 202, 202)
				require.NoError(t, manager.DrainEventDelivery(ctx))
				first := selectedDispatchAwait(t, ctx, attempts)
				second := selectedDispatchAwait(t, ctx, attempts)
				require.NotEqual(t, first.Destination, second.Destination)
				refreshing.Store(true)
				ordinaryRefresh(t, r, native, peers[0], 201, 201, true, route.key.family)
				selectedDispatchAwait(t, ctx, entered)
				waitSelectedRefreshBarrier(t, ctx, r.selectedDispatch, peers[0])

				next := selectedDispatchRoute(102, 0x1020)
				require.NoError(t, r.selectedDispatch.publish(&next))
				require.NoError(t, r.selectedDispatch.barrier(ctx, peers[1]))
				var replay []rpc.SelectRoutesInput
				for {
					written := selectedDispatchAwait(t, ctx, attempts)
					if written.Destination == peers[1].String() {
						require.Equal(t, next.keyNLRI, written.Routes[0].KeyNLRI)
						break
					}
					replay = append(replay, written)
				}
				once.Do(func() { close(release) })
				require.NoError(t, manager.DrainEventDelivery(ctx))
				require.Equal(t, "begin", selectedDispatchAwait(t, ctx, order))
				require.Equal(t, "routes", selectedDispatchAwait(t, ctx, order))
				require.Equal(t, "end", selectedDispatchAwait(t, ctx, order))
				if len(replay) == 0 {
					replay = append(replay, selectedDispatchAwait(t, ctx, attempts))
				}
				found := false
				for _, batch := range replay {
					require.Equal(t, uint64(201), batch.DestinationSession)
					for _, item := range batch.Routes {
						if bytes.Equal(item.KeyNLRI, route.keyNLRI) {
							require.Equal(t, route.route.SourceReceipt, item.SourceReceipt)
							require.Equal(t, uint64(1), item.Revision)
							found = true
						}
					}
				}
				require.True(t, found, "the unchanged current winner must be replayed")
			})
		}
	}
}

// Repeated requests coalesce per family, not into an unbounded event queue.
// The active cut remains family-isolated and a replacement Session cannot use it.
func TestSelectedRefreshWorkerCoalescesFamilyRequests(t *testing.T) {
	family.RegisterTestFamilies()
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.203")
	entered, release := make(chan struct{}), make(chan struct{})
	var first atomic.Bool
	first.Store(true)
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	markers := make(chan rpc.SelectRefreshInput, 8)
	routes := make(chan rpc.SelectRoutesInput, 8)
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		routes <- input
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	require.NoError(t, d.setRefreshSender(func(work context.Context, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
		if first.Swap(false) {
			close(entered)
			select {
			case <-release:
			case <-work.Done():
				return nil, work.Err()
			}
		}
		markers <- input
		return selectedRefreshReceipt(input), nil
	}))
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	four := selectedDispatchRoute(103, 0x1030)
	six := selectedDispatchRoute(104, 0x1040)
	six.key.family = family.IPv6Unicast
	six.key.prefix = netip.MustParsePrefix("2001:db8::/32")
	six.keyNLRI, six.route.NLRI = []byte{32, 0x20, 1, 0x0d, 0xb8}, []byte{32, 0x20, 1, 0x0d, 0xb8}
	require.NoError(t, d.publish(&four))
	require.NoError(t, d.publish(&six))
	require.NoError(t, d.destinationUp(peer, 203))
	require.NoError(t, d.barrier(ctx, peer))
	selectedDispatchAwait(t, ctx, routes)
	unknowns := 0
	for safi := 1; safi < 200; safi++ {
		unknown := family.Family{AFI: family.AFI(65535), SAFI: family.SAFI(safi)}
		if _, registered := family.LookupFamily(unknown.String()); registered {
			continue
		}
		unknowns++
		require.Error(t, d.requestRefresh(peer, 203, 203, unknown))
	}
	require.Positive(t, unknowns)
	d.mu.Lock()
	require.Nil(t, d.destinations[selectedDestinationID{peer: peer, session: 203}].refreshPending,
		"unknown families must be refused before a queue map is allocated")
	d.mu.Unlock()
	require.Error(t, d.requestRefresh(peer, 203, 204, family.IPv4Unicast))
	require.Error(t, d.requestRefresh(peer, 0, 203, family.IPv4Unicast))
	require.NoError(t, d.requestRefresh(peer, 203, 203, family.IPv4Unicast))
	selectedDispatchAwait(t, ctx, entered)
	for range 1000 {
		require.NoError(t, d.requestRefresh(peer, 203, 203, family.IPv4Unicast))
	}
	require.NoError(t, d.requestRefresh(peer, 203, 203, family.IPv6Unicast))
	d.mu.Lock()
	destination := d.destinations[selectedDestinationID{peer: peer, session: 203}]
	require.Len(t, destination.refreshPending, 2)
	require.Len(t, destination.refreshOrder, 2)
	require.NotNil(t, destination.refreshActive)
	d.mu.Unlock()
	once.Do(func() { close(release) })
	require.NoError(t, d.barrier(ctx, peer))
	for _, name := range []string{"ipv4/unicast", "ipv4/unicast", "ipv6/unicast"} {
		begin := selectedDispatchAwait(t, ctx, markers)
		output := selectedDispatchAwait(t, ctx, routes)
		end := selectedDispatchAwait(t, ctx, markers)
		require.Equal(t, name, begin.Family)
		require.Equal(t, rpc.SelectedRefreshBegin, begin.Action)
		require.Equal(t, rpc.SelectedRefreshEnd, end.Action)
		require.Equal(t, begin.PeerOwner, end.PeerOwner)
		require.Equal(t, begin.DestinationSession, end.DestinationSession)
		for _, item := range output.Routes {
			require.Equal(t, name, item.Family)
		}
	}
	require.Empty(t, markers)
	require.Empty(t, routes)
}

// A lost End receipt is still outstanding after every route was acknowledged.
// Restoring that same final writer settles without another Begin or route.
func TestSelectedRefreshWorkerFailedEndKeepsSourceBarrier(t *testing.T) {
	family.RegisterTestFamilies()
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.204")
	var refuse atomic.Bool
	refuse.Store(true)
	failed := make(chan struct{}, 1)
	var begins, writes atomic.Int32
	replayEntered, replayRelease := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	d, err := newSelectedDispatch(ctx, func(work context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		if writes.Add(1) == 2 {
			close(replayEntered)
			select {
			case <-replayRelease:
			case <-work.Done():
				return nil, work.Err()
			}
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	require.NoError(t, d.setRefreshSender(func(_ context.Context, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
		if input.Action == rpc.SelectedRefreshBegin {
			begins.Add(1)
		}
		if input.Action == rpc.SelectedRefreshEnd && refuse.Load() {
			select {
			case failed <- struct{}{}:
			default:
			}
			return nil, errors.New("held final End receipt")
		}
		return selectedRefreshReceipt(input), nil
	}))
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(replayRelease) })
		require.NoError(t, d.stop(ctx))
	})
	route := selectedDispatchRoute(105, 0x1050)
	require.NoError(t, d.publish(&route))
	require.NoError(t, d.destinationUp(peer, 204))
	require.NoError(t, d.barrier(ctx, peer))
	require.NoError(t, d.requestRefresh(peer, 204, 204, route.key.family))
	selectedDispatchAwait(t, ctx, replayEntered)
	require.NoError(t, d.remove(route.key))
	releaseOnce.Do(func() { close(replayRelease) })
	selectedDispatchAwait(t, ctx, failed)
	d.mu.Lock()
	destination := d.destinations[selectedDestinationID{peer: peer, session: 204}]
	remainingCurrent := len(d.current)
	remainingAcknowledged := len(destination.acknowledged)
	remainingPending := len(destination.pending)
	remainingInFlight := len(destination.inFlight)
	d.mu.Unlock()
	require.Zero(t, remainingCurrent)
	require.Zero(t, remainingAcknowledged)
	require.Zero(t, remainingPending)
	require.Zero(t, remainingInFlight)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, d.barrierSource(canceled, []uint64{route.route.SourceReceipt}), context.Canceled)
	require.ErrorIs(t, d.barrier(canceled, peer), context.Canceled)
	require.Equal(t, int32(1), begins.Load())
	require.Equal(t, int32(3), writes.Load(), "replay and its final absence must both settle before End")
	refuse.Store(false)
	d.mu.Lock()
	d.signalLocked()
	d.mu.Unlock()
	require.NoError(t, d.barrier(ctx, peer))
	require.Equal(t, int32(1), begins.Load())
	require.Equal(t, int32(3), writes.Load())
}

func TestSelectedRefreshWorkerWithoutEnhancedStillReplays(t *testing.T) {
	family.RegisterTestFamilies()
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.205")
	var writes, markers atomic.Int32
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		writes.Add(1)
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	require.NoError(t, d.setRefreshSender(func(_ context.Context, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
		markers.Add(1)
		output := selectedRefreshReceipt(input)
		output.Outcome = rpc.SelectedRefreshNotNegotiated
		return output, nil
	}))
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(106, 0x1060)
	require.NoError(t, d.publish(&route))
	require.NoError(t, d.destinationUp(peer, 205))
	require.NoError(t, d.barrier(ctx, peer))
	require.NoError(t, d.requestRefresh(peer, 205, 205, route.key.family))
	require.NoError(t, d.barrier(ctx, peer))
	require.Equal(t, int32(2), writes.Load())
	require.Equal(t, int32(2), markers.Load())
}

// Refusing a passive or replaced event must leave the useful current
// destination intact. Its later exact receiving Session still refreshes.
func TestOrdinaryRefreshIgnoresPassiveAndReplacedEvents(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("native=%t", native), func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			peers := [2]netip.Addr{netip.MustParseAddr("192.0.2.206"), netip.MustParseAddr("192.0.2.207")}
			var writes, markers atomic.Int32
			send := func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				writes.Add(1)
				return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
			}
			marker := func(_ context.Context, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
				if input.PeerOwner != 206 || input.DestinationSession != 206 {
					return nil, errors.New("marker borrowed another receiving Session")
				}
				markers.Add(1)
				return selectedRefreshReceipt(input), nil
			}
			manager, r := startQuiesceRIB(t, ctx, native, peers, send, marker)
			route := selectedDispatchRoute(107, 0x1070)
			require.NoError(t, r.selectedDispatch.publish(&route))
			selectedRefreshState(t, r, native, peers[0], 206, 206)
			require.NoError(t, manager.DrainEventDelivery(ctx))
			ordinaryRefresh(t, r, native, peers[0], 206, 206, false, route.key.family)
			ordinaryRefresh(t, r, native, peers[0], 206, 999, true, route.key.family)
			ordinaryRefresh(t, r, native, peers[0], 0, 206, true, route.key.family)
			require.NoError(t, manager.DrainEventDelivery(ctx))
			require.Equal(t, int32(1), writes.Load())
			require.Zero(t, markers.Load())
			ordinaryRefresh(t, r, native, peers[0], 206, 206, true, route.key.family)
			require.NoError(t, manager.DrainEventDelivery(ctx))
			require.Equal(t, int32(2), writes.Load())
			require.Equal(t, int32(2), markers.Load())
		})
	}
}

// This checks the dispatcher's join of a canceled in-flight final marker.
// Completed-Begin owner death between RPCs needs the engine retirement control;
// a canceled fake callback here is not proof of that separate socket obligation.
func TestSelectedRefreshWorkerStopJoinsCanceledMarker(t *testing.T) {
	family.RegisterTestFamilies()
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.208")
	entered, interrupted, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		once.Do(func() { close(release) })
		require.NoError(t, d.stop(ctx))
	})
	require.NoError(t, d.setRefreshSender(func(work context.Context, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
		if input.Action == rpc.SelectedRefreshEnd {
			close(entered)
			<-work.Done()
			close(interrupted)
			<-release
			return nil, work.Err()
		}
		return selectedRefreshReceipt(input), nil
	}))
	route := selectedDispatchRoute(108, 0x1080)
	require.NoError(t, d.publish(&route))
	require.NoError(t, d.destinationUp(peer, 208))
	require.NoError(t, d.barrier(ctx, peer))
	require.NoError(t, d.requestRefresh(peer, 208, 208, route.key.family))
	selectedDispatchAwait(t, ctx, entered)
	d.mu.Lock()
	done := d.destinations[selectedDestinationID{peer: peer, session: 208}].done
	d.mu.Unlock()
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, d.stop(canceled), context.Canceled)
	selectedDispatchAwait(t, ctx, interrupted)
	select {
	case <-done:
		t.Fatal("stop claimed settlement before the final marker callback joined")
	default:
	}
	once.Do(func() { close(release) })
	require.NoError(t, d.stop(ctx))
}

// Refresh admitted while initial output is held remains part of readiness:
// the Session-ready callback follows both its replay and final End receipt.
func TestSelectedRefreshQueuedDuringInitialOutputDefersReady(t *testing.T) {
	family.RegisterTestFamilies()
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.209")
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var writes atomic.Int32
	order := make(chan string, 5)
	d, err := newSelectedDispatch(ctx, func(work context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		if writes.Add(1) == 1 {
			close(entered)
			select {
			case <-release:
			case <-work.Done():
				return nil, work.Err()
			}
			order <- "initial"
		} else {
			order <- "replay"
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, func(context.Context, string, uint64) error {
		order <- "ready"
		return nil
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		once.Do(func() { close(release) })
		require.NoError(t, d.stop(ctx))
	})
	require.NoError(t, d.setRefreshSender(func(_ context.Context, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
		if input.Action == rpc.SelectedRefreshBegin {
			order <- "begin"
		} else {
			order <- "end"
		}
		return selectedRefreshReceipt(input), nil
	}))
	route := selectedDispatchRoute(109, 0x1090)
	require.NoError(t, d.publish(&route))
	require.NoError(t, d.destinationUp(peer, 209))
	selectedDispatchAwait(t, ctx, entered)
	require.NoError(t, d.requestRefresh(peer, 209, 209, route.key.family))
	once.Do(func() { close(release) })
	require.NoError(t, d.barrier(ctx, peer))
	for _, phase := range []string{"initial", "begin", "replay", "end", "ready"} {
		require.Equal(t, phase, selectedDispatchAwait(t, ctx, order))
	}
}

type refreshBarrierContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (ctx *refreshBarrierContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.entered) })
	return ctx.Context.Done()
}

// The custom context observes only the actual RIB barrier's unresolved select.
// A broken settled predicate returns before Done is read and fails immediately.
// The helper MUST cancel and join its one test goroutine on every exit path.
func waitSelectedRefreshBarrier(t *testing.T, ctx context.Context, d *selectedDispatch, peer netip.Addr) {
	t.Helper()
	work, cancel := context.WithCancel(ctx)
	observed := &refreshBarrierContext{Context: work, entered: make(chan struct{})}
	result := make(chan error, 1)
	joined := false
	t.Cleanup(func() {
		cancel()
		if !joined {
			cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
			defer stop()
			selectedDispatchAwait(t, cleanup, result)
		}
	})
	go func() { result <- d.barrier(observed, peer) }()
	select {
	case <-observed.entered:
	case err := <-result:
		joined = true
		t.Fatalf("refresh barrier completed with a held final receipt: %v", err)
	case <-ctx.Done():
		t.Fatal("refresh barrier did not observe its pending obligation", ctx.Err())
	}
	cancel()
	require.ErrorIs(t, selectedDispatchAwait(t, ctx, result), context.Canceled)
	joined = true
}
