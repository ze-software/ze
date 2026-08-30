// Design: docs/architecture/ospf/ospf-ext-14-debug-introspection.md -- the six debug metric series.
// RFC: rfc/short/rfc5250.md (IPv4 opaque), rfc/short/rfc5340.md (IPv6 native).
//
// The debug/introspection surface owns exactly six Prometheus series, three per address
// family, following the ext-0 ze_ospf_<ext>_* / ze_ospfv3_<ext>_* contract. They are
// process-global and registered ONCE (a sync.Once, like sr_metrics.go), NOT through the
// per-engine setMetrics path -- the v6 ze_ospfv3_debug_* series would otherwise trip the
// ze_ospf_-only naming guards on setMetrics/setBFDMetrics (learned 970). This file renames
// no existing OSPF series.

package ospf

import (
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

// debugMetricsSet holds the ext-14 debug metric series (both address families).
type debugMetricsSet struct {
	v4Injected metrics.GaugeVec   // scope
	v4Inject   metrics.CounterVec // scope, action
	v4Decode   metrics.CounterVec // opaque_type
	v6Injected metrics.GaugeVec   // scope
	v6Inject   metrics.CounterVec // scope, action
	v6Decode   metrics.CounterVec // ls_type
}

func newDebugMetrics(reg metrics.Registry) *debugMetricsSet {
	if reg == nil {
		reg = metrics.NopRegistry{}
	}
	return &debugMetricsSet{
		v4Injected: reg.GaugeVec("ze_ospf_debug_injected_lsas",
			"OSPFv2 debug-injected opaque LSAs currently originated, by scope (link/area/as).",
			[]string{labelScope}),
		v4Inject: reg.CounterVec("ze_ospf_debug_injections_total",
			"OSPFv2 debug LSA injection actions, by scope and action (originate/withdraw).",
			[]string{labelScope, "action"}),
		v4Decode: reg.CounterVec("ze_ospf_debug_decode_errors_total",
			"OSPFv2 debug opaque-LSA body decode errors, by Opaque Type.",
			[]string{labelOpaqueType}),
		v6Injected: reg.GaugeVec("ze_ospfv3_debug_injected_lsas",
			"OSPFv3 debug-injected native LSAs currently originated, by scope (link-local/area/as).",
			[]string{labelScope}),
		v6Inject: reg.CounterVec("ze_ospfv3_debug_injections_total",
			"OSPFv3 debug LSA injection actions, by scope and action (originate/withdraw).",
			[]string{labelScope, "action"}),
		v6Decode: reg.CounterVec("ze_ospfv3_debug_decode_errors_total",
			"OSPFv3 debug native-LSA body decode errors, by LS Type.",
			[]string{"ls_type"}),
	}
}

var (
	debugMetricsOnce sync.Once
	// debugMetrics is an atomic pointer so a goroutine reading it races cleanly with a
	// test that swaps it via Store (go test -race clean). It is seeded with a NopRegistry
	// set so reads before setDebugMetrics never deref nil.
	debugMetrics = func() *atomic.Pointer[debugMetricsSet] {
		p := &atomic.Pointer[debugMetricsSet]{}
		p.Store(newDebugMetrics(metrics.NopRegistry{}))
		return p
	}()
)

// setDebugMetrics binds the six debug series to the real registry exactly once, regardless
// of how many engine instances run (both address families share one set).
func setDebugMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	debugMetricsOnce.Do(func() { debugMetrics.Store(newDebugMetrics(reg)) })
}
