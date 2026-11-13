// Design: docs/architecture/rib/unified-locrib.md -- bounded worker lifecycle.

package sysrib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// A request canceled while queued must never publish its distance table. The
// held Read proves admission and cancellation precede worker mutation.
func TestRouteViewRuntimeQueuedConfigurationCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	reading, release := make(chan struct{}), make(chan struct{})
	var first sync.Once
	var distances atomic.Int32
	feed := runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
		if request.Method == rpc.MethodRouteViewDistances {
			distances.Add(1)
		}
		if request.Method != rpc.MethodRouteViewRead {
			return nil, false, nil
		}
		first.Do(func() {
			close(reading)
			select {
			case <-release:
			case <-ctx.Done():
			}
		})
		return nil, false, nil
	})
	feed.pages <- runtimeSnapshot(1, true)
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reading:
	case <-ctx.Done():
		t.Fatal("worker did not enter held Read")
	}
	caller, canceled := context.WithCancel(ctx)
	task := &routeViewConfigurationTask{ctx: caller, configuration: client.configuration, done: make(chan error, 1)}
	if _, err := client.enqueue(caller, task); err != nil {
		t.Fatal(err)
	}
	canceled()
	close(release)
	select {
	case err := <-task.done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queued cancellation returned %v", err)
		}
	case <-ctx.Done():
		t.Fatal("canceled queue entry never received its verdict")
	}
	if distances.Load() != 1 {
		t.Fatalf("canceled queued request mutated authority: calls=%d", distances.Load())
	}
}

// Shutdown must cancel the outstanding SDK Read and join its caller before
// attempting Close. The peer releases its held response only after that join.
func TestRouteViewRuntimeCloseCancelsReadBeforeCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	reading, release := make(chan struct{}), make(chan struct{})
	var first sync.Once
	feed := runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
		if request.Method != rpc.MethodRouteViewRead {
			return nil, false, nil
		}
		first.Do(func() {
			close(reading)
			select {
			case <-release:
			case <-ctx.Done():
			}
		})
		return nil, false, nil
	})
	feed.pages <- runtimeSnapshot(1, true)
	client := openRuntimeFeedClient(t, ctx, feed)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reading:
	case <-ctx.Done():
		t.Fatal("worker never called Read")
	}
	closed := make(chan error, 1)
	go func() { closed <- client.Close(ctx) }()
	select {
	case <-client.worker.done:
	case <-ctx.Done():
		t.Fatal("Close did not cancel and join Read")
	}
	close(release)
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("Close did not finish lease cleanup")
	}
	if err := client.Failure(); err != nil {
		t.Fatalf("normal canceled worker retained a fatal error: %v", err)
	}
}

// A live cleanup refusal is still an error during shutdown. A root cancellation
// cannot turn a failed output withdrawal into a successful close.
func TestRouteViewRuntimeCloseRetainsCleanupRefusal(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	feed := runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
		if request.Method == rpc.MethodRouteViewClose {
			return nil, true, fmt.Errorf("fixture-output-withdrawal-refused")
		}
		return nil, false, nil
	})
	client := openRuntimeFeedClient(t, ctx, feed)
	canceled, stop := context.WithCancel(ctx)
	stop()
	if err := client.Close(canceled); err == nil {
		t.Fatal("canceled shutdown swallowed a live output cleanup refusal")
	}
}

