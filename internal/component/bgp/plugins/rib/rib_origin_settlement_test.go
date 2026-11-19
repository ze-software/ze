// Design: docs/architecture/plugin/rib-storage-design.md -- bounded origin settlement.
// Related: pkg/plugin/sdk/sdk_origin_intake.go -- real SDK mutation/stop join.
package rib

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type originSettlementAnswer struct {
	input  rpc.OriginIntakeInput
	output *rpc.UpdateRouteOutput
	err    error
}

// originSettlementFixture drives the real SDK startup, callback worker,
// deadline/cancel transport and Run defers. Its selected writer is controlled.
// sdkExited closes BEFORE selected stop is permitted, reproducing the actual
// RIB teardown order without allowing selected stop to rescue a wedged SDK Run.
// Its cleanup releases all held writes and joins both component lifetimes.
type originSettlementFixture struct {
	d             *selectedDispatch
	conn          *ipc.PluginConn
	callbacks     chan originSettlementAnswer
	sdkExited     chan struct{}
	sdkResult     chan error
	stopped       chan struct{}
	transportStop func()
	selectedStop  func()
}

func startOriginSettlementFixture(t *testing.T, ctx context.Context, native bool, send rpc.SelectRoutesHandler) *originSettlementFixture {
	t.Helper()
	pluginEnd, engineEnd := net.Pipe()
	conn := ipc.NewMuxPluginConn(rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)))
	pluginConn := net.Conn(pluginEnd)
	var bridge *rpc.DirectBridge
	if native {
		bridge = rpc.NewDirectBridge()
		pluginConn = rpc.NewBridgedConn(pluginEnd, bridge)
	}
	p := sdk.NewWithConn("origin-settlement-fixture", pluginConn)
	runtime, cancel := context.WithCancel(ctx)
	d, err := newSelectedDispatch(runtime, send, nil, nil)
	require.NoError(t, err)
	fixture := &originSettlementFixture{d: d, conn: conn, callbacks: make(chan originSettlementAnswer, 16), sdkExited: make(chan struct{}), sdkResult: make(chan error, 1), stopped: make(chan struct{})}
	gate := make(chan struct{})
	var gateOnce, transportOnce sync.Once
	fixture.selectedStop = func() { gateOnce.Do(func() { close(gate) }) }
	fixture.transportStop = func() {
		transportOnce.Do(func() {
			if bridge != nil {
				bridge.CloseCallbacks()
			}
			require.NoError(t, conn.Close())
		})
	}
	p.OnOriginIntake(func(work context.Context, input rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
		output, failure := d.originIntake(work, input)
		fixture.callbacks <- originSettlementAnswer{input: input, output: output, err: failure}
		return output, failure
	})
	p.OnCurrentSelection(d.currentSelection)
	manager := newRIBManager(p)
	manager.selectedDispatch = d
	manager.newConfigTx(!native).Register(p)
	started := make(chan struct{})
	p.OnStarted(func(context.Context) error { close(started); return nil })
	go func() {
		fixture.sdkResult <- p.Run(runtime, &sdk.Registration{WantsConfig: []string{configRootBGP}})
		close(fixture.sdkExited)
		<-gate
		stop, finish := context.WithTimeout(context.Background(), rpc.RequestCancelTimeout)
		defer finish()
		if failure := d.stop(stop); failure != nil {
			t.Errorf("selected fixture stop: %v", failure)
		}
		close(fixture.stopped)
	}()
	t.Cleanup(func() {
		fixture.transportStop()
		cancel()
		// A failing pre-fix test must still release its task-owned component. This
		// is cleanup AFTER the asserted SDK exit, never an action satisfying it.
		stop, finish := context.WithTimeout(context.Background(), rpc.RequestCancelTimeout)
		defer finish()
		require.NoError(t, d.stop(stop))
		fixture.selectedStop()
		require.NoError(t, p.Close())
		selectedDispatchAwait(t, stop, fixture.stopped)
		select {
		case failure := <-fixture.sdkResult:
			if failure != nil {
				t.Logf("SDK close result: %v", failure)
			}
		default:
		}
	})
	quiesceRIBStartup(t, ctx, conn, bridge)
	selectedDispatchAwait(t, ctx, started)
	return fixture
}

