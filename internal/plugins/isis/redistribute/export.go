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
// configured: every redistributed route is advertised at the default metric.
var exportChain atomic.Pointer[exportPolicy]

// SetExportChain installs chain as the IS-IS export chain. A nil chain clears
// it. Safe for concurrent use with InjectRoute. A route already in an LSP is
// not judged again by this call.
func SetExportChain(chain ribpolicy.Chain) {
	if chain == nil {
		exportChain.Store(nil)
		return
	}
	exportChain.Store(&exportPolicy{chain: chain})
}

// exportVerdict judges entry against the export chain, answering the prefix
// metric to advertise and whether the chain accepts the route. With no chain
// every route is accepted at DefaultRedistMetric; an accepting chain's metric
// set replaces the default.
func exportVerdict(entry *configredist.RouteEntry) (metric uint32, accepted bool) {
	policy := exportChain.Load()
	if policy == nil {
		return DefaultRedistMetric, true
	}
	verdict := policy.chain.Evaluate(entry.Route)
	if verdict.Action != ribpolicy.ActionAccept {
		return 0, false
	}
	if verdict.Sets.HasMetric {
		return verdict.Sets.Metric, true
	}
	return DefaultRedistMetric, true
}
