// Design: docs/architecture/ospf/ospf-ext-1-opaque-framework.md -- RFC 5250 opaque carrier engine glue.
// RFC: rfc/short/rfc5250.md -- §3 origination/reception, §5 Type-11 reachability.
//
// This is the seam between the process-global opaque consumer registry
// (opaque_registry.go) and the scope-aware LSDB carrier (lsdb package): it drives
// consumer origination from the self-LSA origination pass, delivers received opaque LSAs
// to their registered consumer after a newer install, applies the §5 Type-11 originator
// reachability gate, and owns the five ze_ospf_opaque_* metric series.

package ospf

import (
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/textbuf"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// gaugeSample is one labeled value applied to a GaugeVec on a metric refresh.
type gaugeSample struct {
	labels []string
	value  float64
}

// gaugeVecTracker zeros vanished label-sets on a GaugeVec across refreshes. metrics.GaugeVec
// exposes no Reset(), so when a labeled population shrinks (an opaque type drains, a TE link
// area empties, an interface is removed) the stale series would otherwise keep its last value
// forever. The tracker remembers the label tuples set on the previous refresh and Sets any
// now-absent tuple to 0. It is guarded because the origination pass and the reception path
// can refresh concurrently.
type gaugeVecTracker struct {
	mu   sync.Mutex
	seen map[string][]string
}

func newGaugeVecTracker() *gaugeVecTracker { return &gaugeVecTracker{seen: map[string][]string{}} }

// apply Sets every sample on gv, then Sets to 0 every label tuple that was present on the
// previous apply but is absent now, so a drained population reports 0 rather than a stale value.
func (g *gaugeVecTracker) apply(gv metrics.GaugeVec, samples []gaugeSample) {
	g.mu.Lock()
	defer g.mu.Unlock()
	next := make(map[string][]string, len(samples))
	for _, s := range samples {
		gv.With(s.labels...).Set(s.value)
		next[strings.Join(s.labels, "\x00")] = s.labels
	}
	for k, prev := range g.seen {
		if _, ok := next[k]; !ok {
			gv.With(prev...).Set(0)
		}
	}
	g.seen = next
}

// opaqueMetrics is the RFC 5250 carrier's metric surface (spec-ospf-ext-1). It is
// registered by this component, not by ospf-13.
type opaqueMetrics struct {
	lsas         metrics.GaugeVec   // labels: scope, opaque_type
	originations metrics.CounterVec // labels: opaque_type
	received     metrics.CounterVec // labels: opaque_type, registered
	consumerEr   metrics.CounterVec // labels: opaque_type
	capableNbrs  metrics.GaugeVec   // labels: interface
}

func nopOpaqueMetrics() opaqueMetrics {
	nop := metrics.NopRegistry{}
	return opaqueMetrics{
		lsas:         nop.GaugeVec("", "", nil),
		originations: nop.CounterVec("", "", nil),
		received:     nop.CounterVec("", "", nil),
		consumerEr:   nop.CounterVec("", "", nil),
		capableNbrs:  nop.GaugeVec("", "", nil),
	}
}

func (e *engine) setOpaqueMetrics(reg metrics.Registry) {
	e.opaque = opaqueMetrics{
		lsas:         reg.GaugeVec("ze_ospf_opaque_lsas", "Current OSPF opaque LSAs in the database, by flooding scope and opaque type.", []string{labelScope, labelOpaqueType}),
		originations: reg.CounterVec("ze_ospf_opaque_originations_total", "Total OSPF opaque LSAs originated by a registered consumer, by opaque type.", []string{labelOpaqueType}),
		received:     reg.CounterVec("ze_ospf_opaque_received_total", "Total newer OSPF opaque LSAs received, by opaque type and whether a consumer is registered.", []string{labelOpaqueType, "registered"}),
		consumerEr:   reg.CounterVec("ze_ospf_opaque_consumer_errors_total", "Total OSPF opaque consumer callback panics recovered by the carrier, by opaque type.", []string{labelOpaqueType}),
		capableNbrs:  reg.GaugeVec("ze_ospf_opaque_capable_neighbors", "Current opaque-capable OSPF neighbors (O-bit set in their DD), by interface.", []string{labelInterface}),
	}
	// Fresh trackers for the newly bound gauges, so a label set drained under the previous
	// (nop) registry does not leak a stale zeroing onto the real series.
	e.opaqueLSAsGauge = newGaugeVecTracker()
	e.capableNbrsGauge = newGaugeVecTracker()
}

func opaqueTypeLabel(opaqueType uint8) string { return textbuf.StringUint8(opaqueType) }

func registeredLabel(registered bool) string {
	if registered {
		return "true"
	}
	return "false"
}

// wireOpaqueDelivery connects the LSDB's newer-opaque-install hook to this engine.
func (e *engine) wireOpaqueDelivery() {
	if e.lsdb != nil {
		e.lsdb.SetOpaqueDelivery(e.deliverOpaque)
	}
}

// deliverOpaque is the LSDB reception hook: it counts the receive, delivers a registered
// consumer's OnReceive (with the §5 reachability flag), and refreshes the population
// gauge. It runs outside the LSDB lock. An unregistered opaque type is counted and
// dropped (already stored + reflooded by the carrier) -- never delivered (AC-12).
func (e *engine) deliverOpaque(d ospflsdb.OpaqueDelivery) {
	consumer, registered := lookupOpaqueConsumer(d.OpaqueType)
	label := opaqueTypeLabel(d.OpaqueType)
	e.opaque.received.With(label, registeredLabel(registered)).Inc()
	e.refreshOpaqueMetrics()
	if !registered || consumer.onReceive == nil {
		return
	}
	// RFC 5250 Section 5: a Type-11 opaque LSA is usable only if the originating router is
	// reachable; Type 9/10 are always reachable (link/area local). The consumer receives
	// the flag and must not use an unreachable-originator LSA.
	reachable := true
	if d.Scope == types.LSTypeOpaqueAS {
		reachable = e.routerReachable(d.AdvertisingRouter)
	}
	rcv := opaqueReceived{
		OpaqueType:        d.OpaqueType,
		OpaqueID:          d.OpaqueID,
		Scope:             OpaqueScope(d.Scope),
		Area:              d.Area,
		Interface:         d.Interface,
		AdvertisingRouter: d.AdvertisingRouter,
		Body:              d.Body,
		Age:               d.Age,
		Reachable:         reachable,
		Withdrawn:         d.Withdrawn,
	}
	safeOpaqueCall(func() { e.opaque.consumerEr.With(label).Inc() }, func() { consumer.onReceive(rcv) })
}

// originateOpaqueLSAs invokes every registered consumer's OnOriginate and installs +
// floods the opaque LSAs it returns (RFC 5250 §3). It runs from originateSelfLSAs when
// opaque capability is enabled. A panicking consumer is isolated and counted.
func (e *engine) originateOpaqueLSAs(router types.RouterID) {
	if e.lsdb == nil {
		return
	}
	for _, c := range opaqueConsumerSnapshot() {
		if c.onOriginate == nil {
			continue
		}
		label := opaqueTypeLabel(c.opaqueType)
		var reqs []opaqueOrigination
		safeOpaqueCall(func() { e.opaque.consumerEr.With(label).Inc() }, func() { reqs = c.onOriginate(router) })
		for _, req := range reqs {
			// A per-origination Scope overrides the consumer's registered scope when set
			// (RFC 5392 sec 3.1.1 inter-AS TE floods some links Type 10, others Type 11);
			// zero means "use the registered scope".
			scope := c.scope
			if req.Scope.valid() {
				scope = req.Scope
			}
			_, ok := e.lsdb.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
				Router:     router,
				OpaqueType: c.opaqueType,
				OpaqueID:   req.OpaqueID,
				Scope:      scope.lsType(),
				Area:       req.Area,
				Interface:  req.Interface,
				Options:    types.OptionO,
				Body:       req.Body,
				Withdraw:   req.Withdraw,
			})
			if ok && !req.Withdraw {
				e.opaque.originations.With(label).Inc()
			}
		}
	}
	e.refreshOpaqueMetrics()
}

