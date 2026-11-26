// Design: docs/architecture/api/process-protocol.md — event delivery pipeline
// Overview: process.go — Process struct and lifecycle
// Detail: event_queue.go — bounded growable lossless FIFO event delivery queue
// Related: manager.go — multi-process coordination and respawn

package process

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Env var registration for delivery timeout.
var _ = env.MustRegister(env.EnvEntry{Key: "ze.plugin.delivery.timeout", Type: "duration", Default: "5s", Description: "Timeout for event delivery to plugins"})

// ErrConnectionClosed is returned when the plugin connection is closed during event delivery.
var ErrConnectionClosed = errors.New("connection closed")

// safeBridgeCall calls fn with panic recovery. If the plugin handler panics,
// the panic is caught and returned as an error instead of crashing the
// engine's delivery loop.
func safeBridgeCall(fn func() error) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("plugin panic: %v", rec)
			logger().Error("DirectBridge panic", "panic", rec, "stack", string(debug.Stack()),
				"code", diagnostic.CodeDoctorPluginPanic)
		}
	}()
	return fn()
}

// EventDelivery represents a work item for the per-process delivery goroutine.
// The long-lived goroutine drains these from Process.eventQueue and delivers them
// via DirectBridge or SendDeliverBatch.
// For DirectBridge consumers, Event is set (structured delivery, no text formatting).
// For text/JSON consumers, Output is set (pre-formatted at observation time).
type EventDelivery struct {
	Output    string             // Pre-formatted event payload (text/JSON consumers)
	Event     any                // Structured event for DirectBridge consumers (nil for text/JSON)
	Result    chan<- EventResult // Caller-provided result channel (nil if fire-and-forget)
	OnFailure func()             // Called on fire-and-forget delivery failure (e.g. cache count release)
	// Barrier carries no event and reaches no plugin. Its FIFO receipt reports
	// the first application failure in this Process lifetime, if any.
	// DrainEvents ignores that error for soft quiesce; DrainEventsApplied
	// requires successful application for recovery.
	Barrier bool
}

// EventResult is sent back to the caller after delivery completes.
type EventResult struct {
	ProcName      string // Process name (for logging)
	Err           error  // nil on success
	CacheConsumer bool   // true if delivery succeeded AND process is a cache consumer
}

// Deliver enqueues an event for the long-lived delivery goroutine.
// Returns true if the event was enqueued, false if the process is stopping.
// Thread-safe: uses RLock only to snapshot eventQueue before lossless admission.
func (p *Process) Deliver(d EventDelivery) bool {
	return p.DeliverContext(context.Background(), d)
}

// onDeliveryBackpressure records a producer wait at the eventQueue ceiling in
// both Prometheus metrics and the per-process doctor delivery window.
func (p *Process) onDeliveryBackpressure() {
	p.noteDeliveryBackpressure()
	p.noteBackpressureWait()
}

// DeliverContext preserves the existing FIFO delivery rail while allowing a
// bounded cold caller to cancel queue admission. A refused structured delivery
// remains owned by its caller; an admitted delivery belongs to the worker.
func (p *Process) DeliverContext(ctx context.Context, d EventDelivery) bool {
	p.eventMu.RLock()
	closed := p.eventClosed
	q := p.eventQueue
	p.eventMu.RUnlock()

	if closed {
		return false
	}
	if q == nil {
		return false
	}

	switch q.enqueue(ctx, p.ctx, nil, d, p.onDeliveryBackpressure, p.noteDeliveryQueueDepth) {
	case enqueueAdmitted:
		p.noteDelivered()
		return true
	case enqueueCallerCanceled, enqueueProcessClosed:
		return false
	case enqueueOutcomeUnspecified:
		panic("BUG: plugin/process: unspecified enqueue outcome")
	default:
		panic("BUG: plugin/process: unknown enqueue outcome")
	}
}

// defaultDeliveryTimeout is the per-event timeout for SendDeliverEvent RPCs.
const defaultDeliveryTimeout = 5 * time.Second

// deliveryTimeout caches the result of deliveryTimeoutFromEnv (read once).
var (
	deliveryTimeout     time.Duration
	deliveryTimeoutOnce sync.Once
)

// deliveryTimeoutFromEnv reads ze.plugin.delivery.timeout (dot or underscore notation)
// and returns the parsed duration. Falls back to defaultDeliveryTimeout on missing
// or invalid values. Result is cached via sync.Once.
func deliveryTimeoutFromEnv() time.Duration {
	deliveryTimeoutOnce.Do(func() {
		deliveryTimeout = env.GetDuration("ze.plugin.delivery.timeout", defaultDeliveryTimeout)
	})
	return deliveryTimeout
}

