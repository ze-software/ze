// Design: docs/architecture/traffic/traffic-analysis-layers.md -- per-key rolling window and rate derivation
//
// The per-key accumulation (windowed bytes -> rate, reset, idle eviction, bounded
// history) is provided by the shared internal/core/stats.Window primitive; this
// file keeps only the trafficstat-specific aggregation (four keyed maps, top-N
// finalization, the global total-rate history). The neutral layer holds NO verdict:
// severity is a display concern computed by the CLI from Snapshot.History.

package trafficstat

import (
	"net/netip"
	"sort"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/observation"
	"github.com/ze-software/ze/internal/core/stats"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// evictIdleTicks is the number of consecutive zero-rate ticks after which a
// tracked key is dropped, bounding memory under source churn (AC-10).
const evictIdleTicks = 10

// sourceHistoryLen is how many per-tick rate samples are retained per tracked
// source/dest key so a consumer can baseline a key against its own recent history.
// 60 samples ~= one minute at the 1s tick.
const sourceHistoryLen = 60

type portKey struct {
	port  uint16
	proto uint8
}

type aggregator struct {
	mu sync.Mutex

	sources map[netip.Addr]*stats.Window // by source IP (with history)
	dests   map[netip.Addr]*stats.Window // by destination IP (with history)
	ports   map[portKey]*stats.Window    // by (dest-port, proto)
	protos  map[uint8]*stats.Window      // by IP protocol number

	lastSnap     time.Time
	historyRing  [60]float64
	historyIdx   int
	historyCount int
	hasData      bool
}

func newAggregator() *aggregator {
	return &aggregator{
		sources: make(map[netip.Addr]*stats.Window),
		dests:   make(map[netip.Addr]*stats.Window),
		ports:   make(map[portKey]*stats.Window),
		protos:  make(map[uint8]*stats.Window),
	}
}

func (a *aggregator) ingest(obs observation.Observation) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch obs.Kind {
	case observation.KindSourceIP:
		// trafficusage per-source-IP: cumulative byte counter.
		if obs.Feature == observation.FeatureRxBytes {
			a.cumulative(a.sources, obs.Flow.Src, obs.Value, sourceHistoryLen)
		}
	case observation.KindDestIP:
		if obs.Feature == observation.FeatureRxBytes {
			a.cumulative(a.dests, obs.Flow.Dst, obs.Value, sourceHistoryLen)
		}
	case observation.KindFlow:
		// flowexport conntrack: per-publish byte delta, full 5-tuple.
		if obs.Feature == observation.FeatureFlowBytes {
			a.delta(a.sources, obs.Flow.Src, obs.Value, sourceHistoryLen)
			a.delta(a.dests, obs.Flow.Dst, obs.Value, sourceHistoryLen)
			a.deltaPort(portKey{port: obs.Flow.DstPort, proto: obs.Flow.Proto}, obs.Value)
			a.deltaProto(obs.Flow.Proto, obs.Value)
		}
	case observation.KindInterface:
		// Interface rates are read from iface.ListRates, not the feed.
	}
}

// cumulative folds a cumulative counter sample into the current window.
func (a *aggregator) cumulative(m map[netip.Addr]*stats.Window, addr netip.Addr, value float64, histCap int) {
	if !addr.IsValid() {
		return
	}
	w := getOrCreate(m, addr, histCap)
	if w == nil {
		return
	}
	w.AddCumulative(value)
	a.hasData = true
}

// delta adds a per-publish byte delta to the current window.
func (a *aggregator) delta(m map[netip.Addr]*stats.Window, addr netip.Addr, value float64, histCap int) {
	if !addr.IsValid() || value <= 0 {
		return
	}
	w := getOrCreate(m, addr, histCap)
	if w == nil {
		return
	}
	w.AddDelta(value)
	a.hasData = true
}

func (a *aggregator) deltaProto(proto uint8, value float64) {
	if value <= 0 {
		return
	}
	w := getOrCreate(a.protos, proto, 0)
	if w == nil {
		return
	}
	w.AddDelta(value)
	a.hasData = true
}

func (a *aggregator) deltaPort(k portKey, value float64) {
	if value <= 0 {
		return
	}
	w := getOrCreate(a.ports, k, 0)
	if w == nil {
		return
	}
	w.AddDelta(value)
	a.hasData = true
}

func getOrCreate[K comparable](m map[K]*stats.Window, k K, histCap int) *stats.Window {
	if w, ok := m[k]; ok {
		return w
	}
	if len(m) >= maxTrackedKey {
		return nil
	}
	w := stats.NewWindow(histCap)
	m[k] = w
	return w
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

	snap.History = make([]float64, a.historyCount)
	for i := range a.historyCount {
		idx := (a.historyIdx - a.historyCount + i + len(a.historyRing)) % len(a.historyRing)
		snap.History[i] = a.historyRing[idx]
	}

	return snap
}

// finalizeAddrs closes each key's window into a per-second rate, ages out idle
// keys, and returns the top-N by rate with their bounded rate history.
func finalizeAddrs(m map[netip.Addr]*stats.Window, dt float64) []TalkerEntry {
	type kv struct {
		addr netip.Addr
		w    *stats.Window
	}
	live := make([]kv, 0, len(m))
	for addr, w := range m {
		if w.Tick(dt) > 0 {
			live = append(live, kv{addr: addr, w: w})
		}
		if w.Idle() > evictIdleTicks {
			delete(m, addr)
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].w.Bps() > live[j].w.Bps() })
	n := min(MaxTopN, len(live))
	out := make([]TalkerEntry, n)
	for i := range n {
		out[i] = TalkerEntry{Addr: live[i].addr, Bps: live[i].w.Bps(), History: live[i].w.History()}
	}
	return out
}

func finalizePorts(m map[portKey]*stats.Window, dt float64) []PortEntry {
	type kv struct {
		key portKey
		bps float64
	}
	live := make([]kv, 0, len(m))
	for k, w := range m {
		if bps := w.Tick(dt); bps > 0 {
			live = append(live, kv{key: k, bps: bps})
		}
		if w.Idle() > evictIdleTicks {
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

func finalizeProtos(m map[uint8]*stats.Window, dt float64) []ProtocolMix {
	type kv struct {
		proto uint8
		bps   float64
	}
	var total float64
	live := make([]kv, 0, len(m))
	for proto, w := range m {
		if bps := w.Tick(dt); bps > 0 {
			live = append(live, kv{proto: proto, bps: bps})
			total += bps
		}
		if w.Idle() > evictIdleTicks {
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