// Overflow retains bounded keys and a scan repairs a removal that was dropped
// from the full queue. It must not revive the cached selected value.
func TestRouteViewRuntimeLocalOverflowRepairsRemoval(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	rib := locrib.NewRIB()
	source := redistevents.RegisterProtocol("static")
	prefix := netip.MustParsePrefix("198.18.233.0/24")
	path := locrib.Path{Source: source, RouteType: routetype.Blackhole}
	rib.Insert(family.IPv4Unicast, prefix, path)
	state := newSysRIB()
	state.resolver.Store(newNHResolver(rib))
	capture := newRouteViewLocalCapture(rib)
	defer capture.unsubscribe()
	control, err := capture.scan(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	batches, err := state.applySelectedBatch(control)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches[family.IPv4Unicast]) != 1 || batches[family.IPv4Unicast][0].Action != routeaction.Add {
		t.Fatal("initial local control did not install")
	}
	for range rpc.RouteViewRecordsMax {
		capture.offer(locrib.Change{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("198.18.234.0/24")})
	}
	rib.Remove(family.IPv4Unicast, prefix, source, path.Instance)
	if !capture.overflow.Load() {
		t.Fatal("removal did not exercise the full queue overflow")
	}
	if len(capture.keys) != rpc.RouteViewRecordsMax {
		t.Fatal("overflow retained more than the bounded key queue")
	}
	repaired, err := capture.scan(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	batches, err = state.applySelectedBatch(repaired)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range batches[family.IPv4Unicast] {
		if change.Prefix == prefix && change.Action == routeaction.Withdraw {
			return
		}
	}
	t.Fatal("overflow full scan lost the previously installed removal")
}

// An SDK shutdown closes native dispatch as well as the socket transport.
// Daemon generation cleanup owns the closed bridge's outstanding ledger.
func TestRouteViewRuntimeCloseAfterNativeSDKShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	client := routeViewNativeRuntimeClient(t, ctx, nil)
	if err := client.plugin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(ctx); err != nil {
		t.Fatalf("stopped native transport became failed cleanup: %v", err)
	}
}

// The real native publication callback must see the finite operation deadline,
// and worker shutdown must cancel it before joining and cleaning its lease.
func TestRouteViewRuntimeNativePublicationDeadlineAndStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	prefix := netip.MustParsePrefix("198.18.236.0/24")
	local := locrib.Default()
	if local == nil {
		t.Fatal("native runtime test has no daemon authority")
	}
	path := locrib.Path{Source: redistevents.RegisterProtocol("static"), RouteType: routetype.Blackhole}
	local.Insert(family.IPv4Unicast, prefix, path)
	t.Cleanup(func() { local.Remove(family.IPv4Unicast, prefix, path.Source, path.Instance) })
	entered := make(chan error, 1)
	client := routeViewNativeRuntimeClient(t, ctx, func(call context.Context, method string, value any) (any, bool, error) {
		if method != rpc.MethodRouteViewPublish {
			return nil, false, nil
		}
		input, ok := value.(routeViewPublication)
		if !ok {
			return nil, true, fmt.Errorf("native publisher received %T", value)
		}
		for _, entry := range input.Batch.Changes {
			if entry.Prefix != prefix {
				continue
			}
			deadline, finite := call.Deadline()
			switch {
			case !finite:
				entered <- errors.New("native publication has no finite deadline")
			case time.Until(deadline) > routeViewConfigurationTimeout:
				entered <- errors.New("native publication exceeds its finite budget")
			default:
				entered <- nil
			}
			<-call.Done()
			return nil, true, call.Err()
		}
		return rpc.RouteViewPublishOutput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation}, true, nil
	})
	run, stop := context.WithCancel(context.WithoutCancel(ctx))
	defer stop()
	if err := client.Start(run); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-entered:
		if err != nil {
			t.Error(err)
		}
	case <-ctx.Done():
		t.Fatal("native publication never reached the held callback")
	}
	if err := client.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.Failure(); err != nil {
		t.Fatalf("normal publication cancellation became fatal: %v", err)
	}
}

func routeViewNativeRuntimeClient(t *testing.T, ctx context.Context, native func(context.Context, string, any) (any, bool, error)) *routeViewClient {
	t.Helper()
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	p := sdk.NewWithConn("rib", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = p.Close(); _ = engineEnd.Close() })
	bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		switch method {
		case rpc.MethodRouteViewOpen:
			return json.Marshal(rpc.RouteViewOpenOutput{Session: "native-runtime", Epoch: "daemon", Snapshot: "local"})
		case rpc.MethodRouteViewDistances:
			var input rpc.RouteViewDistancesInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			return json.Marshal(rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation})
		case rpc.MethodRouteViewClose:
			return json.Marshal(rpc.RouteViewCloseOutput{Session: "native-runtime"})
		default:
			return nil, fmt.Errorf("unexpected native runtime method %q", method)
		}
	})
	if native != nil {
		bridge.SetDispatchNative(native)
	}
	bridge.SetReady()
	distances, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	permitted, err := parseFIBImportConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	client, err := openRouteViewClient(ctx, p, newSysRIB(), distances, permitted)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// A removal arriving while initial publication awaits its ACK must survive the