func originSettlementCall(t *testing.T, ctx context.Context, fixture *originSettlementFixture, input rpc.OriginIntakeInput) *rpc.UpdateRouteOutput {
	t.Helper()
	output, err := fixture.conn.SendOriginIntake(ctx, input)
	require.NoError(t, err)
	completed := selectedDispatchAwait(t, ctx, fixture.callbacks)
	require.Equal(t, input.OriginReceipt, completed.input.OriginReceipt)
	require.NoError(t, completed.err)
	require.NoError(t, rpc.ValidateOriginIntakeOutput(input, output))
	return output
}

// TestRIBOriginSDKFailedRestorationReleasesMutationAndShutdown proves actual
// SDK caller cancellation, input deadline and transport close cannot wait forever
// after a useful committed origin loses its final writer. The exact accepted
// writer observes cancellation, remains joined while held, then returns a real
// failure. Failed replacement remains pending/uncertain with its cause; fresh
// B and stale-receipt controls stay useful. No sleep or absence-only oracle.
func TestRIBOriginSDKFailedRestorationReleasesMutationAndShutdown(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, mode := range []string{"cancel", "deadline", "transport-close"} {
			t.Run(fmt.Sprintf("native=%t/%s", native, mode), func(t *testing.T) {
				ctx := selectedDispatchContext(t)
				a, b := netip.MustParseAddr("192.0.2.231"), netip.MustParseAddr("192.0.2.232")
				const heldReceipt = uint64(11002)
				entered, canceled, release, refused := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
				var enteredOnce, canceledOnce, releaseOnce, refusedOnce sync.Once
				var refuseA atomic.Bool
				cause := errors.New("controlled permanent selected writer refusal")
				var mu sync.Mutex
				ledger := make(map[string]uint64)
				recorded := make(chan rpc.SelectRoutesInput, 24)
				t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
				sender := func(work context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
					for _, route := range input.Routes {
						if route.OriginReceipt == heldReceipt {
							mu.Lock()
							ledger[input.Destination] = heldReceipt
							mu.Unlock()
							enteredOnce.Do(func() { close(entered) })
							<-work.Done()
							canceledOnce.Do(func() { close(canceled) })
							<-release
							refuseA.Store(true)
							return nil, cause
						}
					}
					if input.Destination == a.String() {
						if refuseA.Load() {
							refusedOnce.Do(func() { close(refused) })
							return nil, cause
						}
					}
					for _, route := range input.Routes {
						if route.Kind == rpc.SelectedRouteOrigin {
							mu.Lock()
							ledger[input.Destination] = route.OriginReceipt
							mu.Unlock()
						}
					}
					recorded <- input
					return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
				}
				fixture := startOriginSettlementFixture(t, ctx, native, sender)
				// Release the deliberately held writer before fixture teardown even if an
				// assertion fails; a timeout must not leave another task a live goroutine.
				t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
				d := fixture.d
				learned := selectedDispatchRoute(231, 8231)
				require.NoError(t, d.publish(&learned))
				require.NoError(t, d.destinationOwnerUp(a, 1231, 2231))
				require.NoError(t, d.destinationOwnerUp(b, 1232, 2232))
				require.NoError(t, d.barrier(ctx))
				selectedDispatchAwait(t, ctx, recorded)
				selectedDispatchAwait(t, ctx, recorded)
				local := originIntakeFixture(a, 1231, 11001, 231)
				require.Equal(t, uint32(1), originSettlementCall(t, ctx, fixture, local).Announced)
				selectedDispatchAwait(t, ctx, recorded)
				oldB := originIntakeFixture(b, 1232, 12001, 231)
				oldB.Source.HasDistance = true
				oldB.Source.Distance = 0
				require.Equal(t, uint32(1), originSettlementCall(t, ctx, fixture, oldB).Announced)
				selectedDispatchAwait(t, ctx, recorded)
				freshB := rpc.CloneOriginIntake(oldB)
				freshB.OriginReceipt = 12002
				require.Equal(t, uint32(1), originSettlementCall(t, ctx, fixture, freshB).Announced)
				selectedDispatchAwait(t, ctx, recorded)
				oldB.Operation = rpc.OriginIntakeWithdraw
				require.Equal(t, uint32(1), originSettlementCall(t, ctx, fixture, oldB).Withdrawn)
				mu.Lock()
				require.Equal(t, uint64(12002), ledger[b.String()])
				mu.Unlock()
				blocked := rpc.CloneOriginIntake(local)
				blocked.OriginReceipt = heldReceipt
				work, cancel := context.WithCancel(ctx)
				defer cancel()
				if mode == "deadline" {
					blocked.DeadlineMS = time.Now().Add(2 * time.Second).UnixMilli()
				}
				caller := make(chan originSettlementAnswer, 1)
				go func() {
					output, failure := fixture.conn.SendOriginIntake(work, blocked)
					caller <- originSettlementAnswer{output: output, err: failure}
				}()
				selectedDispatchAwait(t, ctx, entered)
				mu.Lock()
				require.Equal(t, heldReceipt, ledger[a.String()])
				require.Equal(t, uint64(12002), ledger[b.String()])
				mu.Unlock()
				switch mode {
				case "cancel":
					cancel()
				case "deadline":
				case "transport-close":
					fixture.transportStop()
				}
				selectedDispatchAwait(t, ctx, canceled)
				select {
				case early := <-fixture.callbacks:
					t.Fatalf("handler returned before its accepted writer joined: %v", early.err)
				default:
				}
				// B can still send useful current learned work while A's real old request
				// remains held. Its retained local winner is independent of this marker.
				marker := selectedDispatchRoute(232, 8232)
				require.NoError(t, d.publish(&marker))
				require.NoError(t, d.barrier(ctx, b))
				useful := selectedDispatchAwait(t, ctx, recorded)
				require.Equal(t, b.String(), useful.Destination)
				releaseOnce.Do(func() { close(release) })
				selectedDispatchAwait(t, ctx, refused)
				completed := selectedDispatchAwait(t, ctx, fixture.callbacks)
				require.Equal(t, heldReceipt, completed.input.OriginReceipt)
				require.Nil(t, completed.output)
				require.ErrorContains(t, completed.err, cause.Error())
				returned := selectedDispatchAwait(t, ctx, caller)
				require.Error(t, returned.err)
				if mode == "deadline" {
					require.True(t, strings.Contains(returned.err.Error(), "deadline"), returned.err.Error())
				}
				subject, keyError := selectedDispatchKey(local.Route.Family, local.Route.KeyNLRI)
				require.NoError(t, keyError)
				d.mu.Lock()
				destination := d.destinations[selectedDestinationID{peer: a, session: 2231}]
				var ack selectedAcknowledgment
				var pending bool
				var flight int
				var failure error
				if destination != nil {
					ack = destination.acknowledged[subject]
					_, pending = destination.pending[subject]
					flight = len(destination.inFlight)
					failure = destination.failure
				}
				currentA := d.currentScopeLocked(originScope{peer: a, owner: 1231}, subject)
				currentB := d.currentScopeLocked(originScope{peer: b, owner: 1232}, subject)
				d.mu.Unlock()
				require.NotNil(t, destination)
				require.True(t, pending)
				require.True(t, ack.possible)
				require.True(t, ack.uncertain)
				require.Zero(t, flight, "the captured accepted call actually joined")
				require.ErrorContains(t, failure, cause.Error())
				require.NotNil(t, currentA)
				require.NotNil(t, currentB)
				require.Equal(t, rpc.SelectedRouteReceived, currentA.wire.Kind)
				require.Equal(t, uint64(12002), currentB.wire.OriginReceipt)
				if mode != "transport-close" {
					next := rpc.CloneOriginIntake(freshB)
					next.OriginReceipt = 12003
					require.Equal(t, uint32(1), originSettlementCall(t, ctx, fixture, next).Announced)
					selectedDispatchAwait(t, ctx, recorded)
					fixture.transportStop()
				}
				// This marker is produced by actual SDK Run BEFORE selected stop. A bounded
				// timeout is only the harness guard; passing requires its positive exit.
				selectedDispatchAwait(t, ctx, fixture.sdkExited)
				if failure := selectedDispatchAwait(t, ctx, fixture.sdkResult); failure != nil {
					require.NotContains(t, failure.Error(), "panicked")
					t.Logf("SDK transport-close result: %v", failure)
				}
				fixture.selectedStop()
				selectedDispatchAwait(t, ctx, fixture.stopped)
			})
		}
	}
}