// deliveryLoop is the long-lived goroutine that processes event deliveries.
// Drains up to eventDeliveryBatchMax available events from eventQueue into a
// batch and delivers them in a single RPC call, reducing syscalls and goroutine
// churn. Exits once eventQueue is closed (by stopEventChan during Stop) and
// drained empty.
func (p *Process) deliveryLoop() {
	timeout := deliveryTimeoutFromEnv()

	var batchBuf []EventDelivery
	var eventsBuf []string

	// Capture the lifecycle queue once so DrainEvents and test gates observe
	// the same queue instance for this worker's lifetime.
	p.eventMu.RLock()
	queue := p.eventQueue
	p.eventMu.RUnlock()
	if queue == nil {
		return
	}
	defer queue.CleanupRemaining(p.config.Name, p.noteDeliveryQueueDepth)

	for {
		var ok bool
		batchBuf, ok = queue.drainBatch(batchBuf, eventDeliveryBatchMax, p.noteDeliveryQueueDepth)
		if !ok {
			return
		}
		eventsBuf = p.safeDeliverBatch(batchBuf, eventsBuf, timeout)
	}
}

// safeDeliverBatch wraps deliverBatch with panic recovery.
// If sendBatch panics, all pending result channels are signaled with
// the panic error so callers (e.g., onPeerStateChange) are not blocked forever.
func (p *Process) safeDeliverBatch(batch []EventDelivery, eventsBuf []string, timeout time.Duration) (result []string) {
	defer func() {
		if rec := recover(); rec != nil {
			panicErr := fmt.Errorf("delivery panic: %v", rec)
			if p.projectionErr == nil {
				p.projectionErr = panicErr
				p.recordProjectionErr(panicErr)
			}
			logger().Error("deliveryLoop panic recovered",
				"plugin", p.config.Name,
				"panic", rec,
				"stack", string(debug.Stack()),
				"code", diagnostic.CodeDoctorPluginPanic,
			)
			// Signal all waiting callers so they are not blocked forever.
			for _, req := range batch {
				if req.Result != nil {
					deliveryErr := panicErr
					if req.Barrier {
						deliveryErr = p.projectionErr
					}
					req.Result <- EventResult{
						ProcName: p.config.Name,
						Err:      deliveryErr,
					}
				} else if req.OnFailure != nil {
					req.OnFailure()
				}
			}
			result = eventsBuf
		}
	}()
	return p.deliverBatch(batch, eventsBuf, timeout)
}

// deliverBatch sends a batch of events and notifies callers.
// Uses DirectBridge for internal plugins (direct function call),
// text lines for text-mode plugins, or SendDeliverBatch for JSON-RPC plugins.
// eventsBuf is a reusable slice for the string events — returned for reuse.
func (p *Process) deliverBatch(batch []EventDelivery, eventsBuf []string, timeout time.Duration) []string {
	eventsBuf = eventsBuf[:0]
	carried := batch[:0:0]
	for _, req := range batch {
		// A barrier is answered like every other item below, and carries
		// nothing to the plugin: it exists to be ordered, not to be delivered.
		if req.Barrier {
			continue
		}
		carried = append(carried, req)
		eventsBuf = append(eventsBuf, req.Output)
	}
	events := eventsBuf

	// A batch of barriers alone has nothing to send. Calling sendBatch with an
	// empty event set would hand the plugin an empty delivery, which a text
	// consumer writes as a bare newline and a bridge consumer counts as a call.
	var batchErr error
	if len(carried) != 0 {
		start := time.Now()
		batchErr = p.sendBatch(carried, events, timeout)
		p.noteBatch(time.Since(start), len(carried), batchErr, hasUnwaited(carried))
		if batchErr != nil {
			if p.projectionErr == nil {
				p.projectionErr = batchErr
				p.recordProjectionErr(batchErr)
			}
		}
	}

	isCacheConsumer := p.IsCacheConsumer()
	for _, req := range batch {
		if req.Result != nil {
			deliveryErr := batchErr
			if req.Barrier {
				deliveryErr = p.projectionErr
			}
			req.Result <- EventResult{
				ProcName:      p.config.Name,
				Err:           deliveryErr,
				CacheConsumer: deliveryErr == nil && isCacheConsumer,
			}
		} else if batchErr != nil {
			// Fire-and-forget delivery (no Result channel): log errors here
			// since no caller is waiting to collect them. This covers sent
			// event delivery which uses nil Result to avoid re-entrant deadlock.
			logger().Warn("event delivery failed (fire-and-forget)", "plugin", p.config.Name, "error", batchErr,
				"code", CodeDoctorPluginDeliveryFailed)
			if req.OnFailure != nil {
				req.OnFailure()
			}
		}
		// Return pooled StructuredEvent after the handler is done with it.
		// Moved here from the dispatcher (events.go) so fire-and-forget
		// delivery can recycle SEs without waiting for result collection.
		if se, ok := req.Event.(*rpc.StructuredEvent); ok {
			rpc.PutStructuredEvent(se)
		}
	}

	return eventsBuf
}