// scan handoff and withdraw that prefix without another producer announcement.
func TestRouteViewRuntimeLocalRemovalDuringInitialPublication(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	local := locrib.Default()
	if local == nil {
		t.Fatal("native runtime test has no daemon authority")
	}
	prefix := netip.MustParsePrefix("198.18.237.0/24")
	path := locrib.Path{Source: redistevents.RegisterProtocol("static"), RouteType: routetype.Blackhole}
	local.Insert(family.IPv4Unicast, prefix, path)
	t.Cleanup(func() { local.Remove(family.IPv4Unicast, prefix, path.Source, path.Instance) })
	initial, release, withdrawn := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var first, removal sync.Once
	client := routeViewNativeRuntimeClient(t, ctx, func(call context.Context, method string, payload any) (any, bool, error) {
		if method != rpc.MethodRouteViewPublish {
			return nil, false, nil
		}
		input, ok := payload.(routeViewPublication)
		if !ok {
			return nil, true, fmt.Errorf("native publication received %T", payload)
		}
		for _, entry := range input.Batch.Changes {
			if entry.Prefix != prefix {
				continue
			}
			if entry.Action == routeaction.Add {
				first.Do(func() { close(initial) })
				select {
				case <-release:
				case <-call.Done():
					return nil, true, call.Err()
				}
			}
			if entry.Action == routeaction.Withdraw {
				removal.Do(func() { close(withdrawn) })
			}
		}
		return rpc.RouteViewPublishOutput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation}, true, nil
	})
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer closeRuntimeFeedClient(t, ctx, client)
	select {
	case <-initial:
	case <-ctx.Done():
		t.Fatal("initial local publication never reached ACK barrier")
	}
	local.Remove(family.IPv4Unicast, prefix, path.Source, path.Instance)
	close(release)
	select {
	case <-withdrawn:
	case <-ctx.Done():
		t.Fatal("local scan handoff lost concurrent removal")
	}
}

// A wedged bounded Read cannot let commands retain an unbounded queue. Refusal
// leaves the sixteen admitted requests intact for cancellation and shutdown.
func TestRouteViewRuntimeTaskQueueBound(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	reading, release := make(chan struct{}), make(chan struct{})
	var first sync.Once
	feed := runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
		if request.Method != rpc.MethodRouteViewRead {
			return nil, false, nil
		}
		first.Do(func() {
			close(reading)
			select {
			case <-release:
			case <-ctx.Done():
			}
		})
		return nil, false, nil
	})
	feed.pages <- runtimeSnapshot(1, true)
	client := openRuntimeFeedClient(t, ctx, feed)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reading:
	case <-ctx.Done():
		t.Fatal("worker did not enter held read")
	}
	for range routeViewTasksMax {
		task := &routeViewCandidatesTask{ctx: ctx, done: make(chan routeViewCandidatesResult, 1)}
		if _, err := client.enqueue(ctx, task); err != nil {
			t.Fatalf("queue refused within its declared bound: %v", err)
		}
	}
	if _, err := client.enqueue(ctx, &routeViewCandidatesTask{ctx: ctx, done: make(chan routeViewCandidatesResult, 1)}); err == nil {
		t.Fatal("queue admitted its seventeenth retained request")
	}
	if len(client.worker.tasks) != routeViewTasksMax {
		t.Fatal("queue refusal changed admitted requests")
	}
	client.worker.cancel()
	close(release)
	if err := client.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

