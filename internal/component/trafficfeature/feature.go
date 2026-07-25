// Design: plan/learned/1046-traffic-analysis-restructure.md -- per-source neutral feature aggregation
//
// Derives domain-NEUTRAL per-source feature signals from the observation feed via
// the shared internal/core/stats primitives. These are FACTS (measurable numbers),
// never verdicts: detection plugins (anomaly, ddos) apply judgment on top.

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
	// evictIdleTicks drops an entity after this many consecutive quiet ticks.
	evictIdleTicks = 10
	// maxDestsPerSource / maxPortsPerSource bound per-entity fan-out and port
	// histogram cardinality so one scanning source cannot exhaust memory.
	maxDestsPerSource = 4096
	maxPortsPerSource = 1024
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

// sourceState is the bounded per-entity feature accumulator. Window-scoped fields
// (outBytes, inBytes, dests, ports) reset each Tick; persistent fields (firstTick,
// lastActiveTick, gaps, activity idle) carry across ticks.
type sourceState struct {
	activity       *stats.Window           // total bytes touching the entity (rate + idle)
	outBytes       float64                 // bytes where the entity is the source (this window)
	inBytes        float64                 // bytes where the entity is the destination (this window)
	dests          map[netip.Addr]struct{} // distinct destinations (this window)
	ports          map[uint16]float64      // destination-port byte histogram (this window)
	firstTick      int                     // tick when first observed (new-peer)
	lastActiveTick int                     // last tick with traffic (beaconing gaps)
	gaps           []float64               // recent active-tick gaps in seconds (bounded)
}

func (st *sourceState) pushGap(g float64) {
	if len(st.gaps) < gapHistoryLen {
		st.gaps = append(st.gaps, g)
		return
	}
	copy(st.gaps, st.gaps[1:])
	st.gaps[len(st.gaps)-1] = g
}

type aggregator struct {
	mu       sync.Mutex
	sources  map[netip.Addr]*sourceState
	tickNum  int
	lastSnap time.Time
	hasData  bool
}

func newAggregator() *aggregator {
	return &aggregator{sources: make(map[netip.Addr]*sourceState)}
}

// getOrCreate returns the entity's state, creating it (bounded by maxTrackedKey)
// on first sight. Caller holds a.mu.
func (a *aggregator) getOrCreate(addr netip.Addr) *sourceState {
	if st, ok := a.sources[addr]; ok {
		return st
	}
	if len(a.sources) >= maxTrackedKey {
		return nil
	}
	st := &sourceState{
		activity:       stats.NewWindow(0),
		dests:          make(map[netip.Addr]struct{}),
		ports:          make(map[uint16]float64),
		firstTick:      a.tickNum,
		lastActiveTick: -1,
	}
	a.sources[addr] = st
	return st
}

// ingest folds one flow observation into both role entities: the source (out
// bytes, fan-out, ports) and the destination (in bytes).
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
			if dst.IsValid() && len(st.dests) < maxDestsPerSource {
				st.dests[dst] = struct{}{}
			}
			if _, ok := st.ports[obs.Flow.DstPort]; ok {
				st.ports[obs.Flow.DstPort] += v
			} else if len(st.ports) < maxPortsPerSource {
				st.ports[obs.Flow.DstPort] = v
			}
			a.hasData = true
		}
	}
	if dst.IsValid() {
		if st := a.getOrCreate(dst); st != nil {
			st.activity.AddDelta(v)
			st.inBytes += v
			a.hasData = true
		}
	}
}

// snapshot closes the current window: it computes each entity's feature vector,
// records beaconing gaps, resets window-scoped state, evicts idle entities, and
// returns the top-N most active entities.
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

	type ranked struct {
		fe   FeatureEntry
		sent float64
	}
	live := make([]ranked, 0, len(a.sources))

	for addr, st := range a.sources {
		// Emit an entity only when it acted as a SOURCE this window. The features
		// are source-behavior signals, so a pure receiver (inbound only) would
		// carry all-zero features and dilute the ranking; its inbound bytes are
		// still counted for other sources' out/in ratio. Idle eviction below still
		// tracks total (in+out) activity so quiet receivers are dropped.
		sent := st.outBytes > 0
		st.activity.Tick(dt) // advance idle/rate (counts in+out) for eviction

		if sent {
			if st.lastActiveTick >= 0 {
				st.pushGap(float64(a.tickNum - st.lastActiveTick))
			}
			st.lastActiveTick = a.tickNum
			live = append(live, ranked{
				fe: FeatureEntry{
					Addr:        addr,
					FanOut:      len(st.dests),
					OutInRatio:  ratio(st.outBytes, st.inBytes),
					PortEntropy: stats.Entropy(portValues(st.ports)),
					NewPeer:     a.tickNum-st.firstTick < newPeerTicks,
					RarePort:    rarePort(st.ports),
					Beaconing:   stats.IntervalRegularity(st.gaps),
				},
				sent: st.outBytes,
			})
		}

		// Reset window-scoped accumulators for the next window.
		st.outBytes = 0
		st.inBytes = 0
		clear(st.dests)
		clear(st.ports)

		if st.activity.Idle() > evictIdleTicks {
			delete(a.sources, addr)
		}
	}

	sort.Slice(live, func(i, j int) bool { return live[i].sent > live[j].sent })
	n := min(MaxTopN, len(live))
	snap.Sources = make([]FeatureEntry, n)
	for i := range n {
		snap.Sources[i] = live[i].fe
	}
	return snap
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

func portValues(ports map[uint16]float64) []float64 {
	out := make([]float64, 0, len(ports))
	for _, b := range ports {
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
	_, common := commonPorts[best]
	return !common
}
