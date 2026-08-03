// Design: docs/plugin-development/metrics.md -- role plugin operator signal
// RFC: rfc/short/rfc9234.md
// Overview: role.go -- role plugin entry point
// Related: otc.go -- the ingress reject and egress suppression sites recorded here

package role

import (
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

// Metric names follow docs/plugin-development/metrics.md
// (ze_{scope}_{subject}_{event}_total). The scope is "role", not "bgprole":
// the bgp prefix is redundant, matching the rpki/gr/watchdog precedent in that
// document's scope-to-prefix table.
const (
	metricRouteRejects      = "ze_role_route_rejects_total"
	metricRouteSuppressions = "ze_role_route_suppressions_total"
)

// dropReason identifies why the role plugin refused a route. It is a typed
// numeric enum rather than a string because it indexes the pre-resolved counter
// array on the forward path (ai/rules/go-standards.md); the string form
// exists only at the metric-label and log boundaries.
type dropReason uint8

const (
	// Ingress: the route is made ineligible (RFC 9234 Section 5 receive rules).
	dropLeak         dropReason = iota // OTC from a Customer/RS-Client, or from a Peer with a mismatched ASN
	dropMalformedOTC                   // OTC length != 4: treat-as-withdraw
	// Egress: the route is not advertised to this destination peer.
	dropOTCPresent  // route carries OTC and the destination is a Provider/Peer/RS
	dropSourceRole  // Gao-Rexford: Provider/Peer/RS-learned route toward a Provider/Peer/RS
	dropExportSet   // operator export policy: destination role not in the export set
	dropReasonCount // sentinel: array length, never a real reason
)

// Label values. Bounded and compile-time constant, so cardinality is flat: five
// series per metric, never per-peer. Peer identity belongs in the log line, not
// in a label (the as112 metrics note gives the same reasoning for client IPs).
const (
	reasonLabelLeak         = "leak"
	reasonLabelMalformedOTC = "malformed-otc"
	reasonLabelOTCPresent   = "otc-present"
	reasonLabelSourceRole   = "source-role"
	reasonLabelExportSet    = "export-set"
)

var dropReasonLabels = [dropReasonCount]string{
	dropLeak:         reasonLabelLeak,
	dropMalformedOTC: reasonLabelMalformedOTC,
	dropOTCPresent:   reasonLabelOTCPresent,
	dropSourceRole:   reasonLabelSourceRole,
	dropExportSet:    reasonLabelExportSet,
}

// dropIsIngress selects the owning metric and the logged direction. Ingress
// drops make a received route ineligible; egress drops withhold an
// advertisement.
var dropIsIngress = [dropReasonCount]bool{
	dropLeak:         true,
	dropMalformedOTC: true,
}

// roleMetrics holds the plugin's drop counters, pre-resolved per reason.
type roleMetrics struct {
	// drops[r] is the child counter for reason r, resolved once at build time.
	// CounterVec.With allocates a []string for its variadic on every call, so
	// resolving the label here keeps the forward path allocation-free
	// (ai/rules/performance.md). Pre-creating every child also makes
	// each series present at 0 from startup, so an alert on a rate does not
	// depend on the series having appeared.
	drops [dropReasonCount]metrics.Counter
}

var roleMetricsPtr atomic.Pointer[roleMetrics]

// buildMetrics registers the metric set against reg.
func buildMetrics(reg metrics.Registry) *roleMetrics {
	rejects := reg.CounterVec(metricRouteRejects,
		"Routes made ineligible on ingress by RFC 9234 role rules, by reason.",
		[]string{"reason"})
	suppressions := reg.CounterVec(metricRouteSuppressions,
		"Routes withheld from a peer by RFC 9234 role rules or role export policy, by reason.",
		[]string{"reason"})

	m := &roleMetrics{}
	for i := range int(dropReasonCount) {
		r := dropReason(i)
		vec := suppressions
		if dropIsIngress[r] {
			vec = rejects
		}
		m.drops[r] = vec.With(dropReasonLabels[r])
	}
	return m
}

// setMetricsRegistry publishes metrics backed by the host registry. Called via
// the plugin Registration's ConfigureMetrics before RunEngine.
func setMetricsRegistry(reg metrics.Registry) { roleMetricsPtr.Store(buildMetrics(reg)) }

// rmetrics returns the current metric set, lazily defaulting to a no-op
// registry. The filters are registered from init() and the reactor may call
// them whether or not ConfigureMetrics ever ran (telemetry disabled, or the
// plugin loaded without a registry), so recording must never depend on it.
func rmetrics() *roleMetrics {
	if m := roleMetricsPtr.Load(); m != nil {
		return m
	}
	roleMetricsPtr.CompareAndSwap(nil, buildMetrics(metrics.NopRegistry{}))
	return roleMetricsPtr.Load()
}

// dropWarned latches the one-per-reason WARN emitted by recordDrop.
var dropWarned [dropReasonCount]atomic.Bool

// resetDropWarnedForTest re-arms the WARN latches so a test can observe the
// first-occurrence log. Test-only, mirroring the ResetForTest convention used
// by internal/core/redistevents and internal/component/config/redistribute.
func resetDropWarnedForTest() {
	for i := range dropWarned {
		dropWarned[i].Store(false)
	}
}

// recordDrop is the single observability seam for every route the role plugin
// refuses. It counts the drop, and the first time each reason occurs in this
// process it also emits one WARN naming the peer.
//
// Why both, and why the latch (ai/rules/evidence.md "or say
// something"): these paths previously logged at Debug and nothing else, so at
// the default log level a peer's advertisements could be withheld with no
// signal at all -- including because of a role typo that used to be inert. A
// counter answers "how many and why" but only for an operator already scraping
// and alerting; the first-occurrence WARN answers "did this start happening"
// from the log alone, with zero setup. Neither may cost a per-UPDATE log or
// allocation on the forward path (ai/rules/performance.md), so the counter
// is a pre-resolved Inc and the WARN is behind an atomic latch: at most one
// line per reason per process, and the fast path is a single atomic load with
// no closure and no boxed arguments. Per-route detail stays at Debug.
func recordDrop(r dropReason, peer netip.Addr, peerRole string) {
	rmetrics().drops[r].Inc()

	// Load first so the steady state is a plain atomic read; CompareAndSwap runs
	// once per reason and settles the race between concurrent forward workers.
	if dropWarned[r].Load() || !dropWarned[r].CompareAndSwap(false, true) {
		return
	}

	direction := "egress"
	metric := metricRouteSuppressions
	if dropIsIngress[r] {
		direction = "ingress"
		metric = metricRouteRejects
	}
	logger().Warn("role dropped a route",
		"reason", dropReasonLabels[r],
		"direction", direction,
		"peer", peer,
		"peer-role", peerRole,
		"metric", metric,
		"note", "first occurrence for this reason; further drops are counted only, per-route detail is at debug level")
}
