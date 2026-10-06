package process

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

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// appliedDeliveryOwner applies sequence updates through the actual transport
// handler. A rejected update leaves the observable projection unchanged.
type appliedDeliveryOwner struct {
	sequence atomic.Int64
	reject   int64
}

func (o *appliedDeliveryOwner) apply(events []string) error {
	for _, event := range events {
		var update struct {
			Sequence int64 `json:"sequence"`
		}
		if err := json.Unmarshal([]byte(event), &update); err != nil {
			return err
		}
		if update.Sequence == o.reject {
			return errors.New("projection rejected sequence")
		}
		o.sequence.Store(update.Sequence)
	}
	return nil
}

func newAppliedDeliveryProcess(t *testing.T, external bool, owner *appliedDeliveryOwner) *Process {
	t.Helper()
	proc := NewProcess(plugin.PluginConfig{Name: t.Name(), Encoder: "json"})
	if external {
		// startInternal allocates a bridge even when the runner retains JSON IPC.
		// Never activate it: these tests must exercise the real socket transport.
		proc.bridge = rpc.NewDirectBridge()
		engineEnd, pluginEnd := net.Pipe()
		engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
		pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
		proc.SetConn(ipc.NewMuxPluginConn(engineMux))
		exited := make(chan struct{})
		go func() {
			defer close(exited)
			for req := range pluginMux.Requests() {
				if req.Method != "ze-plugin-callback:deliver-batch" {
					t.Errorf("unexpected method %q", req.Method)
					return
				}
				var input struct {
					Events []string `json:"events"`
				}
				if err := json.Unmarshal(req.Params, &input); err != nil {
					t.Errorf("decode batch: %v", err)
					return
				}
				var err error
				if applyErr := owner.apply(input.Events); applyErr != nil {
					err = pluginMux.SendError(t.Context(), req.ID, applyErr.Error())
				} else {
					err = pluginMux.SendOK(t.Context(), req.ID)
				}
				if err != nil {
					t.Errorf("answer delivery: %v", err)
					return
				}
			}
		}()
		t.Cleanup(func() {
			pluginMux.Close() //nolint:errcheck // Test transport teardown.
			<-exited
		})
	} else {
		proc.bridge = rpc.NewDirectBridge()
		proc.bridge.SetDeliverEvents(owner.apply)
		proc.bridge.SetReady()
	}
	proc.StartDelivery(t.Context())
	t.Cleanup(func() {
		proc.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proc.Wait(ctx); err != nil {
			t.Errorf("wait for delivery worker: %v", err)
		}
	})
	return proc
}

// TestDrainEventsAppliedRetainsPriorFailure separates the failed fire-and-forget
// batch from a later successful batch and barrier, on both delivery transports.
// Dropping the sticky failure assignment makes the strict assertion return nil.
func TestDrainEventsAppliedRetainsPriorFailure(t *testing.T) {
	for _, external := range []bool{false, true} {
		name := "direct"
		if external {
			name = "external"
		}
		t.Run(name, func(t *testing.T) {
			owner := &appliedDeliveryOwner{reject: 1}
			proc := newAppliedDeliveryProcess(t, external, owner)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			failed := make(chan struct{})
			if !proc.Deliver(EventDelivery{Output: `{"sequence":1}`, OnFailure: func() { close(failed) }}) {
				t.Fatal("failed batch was not admitted")
			}
			select {
			case <-failed:
			case <-ctx.Done():
				t.Fatal("failed batch did not complete")
			}
			if got := owner.sequence.Load(); got != 0 {
				t.Fatalf("rejected projection changed owner to %d", got)
			}

			clean := make(chan EventResult, 1)
			if !proc.Deliver(EventDelivery{Output: `{"sequence":2}`, Result: clean}) {
				t.Fatal("clean batch was not admitted")
			}
			select {
			case receipt := <-clean:
				if receipt.Err != nil {
					t.Fatalf("clean batch inherited prior failure: %v", receipt.Err)
				}
			case <-ctx.Done():
				t.Fatal("clean batch did not complete")
			}
			if got := owner.sequence.Load(); got != 2 {
				t.Fatalf("clean batch did not apply: %d", got)
			}
			for range 2 {
				if err := proc.DrainEventsApplied(ctx); err == nil || !strings.Contains(err.Error(), "projection rejected sequence") {
					t.Fatalf("strict drain forgot earlier rejected projection: %v", err)
				}
			}
			if err := proc.DrainEvents(ctx); err != nil {
				t.Fatalf("soft quiesce must ignore application failure: %v", err)
			}
		})
	}
}