// TestRIBOriginSDKPreferenceFailureReportsUnrestoredOutput runs real ConfigTx
// callbacks. Both new preference output and its restored prior winner refuse;
// apply returns a broken-config cause without claiming a completed rollback.
// A distance-zero B winner and its useful output survive both publications.
func TestRIBOriginSDKPreferenceFailureReportsUnrestoredOutput(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("native=%t", native), func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			a, b := netip.MustParseAddr("192.0.2.233"), netip.MustParseAddr("192.0.2.234")
			var refuse atomic.Bool
			failedLearned, failedRestore := make(chan struct{}), make(chan struct{})
			var learnedOnce, restoreOnce sync.Once
			writes := make(chan rpc.SelectRoutesInput, 24)
			cause := errors.New("controlled permanent preference writer refusal")
			fixture := startOriginSettlementFixture(t, ctx, native, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				if input.Destination == a.String() {
					if refuse.Load() {
						for _, route := range input.Routes {
							if route.Kind == rpc.SelectedRouteReceived {
								learnedOnce.Do(func() { close(failedLearned) })
							}
							if route.Kind == rpc.SelectedRouteOrigin {
								restoreOnce.Do(func() { close(failedRestore) })
							}
						}
						return nil, cause
					}
				}
				writes <- input
				return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
			})
			d := fixture.d
			learned := selectedDispatchRoute(233, 8233)
			require.NoError(t, d.publish(&learned))
			require.NoError(t, d.destinationOwnerUp(a, 1233, 2233))
			require.NoError(t, d.destinationOwnerUp(b, 1234, 2234))
			require.NoError(t, d.barrier(ctx))
			selectedDispatchAwait(t, ctx, writes)
			selectedDispatchAwait(t, ctx, writes)
			local := originIntakeFixture(a, 1233, 13001, 233)
			originSettlementCall(t, ctx, fixture, local)
			selectedDispatchAwait(t, ctx, writes)
			survivor := originIntakeFixture(b, 1234, 13002, 233)
			survivor.Source.HasDistance = true
			survivor.Source.Distance = 0
			originSettlementCall(t, ctx, fixture, survivor)
			selectedDispatchAwait(t, ctx, writes)
			verify, err := fixture.conn.SendConfigVerify(ctx, []rpc.ConfigSection{{Root: "bgp", Data: `{"bgp":{}}`}, {Root: "rib", Data: `{"rib":{"distance":{"static":"250"}}}`}})
			require.NoError(t, err)
			require.Equal(t, rpc.StatusOK, verify.Status)
			refuse.Store(true)
			apply, err := fixture.conn.SendConfigApply(ctx, nil)
			require.NoError(t, err)
			selectedDispatchAwait(t, ctx, failedLearned)
			selectedDispatchAwait(t, ctx, failedRestore)
			require.Equal(t, rpc.StatusError, apply.Status)
			require.Contains(t, apply.Error, cause.Error())
			require.Contains(t, apply.Error, "preferences could not be restored")
			require.NoError(t, d.barrier(ctx, b))
			key, keyError := selectedDispatchKey(local.Route.Family, local.Route.KeyNLRI)
			require.NoError(t, keyError)
			d.mu.Lock()
			current := d.currentScopeLocked(originScope{peer: a, owner: 1233}, key)
			kept := d.currentScopeLocked(originScope{peer: b, owner: 1234}, key)
			distance := d.distances["static"]
			target := d.destinations[selectedDestinationID{peer: a, session: 2233}]
			var pending, uncertain bool
			if target != nil {
				_, pending = target.pending[key]
				uncertain = target.acknowledged[key].uncertain
			}
			d.mu.Unlock()
			require.NotNil(t, current)
			require.NotNil(t, kept)
			require.NotNil(t, target)
			require.Equal(t, uint64(13001), current.wire.OriginReceipt)
			require.Equal(t, uint8(10), distance, "the desired declaration is restored, not falsely acknowledged output")
			require.True(t, pending)
			require.True(t, uncertain)
			require.Equal(t, uint64(13002), kept.wire.OriginReceipt)
			fixture.transportStop()
			selectedDispatchAwait(t, ctx, fixture.sdkExited)
			if failure := selectedDispatchAwait(t, ctx, fixture.sdkResult); failure != nil {
				require.NotContains(t, failure.Error(), "panicked")
				t.Logf("SDK transport-close result: %v", failure)
			}
			fixture.selectedStop()
			selectedDispatchAwait(t, ctx, fixture.stopped)
		})
	}
}