// hasUnwaited reports whether any item of a carried batch has no caller
// waiting for its result, so a failure of the batch reaches nobody but the
// log and the doctor record.
func hasUnwaited(carried []EventDelivery) bool {
	for i := range carried {
		if carried[i].Result == nil {
			return true
		}
	}
	return false
}

// stopEventChan closes the event queue, causing deliveryLoop to drain and exit.
// Safe for concurrent use and idempotent.
func (p *Process) stopEventChan() {
	p.eventMu.Lock()
	if p.eventClosed {
		p.eventMu.Unlock()
		return
	}
	q := p.eventQueue
	if q == nil {
		p.eventMu.Unlock()
		return
	}
	p.eventClosed = true
	p.eventMu.Unlock()

	q.Close()
}

// startDeliveryLocked starts the event delivery goroutine.
// Caller must hold p.mu.
func (p *Process) startDeliveryLocked() {
	q := newEventQueue(p.effectiveDeliveryQueueMax())
	p.eventMu.Lock()
	p.eventQueue = q
	p.eventClosed = false
	p.eventMu.Unlock()
	p.wg.Go(p.deliveryLoop)
}

// StartDelivery starts only the event delivery goroutine.
// Used by tests that inject connections via SetConn without starting a real process.
func (p *Process) StartDelivery(ctx context.Context) {
	p.eventMu.Lock()
	if p.eventQueue != nil {
		p.eventMu.Unlock()
		return
	}
	p.eventMu.Unlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.eventMu.Lock()
	if p.eventQueue != nil {
		p.eventMu.Unlock()
		return
	}
	p.eventMu.Unlock()

	if p.ctx == nil {
		p.ctx, p.cancel = context.WithCancel(ctx)
	}

	p.startDeliveryLocked()
}

// sendBatch sends events through the appropriate transport path.
func (p *Process) sendBatch(batch []EventDelivery, events []string, timeout time.Duration) error {
	bridgeReady := p.bridge != nil && p.bridge.Ready()

	if bridgeReady && p.bridge.HasStructuredHandler() {
		return p.deliverMixedBatch(batch)
	}
	if bridgeReady {
		return safeBridgeCall(func() error { return p.bridge.DeliverEvents(events) })
	}
	return p.deliverViaConn(events, timeout)
}

// deliverMixedBatch handles batches with both structured and text events. It
// delivers them in batch order, one run of consecutive same-kind items per
// bridge call: the FIFO of the process's one event queue is what orders a
// bus event a plugin emitted inside its state handler (a text item, such as
// bgp-gr/retention) ahead of the state event the engine delivers next (a
// structured item), and gathering each kind into one call would reverse them.
func (p *Process) deliverMixedBatch(batch []EventDelivery) error {
	var structuredBuf []any
	var textBuf []string
	for _, req := range batch {
		if req.Event != nil {
			if err := p.flushTextRun(&textBuf); err != nil {
				return err
			}
			structuredBuf = append(structuredBuf, req.Event)
			continue
		}
		if req.Output == "" {
			continue
		}
		if err := p.flushStructuredRun(&structuredBuf); err != nil {
			return err
		}
		textBuf = append(textBuf, req.Output)
	}
	if err := p.flushStructuredRun(&structuredBuf); err != nil {
		return err
	}
	return p.flushTextRun(&textBuf)
}

// flushStructuredRun hands deliverMixedBatch's pending structured run to the
// bridge and empties it. An empty run costs no call.
func (p *Process) flushStructuredRun(run *[]any) error {
	if len(*run) == 0 {
		return nil
	}
	events := *run
	*run = nil
	return safeBridgeCall(func() error { return p.bridge.DeliverStructured(events) })
}

// flushTextRun hands deliverMixedBatch's pending text run to the bridge and
// empties it. An empty run costs no call.
func (p *Process) flushTextRun(run *[]string) error {
	if len(*run) == 0 {
		return nil
	}
	events := *run
	*run = nil
	return safeBridgeCall(func() error { return p.bridge.DeliverEvents(events) })
}

