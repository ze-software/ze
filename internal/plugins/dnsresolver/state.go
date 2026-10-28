// Design: docs/architecture/dns/lan-resolver.md -- the resolver state the query path reads
// Related: server.go -- the answer function that reads this state
// Related: cache.go -- the answer and negative caches
// Related: limit.go -- the outstanding bound, the per-client rate and the sweeper

package dnsresolver

import (
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// allowLabel is the one label every allow-from prefix carries: the matcher
// answers only whether a source is inside some prefix.
const allowLabel = "allow"

// sweepInterval is how often the sweeper frees expired cache entries and
// stale rate windows. A lookup checks expiry itself, so the interval bounds
// memory held by dead entries, never what a client is answered.
const sweepInterval = 10 * time.Second

// policy is the part of one applied config the query path reads. It is
// built once per apply, published through resolver.policy, and never
// mutated after, so the query path reads it with one atomic load.
type policy struct {
	allow       *dnsserver.Matcher
	forward     []upstream
	timeout     time.Duration
	outstanding uint32
	clientRate  uint32
	maxTTL      uint32
	maxNegTTL   uint32
}

// newPolicy converts one parsed config into the query path's policy.
func newPolicy(cfg *resolverConfig) *policy {
	entries := make([]dnsserver.Entry, len(cfg.AllowFrom))
	for i, p := range cfg.AllowFrom {
		entries[i] = dnsserver.Entry{Prefix: p, Label: allowLabel}
	}
	forward := make([]upstream, len(cfg.Forward))
	copy(forward, cfg.Forward)
	return &policy{
		allow:       dnsserver.BuildMatcher(entries),
		forward:     forward,
		timeout:     cfg.Timeout,
		outstanding: cfg.LimitOutstanding,
		clientRate:  cfg.LimitPerClientRate,
		maxTTL:      cfg.CacheMaxTTL,
		maxNegTTL:   cfg.CacheMaxNegTTL,
	}
}

// resolver is the LAN resolver's runtime: the published policy, the two
// caches, the in-flight table and the limits. One resolver lives for the
// whole plugin run; a config apply replaces only the policy and the cache
// bound. The query path is safe for concurrent use.
//
// Lifecycle: newResolver, then start, which launches the one sweeper
// goroutine; stop MUST be called once the plugin stops, and waits for it.
type resolver struct {
	policy    atomic.Pointer[policy]
	shown     atomic.Pointer[resolverConfig] // the applied config, for show and for a reload's narrowed window (narrow)
	answers   *lru
	negatives *lru
	flights   flightTable
	limits    limiter
	stats     resolverStats
	now       func() time.Time

	quit chan struct{}
	done chan struct{}
}

// newResolver builds a resolver with no policy: until the first apply every
// query is answered SERVFAIL, and no listener is bound before that apply.
func newResolver(now func() time.Time) *resolver {
	return &resolver{
		answers:   newLRU(defaultCacheSize),
		negatives: newLRU(defaultCacheSize),
		flights:   flightTable{flights: make(map[cacheKey]*flight)},
		limits:    limiter{clients: make(map[clientKey]clientWindow)},
		now:       now,
		quit:      make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// apply publishes the policy of cfg and bounds both caches by its cache
// size. The next query reads the new policy; nothing is rebound here.
func (r *resolver) apply(cfg *resolverConfig) {
	r.answers.setLimit(int(cfg.CacheSize))
	r.negatives.setLimit(int(cfg.CacheSize))
	shown := *cfg
	r.shown.Store(&shown)
	r.policy.Store(newPolicy(cfg))
}

// runningState is the published policy and shown config of the running
// config, held by an install so a refused reload can put them back.
type runningState struct {
	policy *policy
	shown  *resolverConfig
}

// narrow publishes, for the window a reload rebinds its listeners in, the
// running policy whose allow-from is narrowed to the sources both it and
// next admit, and answers the running state for restore. With no running
// policy nothing is published: every query is answered SERVFAIL until the
// candidate is applied. The shown config stays the running one.
func (r *resolver) narrow(next *resolverConfig) runningState {
	running := runningState{policy: r.policy.Load(), shown: r.shown.Load()}
	// refuse-by-default: guards apply -- no running config means no running policy, the SERVFAIL state newResolver leaves
	if running.shown == nil {
		return running
	}
	window := *running.shown
	window.AllowFrom = intersectPrefixes(running.shown.AllowFrom, next.AllowFrom)
	r.policy.Store(newPolicy(&window))
	return running
}

// restore publishes the running state narrow answered again, after a
// refused reload. The cache bound was never changed by the reload.
func (r *resolver) restore(running runningState) {
	r.shown.Store(running.shown)
	r.policy.Store(running.policy)
}

// intersectPrefixes is the prefix list that holds exactly the addresses
// some prefix of a and some prefix of b hold. Two prefixes that overlap
// share the longer one, since one holds the other. Bounded by
// len(a)*len(b), at config time only.
func intersectPrefixes(a, b []netip.Prefix) []netip.Prefix {
	var out []netip.Prefix
	for _, pa := range a {
		for _, pb := range b {
			if !pa.Overlaps(pb) {
				continue
			}
			if pa.Bits() >= pb.Bits() {
				out = append(out, pa)
				continue
			}
			out = append(out, pb)
		}
	}
	return out
}

// addrPort is the upstream's transport address.
func (u upstream) addrPort() netip.AddrPort { return netip.AddrPortFrom(u.IP, u.Port) }

// label is the YANG enum value of m.
func (m resolveMode) label() string {
	switch m {
	case modeForward:
		return "forward"
	case modeUnspecified:
		panic("BUG: dnsresolver: config without a mode")
	}
	panic("BUG: dnsresolver: unknown mode")
}

// clearCaches empties the answer and the negative cache.
func (r *resolver) clearCaches() {
	r.answers.clear()
	r.negatives.clear()
}

// start launches the sweeper. MUST be paired with stop.
func (r *resolver) start() { go r.sweep(sweepInterval) }

// stop ends the sweeper and waits for it. MUST be called after start, once.
func (r *resolver) stop() {
	close(r.quit)
	<-r.done
}

// sweep is the resolver's one long-lived worker: every interval it frees
// expired cache entries and rate windows older than the current second. It
// runs until stop closes quit.
func (r *resolver) sweep(interval time.Duration) {
	defer close(r.done)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-r.quit:
			return
		case <-t.C:
			now := r.now()
			r.answers.sweep(now)
			r.negatives.sweep(now)
			r.limits.sweep(now)
			r.cacheGauges()
		}
	}
}
