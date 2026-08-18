// Design: docs/architecture/traffic/traffic-analysis-layers.md -- per-entity neutral feature aggregation
//
// Derives domain-NEUTRAL per-entity feature signals from the observation feed via
// the shared internal/core/stats primitives. These are FACTS (measurable numbers),
// never verdicts: detection plugins (anomaly, ddos) apply judgment on top.
//
// One flow feeds three axes, each an independent map with its own ceiling: the
// SOURCE address, the DESTINATION address, and the destination PORT. An entity is
// CREATED only by traffic in its own direction (a source by sending, a destination
// and a port by receiving), which is what stops a spoofed-source flood from
// filling the very maps that would report it.

package trafficfeature

import (
	"math"
	"net/netip"
	"sort"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/observation"
	"github.com/ze-software/ze/internal/core/stats"
)

const (
	// maxTrackedKey bounds distinct tracked source entities (memory ceiling under
	// spoofed-source churn), mirroring trafficstat.
	maxTrackedKey = 10000
	// maxTrackedDest bounds distinct tracked destination entities. It is its OWN
	// ceiling rather than a share of maxTrackedKey: a spoofed-source flood is
	// exactly the traffic the destination axis exists to report, so it must not be
	// able to evict the target it is aimed at.
	maxTrackedDest = 10000
	// maxTrackedPort bounds distinct tracked destination-port entities. Lower than
	// the address ceilings because only a port that RECEIVES becomes an entity, so
	// the natural cardinality is a service list, not a client-socket list.
	maxTrackedPort = 4096
	// evictIdleTicks drops an entity after this many consecutive quiet ticks.
	evictIdleTicks = 10
	// maxPeersPerEntity / maxPortsPerEntity bound one address entity's
	// counterparty and port-histogram cardinality, so one scanning source (or one
	// swept destination) cannot exhaust memory.
	maxPeersPerEntity = 4096
	maxPortsPerEntity = 1024
	// maxSourcesPerPort bounds one port entity's per-source histogram.
	maxSourcesPerPort = 4096
	// gapHistoryLen bounds the retained active-tick gaps used for beaconing.
	gapHistoryLen = 32
	// newPeerTicks is how many ticks an entity is considered newly-seen.
	newPeerTicks = 5
)

// commonPorts is the allowlist of well-known destination ports; a dominant port
// outside it flags the rare-port signal.
var commonPorts = map[uint16]struct{}{
	20: {}, 21: {}, 22: {}, 23: {}, 25: {}, 53: {}, 67: {}, 68: {}, 80: {},
	110: {}, 123: {}, 143: {}, 161: {}, 179: {}, 443: {}, 445: {}, 465: {},
	514: {}, 587: {}, 993: {}, 995: {}, 3306: {}, 3389: {}, 5060: {}, 5061: {},
	5432: {}, 8080: {}, 8443: {},
}

// lifespan is the persistent part of every entity's state, on every axis: when it
// was first seen (new-peer) and the recent active-tick gaps that feed beaconing.
// It survives the per-window reset.
type lifespan struct {
	firstTick      int       // tick when first observed (new-peer)
	lastActiveTick int       // last tick with traffic (beaconing gaps)
	gaps           []float64 // recent active-tick gaps in seconds (bounded)
}

func newLifespan(tick int) lifespan {
	return lifespan{firstTick: tick, lastActiveTick: -1}
}

// pushGap records one gap, dropping the oldest past gapHistoryLen.
func (l *lifespan) pushGap(g float64) {
	if len(l.gaps) < gapHistoryLen {
		l.gaps = append(l.gaps, g)
		return
	}
	copy(l.gaps, l.gaps[1:])
	l.gaps[len(l.gaps)-1] = g
}

// markActive closes one active tick: it records the gap since the entity was last
// active and remembers this tick.
func (l *lifespan) markActive(tick int) {
	if l.lastActiveTick >= 0 {
		l.pushGap(float64(tick - l.lastActiveTick))
	}
	l.lastActiveTick = tick
}

// addrState is the bounded feature accumulator for one ADDRESS entity on ONE axis.
// The sources map holds an address's sender-role state and the dests map its
// receiver-role state, so an address active in both roles has one state in each and
// neither axis can evict the other. Window-scoped fields (outBytes, inBytes, peers,
// ports) reset each Tick; the lifespan, the activity idle counter and srcAS carry
// across ticks.
type addrState struct {
	activity *stats.Window           // total bytes touching the entity (rate + idle)
	outBytes float64                 // bytes where the entity is the source (this window)
	inBytes  float64                 // bytes where the entity is the destination (this window)
	peers    map[netip.Addr]struct{} // distinct counterparties (this window)
	ports    map[uint16]float64      // destination-port byte histogram (this window)
	lifespan
	srcAS uint32 // origin AS of the entity, 0 if never attributed
}

