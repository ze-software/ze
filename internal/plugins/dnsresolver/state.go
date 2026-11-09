// Design: docs/architecture/dns/lan-resolver.md -- the resolver state the query path reads
// Related: server.go -- the answer function that reads this state
// Related: cache.go -- the answer and negative caches
// Related: limit.go -- the outstanding bound, the per-client rate and the sweeper

package dnsresolver

import (
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/dnsserver"
	"github.com/ze-software/ze/internal/core/dnssource"
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

// newPolicy converts one parsed config into the query path's policy, with
// forward the upstreams its dns-source list selected (forwardUpstreams).
func newPolicy(cfg *resolverConfig, forward []upstream) *policy {
	entries := make([]dnsserver.Entry, len(cfg.AllowFrom))
	for i, p := range cfg.AllowFrom {
		entries[i] = dnsserver.Entry{Prefix: p, Label: allowLabel}
	}
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

// forwardUpstreams evaluates cfg's dns-source list over the learned records
// at now and answers the upstreams in the order the resolver asks them. The
// configured class is the forward server entries: each selected address
// keeps every entry written for it, with its port, so a list that selects
// configured asks exactly what forward server names. A received address
// that no entry names is asked on port 53.
func forwardUpstreams(cfg *resolverConfig, records []dnssource.Record, now time.Time) []upstream {
	configured := make([]netip.Addr, len(cfg.Forward))
	for i, u := range cfg.Forward {
		configured[i] = u.IP
	}
	selection := dnssource.Select(cfg.Sources, dnssource.Candidates{
		dnssource.ClassConfigured: configured,
		dnssource.ClassReceived:   cfg.Sources.Received(records, now),
	})
	usedConfigured := slices.Contains(selection.Used, dnssource.ClassConfigured)
	out := make([]upstream, 0, len(selection.Servers))
	for _, addr := range selection.Servers {
		if usedConfigured && slices.Contains(configured, addr) {
			for _, u := range cfg.Forward {
				if u.IP == addr {
					out = append(out, u)
				}
			}
			continue
		}
		out = append(out, upstream{IP: addr, Port: defaultPort})
	}
	// The query path shares the timeout equally among the upstreams, so the
	// learned ones are bounded like the written ones, in list order.
	if len(out) > forwardServersMax {
		loggerPtr.Load().Warn("dns-resolver: upstreams beyond the limit not asked", "path", forwardDNSSourcePath,
			"selected", len(out), "asked", forwardServersMax)
		out = out[:forwardServersMax]
	}
	return out
}

// resolver is the LAN resolver's runtime: the published policy, the two
// caches, the in-flight table and the limits. One resolver lives for the
// whole plugin run; a config apply replaces only the policy and the cache
// bound. The query path is safe for concurrent use.
//
// Lifecycle: newResolver, then start, which launches the one sweeper
// goroutine; stop MUST be called once the plugin stops, and waits for it.
type resolver struct {
	policy atomic.Pointer[policy]
	// publish serializes every store of policy and window: a config apply
	// and a learned-record change both publish, and the second MUST build
	// on the config the first published, never on one it replaced.
	publish sync.Mutex
	// window is the config a reload's narrowed window publishes, nil
	// outside one; a learned change inside the window keeps the narrowing.
	// Guarded by publish.
	window *resolverConfig
	// learned holds the newest system-dns snapshot's records.
	learned   dnssource.Follower
	shown     atomic.Pointer[resolverConfig] // the applied config, for show and for a reload's narrowed window (narrow)
	answers   *lru
	negatives *lru
	flights   flightTable
	limits    limiter
	stats     resolverStats
	listeners listenerStates           // which listener sockets are bound, for show and the gauge
	signed    *dnsserver.SignedQueries // the signed queries the listeners read, for passThrough
	now       func() time.Time

	// replay is closed once the plugin accepted its first system-dns
	// snapshot, the replay the engine answers a starting plugin's request
	// with, so no query is answered from an empty learned list while leases
	// exist (spec-home-cpe-23b AC-13). nil when no replay is expected (a
	// resolver a test builds): admit then never waits. Set by expectReplay
	// before any listener is bound, and closed once through replayOnce.
	replay     chan struct{}
	replayOnce sync.Once

	quit chan struct{}
	done chan struct{}
}

// replayWaitMax bounds how long a query waits for the learned-resolver
// replay. The engine answers the request in milliseconds; past the bound the
// resolver answers from what it holds rather than leave a client without an
// answer until its own timeout, and every later query reads the held list.
const replayWaitMax = 2 * time.Second

// expectReplay makes admit hold queries until replayed runs. It MUST be
// called before the plugin binds a listener, so no query reads replay while
// it is written; replayed MUST then be called once the replay arrived or can
// no longer arrive.
func (r *resolver) expectReplay() {
	r.replay = make(chan struct{})
}

// replayed releases every query admit holds, and every later one. It MUST be
// called after expectReplay. Safe for concurrent use.
func (r *resolver) replayed() {
	r.replayOnce.Do(func() { close(r.replay) })
}

// awaitReplay holds the calling query until replayed ran or replayWaitMax
// passed. Once released it costs one non-blocking channel receive. Safe for
// concurrent use.
func (r *resolver) awaitReplay() {
	ch := r.replay
	if ch == nil {
		return
	}
	select {
	case <-ch:
		return
	default:
	}
	timer := time.NewTimer(replayWaitMax)
	defer timer.Stop()
	select {
	case <-ch:
	case <-timer.C:
		r.replayed()
	}
}

// newResolver builds a resolver with no policy: until the first apply every
// query is answered SERVFAIL, and no listener is bound before that apply.
func newResolver(now func() time.Time) *resolver {
	return &resolver{
		answers:   newLRU(defaultCacheSize),
		negatives: newLRU(defaultCacheSize),
		flights:   flightTable{flights: make(map[cacheKey]*flight)},
		limits:    limiter{clients: make(map[clientKey]clientWindow)},
		signed:    dnsserver.NewSignedQueries(),
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
	r.publish.Lock()
	defer r.publish.Unlock()
	r.window = nil
	r.shown.Store(&shown)
	r.policy.Store(newPolicy(cfg, forwardUpstreams(cfg, r.learned.Records(), r.now())))
}

// relearn takes one system-dns snapshot delivered to the plugin and, when it
// is newer than the last, publishes the running policy again with the
// upstreams its list now selects. Nothing is rebound: only the forward list
// the next query reads changes. Inside a reload's narrowed window the
// window's policy is rebuilt, so the narrowing holds.
func (r *resolver) relearn(event string) error {
	fresh, err := r.learned.Accept(event)
	if err != nil {
		return err
	}
	if r.replay != nil {
		r.replayed()
	}
	if !fresh {
		return nil
	}
	r.publish.Lock()
	defer r.publish.Unlock()
	cfg := r.window
	if cfg == nil {
		cfg = r.shown.Load()
	}
	// refuse-by-default: guards apply -- no applied config means no policy yet; the first apply reads these records
	if cfg == nil {
		return nil
	}
	r.policy.Store(newPolicy(cfg, forwardUpstreams(cfg, r.learned.Records(), r.now())))
	return nil
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
	r.publish.Lock()
	defer r.publish.Unlock()
	running := runningState{policy: r.policy.Load(), shown: r.shown.Load()}
	// refuse-by-default: guards apply -- no running config means no running policy, the SERVFAIL state newResolver leaves
	if running.shown == nil {
		return running
	}
	window := *running.shown
	window.AllowFrom = intersectPrefixes(running.shown.AllowFrom, next.AllowFrom)
	r.window = &window
	r.policy.Store(newPolicy(&window, forwardUpstreams(&window, r.learned.Records(), r.now())))
	return running
}

// restore publishes the running state narrow answered again, after a
// refused reload. The cache bound was never changed by the reload.
func (r *resolver) restore(running runningState) {
	r.publish.Lock()
	defer r.publish.Unlock()
	r.window = nil
	r.shown.Store(running.shown)
	if running.shown == nil {
		r.policy.Store(running.policy)
		return
	}
	// A learned change inside the window moved the upstreams, so the running
	// policy is rebuilt over today's records rather than put back stale.
	r.policy.Store(newPolicy(running.shown, forwardUpstreams(running.shown, r.learned.Records(), r.now())))
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
