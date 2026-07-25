// Design: plan/learned/1046-traffic-analysis-restructure.md -- neutral per-source traffic feature signals
//
// trafficfeature is the FEATURE layer of the traffic-analysis split: a second,
// independent consumer of observation.Feed that derives domain-NEUTRAL detection
// signals (facts, not verdicts) per source entity. Detection plugins (the anomaly
// family, ddos) apply judgment on top; this layer never decides "attack".

package trafficfeature

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/observation"
)

const (
	// MaxTopN bounds how many source feature vectors a Snapshot carries.
	MaxTopN = 50
	// tickInterval is the aggregation/finalize cadence, matched to the feed's
	// 1s observation tick.
	tickInterval = time.Second
)

// FeatureEntry is the neutral feature vector for one source entity in a window.
// Every field is a measurable fact; none is a verdict.
type FeatureEntry struct {
	Addr netip.Addr
	// FanOut is the count of distinct destinations this source talked to in the
	// window (scanning / spread signal).
	FanOut int
	// OutInRatio is out-bytes divided by in-bytes for the entity (exfiltration
	// signal). When in-bytes is zero it is set to ratioInfinity as a sentinel.
	OutInRatio float64
	// PortEntropy is the Shannon entropy (bits) of the entity's destination-port
	// byte distribution: ~0 for a single port, higher for a spread/scan.
	PortEntropy float64
	// NewPeer reports the entity was first observed within the recent new-peer
	// window (never-seen-before source).
	NewPeer bool
	// RarePort reports the entity sent traffic to an uncommon port/proto.
	RarePort bool
	// Beaconing is an interval-regularity score in [0,1] over the entity's flow
	// arrival times: higher means more clock-like (C2 beacon signal). Bounded by
	// the 1s tick, so only periods of a few seconds and up are observable.
	Beaconing float64
}

// Snapshot is the finalized per-tick feature view: the top source entities by
// activity plus their neutral feature vectors.
type Snapshot struct {
	Sources  []FeatureEntry
	Degraded bool
	At       time.Time
}

type serviceMetrics struct {
	consumers metrics.Gauge
}

var svcMetricsPtr atomic.Pointer[serviceMetrics]

// BindMetrics registers trafficfeature counters with the given registry.
func BindMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	m := &serviceMetrics{
		consumers: reg.Gauge("ze_trafficfeature_consumers", "Active trafficfeature consumers"),
	}
	svcMetricsPtr.Store(m)
}

var globalService atomic.Pointer[Service]

// SetGlobal installs s as the process-wide trafficfeature service.
func SetGlobal(s *Service) { globalService.Store(s) }

// Global returns the process-wide trafficfeature service, or nil.
func Global() *Service { return globalService.Load() }

// Service subscribes to observation.Feed and maintains a lazy, consumer-refcounted
// per-source feature aggregation, mirroring trafficstat's lifecycle so it costs
// nothing until a consumer (the CLI view or a detector) attaches.
type Service struct {
	feed *observation.Feed

	mu        sync.Mutex
	consumers int
	nextID    int
	running   bool
	stopCh    chan struct{}
	feedSubID int
	attachIDs map[int]struct{}

	snap atomic.Pointer[Snapshot]
	agg  *aggregator
}

// NewService returns a trafficfeature service bound to feed (may be nil, in which
// case the service degrades to an empty snapshot).
func NewService(feed *observation.Feed) *Service {
	return &Service{
		feed:      feed,
		attachIDs: make(map[int]struct{}),
	}
}

// Attach registers a consumer and starts the service on the 0->1 transition.
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

// Detach removes a consumer and stops the service on the 1->0 transition.
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

// Snapshot returns the latest finalized feature snapshot, or a degraded empty
// one if the service has not yet ticked.
func (s *Service) Snapshot() *Snapshot {
	if p := s.snap.Load(); p != nil {
		return p
	}
	return &Snapshot{Degraded: true, At: time.Now()}
}

// start subscribes to the feed and begins the aggregation loop. Caller holds s.mu.
func (s *Service) start() {
	s.agg = newAggregator()
	s.stopCh = make(chan struct{})
	s.running = true

	if s.feed != nil {
		s.feedSubID = s.feed.Subscribe("trafficfeature", func(obs observation.Observation) {
			s.agg.ingest(obs)
		})
	}

	stopCh := s.stopCh
	go s.run(stopCh)
}

// stop unsubscribes from the feed and stops the loop. Caller holds s.mu.
func (s *Service) stop() {
	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
	if s.feed != nil {
		s.feed.Unsubscribe(s.feedSubID)
	}
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
	s.snap.Store(agg.snapshot(time.Now()))
}
