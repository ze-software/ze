// Design: docs/architecture/l2tp/l2tp-10-metrics.md -- Prometheus metrics exposure
// Related: relay_sink.go -- the Service-Name and realm relay decisions it counts
// Related: kernel_linux.go -- the realm bridge outcome it counts

package l2tp

import "github.com/ze-software/ze/internal/core/metrics"

// relayKind is which relay binding a PPPoE subscriber was matched against.
type relayKind uint8

const (
	relayKindUnspecified relayKind = iota
	relayKindService               // l2tp relay <service-name>, decided at PADS
	relayKindRealm                 // l2tp relay-realm <realm>, decided at authentication
)

// label is the Prometheus "kind" label value.
func (k relayKind) label() string {
	switch k {
	case relayKindService:
		return "service"
	case relayKindRealm:
		return "realm"
	case relayKindUnspecified:
		panic("BUG: l2tp relay counted with no kind")
	}
	panic("BUG: l2tp relay kind out of range")
}

// relayOutcome is how one relay decision ended.
type relayOutcome uint8

const (
	relayOutcomeUnspecified  relayOutcome = iota
	relayOutcomeNoMatch                   // no binding named it; PPPoE terminates locally
	relayOutcomeRefused                   // a binding matched but the call could not start
	relayOutcomePlaced                    // the incoming call was placed toward the remote
	relayOutcomeBridged                   // realm only: the subscriber's channel was bridged
	relayOutcomeBridgeFailed              // realm only: PPPIOCBRIDGECHAN failed; the subscriber is refused
)

// label is the Prometheus "outcome" label value.
func (o relayOutcome) label() string {
	switch o {
	case relayOutcomeNoMatch:
		return "no-match"
	case relayOutcomeRefused:
		return "refused"
	case relayOutcomePlaced:
		return "placed"
	case relayOutcomeBridged:
		return "bridged"
	case relayOutcomeBridgeFailed:
		return "bridge-failed"
	case relayOutcomeUnspecified:
		panic("BUG: l2tp relay counted with no outcome")
	}
	panic("BUG: l2tp relay outcome out of range")
}

// relayLabels are the labels of ze_l2tp_relay_total.
var relayLabels = []string{"kind", "outcome"}

// bindRelayMetrics registers ze_l2tp_relay_total. Called by bindL2TPMetrics.
func bindRelayMetrics(reg metrics.Registry) metrics.CounterVec {
	return reg.CounterVec("ze_l2tp_relay_total",
		"PPPoE relay decisions of this LAC by binding kind (service, realm) and outcome (no-match, refused, placed, bridged, bridge-failed).",
		relayLabels)
}

// countRelay steps ze_l2tp_relay_total. Safe for concurrent use; a no-op
// until the metrics registry is bound, as every L2TP metric is.
func countRelay(kind relayKind, outcome relayOutcome) {
	m := l2tpMetricsPtr.Load()
	if m == nil {
		return
	}
	m.relayTotal.With(kind.label(), outcome.label()).Inc()
}

// countRealmBridge counts the bridge result of a realm call.
func countRealmBridge(err error) {
	if err != nil {
		countRelay(relayKindRealm, relayOutcomeBridgeFailed)
		return
	}
	countRelay(relayKindRealm, relayOutcomeBridged)
}
