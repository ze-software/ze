// Design: docs/plugin-development/metrics.md -- Adj-RIB-In filtered route gauge and NLRI key error counter
// Related: register.go -- ConfigureMetrics wiring
// Related: rib.go -- AdjRIBInManager storage mutations that update these metrics
package adj_rib_in

import (
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
)

const (
	metricRoutesFiltered = "ze_adjribin_routes_filtered"
	metricNLRIKeyErrors  = "ze_adjribin_nlri_key_errors_total"
	metricLabelPeer      = "peer"
	metricLabelFamily    = "family"
	metricLabelOperation = "operation"
)

type adjRIBInMetrics struct {
	routesFiltered metrics.GaugeVec
	nlriKeyErrors  metrics.CounterVec
}

// metricsPtr stores Adj-RIB-In metrics, set by SetMetricsRegistry.
var metricsPtr atomic.Pointer[adjRIBInMetrics]

// SetMetricsRegistry registers the Adj-RIB-In Prometheus metrics on reg.
// Passing nil disables metric updates (used in tests).
func SetMetricsRegistry(reg metrics.Registry) {
	if reg == nil {
		metricsPtr.Store(nil)
		return
	}
	metricsPtr.Store(&adjRIBInMetrics{
		routesFiltered: reg.GaugeVec(metricRoutesFiltered, "Import-filtered Adj-RIB-In route count per peer.", []string{metricLabelPeer}),
		nlriKeyErrors: reg.CounterVec(
			metricNLRIKeyErrors,
			"Complex-family NLRIs whose Adj-RIB-In storage key could not be derived.",
			[]string{metricLabelPeer, metricLabelFamily, metricLabelOperation},
		),
	})
}

func incNLRIKeyErrorMetric(peer netip.Addr, fam family.Family, operation string) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	m.nlriKeyErrors.With(peer.String(), fam.String(), operation).Inc()
}

func setFilteredRoutesMetric(peer netip.Addr, count int) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	m.routesFiltered.With(peer.String()).Set(float64(count))
}

// adjustFilteredCountLocked updates the per-peer import-filtered route count
// and publishes ze_adjribin_routes_filtered. Caller MUST hold r.mu.
func (r *AdjRIBInManager) adjustFilteredCountLocked(addr netip.Addr, delta int) {
	if delta == 0 {
		return
	}
	if r.filteredByPeer == nil {
		r.filteredByPeer = make(map[netip.Addr]int)
	}
	next := max(r.filteredByPeer[addr]+delta, 0)
	if next == 0 {
		delete(r.filteredByPeer, addr)
	} else {
		r.filteredByPeer[addr] = next
	}
	setFilteredRoutesMetric(addr, next)
}

// putInstalledRouteLocked inserts or replaces an installed Adj-RIB-In entry
// and maintains the per-peer import-filtered gauge. Caller MUST hold r.mu.
func (r *AdjRIBInManager) putInstalledRouteLocked(peerAddr receivedPeer, rk compactRouteKey, route *RawRoute) {
	routes := r.ribIn[peerAddr]
	if routes == nil {
		routes = newSeqMap()
		r.ribIn[peerAddr] = routes
	}
	delta := 0
	if old, existed := routes.Get(rk); existed && old.ImportFiltered {
		delta--
	}
	if route.ImportFiltered {
		delta++
	}
	r.seqCounter++
	routes.Put(rk, r.seqCounter, route)
	if delta != 0 {
		r.adjustFilteredCountLocked(peerAddr.address, delta)
	}
}