// TestDrainEventsAppliedSuccess is the clean control: the same actual paths
// apply every queued update before authorizing a read of the projection.
func TestDrainEventsAppliedSuccess(t *testing.T) {
	for _, external := range []bool{false, true} {
		name := "direct"
		if external {
			name = "external"
		}
		t.Run(name, func(t *testing.T) {
			owner := &appliedDeliveryOwner{}
			proc := newAppliedDeliveryProcess(t, external, owner)
			for _, event := range []string{`{"sequence":1}`, `{"sequence":2}`, `{"sequence":3}`} {
				if !proc.Deliver(EventDelivery{Output: event}) {
					t.Fatal("clean event was not admitted")
				}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if err := proc.DrainEventsApplied(ctx); err != nil {
				t.Fatalf("clean strict drain: %v", err)
			}
			if external && (proc.bridge == nil || proc.bridge.Ready() || proc.bridge.Activated()) {
				t.Fatal("JSON receipt required activating the unused bridge")
			}
			if got := owner.sequence.Load(); got != 3 {
				t.Fatalf("strict drain preceded application: got sequence %d", got)
			}
		})
	}
}

// TestDrainEventsAppliedRejectsStopped checks queue, context, bridge and engine
// lifecycle rejection even when no failed event populated the sticky error.
func TestDrainEventsAppliedRejectsStopped(t *testing.T) {
	for _, stop := range []struct {
		name string
		stop func(*Process)
	}{
		{"closed-queue", func(p *Process) { p.stopEventChan() }},
		{"canceled-process", func(p *Process) { p.cancel() }},
		{"delivery-stopped", func(p *Process) { p.bridge.StopDelivery() }},
		{"dispatch-stopped", func(p *Process) { p.bridge.StopDispatch() }},
		{"engine-exited", func(p *Process) { close(p.engineDone) }},
		{"process-stopped", func(p *Process) { p.Stop() }},
	} {
		t.Run(stop.name, func(t *testing.T) {
			proc := newAppliedDeliveryProcess(t, false, &appliedDeliveryOwner{})
			// EngineDone is published before the strict call; the worker never reads it.
			proc.engineDone = make(chan struct{})
			stop.stop(proc)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if err := proc.DrainEventsApplied(ctx); err == nil {
				t.Fatal("strict drain authorized stopped owner")
			}
			if err := proc.DrainEvents(ctx); err != nil {
				t.Fatalf("soft quiesce rejected stopped owner: %v", err)
			}
		})
	}
	t.Run("unstarted", func(t *testing.T) {
		proc := NewProcess(plugin.PluginConfig{Name: t.Name()})
		if err := proc.DrainEventsApplied(t.Context()); !errors.Is(err, ErrConnectionClosed) {
			t.Fatalf("unstarted process: %v", err)
		}
	})
}

// TestDrainEventsAppliedRejectsActivatedBridgeWithLiveIPC prevents a readiness
// check from falling back to healthy startup IPC after the bridge stops.
func TestDrainEventsAppliedRejectsActivatedBridgeWithLiveIPC(t *testing.T) {
	for _, stop := range []struct {
		name string
		stop func(*rpc.DirectBridge)
	}{
		{"delivery-stopped", (*rpc.DirectBridge).StopDelivery},
		{"dispatch-stopped", (*rpc.DirectBridge).StopDispatch},
		{"callbacks-closed", (*rpc.DirectBridge).CloseCallbacks},
		{"callbacks-failed", func(b *rpc.DirectBridge) { b.FailCallbacks(errors.New("callback failure")) }},
	} {
		t.Run(stop.name, func(t *testing.T) {
			proc := newAppliedDeliveryProcess(t, true, &appliedDeliveryOwner{})
			proc.bridge.SetDeliverEvents(func([]string) error {
				t.Error("strict drain invoked a delivery probe")
				return nil
			})
			proc.bridge.SetReady()
			stop.stop(proc.bridge)
			if err := proc.Conn().Err(); err != nil {
				t.Fatalf("startup IPC must remain healthy: %v", err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if err := proc.DrainEventsApplied(ctx); err == nil {
				t.Fatal("strict drain fell back to IPC after activated bridge stopped")
			}
			if err := proc.DrainEvents(ctx); err != nil {
				t.Fatalf("soft quiesce rejected stopped bridge: %v", err)
			}
		})
	}
}

// TestDrainEventsAppliedCancellation holds the real handler and fills the queue
// to cover both admission cancellation and cancellation of an admitted receipt.
func TestDrainEventsAppliedCancellation(t *testing.T) {
	for _, full := range []bool{false, true} {
		name := "receipt"
		if full {
			name = "full-queue"
		}
		t.Run(name, func(t *testing.T) {
			proc := NewProcess(plugin.PluginConfig{Name: t.Name()})
			entered := make(chan struct{})
			release := make(chan struct{})
			var first sync.Once
			proc.bridge = rpc.NewDirectBridge()
			proc.bridge.SetDeliverEvents(func([]string) error {
				first.Do(func() { close(entered); <-release })
				return nil
			})
			proc.bridge.SetReady()
			proc.StartDelivery(t.Context())
			t.Cleanup(func() {
				close(release)
				proc.Stop()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := proc.Wait(ctx); err != nil {
					t.Errorf("wait: %v", err)
				}
			})
			if !proc.Deliver(EventDelivery{Output: "held"}) {
				t.Fatal("held event not admitted")
			}
			select {
			case <-entered:
			case <-t.Context().Done():
				t.Fatal("handler did not start")
			}
			if full {
				for range eventDeliveryCapacity {
					if !proc.Deliver(EventDelivery{Output: "queued"}) {
						t.Fatal("queue fill rejected")
					}
				}
			}
			// The handler cannot finish. Expiry must bound either waiting point.
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
			defer cancel()
			if err := proc.DrainEventsApplied(ctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("strict drain ignored deadline: %v", err)
			}
		})
	}
}

// TestDrainEventsAppliedCanceledContext rejects a canceled caller even when an
// empty, healthy queue could answer immediately.
func TestDrainEventsAppliedCanceledContext(t *testing.T) {
	proc := newAppliedDeliveryProcess(t, false, &appliedDeliveryOwner{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := proc.DrainEventsApplied(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled strict drain: %v", err)
	}
}

// TestDrainEventsAppliedRetainsTimeout leaves a real external batch unanswered.
// A later barrier has no IPC to fail; only the retained earlier timeout can
// prevent it from authorizing a stale projection.
func TestDrainEventsAppliedRetainsTimeout(t *testing.T) {
	engineEnd, pluginEnd := net.Pipe()
	engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
	defer pluginMux.Close() //nolint:errcheck // Test transport teardown.
	proc := NewProcess(plugin.PluginConfig{Name: t.Name(), Encoder: "json"})
	proc.SetConn(ipc.NewMuxPluginConn(engineMux))
	proc.StartDelivery(t.Context())
	defer func() {
		proc.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proc.Wait(ctx); err != nil {
			t.Errorf("wait: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 2*deliveryTimeoutFromEnv()+5*time.Second)
	defer cancel()
	failed := make(chan struct{})
	if !proc.Deliver(EventDelivery{Output: `{"sequence":1}`, OnFailure: func() { close(failed) }}) {
		t.Fatal("timeout batch was not admitted")
	}
	select {
	case req := <-pluginMux.Requests():
		if req.Method != "ze-plugin-callback:deliver-batch" {
			t.Fatalf("unexpected request: %s", req.Method)
		}
		// No application and no acknowledgement: the actual sender must time out.
	case <-ctx.Done():
		t.Fatal("external delivery did not arrive")
	}
	select {
	case <-failed:
	case <-ctx.Done():
		t.Fatal("unanswered delivery did not time out")
	}
	if err := proc.DrainEventsApplied(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("strict drain forgot timed-out delivery: %v", err)
	}
	if err := proc.DrainEvents(ctx); err != nil {
		t.Fatalf("soft drain rejected prior timeout: %v", err)
	}
}

// TestDrainEventsAppliedRetainsStructuredPanic exercises the structured bridge
// producer's panic recovery instead of assigning a receipt or sticky error.
func TestDrainEventsAppliedRetainsStructuredPanic(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: t.Name()})
	proc.bridge = rpc.NewDirectBridge()
	proc.bridge.SetDeliverStructured(func([]any) error {
		panic("BUG: deliberate structured delivery test panic")
	})
	proc.bridge.SetReady()
	proc.StartDelivery(t.Context())
	defer func() {
		proc.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proc.Wait(ctx); err != nil {
			t.Errorf("wait: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	failed := make(chan struct{})
	if !proc.Deliver(EventDelivery{Event: int64(1), OnFailure: func() { close(failed) }}) {
		t.Fatal("structured event was not admitted")
	}
	select {
	case <-failed:
	case <-ctx.Done():
		t.Fatal("structured handler panic did not complete")
	}
	if err := proc.DrainEventsApplied(ctx); err == nil || !strings.Contains(err.Error(), "deliberate structured delivery test panic") {
		t.Fatalf("strict drain forgot structured handler panic: %v", err)
	}
}

// TestDrainEventsAppliedRejectsClosedTransport checks the authoritative mux
// lifecycle, not a missing Process connection pointer or a failed event.
func TestDrainEventsAppliedRejectsClosedTransport(t *testing.T) {
	engineEnd, pluginEnd := net.Pipe()
	engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	proc := NewProcess(plugin.PluginConfig{Name: t.Name(), Encoder: "json"})
	proc.bridge = rpc.NewDirectBridge()
	proc.SetConn(ipc.NewMuxPluginConn(engineMux))
	proc.StartDelivery(t.Context())
	defer func() {
		proc.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proc.Wait(ctx); err != nil {
			t.Errorf("wait: %v", err)
		}
	}()
	if err := pluginEnd.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	select {
	case <-engineMux.Done():
	case <-ctx.Done():
		t.Fatal("mux did not observe remote close")
	}
	if err := proc.DrainEventsApplied(ctx); !errors.Is(err, rpc.ErrMuxConnClosed) {
		t.Fatalf("strict drain authorized closed transport: %v", err)
	}
}

// TestDrainEventsAppliedOwnerStopsWhileWaiting admits the strict barrier behind
// a held handler, then stops the process. Shutdown must fail the wait even
// before the handler can return its otherwise successful receipt.
func TestDrainEventsAppliedOwnerStopsWhileWaiting(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: t.Name()})
	entered := make(chan struct{})
	release := make(chan struct{})
	barrierAdmitted := make(chan struct{})
	var admissions atomic.Int64
	proc.deliveryInc = func() {
		if admissions.Add(1) == 2 {
			close(barrierAdmitted)
		}
	}
	proc.bridge = rpc.NewDirectBridge()
	proc.bridge.SetDeliverEvents(func([]string) error {
		close(entered)
		<-release
		return nil
	})
	proc.bridge.SetReady()
	proc.StartDelivery(t.Context())
	defer func() {
		close(release)
		proc.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proc.Wait(ctx); err != nil {
			t.Errorf("wait: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if !proc.Deliver(EventDelivery{Output: "held"}) {
		t.Fatal("held event not admitted")
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("handler did not start")
	}
	drained := make(chan error, 1)
	go func() { drained <- proc.DrainEventsApplied(ctx) }()
	select {
	case <-barrierAdmitted:
	case <-ctx.Done():
		t.Fatal("strict barrier did not enter queue")
	}
	proc.Stop()
	select {
	case err := <-drained:
		if !errors.Is(err, ErrConnectionClosed) {
			t.Fatalf("stopped owner authorized pending drain: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("strict drain waited for handler after owner stopped")
	}
}
