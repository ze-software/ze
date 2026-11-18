// Design: docs/architecture/api/process-protocol.md -- captured event settlement.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const appliedEventNamespace = "applied-emission-test"
const appliedEventType = "cleanup"

var _ = events.RegisterSignal(appliedEventNamespace, appliedEventType)

type appliedEventResult struct {
	delivered int
	err       error
}

// Both placements use actual registered SDK processes and the host operation
// registry. Wrapping Conn hides only the typed bridge discovery, so the second
// placement uses actual request/cancellation frames on the same real server.
func newAppliedEmitter(t *testing.T, typed, self bool, handler func(string) error, settle func(context.Context) error) (*Server, *sdk.Plugin) {
	t.Helper()
	var namedSettlement func(context.Context, string) error
	if settle != nil {
		namedSettlement = func(ctx context.Context, _ string) error { return settle(ctx) }
	}
	return newAppliedEmitterGroup(t, typed, self, handler, namedSettlement, []string{"applied-consumer"})
}

func newAppliedEmitterGroup(t *testing.T, typed, self bool, handler func(string) error, settle func(context.Context, string) error, consumers []string) (*Server, *sdk.Plugin) {
	t.Helper()
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })
	ownerReady := make(chan *sdk.Plugin, 1)
	names := append([]string{"applied-emitter"}, consumers...)
	for _, name := range names {
		registerLifecyclePlugin(t, name, nil, func(conn net.Conn) int {
			if !typed {
				conn = struct{ net.Conn }{conn}
			}
			p := sdk.NewWithConn(name, conn)
			defer p.Close() //nolint:errcheck // Fixture owns this SDK connection through server stop.
			if name != "applied-emitter" || self {
				p.AddStartupSubscriptionsIn(appliedEventNamespace, []string{appliedEventType})
			}
			if name != "applied-emitter" {
				p.OnEvent(handler)
				if settle != nil {
					p.OnEventSettlement(func(ctx context.Context) error { return settle(ctx, name) })
				}
			}
			if name == "applied-emitter" {
				p.OnStarted(func(context.Context) error { ownerReady <- p; return nil })
			}
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		})
	}
	host, _ := newLifecycleStartupServer(t)
	configs := make([]plugin.PluginConfig, 0, len(names))
	for _, name := range names {
		configs = append(configs, plugin.PluginConfig{Name: name, Internal: true, Encoder: plugin.EncodingJSON})
	}
	require.NoError(t, host.runPluginPhase(configs))
	select {
	case owner := <-ownerReady:
		return host, owner
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
		return nil, nil
	}
}

func awaitAppliedResult(t *testing.T, ctx context.Context, result <-chan appliedEventResult) appliedEventResult {
	t.Helper()
	select {
	case value := <-result:
		return value
	case <-ctx.Done():
		t.Fatal(ctx.Err())
		return appliedEventResult{}
	}
}

