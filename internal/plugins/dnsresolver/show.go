// Design: docs/architecture/dns/lan-resolver.md -- show dns-resolver and clear dns-resolver cache all
// Related: metrics.go -- the counters show reports

package dnsresolver

import (
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// errResolverNotRunning answers clear when the plugin is not running.
var errResolverNotRunning = errors.New("dns-resolver is not running, so it holds no cache to clear. " +
	"Configure service dns-resolver with enabled true, a listener, allow-from and a forward server, then commit")

// handleShowDNSResolver answers `show dns-resolver` from the running
// resolver's published policy and counters, the state the query path
// reads.
//
//nolint:unparam // handler signature fixed by pluginserver.RPCRegistration
func handleShowDNSResolver(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	r := activeResolver.Load()
	if r == nil {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"running": false}}, nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: r.show()}, nil
}

// handleClearDNSResolverCache answers `clear dns-resolver cache all`.
func handleClearDNSResolverCache(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	r := activeResolver.Load()
	if r == nil {
		return nil, errResolverNotRunning
	}
	answers, negatives := r.answers.size(), r.negatives.size()
	r.clearCaches()
	r.cacheGauges()
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{
		"cleared-answer":   answers,
		"cleared-negative": negatives,
	}}, nil
}

// show is the show dns-resolver answer. JSON keys are kebab-case.
func (r *resolver) show() plugin.Map {
	data := plugin.Map{"running": true}
	pol := r.policy.Load()
	if pol == nil {
		data["enabled"] = false
		return data
	}
	cfg := r.shown.Load()
	listeners := make([]plugin.Map, 0, len(cfg.Listeners))
	for _, l := range cfg.Listeners {
		listeners = append(listeners, plugin.Map{"name": l.Name, "ip": l.IP.String(), "port": l.Port, "state": r.listeners.show(netip.AddrPortFrom(l.IP, l.Port))})
	}
	upstreams := make([]string, 0, len(pol.forward))
	for _, u := range pol.forward {
		upstreams = append(upstreams, u.addrPort().String())
	}
	hits, misses := r.stats.hits.Load(), r.stats.misses.Load()
	hitRate := 0.0
	if hits+misses > 0 {
		hitRate = float64(hits) / float64(hits+misses)
	}
	byResult := plugin.Map{}
	for _, u := range upstreamResults {
		byResult[u.label()] = r.stats.upstream[u].Load()
	}

	data["enabled"] = cfg.Enabled
	data["mode"] = cfg.Mode.label()
	data["listeners"] = listeners
	data["allow-from"] = len(cfg.AllowFrom)
	data["upstreams"] = upstreams
	data["timeout-seconds"] = int(pol.timeout.Seconds())
	data["limit-outstanding"] = pol.outstanding
	data["limit-per-client-rate"] = pol.clientRate
	data["outstanding"] = r.limits.outstanding.Load()
	data["cache-max-ttl"] = pol.maxTTL
	data["cache-max-negative-ttl"] = pol.maxNegTTL
	data["cache-answer-entries"] = r.answers.size()
	data["cache-negative-entries"] = r.negatives.size()
	data["cache-hits"] = hits
	data["cache-misses"] = misses
	data["cache-hit-rate"] = hitRate
	data["queries"] = r.stats.queries.Load()
	data["coalesced"] = r.stats.coalesced.Load()
	data["refused-acl"] = r.stats.refusedACL.Load()
	data["refused-rd-clear"] = r.stats.refusedRD.Load()
	data["rate-limited"] = r.stats.rateLimited.Load()
	data["dropped"] = r.stats.dropped.Load()
	data["upstream-queries"] = byResult
	return data
}
