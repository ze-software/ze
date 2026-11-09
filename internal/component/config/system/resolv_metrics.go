// Design: docs/features/dns-resolver.md -- the dns-source metrics
// Related: resolv.go -- Resolvers, whose snapshots and publications these count

package system

import (
	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/metrics"
)

// routerConsumer is the consumer label of the router's own selection, the
// name show system dns lists it under.
const routerConsumer = "system"

// resolverMetrics are the registry's collectors. observe runs only from the
// registry's watcher delivery, which notifyMu serializes, so origins and
// router need no lock of their own; selectionChanged is called with r.mu
// held and touches only the counter, which is safe for concurrent use.
type resolverMetrics struct {
	records metrics.GaugeVec
	changes metrics.CounterVec
	// origins are the origins whose gauge was last set above zero, so an
	// origin whose records all left is set back to zero.
	origins map[string]bool
	// router is the router selection last observed; observed is false until
	// the first snapshot, which is the start and not a change.
	router   dnssource.ConsumerSelection
	observed bool
}

// SetMetrics creates the registry's collectors on reg and starts feeding
// them from every snapshot: ze_dns_source_records{origin} counts the learned
// records each origin holds, and ze_dns_source_selection_changes_total
// {consumer} counts each change of a consumer's published selection after
// its first (the router's under consumer system). It MUST be called once;
// the collectors live as long as the process and the registry.
func (r *Resolvers) SetMetrics(reg metrics.Registry) {
	m := &resolverMetrics{
		records: reg.GaugeVec("ze_dns_source_records",
			"Learned resolver records the system-dns registry holds, by the origin that learned them.",
			[]string{"origin"}),
		changes: reg.CounterVec("ze_dns_source_selection_changes_total",
			"Changes of a consumer's dns-source selection after its first, by consumer.",
			[]string{"consumer"}),
		origins: make(map[string]bool),
	}
	r.mu.Lock()
	r.metrics = m
	r.mu.Unlock()
	// The watch is never stopped: the registry and the collectors share the
	// process's lifetime.
	r.Watch(m.observe)
}

// observe sets the record gauges from snapshot and counts a change of the
// router's selection.
func (m *resolverMetrics) observe(snapshot *dnssource.Snapshot) {
	counts := make(map[string]int)
	for i := range snapshot.Records {
		counts[snapshot.Records[i].Origin]++
	}
	for origin := range m.origins {
		if counts[origin] == 0 {
			m.records.With(origin).Set(0)
			delete(m.origins, origin)
		}
	}
	for origin, count := range counts {
		m.records.With(origin).Set(float64(count))
		m.origins[origin] = true
	}
	if m.observed && !m.router.Equal(&snapshot.Router) {
		m.selectionChanged(routerConsumer)
	}
	m.router = snapshot.Router
	m.observed = true
}

// selectionChanged counts one change of consumer's selection.
func (m *resolverMetrics) selectionChanged(consumer string) {
	m.changes.With(consumer).Inc()
}
