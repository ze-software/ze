// Design: docs/architecture/api/process-protocol.md — event delivery pipeline
// Overview: process.go — Process struct and lifecycle
// Related: manager.go — multi-process coordination and respawn

package process

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
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
			logger().Error("DirectBridge panic", "panic", rec, "stack", string(debug.Stack()))
		}
	}()
	return fn()
}

// EventDelivery represents a work item for the per-process delivery goroutine.
// The long-lived goroutine reads these from Process.eventChan and calls SendDeliverEvent.
// For DirectBridge consumers, Event is set (structured delivery, no text formatting).
// For text/JSON consumers, Output is set (pre-formatted at observation time).
type EventDelivery struct {
	Output string             // Pre-formatted event payload (text/JSON consumers)
	Event  any                // Structured event for DirectBridge consumers (nil for text/JSON)
	Result chan<- EventResult // Caller-provided result channel (nil if fire-and-forget)
}

// EventResult is sent back to the caller after delivery completes.
type EventResult struct {
	ProcName      string // Process name (for logging)
	Err           error  // nil on success
	CacheConsumer bool   // true if delivery succeeded AND process is a cache consumer
}

const (
	// eventDeliveryCapacity is the buffer size for per-process event delivery channels.
	// Each item is small (string + channel pointer). 64 provides headroom without
	// excessive memory use. If a plugin is slow, backpressure propagates naturally.
	eventDeliveryCapacity = 64
)

// Deliver enqueues an event for the long-lived delivery goroutine.
// Returns true if the event was enqueued, false if the process is stopping.
// Thread-safe: uses RLock to allow parallel sends from multiple callers.
func (p *Process) Deliver(d EventDelivery) bool {
	p.eventMu.RLock()
	defer p.eventMu.RUnlock()

	if p.eventClosed || p.eventChan == nil {
		return false
	}

	select {
	case p.eventChan <- d:
		if p.deliveryInc != nil {
			p.deliveryInc()
		}
		return true
	case <-p.ctx.Done():
		return false
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
// Drains all available events from eventChan into a batch and delivers them
// in a single RPC call, reducing syscalls and goroutine churn.
// Exits when eventChan is closed (by stopEventChan during Stop).
func (p *Process) deliveryLoop() {
	timeout := deliveryTimeoutFromEnv()

	var batchBuf []EventDelivery
	var eventsBuf []string

	for first := range p.eventChan {
		batchBuf = p.drainBatch(batchBuf, first)
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
			logger().Error("deliveryLoop panic recovered",
				"plugin", p.config.Name,
				"panic", rec,
				"stack", string(debug.Stack()),
			)
			// Signal all waiting callers so they are not blocked forever.
			for _, req := range batch {
				if req.Result != nil {
					req.Result <- EventResult{
						ProcName: p.config.Name,
						Err:      panicErr,
					}
				}
			}
			result = eventsBuf
		}
	}()
	return p.deliverBatch(batch, eventsBuf, timeout)
}

// drainBatch collects the first event plus any additional events available
// without blocking. Returns when the channel is empty or closed.
// buf is a reusable slice from the caller — reset to [:0] and returned for reuse.
func (p *Process) drainBatch(buf []EventDelivery, first EventDelivery) []EventDelivery {
	buf = append(buf[:0], first)
	for {
		select {
		case req, ok := <-p.eventChan:
			if !ok {
				return buf
			}
			buf = append(buf, req)
		default: // non-blocking drain complete
			return buf
		}
	}
}

// deliverBatch sends a batch of events and notifies callers.
// Uses DirectBridge for internal plugins (direct function call),
// text lines for text-mode plugins, or SendDeliverBatch for JSON-RPC plugins.
// eventsBuf is a reusable slice for the string events — returned for reuse.
func (p *Process) deliverBatch(batch []EventDelivery, eventsBuf []string, timeout time.Duration) []string {
	eventsBuf = eventsBuf[:0]
	for _, req := range batch {
		eventsBuf = append(eventsBuf, req.Output)
	}
	events := eventsBuf

	batchErr := p.sendBatch(batch, events, timeout)

	isCacheConsumer := p.IsCacheConsumer()
	for _, req := range batch {
		if req.Result != nil {
			req.Result <- EventResult{
				ProcName:      p.config.Name,
				Err:           batchErr,
				CacheConsumer: batchErr == nil && isCacheConsumer,
			}
		} else if batchErr != nil {
			// Fire-and-forget delivery (no Result channel): log errors here
			// since no caller is waiting to collect them. This covers sent
			// event delivery which uses nil Result to avoid re-entrant deadlock.
			logger().Warn("event delivery failed (fire-and-forget)", "plugin", p.config.Name, "error", batchErr)
		}
	}

	return eventsBuf
}

// stopEventChan closes the event channel, causing deliveryLoop to drain and exit.
// Uses write lock to prevent concurrent Deliver calls from sending to a closed channel.
func (p *Process) stopEventChan() {
	p.eventMu.Lock()
	defer p.eventMu.Unlock()

	if !p.eventClosed && p.eventChan != nil {
		p.eventClosed = true
		close(p.eventChan)
	}
}

// startDeliveryLocked starts the event delivery goroutine.
// Caller must hold p.mu.
func (p *Process) startDeliveryLocked() {
	p.eventChan = make(chan EventDelivery, eventDeliveryCapacity)
	p.wg.Go(p.deliveryLoop)
}

// StartDelivery starts only the event delivery goroutine.
// Used by tests that inject connections via SetConn without starting a real process.
func (p *Process) StartDelivery(ctx context.Context) {
	p.eventMu.Lock()
	if p.eventChan != nil {
		p.eventMu.Unlock()
		return
	}
	p.eventMu.Unlock()

	p.mu.Lock()
	defer p.mu.Unlock()

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

// deliverMixedBatch handles batches with both structured and text events.
func (p *Process) deliverMixedBatch(batch []EventDelivery) error {
	var structuredBuf []any
	var textBuf []string
	for _, req := range batch {
		if req.Event != nil {
			structuredBuf = append(structuredBuf, req.Event)
		} else if req.Output != "" {
			textBuf = append(textBuf, req.Output)
		}
	}
	if len(structuredBuf) > 0 {
		if err := safeBridgeCall(func() error { return p.bridge.DeliverStructured(structuredBuf) }); err != nil {
			return err
		}
	}
	if len(textBuf) > 0 {
		return safeBridgeCall(func() error { return p.bridge.DeliverEvents(textBuf) })
	}
	return nil
}

// deliverViaConn sends events through the socket connection.
func (p *Process) deliverViaConn(events []string, timeout time.Duration) error {
	conn := p.Conn()
	if conn == nil {
		return ErrConnectionClosed
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()
	err := conn.SendDeliverBatch(ctx, events)
	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		logger().Warn("timing: slow deliverViaConn",
			"plugin", p.config.Name,
			"events", len(events),
			"elapsed", elapsed,
			"error", err,
		)
	}
	return err
}
