// Design: docs/architecture/core-design.md -- FlowSpec-to-firewall bridge
// Related: engine.go -- handleFlowSpecAdd, the only caller of countRuleRefused

package flowspecfirewall

import (
	"errors"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

// Values of the rulesRefused "reason" label. The set is closed on purpose: the
// data that produced the refusal comes from a peer, so a label derived from it
// would let one peer create unbounded time series.
const (
	refusedReasonUnknownProtocol = "unknown-protocol"
	refusedReasonUnsupported     = "unsupported-component"
	refusedReasonNoAction        = "no-action"
	refusedReasonParse           = "parse"
	refusedReasonMaxRules        = "max-rules"
)

// bridgeMetrics counts the FlowSpec routes this bridge accepted from a peer and
// then did not turn into a firewall rule.
//
// A refusal is otherwise invisible. The rule is never registered, so no
// firewall counter moves, no reconcile fails, and `show firewall` has nothing
// to render: the peer believes ze filters the traffic and ze does not.
type bridgeMetrics struct {
	rulesRefused metrics.CounterVec
}

var bridgeMetricsPtr atomic.Pointer[bridgeMetrics]

// bindMetrics is called through Registration.ConfigureMetrics. It does not
// reliably run before the engine: InjectPluginMetrics defers the hook when no
// registry exists at plugin spawn. Until it runs, countRuleRefused finds a nil
// pointer and does nothing, which is why every refusal also writes a log line.
func bindMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	bridgeMetricsPtr.Store(&bridgeMetrics{
		rulesRefused: reg.CounterVec("ze_flowspec_rules_refused_total",
			"FlowSpec routes received from a peer that this bridge did not turn into a firewall rule, by reason.",
			[]string{"reason"}),
	})
}

// countRuleRefused records one FlowSpec route that will not be enforced.
func countRuleRefused(reason string) {
	if m := bridgeMetricsPtr.Load(); m != nil {
		m.rulesRefused.With(reason).Inc()
	}
}

// refusalReason maps a parse or translation error to its label value.
func refusalReason(err error) string {
	switch {
	case errors.Is(err, errUnknownProtocol):
		return refusedReasonUnknownProtocol
	case errors.Is(err, errUnsupportedComponent), errors.Is(err, errUnsupportedOperator):
		return refusedReasonUnsupported
	case errors.Is(err, errNoAction):
		return refusedReasonNoAction
	default:
		return refusedReasonParse
	}
}
