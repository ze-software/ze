// Design: docs/plan/spec-arch-0-system-boundaries.md — Bus implementation
// Design: docs/plan/spec-arch-2-bus.md — Bus spec

package bus

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// deliveryCapacity is the buffer size for per-consumer delivery channels.
// Matches the existing plugin delivery pattern (64 items).
const deliveryCapacity = 64

// Bus is a content-agnostic pub/sub message backbone.
// It moves opaque payloads between producers and consumers without
// inspecting their contents. Topics are hierarchical strings using "/"
// as separator. Subscriptions match on topic prefixes.
type Bus struct {
	topicMu sync.RWMutex
	topics  map[string]struct{}

	subMu   sync.RWMutex
	subs    []*subscription
	nextID  atomic.Uint64
	workers map[uint64]*worker
}

// subscription ties a prefix filter to a consumer's delivery worker.
type subscription struct {
	id       uint64
	prefix   string
	filter   map[string]string
	workerID uint64
}

// worker is a long-lived per-consumer delivery goroutine.
type worker struct {
	id       uint64
	consumer ze.Consumer
	ch       chan ze.Event
	done     chan struct{}
}

// NewBus creates a new Bus.
func NewBus() *Bus {
	return &Bus{
		topics:  make(map[string]struct{}),
		workers: make(map[uint64]*worker),
	}
}

// CreateTopic registers a new hierarchical topic.
func (b *Bus) CreateTopic(name string) (ze.Topic, error) {
	b.topicMu.Lock()
	defer b.topicMu.Unlock()

	if _, exists := b.topics[name]; exists {
		return ze.Topic{}, fmt.Errorf("topic %q already exists", name)
	}
	b.topics[name] = struct{}{}
	return ze.Topic{Name: name}, nil
}

// Publish sends an opaque event to all consumers whose subscription
// prefix matches the topic and whose metadata filter matches.
func (b *Bus) Publish(topic string, payload []byte, metadata map[string]string) {
	event := ze.Event{
		Topic:    topic,
		Payload:  payload,
		Metadata: metadata,
	}

	b.subMu.RLock()
	for _, s := range b.subs {
		if !matchesPrefix(topic, s.prefix) {
			continue
		}
		if !matchesFilter(metadata, s.filter) {
			continue
		}
		w := b.workers[s.workerID]
		if w != nil {
			w.ch <- event
		}
	}
	b.subMu.RUnlock()
}

// Subscribe registers a consumer for all topics matching the given prefix.
// An empty filter matches all events; non-empty filters require all
// key-value pairs to be present in the event metadata.
func (b *Bus) Subscribe(prefix string, filter map[string]string, consumer ze.Consumer) (ze.Subscription, error) {
	id := b.nextID.Add(1)

	w := b.findOrCreateWorker(consumer)

	s := &subscription{
		id:       id,
		prefix:   prefix,
		filter:   filter,
		workerID: w.id,
	}

	b.subMu.Lock()
	b.subs = append(b.subs, s)
	b.subMu.Unlock()

	return ze.Subscription{ID: id, Prefix: prefix}, nil
}

// Unsubscribe removes a subscription. If it was the consumer's last
// subscription, the delivery goroutine is stopped.
func (b *Bus) Unsubscribe(sub ze.Subscription) {
	b.subMu.Lock()

	var removedWorkerID uint64
	var found bool
	for i, s := range b.subs {
		if s.id == sub.ID {
			removedWorkerID = s.workerID
			b.subs = append(b.subs[:i], b.subs[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		b.subMu.Unlock()
		return
	}

	// Check if any remaining subscription uses the same worker.
	workerStillUsed := false
	for _, s := range b.subs {
		if s.workerID == removedWorkerID {
			workerStillUsed = true
			break
		}
	}

	var workerToStop *worker
	if !workerStillUsed {
		workerToStop = b.workers[removedWorkerID]
		delete(b.workers, removedWorkerID)
	}

	b.subMu.Unlock()

	if workerToStop != nil {
		close(workerToStop.ch)
		<-workerToStop.done
	}
}

// Stop shuts down all delivery goroutines.
func (b *Bus) Stop() {
	b.subMu.Lock()
	workers := make([]*worker, 0, len(b.workers))
	for _, w := range b.workers {
		workers = append(workers, w)
	}
	b.workers = make(map[uint64]*worker)
	b.subs = nil
	b.subMu.Unlock()

	for _, w := range workers {
		close(w.ch)
		<-w.done
	}
}

// findOrCreateWorker returns an existing worker for the consumer,
// or creates a new one with a delivery goroutine.
func (b *Bus) findOrCreateWorker(consumer ze.Consumer) *worker {
	// Check for existing worker (same consumer pointer).
	for _, w := range b.workers {
		if w.consumer == consumer {
			return w
		}
	}

	id := b.nextID.Add(1)
	w := &worker{
		id:       id,
		consumer: consumer,
		ch:       make(chan ze.Event, deliveryCapacity),
		done:     make(chan struct{}),
	}
	b.workers[id] = w
	go deliveryLoop(w)
	return w
}

// deliveryLoop is the long-lived goroutine per consumer.
// It drains all available events from the channel into a batch,
// then delivers them in a single Deliver call.
// Recovers from consumer panics to prevent one misbehaving consumer
// from crashing the bus.
func deliveryLoop(w *worker) {
	defer close(w.done)

	var buf []ze.Event
	for first := range w.ch {
		buf = drainBatch(buf, first, w.ch)
		safeDeliver(w.consumer, buf)
	}
}

// safeDeliver calls consumer.Deliver with panic recovery.
func safeDeliver(c ze.Consumer, events []ze.Event) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("bus consumer panic", "panic", rec, "stack", string(debug.Stack()))
		}
	}()
	_ = c.Deliver(events)
}

// drainBatch collects the first event plus all immediately available events.
// Reuses buf by resetting to [:0].
func drainBatch(buf []ze.Event, first ze.Event, ch <-chan ze.Event) []ze.Event {
	buf = append(buf[:0], first)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return buf
			}
			buf = append(buf, ev)
		default: // non-blocking drain complete
			return buf
		}
	}
}

// matchesPrefix returns true if topic starts with prefix.
// An exact match (prefix == topic) also matches.
func matchesPrefix(topic, prefix string) bool {
	return strings.HasPrefix(topic, prefix)
}

// matchesFilter returns true if all filter key-value pairs exist
// in the metadata. An empty/nil filter matches everything.
func matchesFilter(metadata, filter map[string]string) bool {
	if len(filter) == 0 {
		return true
	}
	for k, v := range filter {
		if metadata == nil || metadata[k] != v {
			return false
		}
	}
	return true
}
