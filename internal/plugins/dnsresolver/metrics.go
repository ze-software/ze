// Design: docs/architecture/dns/lan-resolver.md -- the resolver's counters and Prometheus metrics
// Related: show.go -- show dns-resolver reads the same counters

package dnsresolver

import (
	"sync/atomic"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/metrics"
)

// upstreamResult labels one upstream exchange's outcome. Zero is
// unspecified, so an unlabeled exchange is never counted as answered.
type upstreamResult uint8

const (
	upstreamUnspecified upstreamResult = iota
	upstreamAnswered
	upstreamTimeout
	upstreamError
	upstreamFailure
	upstreamTruncated
)

// upstreamResults is every labeled outcome, in the order show prints them.
var upstreamResults = [...]upstreamResult{upstreamAnswered, upstreamTimeout, upstreamError, upstreamFailure, upstreamTruncated}

// label is the metric label and the show key of u.
func (u upstreamResult) label() string {
	switch u {
	case upstreamAnswered:
		return "answered"
	case upstreamTimeout:
		return "timeout"
	case upstreamError:
		return "error"
	case upstreamFailure:
		return "failure"
	case upstreamTruncated:
		return "truncated"
	case upstreamUnspecified:
		panic("BUG: dnsresolver: unlabeled upstream result")
	}
	panic("BUG: dnsresolver: unknown upstream result")
}

// refuseReason labels a REFUSED reply.
type refuseReason uint8

const (
	refuseUnspecified refuseReason = iota
	refuseACL
	refuseRDClear
)

func (r refuseReason) label() string {
	switch r {
	case refuseACL:
		return "acl"
	case refuseRDClear:
		return "rd-clear"
	case refuseUnspecified:
		panic("BUG: dnsresolver: unlabeled refusal")
	}
	panic("BUG: dnsresolver: unknown refusal")
}

// resolverStats are the counters show dns-resolver reports. A Prometheus
// counter cannot be read back, so each event steps both. Safe for
// concurrent use.
type resolverStats struct {
	queries     atomic.Uint64
	hits        atomic.Uint64
	misses      atomic.Uint64
	coalesced   atomic.Uint64
	refusedACL  atomic.Uint64
	refusedRD   atomic.Uint64
	rateLimited atomic.Uint64
	dropped     atomic.Uint64
	upstream    [len(upstreamResults) + 1]atomic.Uint64
}

// resolverMetrics is the Prometheus side. Label sets are bounded: the two
// transports, the RCODE names, the fixed reasons and results; never a
// client address or a queried name.
type resolverMetrics struct {
	queries       metrics.CounterVec // transport, rcode
	refused       metrics.CounterVec // reason
	rateLimited   metrics.Counter
	dropped       metrics.Counter
	outstanding   metrics.Gauge
	cacheEntries  metrics.GaugeVec // cache
	cacheHits     metrics.Counter
	upstreamQuery metrics.CounterVec // result
}

var metricsPtr atomic.Pointer[resolverMetrics]

func buildMetrics(reg metrics.Registry) *resolverMetrics {
	return &resolverMetrics{
		queries:       reg.CounterVec("ze_dns_resolver_queries_total", "Queries answered or dropped, by transport and response code.", []string{"transport", "rcode"}),
		refused:       reg.CounterVec("ze_dns_resolver_refused_total", "Queries refused, by reason: acl (source outside allow-from) or rd-clear (recursion not desired).", []string{"reason"}),
		rateLimited:   reg.Counter("ze_dns_resolver_rate_limited_total", "Queries dropped because their source exceeded limit per-client-rate."),
		dropped:       reg.Counter("ze_dns_resolver_dropped_total", "Queries dropped because limit outstanding resolutions were in progress."),
		outstanding:   reg.Gauge("ze_dns_resolver_outstanding", "Resolutions in progress."),
		cacheEntries:  reg.GaugeVec("ze_dns_resolver_cache_entries", "Entries held, by cache: answer or negative.", []string{"cache"}),
		cacheHits:     reg.Counter("ze_dns_resolver_cache_hits_total", "Queries answered from the cache."),
		upstreamQuery: reg.CounterVec("ze_dns_resolver_upstream_queries_total", "Upstream exchanges, by result.", []string{"result"}),
	}
}

// setMetricsRegistry publishes metrics backed by reg. Called through the
// registration's ConfigureMetrics before the engine runs.
func setMetricsRegistry(reg metrics.Registry) { metricsPtr.Store(buildMetrics(reg)) }

// rmetrics returns the metric set, a no-op one before ConfigureMetrics.
func rmetrics() *resolverMetrics {
	if m := metricsPtr.Load(); m != nil {
		return m
	}
	metricsPtr.CompareAndSwap(nil, buildMetrics(metrics.NopRegistry{}))
	return metricsPtr.Load()
}

func (r *resolver) countQuery(transport string, rcode int) {
	r.stats.queries.Add(1)
	rmetrics().queries.With(transport, dns.RcodeToString[rcode]).Inc()
}

func (r *resolver) countRefused(reason refuseReason) {
	switch reason {
	case refuseACL:
		r.stats.refusedACL.Add(1)
	case refuseRDClear:
		r.stats.refusedRD.Add(1)
	case refuseUnspecified:
		panic("BUG: dnsresolver: unlabeled refusal")
	}
	rmetrics().refused.With(reason.label()).Inc()
}

func (r *resolver) countRateLimited() {
	r.stats.rateLimited.Add(1)
	rmetrics().rateLimited.Inc()
}

func (r *resolver) countDropped() {
	r.stats.dropped.Add(1)
	rmetrics().dropped.Inc()
}

func (r *resolver) countHit() {
	r.stats.hits.Add(1)
	rmetrics().cacheHits.Inc()
}

func (r *resolver) upstreamOutcome(u upstreamResult) {
	r.stats.upstream[u].Add(1)
	rmetrics().upstreamQuery.With(u.label()).Inc()
}

func (r *resolver) outstandingGauge() {
	rmetrics().outstanding.Set(float64(r.limits.outstanding.Load()))
}

// cacheGauges publishes both caches' entry counts.
func (r *resolver) cacheGauges() {
	m := rmetrics()
	m.cacheEntries.With("answer").Set(float64(r.answers.size()))
	m.cacheEntries.With("negative").Set(float64(r.negatives.size()))
}
