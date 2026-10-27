// Design: docs/plugin-development/metrics.md -- community rewrite operator signal
// Related: scrub.go -- scrubOwnGACommunities, the caller of recordScrub
// Related: filter.go -- applyRelationTag, the caller of recordRelationTag
// Related: blackhole.go -- blackholePropagationGuard, the caller of recordBlackholeGuard
// Related: register.go -- Filter.ConfigureMetrics, which binds the registry
//
// The counters of the received-route rewrites this plugin performs.
//
// Each mechanism below changes the attributes of a route a neighbor sent, and
// the route itself was the only trace: the Info line says a rewrite happened on
// one peer, not how often across the router. The counters answer how often; the
// log line answers which peer. A peer address in a label would grow the series
// count with the session count, so it stays in the log.
//
// Every label value is a compile-time constant, so the series count is fixed
// whatever an operator configures: two widths, two guard communities, one tag.
package filter_community

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/metrics"
)

// Metric names, following docs/plugin-development/metrics.md
// (ze_{scope}_{subject}_{event}_total) with the scope filter_community, the
// package name, as bgp-filter-path-asn does.
const (
	metricScrubbed     = "ze_filter_community_value_scrubbed_total"
	metricRelationTag  = "ze_filter_community_relation_tag_written_total"
	metricBlackholeTag = "ze_filter_community_blackhole_guard_added_total"
)

// Label values. Bounded and compile-time constant. The guard labels are the
// YANG keywords an operator writes, so the config, the log and the metric say
// the same thing.
const (
	widthLabelStandard = "standard"
	widthLabelLarge    = "large"
)

// communityMetrics holds every child counter, resolved once at build time.
//
// CounterVec.With allocates for its variadic on every call, so resolving here
// keeps the ingress path allocation-free (ai/rules/performance.md). Pre-creating
// every child also makes each series present at 0 from startup, so an alert on
// a rate does not wait for the first rewrite.
type communityMetrics struct {
	scrubbedStandard metrics.Counter
	scrubbedLarge    metrics.Counter
	relationTag      metrics.Counter
	guardNoExport    metrics.Counter
	guardNoAdvertise metrics.Counter
}

var communityMetricsPtr atomic.Pointer[communityMetrics]

// buildMetrics registers the metric set against reg.
func buildMetrics(reg metrics.Registry) *communityMetrics {
	scrubbed := reg.CounterVec(metricScrubbed,
		"COMMUNITY and LARGE_COMMUNITY values carrying the local AS as Global Administrator removed from received routes by scrub-own-ga (RFC 7454 Section 11), by width.",
		[]string{"width"})
	guard := reg.CounterVec(metricBlackholeTag,
		"Received BLACKHOLE routes the blackhole-propagation guard added a propagation community to (RFC 7999 Section 3.2), by the community added.",
		[]string{"community"})
	return &communityMetrics{
		scrubbedStandard: scrubbed.With(widthLabelStandard),
		scrubbedLarge:    scrubbed.With(widthLabelLarge),
		relationTag: reg.Counter(metricRelationTag,
			"Received routes written the RFC 8195 relation-to-origin large community (Section 3.2)."),
		guardNoExport:    guard.With(blackholeGuardNoExport),
		guardNoAdvertise: guard.With(blackholeGuardNoAdvertise),
	}
}

// setMetricsRegistry publishes metrics backed by the host registry. Called
// through Filter.ConfigureMetrics from filterapi.SetMetricsRegistry, the
// reactor's metrics-enable block: the ingress filters run in the daemon whether
// or not the plugin's engine runs, so a plugin ConfigureMetrics hook would miss
// a config that enables a mechanism with the plugin external.
func setMetricsRegistry(reg metrics.Registry) {
	communityMetricsPtr.Store(buildMetrics(reg))
}

// cmetrics returns the current metric set, lazily defaulting to a no-op
// registry. Telemetry can be off, so a missing registry is a no-op counter
// rather than a nil dereference on the ingress path.
func cmetrics() *communityMetrics {
	if m := communityMetricsPtr.Load(); m != nil {
		return m
	}
	communityMetricsPtr.CompareAndSwap(nil, buildMetrics(metrics.NopRegistry{}))
	return communityMetricsPtr.Load()
}

// recordScrub counts own-GA values one rewrite removed. The caller MUST call it
// only after the rebuild that removes them succeeded.
func recordScrub(code attribute.AttributeCode, removedValues int) {
	m := cmetrics()
	if code == attribute.AttrLargeCommunity {
		m.scrubbedLarge.Add(float64(removedValues))
		return
	}
	m.scrubbedStandard.Add(float64(removedValues))
}

// recordRelationTag counts one route written its relation tag. The caller MUST
// call it only after the tag was appended.
func recordRelationTag() {
	cmetrics().relationTag.Inc()
}

// recordBlackholeGuard counts one route the guard added a community to. The
// caller MUST call it only after the community was appended, with the
// community blackholeGuardCommunity answered: those two are the only values it
// can add, and each has its own series.
func recordBlackholeGuard(added attribute.Community) {
	m := cmetrics()
	if added == attribute.CommunityNoAdvertise {
		m.guardNoAdvertise.Inc()
	}
	if added == attribute.CommunityNoExport {
		m.guardNoExport.Inc()
	}
}
