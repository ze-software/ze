// Design: docs/architecture/api/process-protocol.md — event delivery pipeline
// Related: process.go — Process struct and lifecycle
// Related: process_manager.go — multi-process coordination and respawn

package plugin

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"
)

// EventDelivery represents a work item for the per-process delivery goroutine.
// The long-lived goroutine reads these from Process.eventChan and calls SendDeliverEvent.
type EventDelivery struct {
	Output string             // Pre-formatted event payload
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

// deliveryTimeoutFromEnv reads ze.plugin.delivery.timeout (or ze_plugin_delivery_timeout)
// and returns the parsed duration. Falls back to defaultDeliveryTimeout on missing
// or invalid values. Result is cached via sync.Once.
func deliveryTimeoutFromEnv() time.Duration {
	deliveryTimeoutOnce.Do(func() {
		deliveryTimeout = defaultDeliveryTimeout
		for _, key := range []string{"ze.plugin.delivery.timeout", "ze_plugin_delivery_timeout"} {
			if v := os.Getenv(key); v != "" {
				d, err := time.ParseDuration(v)
				if err != nil {
					logger().Warn("invalid delivery timeout env var", "key", key, "value", v, "error", err)
					return
				}
				deliveryTimeout = d
				return
			}
		}
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
		eventsBuf = p.deliverBatch(batchBuf, eventsBuf, timeout)
	}
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

	var batchErr error
	if p.bridge != nil && p.bridge.Ready() {
		batchErr = p.bridge.DeliverEvents(events)
	} else if tc := p.TextConnB(); tc != nil {
		// Text-mode: write each event as a plain text line (fire-and-forget).
		// WriteLine adds \n, so strip any trailing newline from event text.
		ctx, cancel := context.WithTimeout(p.ctx, timeout)
		for _, event := range events {
			event = strings.TrimRight(event, "\n")
			if err := tc.WriteLine(ctx, event); err != nil {
				batchErr = err
				break
			}
		}
		cancel()
	} else {
		connB := p.ConnB()
		if connB == nil {
			batchErr = errors.New("connection closed")
		} else {
			ctx, cancel := context.WithTimeout(p.ctx, timeout)
			batchErr = connB.SendDeliverBatch(ctx, events)
			cancel()
		}
	}

	isCacheConsumer := p.IsCacheConsumer()
	for _, req := range batch {
		if req.Result != nil {
			req.Result <- EventResult{
				ProcName:      p.config.Name,
				Err:           batchErr,
				CacheConsumer: batchErr == nil && isCacheConsumer,
			}
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
// Used by tests that inject connections via SetConnB without starting a real process.
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
