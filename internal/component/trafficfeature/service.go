// Design: docs/architecture/traffic/traffic-analysis-layers.md -- neutral per-entity traffic feature signals
//
// trafficfeature is the FEATURE layer of the traffic-analysis split: a second,
// independent consumer of observation.Feed that derives domain-NEUTRAL detection
// signals (facts, not verdicts) per entity. Detection plugins (the anomaly
// family, ddos) apply judgment on top; this layer never decides "attack".
//
// An entity sits on one of three axes, each with its own map, its own cardinality
// ceiling and its own list on a Snapshot: the SOURCE address, the DESTINATION
// address, and the destination PORT. One flow contributes to one entity per axis.

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
	// MaxTopN bounds how many feature vectors a Snapshot carries per axis: the
	// busiest MaxTopN sources, destinations and ports, each ranked independently.
	MaxTopN = 50
	// tickInterval is the aggregation/finalize cadence, matched to the feed's
	// 1s observation tick.
	tickInterval = time.Second
)

// FeatureEntry is the neutral feature vector for one ADDRESS entity in a window.
// Every field is a measurable fact; none is a verdict.
//
// The same type serves both address axes, and each field reads in the direction
// the axis names: on Snapshot.Sources the entity is the sender, on Snapshot.Dests
// it is the receiver. Where that flips a field's sense, the field says so.
type FeatureEntry struct {
	Addr netip.Addr
	// FanOut is the count of distinct counterparties in the window (spread
	// signal): for a source entry the destinations it talked to (scanning), for a
	// destination entry the sources that talked to it (fan-in, a distributed sink).
	FanOut int
	// OutInRatio is the entity's OWN bytes divided by the bytes flowing the other
	// way, so it measures asymmetry in the direction the axis names: out over in
	// for a source entry (exfiltration), in over out for a destination entry (a
	// sink that receives far more than it answers). When the divisor is zero it is
	// +Inf as a sentinel; a consumer MUST map that to a finite value before it
	// does arithmetic on it.
	OutInRatio float64
	// PortEntropy is the Shannon entropy (bits) of the entity's destination-port
	// byte distribution: ~0 for a single port, higher for a spread/scan. For a
	// destination entry those are the ports it was addressed ON (its own service
	// spread), for a source entry the ports it reached OUT to.
	PortEntropy float64
	// NewPeer reports the entity was first observed within the recent new-peer
	// window (a never-seen-before source, or a never-seen-before destination).
	NewPeer bool
	// RarePort reports the entity's dominant destination port is uncommon: the
	// port a source sent most bytes to, or the port a destination received most
	// bytes on.
	RarePort bool
	// Beaconing is an interval-regularity score in [0,1] over the entity's flow
	// arrival times: higher means more clock-like (C2 beacon signal). Bounded by
	// the 1s tick, so only periods of a few seconds and up are observable.
	Beaconing float64
	// SrcAS is the origin autonomous system of Addr, as the observation
	// publisher attributed it, and is the last non-zero value seen for this
	// entity. It is 0 when no publisher could attribute the address: AS 0 is
	// reserved (RFC 7607), so 0 is unambiguous. A consumer that groups by AS
	// MUST fall back to the address or its prefix when it reads 0.
	//
	// It is stamped on the SOURCE axis only, because the observation carries the
	// origin AS of the flow's source. On a Snapshot.Dests entry it is always 0.
	SrcAS uint32
}

// PortKey identifies one destination-PORT entity. The IP protocol number is part
// of the identity: TCP 53 and UDP 53 are two services and two entities.
type PortKey struct {
	Port  uint16
	Proto uint8
}

// PortFeatureEntry is the neutral feature vector for one destination-port entity
// in a window. A port has no address, so it carries its own field set rather than
// a FeatureEntry with a zero Addr.
//
// Every field describes the traffic AIMED AT the port, aggregated across all the
// addresses involved, which is what makes the port an entity of its own: it sees
// a spread no single address sees.
type PortFeatureEntry struct {
	PortKey
	// FanOut is the count of distinct sources that sent to this port in the window
	// (a scan sweep, a flood, or a service suddenly popular).
	FanOut int
	// OutInRatio is bytes sent FROM this port divided by bytes sent TO it: the
	// amplification signal, high when a service answers far more than it is asked
	// (reflection). It is 0 when the port only ever received.
	OutInRatio float64
	// SrcEntropy is the Shannon entropy (bits) of the per-source byte distribution
	// on this port: ~0 when one source dominates, log2(n) when n sources send
	// equally (a distributed sweep rather than one busy client).
	SrcEntropy float64
	// NewPort reports the port was first observed within the recent new-peer
	// window (a service that was silent until now).
	NewPort bool
	// RarePort reports the port number itself is outside the well-known allowlist.
	// Unlike every other field this is a property of the KEY, constant for the
	// entity's whole life, so it annotates an incident rather than evidencing one.
	RarePort bool
	// Beaconing is an interval-regularity score in [0,1] over the ticks in which
	// the port carried traffic: higher means more clock-like. Bounded by the 1s
	// tick, so only periods of a few seconds and up are observable.
	Beaconing float64
}

// Snapshot is the finalized per-tick feature view: the busiest entities on each
// axis plus their neutral feature vectors. Each list is independently ranked and
// independently bounded by MaxTopN.
type Snapshot struct {
	// Sources holds the entities that SENT this window, ranked by bytes sent.
	Sources []FeatureEntry
	// Dests holds the entities that RECEIVED this window, ranked by bytes
	// received. An address that only ever sent is absent here, and an address that
	// both sent and received appears on both lists as two entities.
	Dests []FeatureEntry
	// Ports holds the destination ports that received this window, ranked by bytes
	// received.
	Ports    []PortFeatureEntry
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
// per-entity feature aggregation, mirroring trafficstat's lifecycle so it costs
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
