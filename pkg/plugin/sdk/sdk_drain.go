// Design: docs/architecture/api/process-protocol.md — the stop drain RPC
// Overview: sdk_dispatch.go — event loop and callback dispatch
// Related: sdk_callbacks.go — the other engine-to-plugin callbacks

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// DrainHandler drains the plugin's protocol before the daemon stops: it makes
// the protocol unattractive to its neighbors, waits its configured time, and
// returns. It runs on the connection's drain worker, never on the event loop,
// so the plugin keeps answering commands while it waits. It MUST return when
// ctx ends, which happens when the connection closes, and SHOULD return by
// input.Deadline, after which the engine no longer reads the answer. A
// returned error is the drain's failure, logged by the engine by plugin name.
type DrainHandler func(ctx context.Context, input rpc.DrainInput) error

// OnDrain sets the handler for the stop drain (rpc.MethodDrain). Registering
// it makes Run declare DrainHandler at Stage 1; the plugin MUST also declare a
// DrainBudget in its Registration, or the engine refuses the plugin.
func (p *Plugin) OnDrain(fn DrainHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onDrain = fn
}

// drainQueueDepth is the drain worker's request channel capacity. One is
// enough because the event loop admits a single stop-kind drain per
// connection and refuses every later one before queueing it, so the send
// never finds the channel full.
const drainQueueDepth = 1

// drainRequest is one admitted drain: its decoded input and the answer the
// worker sends for the request's own ID once the handler returns.
type drainRequest struct {
	input  rpc.DrainInput
	answer func(err error)
}

// drainWorker runs the plugin's drain handler off the event loop. It is one
// long-lived goroutine per connection: newDrainWorker builds it, the event
// loop starts run once, queues admitted requests with admit, and when the
// connection's loop ends it cancels run's context and waits for done. run
// serves requests in arrival order, one at a time, to completion.
//
// The state has two values, running and stopping, held in one atomic shared
// by the event loop and the worker. The event loop moves it to stopping when
// it admits a stop-kind drain and refuses a stop-kind drain that finds it
// stopping, so a second stop is refused while the first handler still waits.
// Nothing moves it back. Safe for concurrent use.
type drainWorker struct {
	plugin   string
	handler  DrainHandler
	requests chan drainRequest
	stopping atomic.Bool
	done     chan struct{}
}

func newDrainWorker(plugin string, handler DrainHandler) *drainWorker {
	return &drainWorker{
		plugin:   plugin,
		handler:  handler,
		requests: make(chan drainRequest, drainQueueDepth),
		done:     make(chan struct{}),
	}
}

// errDrainAlreadyStopping refuses a second stop-kind drain on one connection.
var errDrainAlreadyStopping = errors.New("a stop drain was already received")

// admit decodes a drain request and queues it for the worker, or returns the
// refusal the event loop answers at once. It runs on the event loop and never
// blocks.
func (w *drainWorker) admit(params json.RawMessage, answer func(err error)) error {
	var input rpc.DrainInput
	if err := json.Unmarshal(params, &input); err != nil {
		return fmt.Errorf("plugin %s: unmarshal drain: %w", w.plugin, err)
	}
	if !w.stopping.CompareAndSwap(false, true) {
		return fmt.Errorf("plugin %s: drain %s refused: %w", w.plugin, input.Kind, errDrainAlreadyStopping)
	}
	select {
	case w.requests <- drainRequest{input: input, answer: answer}:
		return nil
	default:
		// Unreachable while admission lets one stop drain through, kept so a
		// full queue answers rather than blocking the event loop.
		return fmt.Errorf("plugin %s: drain %s refused: drain queue full", w.plugin, input.Kind)
	}
}

// run serves admitted drains until ctx ends. It MUST be started once per
// connection, and the event loop MUST cancel ctx and wait for done before it
// returns.
func (w *drainWorker) run(ctx context.Context) {
	defer close(w.done)
	for {
		select {
		case request := <-w.requests:
			request.answer(w.handler(ctx, request.input))
		case <-ctx.Done():
			return
		}
	}
}

// startDrainWorker starts the connection's drain worker when the plugin
// registered OnDrain, and returns it with the stop function the event loop
// MUST defer. With no handler it returns a nil worker and a no-op stop, and
// a drain request is then answered as an unknown method.
func (p *Plugin) startDrainWorker(ctx context.Context) (*drainWorker, func()) {
	p.mu.Lock()
	handler := p.onDrain
	p.mu.Unlock()
	if handler == nil {
		return nil, func() {}
	}

	worker := newDrainWorker(p.name, handler)
	workerCtx, cancel := context.WithCancel(ctx)
	go worker.run(workerCtx)

	return worker, func() {
		cancel()
		<-worker.done
	}
}
