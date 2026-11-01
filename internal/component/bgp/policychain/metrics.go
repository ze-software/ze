// Design: docs/plugin-development/metrics.md -- ze_policy_route_set_refusals_total
// Related: external.go -- applyExternalSets, whose refusals this counts
// Related: chain.go -- Chain.failure, where a live run counts them

package policychain

import (
	"errors"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

// chainMetrics holds the metrics of the BGP library chains.
type chainMetrics struct {
	setRefusals metrics.CounterVec // labels: filter
}

// chainMetricsPtr stays nil until the reactor wires a registry, so
// countSetRefusal guards its use. A build with metrics disabled leaves it nil.
var chainMetricsPtr atomic.Pointer[chainMetrics]

// SetMetricsRegistry creates the library chains' metrics from reg. The
// reactor calls it from its metrics-enable block: a chain is compiled from
// config, not run as a plugin, so no plugin's ConfigureMetrics reaches it. A
// nil registry is a no-op.
func SetMetricsRegistry(reg metrics.Registry) {
	if reg == nil {
		return
	}
	chainMetricsPtr.Store(&chainMetrics{
		setRefusals: reg.CounterVec(
			"ze_policy_route_set_refusals_total",
			"Routes a plugin filter failed by answering a set Ze refused (undeclared, not settable at the attachment, unknown, or of the wrong shape), by filter.",
			[]string{"filter"},
		),
	})
}

// refusedSet marks a reply's set the chain refused (D-9), so Chain.failure
// can count it apart from the other member failures. It reads as the error
// it wraps.
type refusedSet struct{ err error }

func (r refusedSet) Error() string { return r.err.Error() }
func (r refusedSet) Unwrap() error { return r.err }

// countSetRefusal counts one route filter failed by a refused set.
func countSetRefusal(filter string, err error) {
	m := chainMetricsPtr.Load()
	if m == nil {
		return
	}
	if _, refused := errors.AsType[refusedSet](err); !refused {
		return
	}
	m.setRefusals.With(filter).Inc()
}
