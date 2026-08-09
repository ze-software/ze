// Design: docs/architecture/observation-feed.md -- shared traffic-observation feed

package observation

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
)

type feedMetrics struct {
	published   metrics.Counter
	dropped     metrics.Counter
	subscribers metrics.Gauge
}

var feedMetricsPtr atomic.Pointer[feedMetrics]

// BindMetrics registers observation feed counters with the given registry.
func BindMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	m := &feedMetrics{
		published:   reg.Counter("ze_observation_published_total", "Total observations published to the feed"),
		dropped:     reg.Counter("ze_observation_dropped_total", "Total observations dropped due to full subscriber buffers"),
		subscribers: reg.Gauge("ze_observation_subscribers", "Current number of feed subscribers"),
	}
	feedMetricsPtr.Store(m)
}

type Kind uint8

const (
	KindInterface Kind = iota + 1
	KindSourceIP
	KindDestIP
	KindFlow
)

type Feature uint8

const (
	FeatureRxBytes Feature = iota + 1
	FeatureRxPackets
	FeatureFlowBytes
	FeatureFlowPackets
	FeatureNewFlowCount
)

type FlowKey struct {
	Src     netip.Addr
	Dst     netip.Addr
	SrcPort uint16
	DstPort uint16
	Proto   uint8
}

type Observation struct {
	Kind    Kind
	Iface   string
	Flow    FlowKey
	Feature Feature
	Value   float64
	At      time.Time
}

const bufferCap = 1024

type subscriber struct {
	id   int
	name string
	ch   chan Observation
	quit chan struct{}
	fn   func(Observation)
	done chan struct{}
}

type subscriberSnapshot struct {
	entries []subscriber
}

// Feed is an in-process, typed, multi-subscriber observation bus.
// Publishers call Publish (non-blocking); subscribers receive on
// their own goroutine via a buffered channel.
type Feed struct {
	mu      sync.Mutex
	seq     int
	subsPtr atomic.Pointer[subscriberSnapshot]
	dropped atomic.Int64
}

// NewFeed creates a ready-to-use Feed.
func NewFeed() *Feed {
	f := &Feed{}
	f.subsPtr.Store(&subscriberSnapshot{})
	return f
}

// Subscribe registers a handler that will be called for every published
// observation on its own goroutine. Returns an ID for Unsubscribe.
func (f *Feed) Subscribe(name string, fn func(Observation)) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.seq++
	id := f.seq

	s := subscriber{
		id:   id,
		name: name,
		ch:   make(chan Observation, bufferCap),
		quit: make(chan struct{}),
		fn:   fn,
		done: make(chan struct{}),
	}
	go s.drain()

	old := f.subsPtr.Load()
	next := &subscriberSnapshot{
		entries: make([]subscriber, len(old.entries)+1),
	}
	copy(next.entries, old.entries)
	next.entries[len(old.entries)] = s
	f.subsPtr.Store(next)
	if m := feedMetricsPtr.Load(); m != nil {
		m.subscribers.Inc()
	}
	return id
}

func (s *subscriber) drain() {
	defer close(s.done)
	for {
		select {
		case obs := <-s.ch:
			s.fn(obs)
		case <-s.quit:
			return
		}
	}
}

// Unsubscribe removes the subscriber with the given ID and waits for
// its goroutine to exit. The data channel is left open so a concurrent
// Publish holding a stale snapshot does not panic on a closed channel.
func (f *Feed) Unsubscribe(id int) {
	f.mu.Lock()

	old := f.subsPtr.Load()
	var removed *subscriber
	next := &subscriberSnapshot{
		entries: make([]subscriber, 0, len(old.entries)),
	}
	for i := range old.entries {
		if old.entries[i].id == id {
			removed = &old.entries[i]
		} else {
			next.entries = append(next.entries, old.entries[i])
		}
	}
	f.subsPtr.Store(next)
	f.mu.Unlock()

	if removed != nil {
		close(removed.quit)
		<-removed.done
		if m := feedMetricsPtr.Load(); m != nil {
			m.subscribers.Dec()
		}
	}
}

// Publish fans out an observation to all subscribers. Non-blocking: if a
// subscriber's buffer is full the observation is dropped and the drop
// counter increments. The publisher's goroutine is never stalled.
func (f *Feed) Publish(obs Observation) {
	snap := f.subsPtr.Load()
	for i := range snap.entries {
		select {
		case snap.entries[i].ch <- obs:
		default:
			f.dropped.Add(1)
			if m := feedMetricsPtr.Load(); m != nil {
				m.dropped.Inc()
			}
		}
	}
	if m := feedMetricsPtr.Load(); m != nil {
		m.published.Inc()
	}
}

// Dropped returns the total number of observations dropped across all
// subscribers due to full buffers.
func (f *Feed) Dropped() int64 {
	return f.dropped.Load()
}

// Close unsubscribes all subscribers and waits for their goroutines
// to finish.
func (f *Feed) Close() {
	f.mu.Lock()
	snap := f.subsPtr.Load()
	f.subsPtr.Store(&subscriberSnapshot{})
	f.mu.Unlock()

	for i := range snap.entries {
		close(snap.entries[i].quit)
		<-snap.entries[i].done
	}
}

var globalFeed = NewFeed()

// Global returns the process-wide observation feed.
func Global() *Feed { return globalFeed }
