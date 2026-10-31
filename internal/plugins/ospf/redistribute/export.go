// Design: docs/guide/redistribution.md -- the OSPF export chain over redistributed routes
// Related: consumer.go -- InjectRoute, which judges every route with exportVerdict
// Related: ../export_chain.go -- compiles `ospf filter export` and calls SetExportChain

package ospfredistribute

import (
	"sync/atomic"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
)

// exportPolicy is the compiled `ospf filter export` chain.
type exportPolicy struct {
	chain ribpolicy.Chain
}

// exportChain holds the export chain the OSPF config applies. Written on every
// config apply, read on every inject. Nil means no export chain is configured:
// every redistributed route is originated at the metric the import chain set
// (else DefaultExternalMetric), type 2, with its own tag.
var exportChain atomic.Pointer[exportPolicy]

// SetExportChain installs chain as the OSPF export chain. A nil chain clears
// it. Safe for concurrent use with InjectRoute. An AS-external LSA already
// originated is not judged again by this call: the config apply then calls
// Consumer.Rejudge.
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

// exportVerdict judges entry against the export chain, answering the
// AS-external fields to originate and whether the chain accepts the route.
// The fields start from what the redistribute import chain gave the route: its
// set metric (entry.HasMetric), else DefaultExternalMetric, type 2, and its tag,
// which already carries an import `set tag`. An accepting export chain's sets
// replace them. FRR's one route-map at `redistribute` does the same: its
// ROUTEMAP_METRIC, when set, wins over metric_value (ospf_external_lsa_body_set).
func exportVerdict(entry *configredist.RouteEntry) (External, bool) {
	ext := External{Metric: DefaultExternalMetric, Type2: true, Tag: entry.Tag}
	if entry.HasMetric {
		ext.Metric = entry.Metric
	}
	policy := exportChain.Load()
	if policy == nil {
		return saturateExternalMetric(ext), true
	}
	verdict := policy.chain.Evaluate(entry.Route)
	if verdict.Action != ribpolicy.ActionAccept {
		return External{}, false
	}
	return saturateExternalMetric(applyExportSets(ext, verdict.Sets)), true
}

// saturateExternalMetric caps ext's metric at the 24-bit AS-external metric
// field, so a `set metric` above it originates an unreachable route rather
// than the low 24 bits the encoder would keep. FRR's route_set_metric clamps
// the same way, at its default max_metric OSPF_LS_INFINITY.
func saturateExternalMetric(ext External) External {
	// RFC 2328 Appendix B: "It is defined to be the 24-bit binary value of all ones: 0xffffff."
	if ext.Metric > packet.ExternalMetricMax {
		ext.Metric = packet.ExternalMetricMax
	}
	return ext
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