// addPeer records one distinct counterparty, up to maxPeersPerEntity.
func (st *addrState) addPeer(peer netip.Addr) {
	if peer.IsValid() && len(st.peers) < maxPeersPerEntity {
		st.peers[peer] = struct{}{}
	}
}

// addPort folds bytes into the destination-port histogram. A port already tracked
// keeps accumulating past the cap; only a NEW port is refused, so the cap bounds
// cardinality without losing bytes.
func (st *addrState) addPort(port uint16, v float64) {
	if _, ok := st.ports[port]; ok {
		st.ports[port] += v
	} else if len(st.ports) < maxPortsPerEntity {
		st.ports[port] = v
	}
}

// portState is the bounded feature accumulator for one destination-PORT entity.
// Window-scoped fields (inBytes, outBytes, srcs) reset each Tick.
type portState struct {
	activity *stats.Window          // total bytes touching the port (rate + idle)
	inBytes  float64                // bytes sent TO the port (this window)
	outBytes float64                // bytes sent FROM the port (this window)
	srcs     map[netip.Addr]float64 // per-source byte histogram (this window)
	lifespan
}

// addSource folds bytes into the per-source histogram, which serves both the port's
// fan-out (its length) and its source entropy (its values). A source already
// tracked keeps accumulating past the cap.
func (st *portState) addSource(src netip.Addr, v float64) {
	if !src.IsValid() {
		return
	}
	if _, ok := st.srcs[src]; ok {
		st.srcs[src] += v
	} else if len(st.srcs) < maxSourcesPerPort {
		st.srcs[src] = v
	}
}

type aggregator struct {
	mu       sync.Mutex
	sources  map[netip.Addr]*addrState
	dests    map[netip.Addr]*addrState
	ports    map[PortKey]*portState
	tickNum  int
	lastSnap time.Time
	hasData  bool
}

func newAggregator() *aggregator {
	return &aggregator{
		sources: make(map[netip.Addr]*addrState),
		dests:   make(map[netip.Addr]*addrState),
		ports:   make(map[PortKey]*portState),
	}
}

// getOrCreate returns the entity's state in the SOURCE axis, creating it (bounded
// by maxTrackedKey) on first sight. Caller holds a.mu.
func (a *aggregator) getOrCreate(addr netip.Addr) *addrState {
	return getOrCreateAddr(a.sources, addr, maxTrackedKey, a.tickNum)
}

// destFor returns the entity's state in the DESTINATION axis, creating it (bounded
// by maxTrackedDest) on first sight. Caller holds a.mu.
func (a *aggregator) destFor(addr netip.Addr) *addrState {
	return getOrCreateAddr(a.dests, addr, maxTrackedDest, a.tickNum)
}

// getOrCreateAddr is the shared bounded-insert for both address axes: at the cap it
// returns nil rather than growing, and the caller drops the flow's contribution to
// that axis.
func getOrCreateAddr(m map[netip.Addr]*addrState, addr netip.Addr, limit, tick int) *addrState {
	if st, ok := m[addr]; ok {
		return st
	}
	if len(m) >= limit {
		return nil
	}
	st := &addrState{
		activity: stats.NewWindow(0),
		peers:    make(map[netip.Addr]struct{}),
		ports:    make(map[uint16]float64),
		lifespan: newLifespan(tick),
	}
	m[addr] = st
	return st
}

// portFor returns the port entity's state, creating it (bounded by maxTrackedPort)
// on first sight. Caller holds a.mu.
func (a *aggregator) portFor(k PortKey) *portState {
	if st, ok := a.ports[k]; ok {
		return st
	}
	if len(a.ports) >= maxTrackedPort {
		return nil
	}
	st := &portState{
		activity: stats.NewWindow(0),
		srcs:     make(map[netip.Addr]float64),
		lifespan: newLifespan(a.tickNum),
	}
	a.ports[k] = st
	return st
}