// Queue residence belongs to the forward deadline. Sixteen slow candidate
// requests must not each grant a later configuration a fresh two-minute wait.
func TestRouteViewRuntimeQueuedOperationsHaveDeadline(t *testing.T) {
	for _, configuration := range []bool{false, true} {
		t.Run(map[bool]string{false: "candidates", true: "configuration"}[configuration], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			reading, release := make(chan struct{}), make(chan struct{})
			var first sync.Once
			feed := runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
				if request.Method != rpc.MethodRouteViewRead {
					return nil, false, nil
				}
				first.Do(func() {
					close(reading)
					select {
					case <-release:
					case <-ctx.Done():
					}
				})
				return nil, false, nil
			})
			feed.pages <- runtimeSnapshot(1, true)
			client := openRuntimeFeedClient(t, ctx, feed)
			if err := client.Start(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case <-reading:
			case <-ctx.Done():
				t.Fatal("worker did not enter held read")
			}
			caller, stop := context.WithCancel(context.WithoutCancel(ctx))
			defer stop()
			result := make(chan error, 1)
			go func() {
				if configuration {
					result <- client.ApplyConfiguration(caller, client.configuration.distances, client.configuration.permitted)
					return
				}
				_, err := client.Candidates(caller, nil)
				result <- err
			}()
			var task routeViewTask
			select {
			case task = <-client.worker.tasks:
			case <-ctx.Done():
				t.Fatal("public request never entered the queue")
			}
			var queued context.Context
			switch value := task.(type) {
			case *routeViewCandidatesTask:
				queued = value.ctx
			case *routeViewConfigurationTask:
				queued = value.ctx
			default:
				t.Fatalf("queue received %T", task)
			}
			deadline, finite := queued.Deadline()
			if !finite {
				t.Error("queued runtime request has no finite forward deadline")
			} else if time.Until(deadline) > routeViewConfigurationTimeout {
				t.Error("queue residence exceeds the forward deadline")
			}
			stop()
			if err := task.perform(client.worker); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Errorf("canceled queued request returned %v", err)
				}
			case <-ctx.Done():
				t.Fatal("canceled public caller did not finish")
			}
			client.worker.cancel()
			close(release)
			if err := client.Close(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Cancellation can race a real publication refusal. Only cancellation and a
// stopped transport are normal shutdown; a live refusal must retain its cause.
func TestRouteViewRuntimeCancellationRetainsLivePublicationFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	run, stop := context.WithCancel(ctx)
	defer stop()
	prefix := netip.MustParsePrefix("198.18.239.0/24")
	local := locrib.Default()
	if local == nil {
		t.Fatal("native runtime test has no daemon authority")
	}
	path := locrib.Path{Source: redistevents.RegisterProtocol("static"), RouteType: routetype.Blackhole}
	local.Insert(family.IPv4Unicast, prefix, path)
	t.Cleanup(func() { local.Remove(family.IPv4Unicast, prefix, path.Source, path.Instance) })
	entered := make(chan struct{})
	cause := errors.New("fixture-live-publication-refused-during-shutdown")
	client := routeViewNativeRuntimeClient(t, ctx, func(_ context.Context, method string, value any) (any, bool, error) {
		if method != rpc.MethodRouteViewPublish {
			return nil, false, nil
		}
		input, ok := value.(routeViewPublication)
		if !ok {
			return nil, true, fmt.Errorf("native publication received %T", value)
		}
		for _, entry := range input.Batch.Changes {
			if entry.Prefix != prefix {
				continue
			}
			close(entered)
			<-run.Done()
			return nil, true, cause
		}
		return rpc.RouteViewPublishOutput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation}, true, nil
	})
	if err := client.Start(run); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("publication did not reach the controlled refusal")
	}
	stop()
	select {
	case <-client.worker.done:
	case <-ctx.Done():
		t.Fatal("worker did not join after publication refusal")
	}
	if !errors.Is(client.Failure(), cause) {
		t.Fatalf("concurrent cancellation swallowed a live publication refusal: %v", client.Failure())
	}
	if err := client.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