// TestRIBOriginSDKWithdrawFailureKeepsExactFrontier requires a real callback
// withdrawal to report its permanent absence-writer failure. The candidate is
// removed but possible output/pending retirement remains. A newly useful winner
// then completes on that same key, and stale cleanup cannot erase it or B.
func TestRIBOriginSDKWithdrawFailureKeepsExactFrontier(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("native=%t", native), func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			a, b := netip.MustParseAddr("192.0.2.241"), netip.MustParseAddr("192.0.2.242")
			refused := make(chan struct{})
			var refusedOnce sync.Once
			cause := errors.New("controlled permanent origin absence refusal")
			writes := make(chan rpc.SelectRoutesInput, 12)
			fixture := startOriginSettlementFixture(t, ctx, native, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				if input.Destination == a.String() {
					for _, route := range input.Routes {
						if route.Kind == rpc.SelectedRouteAbsent {
							refusedOnce.Do(func() { close(refused) })
							return nil, cause
						}
					}
				}
				writes <- input
				return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
			})
			d := fixture.d
			require.NoError(t, d.destinationOwnerUp(a, 1241, 2241))
			require.NoError(t, d.destinationOwnerUp(b, 1242, 2242))
			require.NoError(t, d.barrier(ctx))
			old := originIntakeFixture(a, 1241, 14001, 241)
			originSettlementCall(t, ctx, fixture, old)
			selectedDispatchAwait(t, ctx, writes)
			sibling := originIntakeFixture(b, 1242, 14002, 241)
			originSettlementCall(t, ctx, fixture, sibling)
			selectedDispatchAwait(t, ctx, writes)
			old.Operation = rpc.OriginIntakeWithdraw
			cleanup, finish := context.WithTimeout(ctx, rpc.RequestCancelTimeout)
			defer finish()
			output, err := fixture.conn.SendOriginIntake(cleanup, old)
			selectedDispatchAwait(t, ctx, refused)
			require.Nil(t, output)
			require.ErrorContains(t, err, cause.Error())
			callback := selectedDispatchAwait(t, ctx, fixture.callbacks)
			require.Nil(t, callback.output)
			require.ErrorContains(t, callback.err, cause.Error())
			key, keyError := selectedDispatchKey(old.Route.Family, old.Route.KeyNLRI)
			require.NoError(t, keyError)
			d.mu.Lock()
			target := d.destinations[selectedDestinationID{peer: a, session: 2241}]
			var pending, possible, uncertain bool
			if target != nil {
				_, pending = target.pending[key]
				possible = target.acknowledged[key].possible
				uncertain = target.acknowledged[key].uncertain
			}
			absent := d.currentScopeLocked(originScope{peer: a, owner: 1241}, key)
			currentB := d.currentScopeLocked(originScope{peer: b, owner: 1242}, key)
			d.mu.Unlock()
			require.NotNil(t, target)
			require.True(t, pending)
			require.True(t, possible)
			require.True(t, uncertain)
			require.Nil(t, absent, "desired removal must not masquerade as confirmed frontier retirement")
			require.NotNil(t, currentB)
			require.Equal(t, uint64(14002), currentB.wire.OriginReceipt)
			fresh := originIntakeFixture(a, 1241, 14003, 241)
			fresh.Producer.Name = "surviving-producer"
			require.Equal(t, uint32(1), originSettlementCall(t, ctx, fixture, fresh).Announced)
			selectedDispatchAwait(t, ctx, writes)
			require.Equal(t, uint32(1), originSettlementCall(t, ctx, fixture, old).Withdrawn)
			d.mu.Lock()
			currentA := d.currentScopeLocked(originScope{peer: a, owner: 1241}, key)
			currentB = d.currentScopeLocked(originScope{peer: b, owner: 1242}, key)
			d.mu.Unlock()
			require.Equal(t, uint64(14003), currentA.wire.OriginReceipt)
			require.Equal(t, uint64(14002), currentB.wire.OriginReceipt)
			require.Empty(t, writes, "stale old-receipt cleanup must not send over either current winner")
			fixture.transportStop()
			selectedDispatchAwait(t, ctx, fixture.sdkExited)
			if failure := selectedDispatchAwait(t, ctx, fixture.sdkResult); failure != nil {
				require.NotContains(t, failure.Error(), "panicked")
				t.Logf("SDK transport-close result: %v", failure)
			}
			fixture.selectedStop()
			selectedDispatchAwait(t, ctx, fixture.stopped)
		})
	}
}