// The actual emitter cannot finish before the captured receiver's asynchronous
// output. A subsequent ordinary EmitEvent remains queue-only and does not run
// that receiver's settlement callback at all.
func TestEmitAppliedJoinsDeferredOutputBothPlacements(t *testing.T) {
	for _, typed := range []bool{true, false} {
		t.Run(map[bool]string{true: "typed", false: "framed"}[typed], func(t *testing.T) {
			entered, release := make(chan struct{}, 1), make(chan struct{})
			releaseOutput := sync.OnceFunc(func() { close(release) })
			defer releaseOutput()
			applied := make(chan struct{}, 2)
			_, owner := newAppliedEmitter(t, typed, false, func(string) error { applied <- struct{}{}; return nil }, func(ctx context.Context) error {
				entered <- struct{}{}
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			result := make(chan appliedEventResult, 1)
			go func() {
				count, err := owner.EmitEventApplied(ctx, appliedEventNamespace, appliedEventType, "", "", "null")
				result <- appliedEventResult{count, err}
			}()
			select {
			case <-entered:
			case value := <-result:
				t.Fatalf("emitter returned before deferred output: %v", value.err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			select {
			case value := <-result:
				t.Fatalf("emitter returned before held output: %v", value.err)
			default:
			}
			releaseOutput()
			value := awaitAppliedResult(t, ctx, result)
			require.NoError(t, value.err)
			require.Equal(t, 1, value.delivered)
			count, err := owner.EmitEvent(ctx, appliedEventNamespace, appliedEventType, "", "", "null")
			require.NoError(t, err)
			require.Equal(t, 1, count)
			select {
			case <-entered:
				t.Fatal("ordinary emission acquired applied semantics")
			default:
			}
			for range 2 {
				select {
				case <-applied:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
		})
	}
}

// Self-wait is a refusal before any receiver is invoked, including a useful
// independent consumer. Skipping self and returning the other's count would
// create a false all-applied receipt.
func TestEmitAppliedRefusesSelfBeforePublicationBothPlacements(t *testing.T) {
	for _, typed := range []bool{true, false} {
		t.Run(map[bool]string{true: "typed", false: "framed"}[typed], func(t *testing.T) {
			observed := make(chan struct{}, 1)
			host, owner := newAppliedEmitter(t, typed, true, func(string) error { observed <- struct{}{}; return nil }, nil)
			engineObserved := make(chan struct{}, 1)
			unsubscribe := host.SubscribeEngineEvent(appliedEventNamespace, appliedEventType, func(any) { engineObserved <- struct{}{} })
			defer unsubscribe()
			count, err := owner.EmitEventApplied(t.Context(), appliedEventNamespace, appliedEventType, "", "", "null")
			require.ErrorContains(t, err, "own subscription")
			require.Zero(t, count)
			select {
			case <-observed:
				t.Fatal("refused self-wait partially published")
			default:
			}
			select {
			case <-engineObserved:
				t.Fatal("refused self-wait reached an engine callback")
			default:
			}
		})
	}
}

func TestEmitAppliedPreservesHandlerFailureBothPlacements(t *testing.T) {
	for _, typed := range []bool{true, false} {
		t.Run(map[bool]string{true: "typed", false: "framed"}[typed], func(t *testing.T) {
			_, owner := newAppliedEmitter(t, typed, false, func(string) error { return errors.New("original cleanup refused") }, nil)
			count, err := owner.EmitEventApplied(t.Context(), appliedEventNamespace, appliedEventType, "", "", "null")
			require.ErrorContains(t, err, "original cleanup refused")
			require.Zero(t, count)
		})
	}
}

// Cancellation is not an all-applied answer. The outer cancellable request is
// remotely joined; if the already-admitted subscriber callback cannot be joined
// within that budget, the error explicitly says its deferred work remains owed.
func TestEmitAppliedCancellationNeverConfirmsCleanupBothPlacements(t *testing.T) {
	for _, typed := range []bool{true, false} {
		t.Run(map[bool]string{true: "typed", false: "framed"}[typed], func(t *testing.T) {
			entered, release, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
			releaseOutput := sync.OnceFunc(func() { close(release) })
			defer releaseOutput()
			_, owner := newAppliedEmitter(t, typed, false, func(string) error { return nil }, func(ctx context.Context) error {
				close(entered)
				<-release
				defer close(joined)
				return ctx.Err()
			})
			ceiling, stop := context.WithTimeout(t.Context(), 5*time.Second)
			defer stop()
			work, cancel := context.WithCancel(ceiling)
			defer cancel()
			result := make(chan appliedEventResult, 1)
			go func() {
				count, err := owner.EmitEventApplied(work, appliedEventNamespace, appliedEventType, "", "", "null")
				result <- appliedEventResult{count, err}
			}()
			select {
			case <-entered:
			case value := <-result:
				t.Fatalf("emission did not enter deferred work: %v", value.err)
			case <-ceiling.Done():
				t.Fatal(ceiling.Err())
			}
			cancel()
			value := awaitAppliedResult(t, ceiling, result)
			require.Error(t, value.err)
			require.Zero(t, value.delivered)
			require.True(t, strings.Contains(value.err.Error(), "owed") || strings.Contains(value.err.Error(), "settle"), value.err.Error())
			releaseOutput()
			select {
			case <-joined:
			case <-ceiling.Done():
				t.Fatal("admitted callback was not joined on fixture cleanup")
			}
		})
	}
}

// The captured connection, rather than a later connection on the same Process,
// owns completion. Even an already-applied handler and successful deferred
// callback cannot acknowledge a departed connection's barrier.
func TestEmitAppliedRejectsChangedCapturedConnectionBothPlacements(t *testing.T) {
	for _, typed := range []bool{true, false} {
		t.Run(map[bool]string{true: "typed", false: "framed"}[typed], func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			releaseOutput := sync.OnceFunc(func() { close(release) })
			defer releaseOutput()
			host, owner := newAppliedEmitter(t, typed, false, func(string) error { return nil }, func(context.Context) error { close(entered); <-release; return nil })
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			result := make(chan appliedEventResult, 1)
			go func() {
				count, err := owner.EmitEventApplied(ctx, appliedEventNamespace, appliedEventType, "", "", "null")
				result <- appliedEventResult{count, err}
			}()
			select {
			case <-entered:
			case value := <-result:
				t.Fatalf("did not capture deferred receiver: %v", value.err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			proc := host.ProcessManager().GetProcess("applied-consumer")
			require.NotNil(t, proc)
			original := proc.Conn()
			proc.SetConn(nil)
			defer proc.SetConn(original)
			releaseOutput()
			value := awaitAppliedResult(t, ctx, result)
			require.Error(t, value.err)
			require.Zero(t, value.delivered)
		})
	}
}

// The first settled subscriber can end while another subscriber still runs.
// Method: hold the second settlement, retire only the first original Process,
// then release the second. A previous per-receiver success cannot certify it.
func TestEmitAppliedRechecksWholeCapturedPopulationBothPlacements(t *testing.T) {
	for _, typed := range []bool{true, false} {
		t.Run(map[bool]string{true: "typed", false: "framed"}[typed], func(t *testing.T) {
			for _, failure := range []string{"process", "connection"} {
				t.Run(failure, func(t *testing.T) {
					first, second := make(chan string, 1), make(chan struct{})
					release := make(chan struct{})
					releaseOutput := sync.OnceFunc(func() { close(release) })
					defer releaseOutput()
					var calls atomic.Int32
					host, owner := newAppliedEmitterGroup(t, typed, false, func(string) error { return nil }, func(ctx context.Context, name string) error {
						if calls.Add(1) == 1 {
							first <- name
							return nil
						}
						close(second)
						select {
						case <-release:
							return nil
						case <-ctx.Done():
							return ctx.Err()
						}
					}, []string{"applied-consumer", "applied-consumer-two"})
					ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
					defer cancel()
					result := make(chan appliedEventResult, 1)
					go func() {
						count, err := owner.EmitEventApplied(ctx, appliedEventNamespace, appliedEventType, "", "", "null")
						result <- appliedEventResult{count, err}
					}()
					select {
					case <-second:
					case value := <-result:
						t.Fatalf("second settlement not entered: %v", value.err)
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					}
					var retired string
					select {
					case retired = <-first:
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					}
					proc := host.ProcessManager().GetProcess(retired)
					require.NotNil(t, proc)
					expected := "original process stopped"
					if failure == "process" {
						// Keep the same Conn to isolate the Process lifetime from pointer matching.
						proc.SetRunning(false)
						defer proc.SetRunning(true)
					} else {
						expected = "original connection changed"
						original := proc.Conn()
						proc.SetConn(nil)
						defer proc.SetConn(original)
					}
					releaseOutput()
					value := awaitAppliedResult(t, ctx, result)
					require.ErrorContains(t, value.err, expected)
					require.Zero(t, value.delivered)
				})
			}
		})
	}
}

// Admission and evaluation use one header predicate. Ordinary headers retain
// queue-only behavior; applied input cannot run without authenticated identity
// and a positive deadline, whether admission or the raw handler receives it.
func TestEmitAppliedHeaderContract(t *testing.T) {
	proc := reportingProcess("applied-header")
	for _, deadline := range []int64{0, -1} {
		raw, err := json.Marshal(rpc.EmitEventInput{AwaitApplied: true, DeadlineMS: deadline})
		require.NoError(t, err)
		_, err = eventEmissionInput(proc, raw)
		require.ErrorContains(t, err, "positive absolute deadline-ms")
		_, err = emitEventCancellationScope(nil, proc, raw)
		require.ErrorContains(t, err, "positive absolute deadline-ms")
		_, err = (&Server{}).emitEventApplied(t.Context(), proc, raw)
		require.ErrorContains(t, err, "positive absolute deadline-ms")
	}
	raw := json.RawMessage(`{"await-applied":true,"deadline-ms":1}`)
	_, err := eventEmissionInput(nil, raw)
	require.ErrorContains(t, err, "authenticated process")
	ordinary, err := eventEmissionInput(nil, json.RawMessage(`{"deadline-ms":-1}`))
	require.NoError(t, err)
	require.False(t, ordinary.AwaitApplied)
}

// Both bridge evaluation and socket admission must bound an outside author's
// distant deadline. Method: call the actual direct RPC with twice the ceiling
// and inspect the real subscriber callback's remaining work budget.
func TestEmitAppliedBoundsDirectDeadline(t *testing.T) {
	observed := make(chan time.Duration, 1)
	host, _ := newAppliedEmitter(t, true, false, func(string) error { return nil }, func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			return errors.New("settlement has no deadline")
		}
		observed <- time.Until(deadline)
		return nil
	})
	proc := host.ProcessManager().GetProcess("applied-emitter")
	require.NotNil(t, proc)
	raw, err := json.Marshal(rpc.EmitEventInput{AwaitApplied: true, DeadlineMS: time.Now().Add(2 * rpc.CancellableRequestTimeout).UnixMilli(), Namespace: appliedEventNamespace, EventType: appliedEventType, Event: "null"})
	require.NoError(t, err)
	result, err := host.dispatchPluginRPCDirect(t.Context(), proc, rpc.MethodEmitEvent, raw)
	require.NoError(t, err)
	var receipt rpc.EmitEventOutput
	require.NoError(t, json.Unmarshal(result, &receipt))
	require.True(t, receipt.Applied)
	select {
	case remaining := <-observed:
		require.Positive(t, remaining)
		require.LessOrEqual(t, remaining, rpc.CancellableRequestTimeout)
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
}

// Applied mode uses the same subscription direction and configured peer funnel.
// Method: narrow one live consumer to sent events, retain a useful matching
// control, and emit a peer-scoped event whose graph grants no delivery.
func TestEmitAppliedPreservesMatchingBothPlacements(t *testing.T) {
	for _, typed := range []bool{true, false} {
		t.Run(map[bool]string{true: "typed", false: "framed"}[typed], func(t *testing.T) {
			var received atomic.Int32
			host, owner := newAppliedEmitter(t, typed, false, func(string) error { received.Add(1); return nil }, nil)
			consumer := host.ProcessManager().GetProcess("applied-consumer")
			require.NotNil(t, consumer)
			host.subscriptions.clearProcess(consumer)
			host.subscriptions.Add(consumer, &Subscription{Namespace: events.LookupNamespaceID(appliedEventNamespace), EventType: events.LookupEventTypeID(appliedEventType), Direction: events.DirSent})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			count, err := owner.EmitEventApplied(ctx, appliedEventNamespace, appliedEventType, "received", "", "null")
			require.NoError(t, err)
			require.Zero(t, count)
			require.Zero(t, received.Load())
			count, err = owner.EmitEventApplied(ctx, appliedEventNamespace, appliedEventType, "sent", "", "null")
			require.NoError(t, err)
			require.Equal(t, 1, count)
			require.Equal(t, int32(1), received.Load())
			count, err = owner.EmitEventApplied(ctx, appliedEventNamespace, appliedEventType, "sent", "192.0.2.99", "null")
			require.NoError(t, err)
			require.Zero(t, count)
			require.Equal(t, int32(1), received.Load())
		})
	}
}
