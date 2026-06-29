// Design: plan/learned/1019-traffic-usage-monitor.md -- per-key rolling window and rate derivation

package trafficstat

import (
	"net/netip"
	"sort"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/observation"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// evictIdleTicks is the number of consecutive zero-rate ticks after which a
// tracked key is dropped, bounding memory under source churn (AC-10).
const evictIdleTicks = 10

type portKey struct {
	port  uint16
	proto uint8
}

// entry accumulates the bytes attributed to one tracked key during the CURRENT
// (not-yet-snapshotted) tick window, and remembers the last cumulative counter
// value for cumulative features. snapshot() converts windowBytes into a rate and
// resets it, so a key that stops receiving traffic decays to 0 rather than
// reporting a stale or ever-growing value.
type entry struct {
	windowBytes float64 // bytes seen since the last snapshot
	lastCumul   float64 // last cumulative counter value (cumulative features only)
	hasCumul    bool
	bps         float64 // rate computed at the last snapshot
	idle        int     // consecutive snapshots with zero windowBytes
}

type aggregator struct {
	mu sync.Mutex

	sources map[netip.Addr]*entry // by source IP
	dests   map[netip.Addr]*entry // by destination IP
	ports   map[portKey]*entry    // by (dest-port, proto)
	protos  map[uint8]*entry      // by IP protocol number

	lastSnap     time.Time
	historyRing  [60]float64
	historyIdx   int
	historyCount int
	hasData      bool
}

func newAggregator() *aggregator {
	return &aggregator{
		sources: make(map[netip.Addr]*entry),
		dests:   make(map[netip.Addr]*entry),
		ports:   make(map[portKey]*entry),
		protos:  make(map[uint8]*entry),
	}
}

func (a *aggregator) ingest(obs observation.Observation) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch obs.Kind {
	case observation.KindSourceIP:
		// trafficusage per-source-IP: cumulative byte counter.
		if obs.Feature == observation.FeatureRxBytes {
			a.cumulative(a.sources, obs.Flow.Src, obs.Value)
		}
	case observation.KindDestIP:
		if obs.Feature == observation.FeatureRxBytes {
			a.cumulative(a.dests, obs.Flow.Dst, obs.Value)
		}
	case observation.KindFlow:
		// flowexport conntrack: per-publish byte delta, full 5-tuple.
		if obs.Feature == observation.FeatureFlowBytes {
			a.delta(a.sources, obs.Flow.Src, obs.Value)
			a.delta(a.dests, obs.Flow.Dst, obs.Value)
			a.deltaPort(portKey{port: obs.Flow.DstPort, proto: obs.Flow.Proto}, obs.Value)
			a.deltaProto(obs.Flow.Proto, obs.Value)
		}
	case observation.KindInterface:
		// Interface rates are read from iface.ListRates, not the feed.
	}
}

// cumulative folds a cumulative counter sample into the current window: the
// per-window contribution is (value - lastValue), clamped at 0 across a counter
// reset. The first sample only primes lastCumul.
func (a *aggregator) cumulative(m map[netip.Addr]*entry, addr netip.Addr, value float64) {
	if !addr.IsValid() {
		return
	}
	e := getOrCreate(m, addr)
	if e == nil {
		return
	}
	if e.hasCumul && value >= e.lastCumul {
		e.windowBytes += value - e.lastCumul
	}
	e.lastCumul = value
	e.hasCumul = true
	a.hasData = true
}

// delta adds a per-publish byte delta to the current window.
func (a *aggregator) delta(m map[netip.Addr]*entry, addr netip.Addr, value float64) {
	if !addr.IsValid() || value <= 0 {
		return
	}
	e := getOrCreate(m, addr)
	if e == nil {
		return
	}
	e.windowBytes += value
	a.hasData = true
}

func (a *aggregator) deltaProto(proto uint8, value float64) {
	if value <= 0 {
		return
	}
	e := getOrCreate(a.protos, proto)
	if e == nil {
		return
	}
	e.windowBytes += value
	a.hasData = true
}

func (a *aggregator) deltaPort(k portKey, value float64) {
	if value <= 0 {
		return
	}
	e := getOrCreate(a.ports, k)
	if e == nil {
		return
	}
	e.windowBytes += value
	a.hasData = true
}

func getOrCreate[K comparable](m map[K]*entry, k K) *entry {
	if e, ok := m[k]; ok {
		return e
	}
	if len(m) >= maxTrackedKey {
		return nil
	}
	e := &entry{}
	m[k] = e
	return e
}

