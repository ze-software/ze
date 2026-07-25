// Design: plan/learned/1019-traffic-usage-monitor.md -- lazy refcounted traffic aggregation service

package trafficstat

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/observation"
)

type serviceMetrics struct {
	consumers metrics.Gauge
}

var svcMetricsPtr atomic.Pointer[serviceMetrics]

// BindMetrics registers trafficstat counters with the given registry.
func BindMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	m := &serviceMetrics{
		consumers: reg.Gauge("ze_trafficstat_consumers", "Active trafficstat consumers"),
	}
	svcMetricsPtr.Store(m)
}

const (
	MaxTopN       = 50
	maxTrackedKey = 10000
	tickInterval  = time.Second
)

type TalkerEntry struct {
	Addr netip.Addr
	Bps  float64
	// History is the source's recent per-tick rate samples (newest last), used
	// by the behavioral detector to baseline a source against itself. Populated
	// for per-source/per-dest talkers; nil for ports/protocols.
	History []float64
}

type PortEntry struct {
	Port  uint16
	Proto uint8
	Bps   float64
}

type InterfaceEntry struct {
	Name  string
	RxBps float64
	TxBps float64
	RxPps float64
	TxPps float64
}

type ProtocolMix struct {
	Proto uint8
	Name  string
	Bps   float64
	Pct   float64
}

type Snapshot struct {
	Interfaces   []InterfaceEntry
	TopSourceIPs []TalkerEntry
	TopDestIPs   []TalkerEntry
	TopPorts     []PortEntry
	Protocols    []ProtocolMix
	History      []float64
	Degraded     bool
	At           time.Time
}

var globalService atomic.Pointer[Service]

// SetGlobal installs s as the process-wide trafficstat service.
func SetGlobal(s *Service) { globalService.Store(s) }

// Global returns the process-wide trafficstat service, or nil.
func Global() *Service { return globalService.Load() }

// RateFunc is called after each tick with the latest interface rates.
type RateFunc func([]InterfaceEntry)

type rateSubscriber struct {
	id int
	fn RateFunc
}

type Service struct {
	feed *observation.Feed

	mu        sync.Mutex
	consumers int
	nextID    int
	running   bool
	stopCh    chan struct{}
	feedSubID int

	attachIDs map[int]struct{}
	rateSubs  []rateSubscriber

	snap atomic.Pointer[Snapshot]
	agg  *aggregator
}

func NewService(feed *observation.Feed) *Service {
	return &Service{
		feed:      feed,
		attachIDs: make(map[int]struct{}),
	}
}

func (s *Service) consumerCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.consumers
}

func (s *Service) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Attach registers a consumer and starts the service if this is the
// first one (0->1 transition). Returns a consumer ID for Detach.
func (s *Service) Attach() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	id := s.nextID
	s.attachIDs[id] = struct{}{}
	s.consumers++
	if m := svcMetricsPtr.Load(); m != nil {
		m.consumers.Inc()
	}

	if s.consumers == 1 {
		s.start()
	}

	return id
}

// Detach removes a consumer and stops the service if this was the
// last one (1->0 transition). A duplicate call with the same ID is a no-op.
func (s *Service) Detach(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.attachIDs[id]; !ok {
		return
	}
	delete(s.attachIDs, id)
	s.consumers--
	if m := svcMetricsPtr.Load(); m != nil {
		m.consumers.Dec()
	}

	if s.consumers == 0 {
		s.stop()
	}
}

// Snapshot returns the latest aggregated snapshot, or a degraded
// empty snapshot if the service has not yet ticked.
func (s *Service) Snapshot() *Snapshot {
	if p := s.snap.Load(); p != nil {
		return p
	}
	return &Snapshot{Degraded: true, At: time.Now()}
}

// SubscribeRates registers a callback invoked after each tick with the
// latest interface rates. Returns an ID for UnsubscribeRates.
// The subscriber also counts as a consumer (starts the service if needed).
func (s *Service) SubscribeRates(fn RateFunc) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	id := s.nextID
	s.rateSubs = append(s.rateSubs, rateSubscriber{id: id, fn: fn})
	s.consumers++
	if m := svcMetricsPtr.Load(); m != nil {
		m.consumers.Inc()
	}

	if s.consumers == 1 {
		s.start()
	}
	return id
}

// UnsubscribeRates removes a rate subscriber and stops the service if
// it was the last consumer.
func (s *Service) UnsubscribeRates(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for i, sub := range s.rateSubs {
		if sub.id == id {
			s.rateSubs = append(s.rateSubs[:i], s.rateSubs[i+1:]...)
			found = true
			break
		}
	}

	if !found || s.consumers <= 0 {
		return
	}
	s.consumers--
	if m := svcMetricsPtr.Load(); m != nil {
		m.consumers.Dec()
	}

	if s.consumers == 0 {
		s.stop()
	}
}

// Close stops the service unconditionally, releasing all resources.
func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		s.stop()
	}
	s.consumers = 0
}

// start subscribes to the feed and begins the aggregation loop.
// Caller must hold s.mu.
func (s *Service) start() {
	s.agg = newAggregator()
	s.stopCh = make(chan struct{})
	s.running = true

	s.feedSubID = s.feed.Subscribe("trafficstat", func(obs observation.Observation) {
		s.agg.ingest(obs)
	})

	stopCh := s.stopCh
	go s.run(stopCh)
}

// stop unsubscribes from the feed and stops the aggregation loop.
// Caller must hold s.mu.
func (s *Service) stop() {
	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
	s.feed.Unsubscribe(s.feedSubID)
	s.agg = nil
}

func (s *Service) run(stopCh chan struct{}) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	s.snap.Store(&Snapshot{Degraded: true, At: time.Now()})

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *Service) tick() {
	s.mu.Lock()
	agg := s.agg
	s.mu.Unlock()

	if agg == nil {
		return
	}

	now := time.Now()

	var ifaces []InterfaceEntry
	if rates := iface.ListRates(); rates != nil {
		ifaces = make([]InterfaceEntry, 0, len(rates))
		for _, r := range rates {
			ifaces = append(ifaces, InterfaceEntry{
				Name:  r.Name,
				RxBps: r.RxBps,
				TxBps: r.TxBps,
				RxPps: r.RxPps,
				TxPps: r.TxPps,
			})
		}
	}

	snap := agg.snapshot(now, ifaces)
	s.snap.Store(snap)

	s.mu.Lock()
	subs := make([]rateSubscriber, len(s.rateSubs))
	copy(subs, s.rateSubs)
	s.mu.Unlock()

	for i := range subs {
		subs[i].fn(ifaces)
	}
}