// ingest folds one flow observation into every axis it touches: the SOURCE address
// (out bytes, fan-out, ports), the DESTINATION address (in bytes, fan-in, the ports
// it was addressed on) and the destination PORT (in bytes, its source spread).
//
// Each axis also counts the bytes flowing the OTHER way, because every axis scores
// an asymmetry ratio. Those reverse folds never CREATE an entity: a pure sender
// must not occupy a slot in the destination map, and an ephemeral source port must
// not occupy one in the port map.
func (a *aggregator) ingest(obs observation.Observation) {
	if obs.Kind != observation.KindFlow || obs.Feature != observation.FeatureFlowBytes {
		return
	}
	v := obs.Value
	if v <= 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	src, dst := obs.Flow.Src, obs.Flow.Dst
	if src.IsValid() {
		if st := a.getOrCreate(src); st != nil {
			st.activity.AddDelta(v)
			st.outBytes += v
			// obs.SrcAS describes the flow's SOURCE, so it labels the entity only
			// in this branch. Keep the last attributed value: a later flow whose
			// AS lookup missed must not erase it.
			if obs.SrcAS != 0 {
				st.srcAS = obs.SrcAS
			}
			st.addPeer(dst)
			st.addPort(obs.Flow.DstPort, v)
			a.hasData = true
		}
		// A destination that is now sending: the denominator of its in/out ratio.
		// Lookup-only, so a pure sender never becomes a destination entity.
		if st := a.dests[src]; st != nil {
			st.activity.AddDelta(v)
			st.outBytes += v
		}
	}
	if dst.IsValid() {
		if st := a.getOrCreate(dst); st != nil {
			st.activity.AddDelta(v)
			st.inBytes += v
			a.hasData = true
		}
		if st := a.destFor(dst); st != nil {
			st.activity.AddDelta(v)
			st.inBytes += v
			st.addPeer(src)
			st.addPort(obs.Flow.DstPort, v)
			a.hasData = true
		}
		// The port entity is keyed by the DESTINATION port, and only when that port
		// is the flow's SERVICE side (isServicePort).
		if isServicePort(obs.Flow.SrcPort, obs.Flow.DstPort) {
			if st := a.portFor(PortKey{Port: obs.Flow.DstPort, Proto: obs.Flow.Proto}); st != nil {
				st.activity.AddDelta(v)
				st.inBytes += v
				st.addSource(src, v)
				a.hasData = true
			}
		}
	}
	// Bytes leaving a port already tracked: the numerator of its amplification
	// ratio. Lookup-only, so a client socket never becomes an entity.
	if st := a.ports[PortKey{Port: obs.Flow.SrcPort, Proto: obs.Flow.Proto}]; st != nil {
		st.activity.AddDelta(v)
		st.outBytes += v
	}
}

// snapshot closes the current window on every axis: it computes each entity's
// feature vector, records beaconing gaps, resets window-scoped state, evicts idle
// entities, and returns the busiest MaxTopN entities per axis.
func (a *aggregator) snapshot(now time.Time) *Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	dt := now.Sub(a.lastSnap).Seconds()
	if a.lastSnap.IsZero() || dt <= 0 {
		dt = tickInterval.Seconds()
	}
	a.lastSnap = now
	a.tickNum++

	snap := &Snapshot{Degraded: !a.hasData, At: now}
	if !a.hasData {
		return snap
	}
	snap.Sources = a.finalizeAddrs(a.sources, dt, dirSent)
	snap.Dests = a.finalizeAddrs(a.dests, dt, dirReceived)
	snap.Ports = a.finalizePorts(dt)
	return snap
}

// direction says which way an address axis reads. A source entity's OWN bytes are
// the ones it sent, a destination entity's the ones it received; that choice drives
// the emission gate, the asymmetry ratio's numerator, and the ranking key.
type direction int

const (
	dirSent direction = iota
	dirReceived
)

// finalizeAddrs closes one address axis: it emits a feature vector for every entity
// active in that axis's own direction, resets the window, evicts idle entities, and
// returns the busiest MaxTopN. Caller holds a.mu.
func (a *aggregator) finalizeAddrs(m map[netip.Addr]*addrState, dt float64, dir direction) []FeatureEntry {
	type ranked struct {
		fe    FeatureEntry
		bytes float64
	}
	live := make([]ranked, 0, len(m))

	for addr, st := range m {
		own, other := st.outBytes, st.inBytes
		if dir == dirReceived {
			own, other = st.inBytes, st.outBytes
		}
		// Emit an entity only when it was active in its OWN direction. The features
		// describe that direction, so an address on the wrong side of it (a pure
		// receiver on the source axis, a pure sender on the destination axis) would
		// carry all-zero features and dilute the ranking. Its bytes still count for
		// the other axis and for its own ratio. Idle eviction below tracks total
		// (in+out) activity, so a quiet entity is dropped whichever way it was busy.
		acted := own > 0
		st.activity.Tick(dt) // advance idle/rate (counts in+out) for eviction

		if acted {
			st.markActive(a.tickNum)
			live = append(live, ranked{
				fe: FeatureEntry{
					Addr:        addr,
					FanOut:      len(st.peers),
					OutInRatio:  ratio(own, other),
					PortEntropy: stats.Entropy(histValues(st.ports)),
					NewPeer:     a.tickNum-st.firstTick < newPeerTicks,
					RarePort:    rarePort(st.ports),
					Beaconing:   stats.IntervalRegularity(st.gaps),
					// Only the source axis is ever stamped with an AS (ingest), so
					// this reads 0 on the destination axis.
					SrcAS: st.srcAS,
				},
				bytes: own,
			})
		}

		// Reset window-scoped accumulators for the next window.
		st.outBytes = 0
		st.inBytes = 0
		clear(st.peers)
		clear(st.ports)

		if st.activity.Idle() > evictIdleTicks {
			delete(m, addr)
		}
	}

	sort.Slice(live, func(i, j int) bool { return live[i].bytes > live[j].bytes })
	n := min(MaxTopN, len(live))
	out := make([]FeatureEntry, n)
	for i := range n {
		out[i] = live[i].fe
	}
	return out
}

