// Design: docs/architecture/api/process-protocol.md — monitor client management
// Overview: server.go — Server struct holds MonitorManager
// Related: event_monitor.go — monitor event streaming handler

package server

import (
	"context"
	"sync"
	"sync/atomic"
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
// Returns true if the event was enqueued, false if dropped due to backpressure.
func (mc *MonitorClient) enqueue(output string) bool {
	select {
	case mc.EventChan <- output:
		return true
	default: // channel full — backpressure drop
		mc.Dropped.Add(1)
		return false
	}
}

// MonitorManager manages active monitor clients.
// Parallel to SubscriptionManager (which manages plugin process subscriptions).
type MonitorManager struct {
	mu       sync.RWMutex
	monitors map[string]*MonitorClient
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
	mm.monitors[mc.id] = mc
}

// Remove unregisters a monitor client by ID.
func (mm *MonitorManager) Remove(id string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	delete(mm.monitors, id)
}

// Count returns the number of active monitors.
func (mm *MonitorManager) Count() int {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return len(mm.monitors)
}

// GetMatching returns monitors with subscriptions matching the event.
// A monitor matches if any of its subscriptions match.
// peerName is the configured peer name (may be empty).
func (mm *MonitorManager) GetMatching(namespace, eventType, direction, peerAddr, peerName string) []*MonitorClient {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	var result []*MonitorClient
	for _, mc := range mm.monitors {
		for _, sub := range mc.subscriptions {
			if sub.Matches(namespace, eventType, direction, peerAddr, peerName) {
				result = append(result, mc)
				break // Only add monitor once, even if multiple subs match
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
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, mc := range mm.monitors {
		for _, sub := range mc.subscriptions {
			if sub.Matches(namespace, eventType, direction, peerAddr, peerName) {
				mc.enqueue(output)
				break // Deliver once per monitor
			}
		}
	}
}