// deliverViaConn sends events through the socket connection.
func (p *Process) deliverViaConn(events []string, timeout time.Duration) error {
	conn := p.Conn()
	if conn == nil {
		return ErrConnectionClosed
	}
	// deliverBatch times every transport, so the slow-batch line is there.
	ctx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()
	return conn.SendDeliverBatch(ctx, events)
}

// DrainEvents returns when every event enqueued before the call has finished
// its delivery attempt, or when ctx ends.
//
// This is soft quiesce: failed deliveries and a stopped process do not fail the
// drain. Callers requiring successful application MUST use DrainEventsApplied.
func (p *Process) DrainEvents(ctx context.Context) error {
	result := make(chan EventResult, 1)
	if !p.Deliver(EventDelivery{Barrier: true, Result: result}) {
		return nil
	}
	select {
	case <-result:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("plugin %s: event delivery did not drain: %w", p.Name(), ctx.Err())
	}
}

// DrainEventsApplied waits for successful application of every prior admission.
// The first delivery failure is retained for this Process lifetime, including
// fire-and-forget failures, timeouts and panics. Success never clears it.
//
// Nil means the FIFO receipt succeeded and the owner was still accepting work
// at the final liveness check; it does not lease the owner's future lifetime.
// Callers MUST NOT hold locks needed by plugin handlers while waiting.
// Safe for concurrent use after process startup.
func (p *Process) DrainEventsApplied(ctx context.Context) error {
	result := make(chan EventResult, 1)
	if err := p.admitAppliedBarrier(ctx, result); err != nil {
		return fmt.Errorf("plugin %s: event application did not drain: %w", p.Name(), err)
	}
	select {
	case receipt := <-result:
		if receipt.Err != nil {
			return fmt.Errorf("plugin %s: event application failed: %w", p.Name(), receipt.Err)
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("plugin %s: event application did not drain: %w", p.Name(), err)
		}
		p.eventMu.RLock()
		err := p.deliveryOwnerError()
		p.eventMu.RUnlock()
		if err != nil {
			return fmt.Errorf("plugin %s: event owner stopped: %w", p.Name(), err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("plugin %s: event application did not drain: %w", p.Name(), ctx.Err())
	case <-p.ctx.Done():
		return fmt.Errorf("plugin %s: event owner stopped: %w", p.Name(), ErrConnectionClosed)
	case <-p.engineDone:
		return fmt.Errorf("plugin %s: event owner exited: %w", p.Name(), ErrConnectionClosed)
	}
}

// admitAppliedBarrier bounds queue admission by the caller's context too.
// Deliver's ordinary fire-and-forget admission semantics remain unchanged.
func (p *Process) admitAppliedBarrier(ctx context.Context, result chan<- EventResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.eventMu.RLock()
	err := p.deliveryOwnerError()
	q := p.eventQueue
	p.eventMu.RUnlock()
	if err != nil {
		return err
	}
	switch q.enqueue(ctx, p.ctx, p.engineDone, EventDelivery{Barrier: true, Result: result}, p.onDeliveryBackpressure, p.noteDeliveryQueueDepth) {
	case enqueueAdmitted:
		p.noteDelivered()
		return nil
	case enqueueCallerCanceled:
		return ctx.Err()
	case enqueueProcessClosed:
		return ErrConnectionClosed
	case enqueueOutcomeUnspecified:
		panic("BUG: plugin/process: unspecified enqueue outcome")
	default:
		panic("BUG: plugin/process: unknown enqueue outcome")
	}
}

// deliveryOwnerError reads lifecycle state without invoking a handler or IPC.
// Caller MUST hold eventMu for reading; no lock survives a handler call.
func (p *Process) deliveryOwnerError() error {
	if p.eventClosed {
		return ErrConnectionClosed
	}
	if p.eventQueue == nil {
		return ErrConnectionClosed
	}
	if p.ctx == nil {
		return ErrConnectionClosed
	}
	if p.ctx.Err() != nil {
		return ErrConnectionClosed
	}
	select {
	case <-p.engineDone:
		return ErrConnectionClosed
	default:
	}
	// Internal runners may leave their allocated bridge unused and retain IPC.
	// Once activated, a stopped bridge must fail rather than fall back to IPC.
	if p.bridge != nil && p.bridge.Activated() {
		return p.bridge.DeliveryError()
	}
	conn := p.Conn()
	if conn == nil {
		return ErrConnectionClosed
	}
	return conn.Err()
}