// opaqueCountSample renders one opaque-population bucket as a gauge sample. Its parameter
// names the carrier's cross-package ospflsdb.OpaqueLSACount, the element type of
// OpaqueLSACounts(), so the LSDB type stays exported for this and any future consumer.
func opaqueCountSample(c ospflsdb.OpaqueLSACount) gaugeSample {
	return gaugeSample{
		labels: []string{OpaqueScope(c.Scope).String(), opaqueTypeLabel(c.OpaqueType)},
		value:  float64(c.Count),
	}
}

// refreshOpaqueMetrics sets the opaque population and opaque-capable-neighbor gauges from
// current state. Cheap and called on each origination pass (per change + per second). Each
// gauge is set through a tracker so a label set that drains (an opaque type or an interface
// disappears) is zeroed rather than left at a stale value.
func (e *engine) refreshOpaqueMetrics() {
	if e.lsdb != nil {
		counts := e.lsdb.OpaqueLSACounts()
		samples := make([]gaugeSample, 0, len(counts))
		for _, c := range counts {
			samples = append(samples, opaqueCountSample(c))
		}
		e.opaqueLSAsGauge.apply(e.opaque.lsas, samples)
	}
	if e.neighbors == nil {
		return
	}
	e.mu.Lock()
	names := make([]string, 0, len(e.running))
	for name := range e.running {
		names = append(names, name)
	}
	e.mu.Unlock()
	samples := make([]gaugeSample, 0, len(names))
	for _, name := range names {
		count := 0
		for _, n := range e.neighbors.FloodNeighbors(name) {
			if n.OpaqueCapable {
				count++
			}
		}
		samples = append(samples, gaugeSample{labels: []string{name}, value: float64(count)})
	}
	e.capableNbrsGauge.apply(e.opaque.capableNbrs, samples)
}

// routerReachable reports whether an originating router is reachable, for the §5 Type-11
// gate. It uses the SPF reachability seam (set in newEngine to the SPF route table);
// tests may inject an alternative.
func (e *engine) routerReachable(id types.RouterID) bool {
	if e.opaqueReachableFn == nil {
		return false
	}
	return e.opaqueReachableFn(id)
}

// spfRouterReachable is the production §5 reachability source: a router is reachable when
// the SPF computer reports it reachable (reusing the ASBR reachability already computed
// for Type-5 AS-External LSAs, RFC 5250 §5).
func (e *engine) spfRouterReachable(id types.RouterID) bool {
	if e.spf == nil {
		return false
	}
	return e.spf.RouterReachable(id)
}
