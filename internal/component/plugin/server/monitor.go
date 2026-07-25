// Design: docs/architecture/api/process-protocol.md — monitor client management
// Overview: server.go — Server struct holds MonitorManager
// Related: event_monitor.go — monitor event streaming handler

package server

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/events"
)

// MonitorClient represents an active monitor session.
type MonitorClient struct {
	id            string
	subscriptions []*Subscription
	EventChan     chan string     // Buffered channel for formatted events.
	Ctx           context.Context // Client-scoped context for cancellation.
	Dropped       atomic.Uint64   // Count of events dropped due to full channel.
}

// NewMonitorClient creates a monitor client with the given subscriptions and buffer size.
// Caller MUST call MonitorManager.Remove(id) when done to release resources.
func NewMonitorClient(ctx context.Context, id string, subs []*Subscription, bufSize int) *MonitorClient {
	return &MonitorClient{
		id:            id,
		subscriptions: subs,
		EventChan:     make(chan string, bufSize),
		Ctx:           ctx,
	}
}

// enqueue attempts a non-blocking send to the event channel.
func (mc *MonitorClient) enqueue(output string) {
	select {
	case mc.EventChan <- output:
	default: // channel full — backpressure drop
		mc.Dropped.Add(1)
	}
}

// MonitorManager manages active monitor clients.
// Parallel to SubscriptionManager (which manages plugin process subscriptions).
type MonitorManager struct {
	mu           sync.RWMutex
	monitors     map[string]*MonitorClient
	monitorCount atomic.Int64
}

// NewMonitorManager creates a new monitor manager.
func NewMonitorManager() *MonitorManager {
	return &MonitorManager{
		monitors: make(map[string]*MonitorClient),
	}
}

// Add registers a monitor client.
func (mm *MonitorManager) Add(mc *MonitorClient) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if _, exists := mm.monitors[mc.id]; !exists {
		mm.monitorCount.Add(1)
	}
	mm.monitors[mc.id] = mc
}

// Remove unregisters a monitor client by ID.
func (mm *MonitorManager) Remove(id string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if _, exists := mm.monitors[id]; exists {
		mm.monitorCount.Add(-1)
	}
	delete(mm.monitors, id)
}

// Count returns the number of active monitors.
func (mm *MonitorManager) Count() int {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return len(mm.monitors)
}

// HasMonitors returns true if any monitor clients are registered.
// Uses an atomic load instead of acquiring the mutex, making it suitable
// for hot-path early-exit checks.
func (mm *MonitorManager) HasMonitors() bool {
	return mm.monitorCount.Load() > 0
}

// GetMatching returns monitors with subscriptions matching the event.
// A monitor matches if any of its subscriptions match.
// peerName is the configured peer name (may be empty).
func (mm *MonitorManager) GetMatching(namespace, eventType, direction, peerAddr, peerName string) []*MonitorClient {
	nsID := events.LookupNamespaceID(namespace)
	etID := events.LookupEventTypeID(eventType)
	dirID := events.ParseDirection(direction)

	mm.mu.RLock()
	defer mm.mu.RUnlock()

	var result []*MonitorClient
	for _, mc := range mm.monitors {
		for _, sub := range mc.subscriptions {
			if sub.Matches(nsID, etID, dirID, peerAddr, peerName) {
				result = append(result, mc)
				break // Only add monitor once, even if multiple subs match
			}
		}
	}
	return result
}

// GetMatchingTyped returns monitors with subscriptions matching the event,
// using pre-resolved typed IDs. Avoids the string-to-ID lookups in GetMatching.
func (mm *MonitorManager) GetMatchingTyped(ns events.NamespaceID, et events.EventTypeID, dir events.Direction, peerAddr, peerName string) []*MonitorClient {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	var result []*MonitorClient
	for _, mc := range mm.monitors {
		for _, sub := range mc.subscriptions {
			if sub.Matches(ns, et, dir, peerAddr, peerName) {
				result = append(result, mc)
				break
			}
		}
	}
	return result
}

// Deliver sends a formatted event to all matching monitors.
// Uses non-blocking send: if a monitor's channel is full, the event is dropped
// and the dropped counter is incremented (backpressure).
// peerName is the configured peer name (may be empty).
func (mm *MonitorManager) Deliver(namespace, eventType, direction, peerAddr, peerName, output string) {
	nsID := events.LookupNamespaceID(namespace)
	etID := events.LookupEventTypeID(eventType)
	dirID := events.ParseDirection(direction)

	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, mc := range mm.monitors {
		for _, sub := range mc.subscriptions {
			if sub.Matches(nsID, etID, dirID, peerAddr, peerName) {
				mc.enqueue(output)
				break // Deliver once per monitor
			}
		}
	}
}

// DeliverLazy sends an event to matching monitors, invoking build only when
// at least one monitor matches. This avoids formatting cost for events that
// no monitor subscribes to (the common case when structured plugin consumers
// are present but no CLI monitor is attached). build is called outside the
// manager lock so JSON formatting does not block monitor registration.
// peerName is the configured peer name (may be empty).
//
// Race note: GetMatching releases mm.mu before build() and enqueue() run, so
// a concurrent Remove(id) may drop a monitor between matching and delivery.
// That is safe: enqueue uses a non-blocking send on a buffered channel that
// the removed client's reader will simply stop consuming on Ctx cancellation.
// The dropped counter on the removed client may tick up, which is harmless.
func (mm *MonitorManager) DeliverLazy(namespace, eventType, direction, peerAddr, peerName string, build func() string) {
	matches := mm.GetMatching(namespace, eventType, direction, peerAddr, peerName)
	if len(matches) == 0 {
		return
	}
	output := build()
	for _, mc := range matches {
		mc.enqueue(output)
	}
}

// DeliverLazyTyped is the hot-path variant of DeliverLazy. It accepts
// pre-resolved typed IDs, skipping the string-to-ID lookups and their
// associated global event registry RLock acquisitions. Returns immediately
// via an atomic load when no monitors are registered (the common production case).
func (mm *MonitorManager) DeliverLazyTyped(ns events.NamespaceID, et events.EventTypeID, dir events.Direction, peerAddr, peerName string, build func() string) {
	if mm.monitorCount.Load() == 0 {
		return
	}
	matches := mm.GetMatchingTyped(ns, et, dir, peerAddr, peerName)
	if len(matches) == 0 {
		return
	}
	output := build()
	for _, mc := range matches {
		mc.enqueue(output)
	}
}
