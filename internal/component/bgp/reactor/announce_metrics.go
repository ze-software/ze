// Design: docs/architecture/core-design.md -- announce writers, fail-closed size guard
// Related: reactor_api_batch.go -- logAnnounceTooLarge, the batch rail's drop
// Related: peer_rib_routes.go -- logRIBRouteTooLarge, the queued rail's drop
// Related: announce_metrics_test.go -- label vocabulary and the unwired-is-silent contract

package reactor

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

// announceMetrics holds counters for announces the size guard refused.
type announceMetrics struct {
	droppedOversize metrics.CounterVec // labels: rail, stage
}

// announceMetricsPtr stays nil until the reactor wires a registry, so the
// recorder below guards its use. A build with metrics disabled leaves it nil,
// and the drop is then reported by its log line alone.
//
// A package-level pointer rather than a field read off the Reactor, because
// neither drop site can reach one. logAnnounceTooLarge and logRIBRouteTooLarge
// are free functions, and the queued rail's whole builder, buildRIBRouteUpdate,
// takes no receiver either. This is the same shape, and the same reason, as
// filterapi.SetMetricsRegistry.
var announceMetricsPtr atomic.Pointer[announceMetrics]

// setAnnounceMetricsRegistry creates the announce counters from the given
// registry. A nil registry is a no-op, leaving the recorder disabled.
func setAnnounceMetricsRegistry(reg metrics.Registry) {
	if reg == nil {
		return
	}
	announceMetricsPtr.Store(&announceMetrics{
		droppedOversize: reg.CounterVec(
			"ze_bgp_announce_dropped_oversize_total",
			"Announces refused because the encoded route did not fit its build buffer. The route was not sent to that peer, and nothing truncated went out.",
			[]string{"rail", "stage"},
		),
	})
}

// Rail labels. A closed set: an announce is built by one of exactly two writers.
const (
	// announceRailBatch is the API rail, reactorAPIAdapter.buildBatchAnnounceUpdate.
	announceRailBatch = "batch"
	// announceRailQueued is the RIB rail, buildRIBRouteUpdate.
	announceRailQueued = "queued"
)

// Stage labels. A closed set: the two regions an announce is written into.
const (
	// announceStageNLRI is the prefix block.
	announceStageNLRI = "nlri"
	// announceStageAttributes is the path-attribute block.
	announceStageAttributes = "attributes"
)

// recordAnnounceDroppedOversize counts one refused announce.
//
// The drop is fail-closed and correct: the size query runs before the write, so
// no truncated UPDATE is a state either writer can produce. What it is NOT is
// visible. The route simply never arrives at that peer, and until this counter
// the only trace was a Warn line. A drop an operator can find only by grepping
// is a drop nobody alerts on (ai/rules/evidence.md: fail closed OR say
// something, and a log line says it to nobody at three in the morning).
//
// Both labels are closed sets chosen at the call site, never derived from a
// route, a peer, or a family. The batch rail's log line already carries the
// family and the NLRI count, and the queued rail's carries the prefix; as labels
// those would be peer-driven cardinality on a path that is already an error.
func recordAnnounceDroppedOversize(rail, stage string) {
	if m := announceMetricsPtr.Load(); m != nil {
		m.droppedOversize.With(rail, stage).Inc()
	}
}
