// Design: docs/guide/redistribution.md -- the IS-IS export chain over redistributed routes
// Related: consumer.go -- InjectRoute, which judges every route with exportVerdict
// Related: ../register.go -- compiles `isis filter export` and calls SetExportChain

package isisredistribute

import (
	"sync/atomic"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// exportPolicy is the compiled `isis filter export` chain.
type exportPolicy struct {
	chain ribpolicy.Chain
}

// exportChain holds the export chain the IS-IS config applies. Written on
// every config apply, read on every inject. Nil means no export chain is
// configured: every redistributed route is advertised at the metric the import
// chain set, or the default metric.
var exportChain atomic.Pointer[exportPolicy]

// SetExportChain installs chain as the IS-IS export chain. A nil chain clears
// it. Safe for concurrent use with InjectRoute. A route already in an LSP is
// not judged again by this call: the caller MUST call Consumer.Rejudge after.
func SetExportChain(chain ribpolicy.Chain) {
	if chain == nil {
		exportChain.Store(nil)
		return
	}
	exportChain.Store(&exportPolicy{chain: chain})
}

// ExportChain answers the export chain SetExportChain installed last, nil when
// none is. Safe for concurrent use.
func ExportChain() ribpolicy.Chain {
	policy := exportChain.Load()
	if policy == nil {
		return nil
	}
	return policy.chain
}

// exportVerdict judges entry against the export chain, answering the prefix
// metric to advertise and whether the chain accepts the route. The metric
// starts from the one the redistribute import chain set (entry.HasMetric), or
// DefaultRedistMetric when it set none, and an accepting export chain's metric
// set replaces it. FRR's isis_redist_update does the same with its one
// route-map: route_set_metric overwrites the redistribute default.
func exportVerdict(entry *configredist.RouteEntry) (metric uint32, accepted bool) {
	metric = DefaultRedistMetric
	if entry.HasMetric {
		metric = entry.Metric
	}
	policy := exportChain.Load()
	if policy == nil {
		return metric, true
	}
	verdict := policy.chain.Evaluate(entry.Route)
	if verdict.Action != ribpolicy.ActionAccept {
		return 0, false
	}
	if verdict.Sets.HasMetric {
		return verdict.Sets.Metric, true
	}
	return metric, true
}
