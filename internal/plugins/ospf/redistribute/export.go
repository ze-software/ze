// Design: docs/guide/redistribution.md -- the OSPF export chain over redistributed routes
// Related: consumer.go -- InjectRoute, which judges every route with exportVerdict
// Related: ../export_chain.go -- compiles `ospf filter export` and calls SetExportChain

package ospfredistribute

import (
	"sync/atomic"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// exportPolicy is the compiled `ospf filter export` chain.
type exportPolicy struct {
	chain ribpolicy.Chain
}

// exportChain holds the export chain the OSPF config applies. Written on every
// config apply, read on every inject. Nil means no export chain is configured:
// every redistributed route is originated at DefaultExternalMetric, type 2,
// with its own tag.
var exportChain atomic.Pointer[exportPolicy]

// SetExportChain installs chain as the OSPF export chain. A nil chain clears
// it. Safe for concurrent use with InjectRoute. An AS-external LSA already
// originated is not judged again by this call.
func SetExportChain(chain ribpolicy.Chain) {
	if chain == nil {
		exportChain.Store(nil)
		return
	}
	exportChain.Store(&exportPolicy{chain: chain})
}

// exportVerdict judges entry against the export chain, answering the
// AS-external fields to originate and whether the chain accepts the route.
// With no chain every route is accepted at the defaults; an accepting chain's
// sets replace the default metric, the default type 2, and the route's own tag.
func exportVerdict(entry *configredist.RouteEntry) (External, bool) {
	ext := External{Metric: DefaultExternalMetric, Type2: true, Tag: entry.Tag}
	policy := exportChain.Load()
	if policy == nil {
		return ext, true
	}
	verdict := policy.chain.Evaluate(entry.Route)
	if verdict.Action != ribpolicy.ActionAccept {
		return External{}, false
	}
	return applyExportSets(ext, verdict.Sets), true
}

// applyExportSets answers ext with the fields sets carries replaced.
func applyExportSets(ext External, sets ribpolicy.Sets) External {
	if sets.HasMetric {
		ext.Metric = sets.Metric
	}
	if sets.HasTag {
		ext.Tag = sets.Tag
	}
	// RFC 2328 Section 2.3: a Type 1 metric is in link-state units, a Type 2
	// metric is larger than any internal path.
	switch sets.MetricType {
	case ribpolicy.ExternalMetricType1:
		ext.Type2 = false
	case ribpolicy.ExternalMetricType2:
		ext.Type2 = true
	case ribpolicy.ExternalMetricTypeUnspecified:
		// No filter set the type: the default stays.
	}
	return ext
}