func (a *aggregator) snapshot(now time.Time, ifaces []InterfaceEntry) *Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	dt := now.Sub(a.lastSnap).Seconds()
	if a.lastSnap.IsZero() || dt <= 0 {
		dt = tickInterval.Seconds()
	}
	a.lastSnap = now

	snap := &Snapshot{
		Interfaces: ifaces,
		Degraded:   !a.hasData,
		At:         now,
	}
	if !a.hasData {
		return snap
	}

	snap.TopSourceIPs = finalizeAddrs(a.sources, dt)
	snap.TopDestIPs = finalizeAddrs(a.dests, dt)
	snap.TopPorts = finalizePorts(a.ports, dt)
	snap.Protocols = finalizeProtos(a.protos, dt)

	var totalBps float64
	for i := range snap.TopSourceIPs {
		totalBps += snap.TopSourceIPs[i].Bps
	}
	a.historyRing[a.historyIdx] = totalBps
	a.historyIdx = (a.historyIdx + 1) % len(a.historyRing)
	if a.historyCount < len(a.historyRing) {
		a.historyCount++
	}
	snap.Severity = a.computeSeverity(totalBps)

	snap.History = make([]float64, a.historyCount)
	for i := range a.historyCount {
		idx := (a.historyIdx - a.historyCount + i + len(a.historyRing)) % len(a.historyRing)
		snap.History[i] = a.historyRing[idx]
	}

	return snap
}

// finalizeAddrs converts each key's accumulated window bytes into a per-second
// rate, resets the window, ages out idle keys, and returns the top-N by rate.
func finalizeAddrs(m map[netip.Addr]*entry, dt float64) []TalkerEntry {
	type kv struct {
		addr netip.Addr
		bps  float64
	}
	live := make([]kv, 0, len(m))
	for addr, e := range m {
		e.bps = e.windowBytes / dt
		if e.windowBytes <= 0 {
			e.idle++
		} else {
			e.idle = 0
		}
		e.windowBytes = 0
		if e.bps > 0 {
			live = append(live, kv{addr: addr, bps: e.bps})
		}
		if e.idle > evictIdleTicks {
			delete(m, addr)
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].bps > live[j].bps })
	n := min(MaxTopN, len(live))
	out := make([]TalkerEntry, n)
	for i := range n {
		out[i] = TalkerEntry{Addr: live[i].addr, Bps: live[i].bps}
	}
	return out
}

func finalizePorts(m map[portKey]*entry, dt float64) []PortEntry {
	type kv struct {
		key portKey
		bps float64
	}
	live := make([]kv, 0, len(m))
	for k, e := range m {
		e.bps = e.windowBytes / dt
		if e.windowBytes <= 0 {
			e.idle++
		} else {
			e.idle = 0
		}
		e.windowBytes = 0
		if e.bps > 0 {
			live = append(live, kv{key: k, bps: e.bps})
		}
		if e.idle > evictIdleTicks {
			delete(m, k)
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].bps > live[j].bps })
	n := min(MaxTopN, len(live))
	out := make([]PortEntry, n)
	for i := range n {
		out[i] = PortEntry{Port: live[i].key.port, Proto: live[i].key.proto, Bps: live[i].bps}
	}
	return out
}

var protoNames = map[uint8]string{
	1: "ICMP", 2: "IGMP", 6: "TCP", 17: "UDP", 47: "GRE",
	50: "ESP", 51: "AH", 58: "ICMPv6", 89: "OSPF", 103: "PIM",
	132: "SCTP",
}

func finalizeProtos(m map[uint8]*entry, dt float64) []ProtocolMix {
	type kv struct {
		proto uint8
		bps   float64
	}
	var total float64
	live := make([]kv, 0, len(m))
	for proto, e := range m {
		e.bps = e.windowBytes / dt
		if e.windowBytes <= 0 {
			e.idle++
		} else {
			e.idle = 0
		}
		e.windowBytes = 0
		if e.bps > 0 {
			live = append(live, kv{proto: proto, bps: e.bps})
			total += e.bps
		}
		if e.idle > evictIdleTicks {
			delete(m, proto)
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].bps > live[j].bps })
	out := make([]ProtocolMix, len(live))
	for i, kv := range live {
		name := protoNames[kv.proto]
		if name == "" {
			name = textbuf.StringUint8(kv.proto)
		}
		var pct float64
		if total > 0 {
			pct = kv.bps / total * 100
		}
		out[i] = ProtocolMix{Proto: kv.proto, Name: name, Bps: kv.bps, Pct: pct}
	}
	return out
}

func (a *aggregator) computeSeverity(totalBps float64) Severity {
	if a.historyCount < 5 {
		return SeverityNormal
	}
	var sum float64
	for i := range a.historyCount {
		sum += a.historyRing[i]
	}
	avg := sum / float64(a.historyCount)
	if avg <= 0 {
		return SeverityNormal
	}
	ratio := totalBps / avg
	switch {
	case ratio > 5:
		return SeverityDanger
	case ratio > 2:
		return SeverityCaution
	default:
		return SeverityNormal
	}
}
