// Design: docs/architecture/api/process-protocol.md -- source-policy lifecycle execution.
// Related: sdk_source_policy.go -- the source-policy worker these tests drive.
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sourcePolicyHarness drives one plugin's runtime callback loop over either
// transport: the wire MuxConn through Run, or the DirectBridge callback loop.
// engineRPC answers the plugin's engine RPCs on both transports.
type sourcePolicyHarness struct {
	p         *Plugin
	engine    *engineSide
	bridge    *rpc.DirectBridge
	bridged   bool
	engineRPC func(ctx context.Context) error
}

func newSourcePolicyHarness(t *testing.T, bridged bool) *sourcePolicyHarness {
	t.Helper()
	h := &sourcePolicyHarness{bridged: bridged}
	if bridged {
		h.p, _, h.bridge = newBridgedTestPair(t)
		return h
	}
	h.p, h.engine = newTestPair(t)
	return h
}

// start runs the plugin's loop until the test ends. Handlers MUST be registered first.
func (h *sourcePolicyHarness) start(t *testing.T, ctx context.Context) {
	t.Helper()
	loopCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	if h.bridged {
		h.bridge.SetDispatchRPC(func(ctx context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{}`), h.answerEngineRPC(ctx)
		})
		h.bridge.SetReady()
		go func() { done <- h.p.bridgeEventLoop(loopCtx) }()
	} else {
		go func() { done <- h.p.Run(loopCtx, &Registration{}) }()
		completeStartup(t, ctx, h.engine)
		go h.serveEngineRPCs(loopCtx)
	}
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func (h *sourcePolicyHarness) answerEngineRPC(ctx context.Context) error {
	if h.engineRPC == nil {
		return nil
	}
	return h.engineRPC(ctx)
}

// serveEngineRPCs answers the plugin's wire engine RPCs after startup.
func (h *sourcePolicyHarness) serveEngineRPCs(ctx context.Context) {
	for {
		select {
		case req, ok := <-h.engine.mux.Requests():
			if !ok {
				return
			}
			if err := h.answerEngineRPC(ctx); err != nil {
				h.engine.mux.SendError(ctx, req.ID, err.Error()) //nolint:errcheck // test engine
				continue
			}
			h.engine.mux.SendOK(ctx, req.ID) //nolint:errcheck // test engine
		case <-ctx.Done():
			return
		}
	}
}

// call sends one source-policy request and decodes its receipt.
func (h *sourcePolicyHarness) call(ctx context.Context, params map[string]any) (rpc.SourcePolicyOutput, error) {
	var out rpc.SourcePolicyOutput
	var raw json.RawMessage
	var err error
	if h.bridged {
		var body json.RawMessage
		body, err = json.Marshal(params)
		if err != nil {
			return out, err
		}
		raw, err = h.bridge.SendCallback(ctx, callbackSourcePolicy, body)
	} else {
		raw, err = h.engine.mux.CallRPC(ctx, callbackSourcePolicy, params)
	}
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(raw, &out)
	return out, err
}

// event delivers one event through the same callback loop as source-policy.
func (h *sourcePolicyHarness) event(ctx context.Context, event string) error {
	params := map[string]any{"event": event}
	if h.bridged {
		body, err := json.Marshal(params)
		if err != nil {
			return err
		}
		_, err = h.bridge.SendCallback(ctx, callbackDeliverEvent, body)
		return err
	}
	_, err := h.engine.mux.CallRPC(ctx, callbackDeliverEvent, params)
	return err
}

func sourcePolicyParams(peer, operation string, next uint64, deadline time.Time) map[string]any {
	return map[string]any{
		"transaction-id": "tx-" + peer, "operation": operation, "next-policy-epoch": next,
		"deadline-ms": deadline.UnixMilli(),
		"source": map[string]any{
			"peer": peer, "peer-owner": 1, "session-generation": 2, "policy-epoch": 3,
			"receive-frontier": 0, "template-generation": 0,
		},
	}
}

type sourcePolicyResult struct {
	out rpc.SourcePolicyOutput
	err error
}

func eachSourcePolicyTransport(t *testing.T, run func(t *testing.T, bridged bool)) {
	for _, bridged := range []bool{false, true} {
		t.Run(fmt.Sprintf("bridge=%t", bridged), func(t *testing.T) { run(t, bridged) })
	}
}

// TestSourcePolicyHandlerAwaitsLaterEvent proves a source-policy handler can
// wait for an event the engine delivers to the same plugin after the request.
// Method: the handler blocks until the event handler sees "settled"; the engine
// sends the request, waits for the handler to start, then delivers that event
// through the same callback transport. A handler run on the callback loop
// blocks the delivery, so the event times out and the receipt fails.
func TestSourcePolicyHandlerAwaitsLaterEvent(t *testing.T) {
	eachSourcePolicyTransport(t, func(t *testing.T, bridged bool) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		h := newSourcePolicyHarness(t, bridged)
		settled := make(chan struct{})
		var once sync.Once
		h.p.OnEvent(func(event string) error {
			if event == "settled" {
				once.Do(func() { close(settled) })
			}
			return nil
		})
		started := make(chan struct{})
		h.p.OnSourcePolicy(func(ctx context.Context, _ rpc.SourcePolicyInput) error {
			close(started)
			select {
			case <-settled:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		h.start(t, ctx)

		results := make(chan sourcePolicyResult, 1)
		go func() {
			out, err := h.call(ctx, sourcePolicyParams("a", "prepare", 0, time.Now().Add(3*time.Second)))
			results <- sourcePolicyResult{out, err}
		}()
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("handler never started")
		}
		eventCtx, eventCancel := context.WithTimeout(ctx, time.Second)
		defer eventCancel()
		if err := h.event(eventCtx, "settled"); err != nil {
			t.Fatalf("event behind a running source-policy handler was not applied: %v", err)
		}
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.out.Status != rpc.StatusOK {
			t.Fatalf("receipt failed: %+v", result.out)
		}
	})
}

// TestSourcePolicyHandlerEngineRPC proves a handler can make an engine RPC
// whose answer depends on the plugin's callback loop. Method: the test engine
// answers the RPC only after it has delivered an event to the same plugin and
// seen it applied, as an engine whose command re-enters the owner does. A
// handler run on the callback loop deadlocks until its deadline.
func TestSourcePolicyHandlerEngineRPC(t *testing.T) {
	eachSourcePolicyTransport(t, func(t *testing.T, bridged bool) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		h := newSourcePolicyHarness(t, bridged)
		var applied atomic.Int64
		h.p.OnEvent(func(string) error {
			applied.Add(1)
			return nil
		})
		h.engineRPC = func(ctx context.Context) error {
			eventCtx, eventCancel := context.WithTimeout(ctx, 2*time.Second)
			defer eventCancel()
			return h.event(eventCtx, "re-entered")
		}
		h.p.OnSourcePolicy(func(ctx context.Context, _ rpc.SourcePolicyInput) error {
			_, err := h.p.callEngineRaw(ctx, "ze-plugin-engine:dispatch-command", map[string]any{"command": "rib show"})
			return err
		})
		h.start(t, ctx)

		for _, step := range []struct {
			operation string
			next      uint64
		}{{"prepare", 0}, {"apply", 4}} {
			out, err := h.call(ctx, sourcePolicyParams("a", step.operation, step.next, time.Now().Add(4*time.Second)))
			if err != nil {
				t.Fatalf("%s: %v", step.operation, err)
			}
			if out.Status != rpc.StatusOK {
				t.Fatalf("%s: engine RPC from the handler failed: %+v", step.operation, out)
			}
		}
		if applied.Load() != 2 {
			t.Fatalf("re-entering events applied = %d, want 2", applied.Load())
		}
	})
}

// TestSourcePolicyQueueFullRefusesPromptly proves a full source-policy queue
// refuses at once with an error receipt and never blocks the callback loop.
// Method: hold the first handler, then send queue depth plus one more
// requests. Exactly one is refused while the first handler is still held;
// after release every queued request is served. A handler run on the loop
// refuses nothing and the test times out.
func TestSourcePolicyQueueFullRefusesPromptly(t *testing.T) {
	eachSourcePolicyTransport(t, func(t *testing.T, bridged bool) {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		h := newSourcePolicyHarness(t, bridged)
		started := make(chan struct{})
		release := make(chan struct{})
		var calls atomic.Int64
		h.p.OnSourcePolicy(func(ctx context.Context, input rpc.SourcePolicyInput) error {
			if input.Source.Peer != "held" {
				calls.Add(1)
				return nil
			}
			close(started)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		h.start(t, ctx)

		deadline := time.Now().Add(20 * time.Second)
		held := make(chan sourcePolicyResult, 1)
		go func() {
			out, err := h.call(ctx, sourcePolicyParams("held", "prepare", 0, deadline))
			held <- sourcePolicyResult{out, err}
		}()
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("handler never started")
		}
		queued := make(chan sourcePolicyResult, sourcePolicyQueueDepth+1)
		for i := range sourcePolicyQueueDepth + 1 {
			go func() {
				out, err := h.call(ctx, sourcePolicyParams(fmt.Sprintf("q%d", i), "prepare", 0, deadline))
				queued <- sourcePolicyResult{out, err}
			}()
		}
		select {
		case refused := <-queued:
			if refused.err != nil {
				t.Fatalf("queue-full answered as a callback error, want an error receipt: %v", refused.err)
			}
			if refused.out.Status != rpc.StatusError || !strings.Contains(refused.out.Error, "queue full") {
				t.Fatalf("first answer while held is not a queue-full receipt: %+v", refused.out)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("queue-full request was not refused while the handler was held")
		}
		close(release)
		for range sourcePolicyQueueDepth {
			result := <-queued
			if result.err != nil || result.out.Status != rpc.StatusOK {
				t.Fatalf("queued request failed: %+v %v", result.out, result.err)
			}
		}
		if result := <-held; result.err != nil || result.out.Status != rpc.StatusOK {
			t.Fatalf("held request failed: %+v %v", result.out, result.err)
		}
		if calls.Load() != sourcePolicyQueueDepth {
			t.Fatalf("handler ran %d queued requests, want %d", calls.Load(), sourcePolicyQueueDepth)
		}
	})
}

// TestSourcePolicyDeadlineEndsHandler proves the request's absolute deadline
// bounds the handler. Method: a handler that waits only for its context must
// see DeadlineExceeded and the receipt must fail; a request whose deadline
// passes while it waits behind another handler must fail without its handler
// ever running.
func TestSourcePolicyDeadlineEndsHandler(t *testing.T) {
	eachSourcePolicyTransport(t, func(t *testing.T, bridged bool) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		h := newSourcePolicyHarness(t, bridged)
		ended := make(chan error, 1)
		started := make(chan struct{})
		release := make(chan struct{})
		var late atomic.Int64
		h.p.OnSourcePolicy(func(ctx context.Context, input rpc.SourcePolicyInput) error {
			switch input.Source.Peer {
			case "expire":
				<-ctx.Done()
				ended <- ctx.Err()
				return ctx.Err()
			case "held":
				close(started)
				<-release
				return nil
			}
			late.Add(1)
			return nil
		})
		h.start(t, ctx)

		begin := time.Now()
		out, err := h.call(ctx, sourcePolicyParams("expire", "prepare", 0, begin.Add(300*time.Millisecond)))
		if err != nil {
			t.Fatal(err)
		}
		if out.Status != rpc.StatusError {
			t.Fatalf("expired handler acknowledged: %+v", out)
		}
		if elapsed := time.Since(begin); elapsed > 3*time.Second {
			t.Fatalf("receipt took %s after a 300ms deadline", elapsed)
		}
		if handlerErr := <-ended; !errors.Is(handlerErr, context.DeadlineExceeded) {
			t.Fatalf("handler context ended with %v, want deadline exceeded", handlerErr)
		}

		held := make(chan sourcePolicyResult, 1)
		go func() {
			out, err := h.call(ctx, sourcePolicyParams("held", "prepare", 0, time.Now().Add(10*time.Second)))
			held <- sourcePolicyResult{out, err}
		}()
		<-started
		waiting := make(chan sourcePolicyResult, 1)
		go func() {
			out, err := h.call(ctx, sourcePolicyParams("late", "prepare", 0, time.Now().Add(300*time.Millisecond)))
			waiting <- sourcePolicyResult{out, err}
		}()
		time.Sleep(600 * time.Millisecond)
		close(release)
		result := <-waiting
		if result.err == nil && result.out.Status == rpc.StatusOK {
			t.Fatalf("request that expired in the queue was acknowledged: %+v", result.out)
		}
		if late.Load() != 0 {
			t.Fatal("handler ran for a request that expired in the queue")
		}
		if result := <-held; result.err != nil || result.out.Status != rpc.StatusOK {
			t.Fatalf("held request failed: %+v %v", result.out, result.err)
		}
	})
}

// TestSourcePolicyPerSourceOrder proves the off-loop handler sees every event
// applied before its request and never overlaps itself. Method: deliver five
// events, then prepare; the handler must observe all five. Then two sources
// each run prepare, apply and finalize while events keep arriving; each
// source's operations must reach the handler in send order, one at a time.
func TestSourcePolicyPerSourceOrder(t *testing.T) {
	eachSourcePolicyTransport(t, func(t *testing.T, bridged bool) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		h := newSourcePolicyHarness(t, bridged)
		var applied atomic.Int64
		h.p.OnEvent(func(string) error {
			applied.Add(1)
			return nil
		})
		var mu sync.Mutex
		seen := map[string][]rpc.SourcePolicyOperation{}
		var active atomic.Int64
		var overlap atomic.Bool
		var observed atomic.Int64
		h.p.OnSourcePolicy(func(_ context.Context, input rpc.SourcePolicyInput) error {
			if active.Add(1) > 1 {
				overlap.Store(true)
			}
			defer active.Add(-1)
			if input.Source.Peer == "first" {
				observed.Store(applied.Load())
			}
			mu.Lock()
			seen[input.Source.Peer] = append(seen[input.Source.Peer], input.Operation)
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			return nil
		})
		h.start(t, ctx)

		for i := range 5 {
			if err := h.event(ctx, fmt.Sprintf("before-%d", i)); err != nil {
				t.Fatal(err)
			}
		}
		if out, err := h.call(ctx, sourcePolicyParams("first", "prepare", 0, time.Now().Add(5*time.Second))); err != nil || out.Status != rpc.StatusOK {
			t.Fatalf("prepare failed: %+v %v", out, err)
		}
		if observed.Load() != 5 {
			t.Fatalf("handler observed %d applied events, want the 5 delivered before it", observed.Load())
		}

		var wg sync.WaitGroup
		failures := make(chan error, 8)
		for _, peer := range []string{"a", "b"} {
			wg.Go(func() {
				for _, step := range []struct {
					operation string
					next      uint64
				}{{"prepare", 0}, {"apply", 4}, {"finalize", 0}} {
					out, err := h.call(ctx, sourcePolicyParams(peer, step.operation, step.next, time.Now().Add(5*time.Second)))
					if err != nil || out.Status != rpc.StatusOK {
						failures <- fmt.Errorf("%s %s: %+v %w", peer, step.operation, out, err)
						return
					}
				}
			})
		}
		for i := range 10 {
			if err := h.event(ctx, fmt.Sprintf("during-%d", i)); err != nil {
				t.Fatal(err)
			}
		}
		wg.Wait()
		close(failures)
		for err := range failures {
			t.Fatal(err)
		}
		if overlap.Load() {
			t.Fatal("source-policy handler invocations overlapped")
		}
		want := []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply, rpc.SourcePolicyFinalize}
		for _, peer := range []string{"a", "b"} {
			if got := seen[peer]; !slices.Equal(got, want) {
				t.Fatalf("source %s operations = %v, want %v", peer, got, want)
			}
		}
	})
}

// TestSourcePolicyWorkerArrivalOrder proves the worker serves requests in the
// order the callback loop admitted them. Method: admit operations for two
// sources, interleaved, before the worker starts, then record the order the
// handler sees. Both loops call admit in arrival order, so this is the order
// the engine's requests reach the owner on either transport.
func TestSourcePolicyWorkerArrivalOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var mu sync.Mutex
	var seen []string
	worker := newSourcePolicyWorker("order", func(_ context.Context, input rpc.SourcePolicyInput) error {
		mu.Lock()
		seen = append(seen, fmt.Sprint(input.Source.Peer, " ", input.Operation))
		mu.Unlock()
		return nil
	}, func(error) {})
	steps := []struct {
		peer      string
		operation string
		next      uint64
	}{{"a", "prepare", 0}, {"b", "prepare", 0}, {"a", "apply", 4}, {"b", "abort", 5}, {"a", "finalize", 0}}
	answers := make(chan error, len(steps))
	for _, step := range steps {
		params, err := json.Marshal(sourcePolicyParams(step.peer, step.operation, step.next, time.Now().Add(5*time.Second)))
		if err != nil {
			t.Fatal(err)
		}
		if err := worker.admit(params, func(_ time.Time, _ json.RawMessage, err error) { answers <- err }); err != nil {
			t.Fatal(err)
		}
	}
	runCtx, stop := context.WithCancel(ctx)
	go worker.run(runCtx)
	for range steps {
		if err := <-answers; err != nil {
			t.Fatal(err)
		}
	}
	stop()
	<-worker.done
	want := []string{
		fmt.Sprint("a ", rpc.SourcePolicyPrepare), fmt.Sprint("b ", rpc.SourcePolicyPrepare),
		fmt.Sprint("a ", rpc.SourcePolicyApply), fmt.Sprint("b ", rpc.SourcePolicyAbort),
		fmt.Sprint("a ", rpc.SourcePolicyFinalize),
	}
	if fmt.Sprint(seen) != fmt.Sprint(want) {
		t.Fatalf("served %v, want %v", seen, want)
	}
}

// TestSourcePolicyWorkerStopAnswersQueued proves stopping the worker ends the
// running handler's context and answers every queued request with a failed
// receipt, so no engine caller waits on a request that will never run.
// Method: hold one handler, queue a second request, cancel the worker, then
// read both answers.
func TestSourcePolicyWorkerStopAnswersQueued(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	started := make(chan struct{})
	var runs atomic.Int64
	worker := newSourcePolicyWorker("stop", func(ctx context.Context, _ rpc.SourcePolicyInput) error {
		if runs.Add(1) == 1 {
			close(started)
		}
		<-ctx.Done()
		return ctx.Err()
	}, func(error) {})
	answers := make(chan rpc.SourcePolicyOutput, 2)
	for _, peer := range []string{"running", "queued"} {
		params, err := json.Marshal(sourcePolicyParams(peer, "prepare", 0, time.Now().Add(5*time.Second)))
		if err != nil {
			t.Fatal(err)
		}
		if err := worker.admit(params, func(_ time.Time, receipt json.RawMessage, err error) {
			var out rpc.SourcePolicyOutput
			if err == nil {
				err = json.Unmarshal(receipt, &out)
			}
			if err != nil {
				out.Error = err.Error()
			}
			answers <- out
		}); err != nil {
			t.Fatal(err)
		}
	}
	runCtx, stop := context.WithCancel(ctx)
	go worker.run(runCtx)
	<-started
	stop()
	<-worker.done
	for range 2 {
		out := <-answers
		if out.Status != rpc.StatusError {
			t.Fatalf("request for %s acknowledged after stop: %+v", out.Source.Peer, out)
		}
	}
	if runs.Load() != 1 {
		t.Fatalf("handler ran %d times, want only the running request", runs.Load())
	}
}

// TestSourcePolicyHandlerPanicFailsConnection proves a panicking handler is
// answered with an error and ends the plugin's loop with that failure on both
// transports, instead of crashing the daemon from the worker goroutine or
// reading as a clean shutdown. Method: register a handler that panics, send
// one request, then check the answer and the loop's return value.
func TestSourcePolicyHandlerPanicFailsConnection(t *testing.T) {
	eachSourcePolicyTransport(t, func(t *testing.T, bridged bool) {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		h := newSourcePolicyHarness(t, bridged)
		h.p.OnSourcePolicy(func(context.Context, rpc.SourcePolicyInput) error {
			panic("owner boom")
		})
		done := make(chan error, 1)
		if bridged {
			go func() { done <- h.p.bridgeEventLoop(ctx) }()
		} else {
			go func() { done <- h.p.Run(ctx, &Registration{}) }()
			completeStartup(t, ctx, h.engine)
		}
		if out, err := h.call(ctx, sourcePolicyParams("a", "prepare", 0, time.Now().Add(5*time.Second))); err == nil {
			t.Fatalf("panicking handler acknowledged: %+v", out)
		}
		select {
		case err := <-done:
			if err == nil || !strings.Contains(err.Error(), "source-policy handler panic") {
				t.Fatalf("loop ended with %v, want the handler panic", err)
			}
		case <-ctx.Done():
			t.Fatal("loop did not end after the handler panic")
		}
	})
}

// TestSourcePolicyPanicRefusesQueued proves a handler panic stops the worker:
// a request already queued behind the panicking one never reaches the handler,
// whose owner state is now unknown, and gets a failed receipt before the
// connection ends. Method: hold a handler that will panic, queue a second
// request behind it, release the panic, then check the handler ran once and
// the second request failed.
func TestSourcePolicyPanicRefusesQueued(t *testing.T) {
	eachSourcePolicyTransport(t, func(t *testing.T, bridged bool) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		h := newSourcePolicyHarness(t, bridged)
		started := make(chan struct{})
		release := make(chan struct{})
		var runs atomic.Int64
		h.p.OnSourcePolicy(func(_ context.Context, input rpc.SourcePolicyInput) error {
			runs.Add(1)
			if input.Source.Peer == "boom" {
				close(started)
				<-release
				panic("owner boom")
			}
			return nil
		})
		h.start(t, ctx)

		first := make(chan sourcePolicyResult, 1)
		go func() {
			out, err := h.call(ctx, sourcePolicyParams("boom", "prepare", 0, time.Now().Add(10*time.Second)))
			first <- sourcePolicyResult{out, err}
		}()
		<-started
		queued := make(chan sourcePolicyResult, 1)
		go func() {
			out, err := h.call(ctx, sourcePolicyParams("behind", "prepare", 0, time.Now().Add(10*time.Second)))
			queued <- sourcePolicyResult{out, err}
		}()
		time.Sleep(200 * time.Millisecond)
		close(release)
		if result := <-first; result.err == nil {
			t.Fatalf("panicking request acknowledged: %+v", result.out)
		}
		result := <-queued
		if result.err == nil && result.out.Status == rpc.StatusOK {
			t.Fatalf("request queued behind a panic acknowledged: %+v", result.out)
		}
		if runs.Load() != 1 {
			t.Fatalf("handler ran %d times, want once: nothing may run after a panic", runs.Load())
		}
	})
}

// TestSourcePolicyStopReceiptsReachEngine proves that when the plugin's loop
// context is canceled, the failed receipts for the running request and for a
// queued one still reach the engine over the wire. Method: hold a handler on
// its context, queue a second request, cancel only the loop's context, and
// require both receipts on the engine side. Sending with the canceled loop
// context writes nothing, and both calls time out.
func TestSourcePolicyStopReceiptsReachEngine(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	h := newSourcePolicyHarness(t, false)
	started := make(chan struct{})
	h.p.OnSourcePolicy(func(ctx context.Context, input rpc.SourcePolicyInput) error {
		if input.Source.Peer == "running" {
			close(started)
		}
		<-ctx.Done()
		return ctx.Err()
	})
	loopCtx, stopLoop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- h.p.Run(loopCtx, &Registration{}) }()
	completeStartup(t, ctx, h.engine)

	results := make(chan sourcePolicyResult, 2)
	go func() {
		out, err := h.call(ctx, sourcePolicyParams("running", "prepare", 0, time.Now().Add(10*time.Second)))
		results <- sourcePolicyResult{out, err}
	}()
	<-started
	go func() {
		out, err := h.call(ctx, sourcePolicyParams("queued", "prepare", 0, time.Now().Add(10*time.Second)))
		results <- sourcePolicyResult{out, err}
	}()
	time.Sleep(200 * time.Millisecond)
	stopLoop()
	<-done
	for range 2 {
		select {
		case result := <-results:
			if result.err != nil {
				t.Fatalf("receipt did not reach the engine: %v", result.err)
			}
			if result.out.Status != rpc.StatusError {
				t.Fatalf("request at stop acknowledged: %+v", result.out)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no receipt reached the engine after the loop stopped")
		}
	}
}

// TestDrainAnswerReachesEngineAfterLoopStop proves the drain worker's answer
// still reaches the engine over the wire when the plugin's loop context is
// canceled while the drain handler runs. Method: hold the drain handler on
// its context, cancel the loop's context, and require the engine call to end
// with the handler's error answer rather than its own timeout.
func TestDrainAnswerReachesEngineAfterLoopStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	p, engine := newTestPair(t)
	started := make(chan struct{})
	p.OnDrain(func(ctx context.Context, _ rpc.DrainInput) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	loopCtx, stopLoop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- p.Run(loopCtx, &Registration{}) }()
	completeStartup(t, ctx, engine)

	answered := make(chan error, 1)
	go func() {
		callCtx, callCancel := context.WithTimeout(ctx, 5*time.Second)
		defer callCancel()
		_, err := engine.mux.CallRPC(callCtx, rpc.MethodDrain, drainParams(t, rpc.DrainKindStop))
		answered <- err
	}()
	<-started
	stopLoop()
	<-done
	err := <-answered
	if err == nil {
		t.Fatal("drain cut short by the loop stop answered success")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("drain answer never reached the engine: %v", err)
	}
}