// finalizePorts closes the destination-port axis: it emits a feature vector for
// every port that RECEIVED this window, resets the window, evicts idle ports, and
// returns the busiest MaxTopN. Caller holds a.mu.
func (a *aggregator) finalizePorts(dt float64) []PortFeatureEntry {
	type ranked struct {
		pe    PortFeatureEntry
		bytes float64
	}
	live := make([]ranked, 0, len(a.ports))

	for key, st := range a.ports {
		// A port is an entity because traffic is AIMED at it, so receiving is what
		// makes it live this window. A port that only sent (a reply after its
		// inbound flow landed in an earlier window) carries no spread to report.
		received := st.inBytes > 0
		st.activity.Tick(dt)

		if received {
			st.markActive(a.tickNum)
			live = append(live, ranked{
				pe: PortFeatureEntry{
					PortKey:    key,
					FanOut:     len(st.srcs),
					OutInRatio: ratio(st.outBytes, st.inBytes),
					SrcEntropy: stats.Entropy(histValues(st.srcs)),
					NewPort:    a.tickNum-st.firstTick < newPeerTicks,
					RarePort:   portIsRare(key.Port),
					Beaconing:  stats.IntervalRegularity(st.gaps),
				},
				bytes: st.inBytes,
			})
		}

		st.inBytes = 0
		st.outBytes = 0
		clear(st.srcs)

		if st.activity.Idle() > evictIdleTicks {
			delete(a.ports, key)
		}
	}

	sort.Slice(live, func(i, j int) bool { return live[i].bytes > live[j].bytes })
	n := min(MaxTopN, len(live))
	out := make([]PortFeatureEntry, n)
	for i := range n {
		out[i] = live[i].pe
	}
	return out
}

// ratio returns out divided by in (the exfiltration signal). With no inbound
// bytes it is +Inf when there is outbound traffic, else 0.
func ratio(out, in float64) float64 {
	if in <= 0 {
		if out > 0 {
			return math.Inf(1)
		}
		return 0
	}
	return out / in
}

// histValues collects a byte histogram's weights for an entropy computation. The
// keys are irrelevant to entropy, so one helper serves the port histogram and the
// per-source histogram alike.
func histValues[K comparable](hist map[K]float64) []float64 {
	out := make([]float64, 0, len(hist))
	for _, b := range hist {
		out = append(out, b)
	}
	return out
}

// rarePort reports whether the entity's dominant destination port (most bytes) is
// outside the well-known allowlist. An entity with no port traffic is not rare.
func rarePort(ports map[uint16]float64) bool {
	var best uint16
	var bestBytes float64
	found := false
	for p, b := range ports {
		if b > bestBytes {
			bestBytes, best, found = b, p, true
		}
	}
	if !found {
		return false
	}
	return portIsRare(best)
}

// isServicePort reports whether a flow's DESTINATION port is the service side of
// the conversation, which is the only side that becomes a port entity.
//
// The rule is the lower of the two ports, the convention every flow analyzer uses:
// a reply's destination is the client's ephemeral socket, and letting those in would
// spend the port ceiling on client sockets and evict the services it exists to
// watch. A flow source that leaves the source port at 0 (ICMP, or an exporter that
// omits it) leaves the destination port as the only candidate, so it is kept.
//
// The limit is an attacker who spoofs a LOW source port to keep a swept port off
// this axis. The source and destination address axes still see that flow.
func isServicePort(srcPort, dstPort uint16) bool {
	return srcPort == 0 || dstPort <= srcPort
}

// portIsRare reports whether a port number is outside the well-known allowlist.
func portIsRare(p uint16) bool {
	_, common := commonPorts[p]
	return !common
}
