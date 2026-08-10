// Design: docs/architecture/isis/isis-9-spf-rib.md -- the SPF orchestrator (trigger -> run -> install).
// Related: spflog.go -- the bounded SPF-run history `show isis spf-log` renders.
// Related: leak.go -- the RFC 2966 inter-level leak set this run hands the engine.
// The Computer ties the pieces together: a debounced, event-driven trigger
// (ISO/IEC 10589 clause 7.2 / research guide sec 5: re-run SPF on an LSDB change,
// coalescing a burst of LSP arrivals into one run per level), the per-level graph
// build (graph.go) + Dijkstra (spf.go), the prefix attach + L1/L2 leak + diff
// (route.go), and the Loc-RIB install (install.go). It also owns and registers
// the SPF-specific Prometheus series (umbrella canonical, owner isis-9):
// ze_isis_spf_runs_total{level}, ze_isis_spf_duration_seconds{level},
// ze_isis_spf_nodes{level}, plus ze_isis_routes_installed via the Installer.
//
// RFC: rfc/short/rfc2966.md -- after each Run, LeakPrefixes (leak.go) computes the
//   inter-level leak set an L1L2 router re-originates (the engine wires SetOnLeak).
//
// The Computer is engine-agnostic: it reads the LSDB through the Source interface
// and resolves next-hops through NextHopResolver, so it is fully unit-testable on
// a hand-built topology with no transport, no goroutines, and no real clock.

package spf

import (
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// DefaultDebounce is the SPF debounce window: an LSDB change marks the level
// dirty and arms this timer; a burst of changes within the window collapses to
// one SPF run per level (spec R-3 / AC-9, research guide sec 5). A few hundred ms
// balances convergence latency against thrash on a flapping link.
const DefaultDebounce = 200 * time.Millisecond

// spfDurationBuckets are the histogram buckets for ze_isis_spf_duration_seconds:
// SPF over a small-to-medium area runs in microseconds to low milliseconds, so
// the buckets span 50us..500ms to keep useful resolution at both ends.
var spfDurationBuckets = []float64{
	0.00005, 0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5,
}

// Computer runs SPF per level and installs the result into the Loc-RIB. It is
// safe for concurrent Trigger calls (the debounce + run are serialized
// internally). One Computer instance serves one IS-IS engine.
type Computer struct {
	src      Source
	resolver NextHopResolver
	levels   []Level
	root     types.SystemID
	debounce time.Duration

	installer *Installer

	// IPv6 (isis-12): the IPv6 next-hop resolver and Loc-RIB Installer. Both are
	// optional; when resolverV6 or installerV6 is nil the IPv6 pass is skipped
	// (IPv4-only node). The SPF tree (graphs/results) is SHARED with IPv4: the
	// IPv6 pass only re-extracts TLV 236 leaves and installs the IPv6 family.
	resolverV6  NextHopResolverV6
	installerV6 *Installer

	mu sync.Mutex
	// last holds the most recent installed IPv4 route set so a re-run diffs against
	// it and the snapshot reflects what is in the Loc-RIB.
	last []RouteEntry
	// lastV6 is the IPv6 equivalent of last (isis-12).
	lastV6 []RouteEntry

	// onChange, when set, is called after every Run that produced a non-empty
	// route delta, with the applied delta. It is the REDISTRIBUTION read seam
	// (spec-isis-11): the engine wires it to emit redistevents batches so IS-IS
	// SPF routes reach the redistribute-orchestrator and BGP. It is NOT the FIB
	// install path (the Installer above owns that). Called outside the run lock so
	// the callback cannot deadlock on a re-entrant Trigger.
	onChange func(RouteDelta)
	// onChangeV6 is the IPv6 redistribution read seam (isis-12): the engine wires
	// it to emit redistevents batches at AFI=2 for IS-IS IPv6 SPF routes. Separate
	// from onChange so the IPv4 and IPv6 deltas carry their own family.
	onChangeV6 func(RouteDelta)
	// onLeak is the RFC 2966 inter-level leak seam (isis-9 AC-4 / AC-5): after every
	// Run computes the per-level reachable prefixes, the Computer hands the engine
	// the set of prefixes to re-originate into each level (L2->L1 with the up/down
	// bit set, L1->L2 up), so an L1L2 router's own LSP carries the other level's
	// reachability. The engine stores the set and re-originates ONLY when it
	// changed; the leak skips down-bit prefixes, so the re-origination's SPF run
	// recomputes the SAME set and the loop terminates in one pass. nil disables it
	// (a single-level node, or an early test). Called outside the run lock so the
	// re-Trigger it provokes cannot deadlock.
	onLeak func(LeakResult)
	// lastLeak is the most recent computed leak set, exposed via LeakResult() for
	// the engine to read after a Run (and for tests). Guarded by c.mu.
	lastLeak LeakResult

	// Debounce state: a Trigger sets dirty and (if no timer is pending) arms one.
	timer   *time.Timer
	pending bool
	// stopped is set under c.mu by Stop(); once set, a debounce callback that
	// fires after Stop (or a direct Run) is a NO-OP, so a Run racing shutdown can
	// never re-install IS-IS routes the engine just removed (stale FIB on
	// shutdown / NET removal). Set-once: a stopped Computer stays stopped.
	stopped bool
	// runWG tracks the debounce timer goroutine (the time.AfterFunc callback in
	// Trigger). Add(1) happens under c.mu when a timer is armed; the callback
	// Done()s when it returns. Stop() Wait()s on it OUTSIDE the lock so engine
	// shutdown can drain any in-flight Run before transport.Close, guaranteeing no
	// SPF run is still touching the Loc-RIB after Stop returns.
	runWG sync.WaitGroup

	// now/newTimer are injectable for tests (a fake clock); production uses
	// time.Now and time.AfterFunc.
	afterFunc func(time.Duration, func()) *time.Timer

	// SPF metrics (umbrella canonical, owner isis-9).
	mRuns     metrics.CounterVec   // ze_isis_spf_runs_total{level}
	mDuration metrics.HistogramVec // ze_isis_spf_duration_seconds{level}
	mNodes    metrics.GaugeVec     // ze_isis_spf_nodes{level}

	// spflog is the bounded SPF-run history surfaced by `show isis spf-log`
	// (spec-isis-13). It has its own lock so recording on the run goroutine and
	// reading on the CLI goroutine never contend on c.mu. Observational only.
	spflog spfLog
}

// Config configures a Computer. src and resolver are mandatory (a nil Source
// yields empty graphs; a nil resolver drops every next-hop). Levels lists the
// levels to compute (L1, L2, or both for an L1L2 node). Loc may be nil (forked
// subprocess; install no-ops).
type Config struct {
	Source    Source
	Resolver  NextHopResolver
	Root      types.SystemID
	Levels    []Level
	Debounce  time.Duration
	Installer *Installer
	// ResolverV6 / InstallerV6 enable the IPv6 install pass (isis-12). Both nil
	// (an IPv4-only node) skips IPv6 entirely. The SPF tree is shared; the IPv6
	// pass only adds TLV 236 leaf extraction + IPv6-family Loc-RIB insertion.
	ResolverV6  NextHopResolverV6
	InstallerV6 *Installer
}

// NewComputer constructs a Computer from cfg. A zero Debounce uses
// DefaultDebounce. A nil Installer is created with a nil Loc-RIB (install
// no-ops); callers wanting real install pass NewInstaller(locrib.Default()).
func NewComputer(cfg Config) *Computer {
	deb := cfg.Debounce
	if deb <= 0 {
		deb = DefaultDebounce
	}
	levels := cfg.Levels
	if len(levels) == 0 {
		levels = []Level{Level1, Level2}
	}
	inst := cfg.Installer
	if inst == nil {
		inst = NewInstaller(nil)
	}
	nop := metrics.NopRegistry{}
	return &Computer{
		src:         cfg.Source,
		resolver:    cfg.Resolver,
		levels:      levels,
		root:        cfg.Root,
		debounce:    deb,
		installer:   inst,
		resolverV6:  cfg.ResolverV6,
		installerV6: cfg.InstallerV6,
		afterFunc:   time.AfterFunc,
		mRuns:       nop.CounterVec("", "", nil),
		mDuration:   nop.HistogramVec("", "", nil, nil),
		mNodes:      nop.GaugeVec("", "", nil),
	}
}

// SetMetrics registers the SPF-owned Prometheus series (this spec OWNS exactly
// these rows from the umbrella canonical Metrics table) and forwards the registry
// to the Installer for ze_isis_routes_installed. A nil registry is ignored.
func (c *Computer) SetMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	c.mu.Lock()
	c.mRuns = reg.CounterVec(
		"ze_isis_spf_runs_total",
		"Total IS-IS SPF (Dijkstra) runs, by level.",
		[]string{"level"},
	)
	c.mDuration = reg.HistogramVec(
		"ze_isis_spf_duration_seconds",
		"IS-IS SPF run duration in seconds, by level.",
		spfDurationBuckets,
		[]string{"level"},
	)
	c.mNodes = reg.GaugeVec(
		"ze_isis_spf_nodes",
		"Number of nodes in the last IS-IS SPF run, by level.",
		[]string{"level"},
	)
	c.mu.Unlock()
	c.installer.SetMetrics(reg)
	if c.installerV6 != nil {
		c.installerV6.SetMetrics(reg)
	}
}

// SetOnChange installs the redistribution read callback (spec-isis-11): after
// every Run that produced a non-empty delta, the Computer calls fn with the
// applied delta so the engine can emit redistevents batches (export IS-IS -> BGP).
// nil disables it. Safe to call before any Trigger; the callback runs outside the
// run lock so it may re-Trigger without deadlocking. This is SEPARATE from the FIB
// install (the Installer): redistribution NEVER installs to the kernel.
func (c *Computer) SetOnChange(fn func(RouteDelta)) {
	c.mu.Lock()
	c.onChange = fn
	c.mu.Unlock()
}

// SetOnChangeV6 installs the IPv6 redistribution read callback (isis-12): after
// every Run that produced a non-empty IPv6 delta, the Computer calls fn so the
// engine can emit redistevents batches at AFI=2 (export IS-IS IPv6 -> BGP). nil
// disables it. Separate from SetOnChange so the IPv4 and IPv6 deltas are emitted
// with their own family. Like SetOnChange this is the REDISTRIBUTION path only,
// never the FIB install.
func (c *Computer) SetOnChangeV6(fn func(RouteDelta)) {
	c.mu.Lock()
	c.onChangeV6 = fn
	c.mu.Unlock()
}

// SetOnLeak installs the RFC 2966 inter-level leak callback (isis-9 AC-4/AC-5):
// after every Run, the Computer calls fn with the prefixes to re-originate into
// each level (L2->L1 down, L1->L2 up). The engine stores the set and
// re-originates only when it changed; because the leak skips down-bit prefixes,
// the re-origination's SPF run recomputes the same set and the feedback loop
// terminates in one pass (no churn). nil disables leaking. Safe to call before
// any Trigger; the callback runs outside the run lock so it may re-Trigger
// without deadlocking. This is SEPARATE from the FIB install and the
// redistribution seam: leaking only re-originates the node's own LSP.
func (c *Computer) SetOnLeak(fn func(LeakResult)) {
	c.mu.Lock()
	c.onLeak = fn
	c.mu.Unlock()
}

// LeakResult returns the most recent inter-level leak set computed by Run (the
// prefixes leaked into each level). Empty on a single-level node or before the
// first Run. A copy of the slices is not made (the engine treats them as
// read-only); they are replaced wholesale on the next Run.
func (c *Computer) LeakResult() LeakResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastLeak
}

// SetRoot updates the System ID SPF is rooted at (the node's own ID). Called once
// the config resolves. Safe to call before any Trigger.
func (c *Computer) SetRoot(root types.SystemID) {
	c.mu.Lock()
	c.root = root
	c.mu.Unlock()
}

// SetLevels updates the levels SPF computes (the node's configured level: L1, L2,
// or both). An empty slice leaves both levels. Called from setConfig once the
// level is known; safe before any Trigger.
func (c *Computer) SetLevels(levels []Level) {
	if len(levels) == 0 {
		return
	}
	c.mu.Lock()
	c.levels = append([]Level(nil), levels...)
	c.mu.Unlock()
}

// Trigger marks SPF dirty and arms the debounce timer if one is not already
// pending; a burst of Triggers within the window results in a single Run (spec
// AC-9). It returns immediately (non-blocking); the actual SPF runs from the
// timer callback on a separate goroutine. A nil afterFunc (never in production)
// would run synchronously.
func (c *Computer) Trigger() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return // shutting down: do not arm a timer that would re-install routes
	}
	if c.pending {
		return // a run is already scheduled; it will pick up this change
	}
	c.pending = true
	// Track the timer goroutine so Stop() can drain an in-flight Run before the
	// engine closes the transport (no Run touching the Loc-RIB after Stop). Add(1)
	// under the lock, paired with the callback's Done(), so Stop cannot miss it.
	c.runWG.Add(1)
	// Arm the timer under the lock so concurrent Trigger calls (LSDB changes from
	// different circuit goroutines) never race on c.timer. time.AfterFunc runs the
	// callback on its OWN goroutine, so it cannot re-enter and deadlock on c.mu
	// here. The callback clears pending and runs SPF.
	c.timer = c.afterFunc(c.debounce, func() {
		defer c.runWG.Done()
		c.mu.Lock()
		c.pending = false
		stopped := c.stopped
		c.mu.Unlock()
		if stopped {
			return // Stop() ran before the timer fired: do not re-install routes
		}
		c.Run()
	})
}

// Run computes SPF for every configured level NOW (bypassing the debounce) and
// installs the arbitrated route set into the Loc-RIB. It is the synchronous core
// the debounce timer calls; tests call it directly. It returns the applied
// delta. Concurrent Run calls are serialized by the run lock so two timers (or a
// Run plus a timer) never interleave a half-applied set.
func (c *Computer) Run() RouteDelta {
	c.mu.Lock()
	if c.stopped {
		// A Run racing Stop (a debounce callback that slipped past its own guard,
		// or a direct call after Stop) must not re-install the routes Stop removed.
		// Re-checked here under the lock so the window between the callback's guard
		// and entering Run is closed; Stop's RemoveAll is serialized by this lock.
		c.mu.Unlock()
		return RouteDelta{}
	}
	root := c.root
	levels := append([]Level(nil), c.levels...)
	c.mu.Unlock()

	results := make([]*Result, 0, len(levels))
	graphs := make(map[Level]*Graph, len(levels))
	for _, level := range levels {
		start := time.Now()
		g := BuildGraph(c.src, level)
		res := Compute(g, root, level)
		graphs[level] = g
		results = append(results, res)

		// SPF metrics: count the run, record its duration, and the node count.
		lvl := level.String()
		dur := time.Since(start)
		c.mRuns.With(lvl).Inc()
		c.mDuration.With(lvl).Observe(dur.Seconds())
		c.mNodes.With(lvl).Set(float64(len(g.Nodes)))
		// Record the run in the bounded SPF log surfaced by `show isis spf-log`
		// (spec-isis-13 AC-6). Observational only; never alters routing state.
		c.spflog.record(start, lvl, dur, len(g.Nodes))
	}

	routes := BuildRoutes(results, graphs, c.resolver)

	// IPv6 pass (isis-12): reuse the SAME results/graphs (the shared SPF tree),
	// re-extract TLV 236 leaves, resolve IPv6 next-hops, and install the IPv6
	// family. Skipped entirely when IPv6 is not wired (IPv4-only node). No second
	// Dijkstra runs; only the leaf attach + install differ.
	var routesV6 []RouteEntry
	if c.installerV6 != nil {
		routesV6 = BuildRoutesV6(results, graphs, c.resolverV6)
	}

	// RFC 2966 inter-level leak (isis-9 AC-4/AC-5): compute the prefixes an L1L2
	// router re-originates into each level from the OTHER level's reachable set
	// (L2->L1 with the up/down bit, L1->L2 up; down-bit prefixes skipped for loop
	// prevention). Over the SHARED SPF tree, no extra Dijkstra runs. Empty on a
	// single-level node.
	leak := LeakPrefixes(results, graphs)

	c.mu.Lock()
	if c.stopped {
		// Stop() ran while this Run was in its compute phase: between releasing the
		// lock after the entry guard (above) and re-acquiring it here, the compute
		// holds NO lock, so Stop's RemoveAll can land in that window. Applying now
		// would re-install the routes Stop just removed (stale FIB on shutdown / NET
		// removal). The entry guard alone does NOT cover this: it runs before the
		// lock-free compute. RemoveAll is serialized with this section by c.mu, so a
		// stopped flag seen here means the Loc-RIB was already cleared -- skip the
		// apply (and the redistribution/leak seams below) entirely.
		c.mu.Unlock()
		return RouteDelta{}
	}
	delta := c.installer.Apply(routes)
	c.last = routes
	var deltaV6 RouteDelta
	if c.installerV6 != nil {
		deltaV6 = c.installerV6.Apply(routesV6)
		c.lastV6 = routesV6
	}
	c.lastLeak = leak
	onChange := c.onChange
	onChangeV6 := c.onChangeV6
	onLeak := c.onLeak
	c.mu.Unlock()

	// Redistribution read seam (spec-isis-11 / isis-12): notify the engine of the
	// per-family delta so it can emit redistevents batches (export IS-IS -> BGP).
	// Outside the run lock so the callback may re-Trigger without deadlocking; only
	// fired on a real change.
	if onChange != nil && !delta.Empty() {
		onChange(delta)
	}
	if onChangeV6 != nil && !deltaV6.Empty() {
		onChangeV6(deltaV6)
	}
	// Inter-level leak seam (isis-9 AC-4/AC-5): hand the engine the per-level leak
	// set so it re-originates the other level's reachability into this node's own
	// LSP. Always invoked (even on an empty leak) so the engine can CLEAR a stale
	// leaked set when the source level loses a prefix; the engine re-originates
	// only on a real change, so an unchanged leak is cheap. Outside the run lock so
	// the re-origination's Trigger cannot deadlock.
	if onLeak != nil {
		onLeak(leak)
	}
	return delta
}

// Snapshot returns the `show isis route` view of the currently installed route
// set (rendered by isis-13). It reflects the last Run.
func (c *Computer) Snapshot() []RouteSnapshotEntry {
	c.mu.Lock()
	routes := append([]RouteEntry(nil), c.last...)
	c.mu.Unlock()
	return Snapshot(routes)
}

// Routes returns a copy of the currently installed IPv4 route set (for tests and
// the engine snapshot). Order is unspecified.
func (c *Computer) Routes() []RouteEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]RouteEntry(nil), c.last...)
}

// SnapshotV6 returns the `show isis route ipv6` view of the currently installed
// IPv6 route set (isis-12), reflecting the last Run.
func (c *Computer) SnapshotV6() []RouteSnapshotEntry {
	c.mu.Lock()
	routes := append([]RouteEntry(nil), c.lastV6...)
	c.mu.Unlock()
	return Snapshot(routes)
}

// SetSPFLogTrigger records the reason the next recorded SPF run will report in
// `show isis spf-log` (spec-isis-13). The engine debounce path passes
// "lsdb-change"; a direct Run with no trigger set reports "manual". Safe to call
// concurrently with Run.
func (c *Computer) SetSPFLogTrigger(reason string) { c.spflog.setTrigger(reason) }

// SPFLog returns the recorded SPF-run history, newest first (the `show isis
// spf-log` rows, spec-isis-13 AC-6). It never exposes the live ring.
func (c *Computer) SPFLog() []SPFLogEntry { return c.spflog.snapshot() }

// ResetSPFLog clears the SPF-run history. `clear isis counters` calls it to
// reset observational state without disturbing the route set (spec-isis-13 AC-8).
func (c *Computer) ResetSPFLog() { c.spflog.reset() }

// Stop forward-removes every installed IS-IS route from the Loc-RIB, cancels any
// pending debounce timer, and marks the Computer stopped so a debounce callback
// that already fired (or a later Trigger/Run) cannot re-install routes (engine
// shutdown / NET removal -> no stale FIB). It then drains any in-flight timer
// goroutine OUTSIDE the lock so the engine can sequence Stop before
// transport.Close with the guarantee that no SPF run is still touching the
// Loc-RIB. After Stop the Computer holds no routes and stays stopped.
func (c *Computer) Stop() {
	c.mu.Lock()
	c.stopped = true
	if c.timer != nil && c.pending {
		// timer.Stop() reports true only if it stopped the AfterFunc BEFORE it
		// started: then the callback (and its deferred runWG.Done) will NEVER run,
		// so balance the Add here. A false return means the callback has already
		// started/finished and will Done() itself; do not double-decrement.
		if c.timer.Stop() {
			c.runWG.Done()
		}
	}
	c.pending = false
	c.installer.RemoveAll()
	c.last = nil
	if c.installerV6 != nil {
		c.installerV6.RemoveAll()
	}
	c.lastV6 = nil
	c.mu.Unlock()
	// Drain a timer callback that already fired (it is on its own goroutine and
	// will Done()). Its Run is a no-op now (stopped guard), but waiting guarantees
	// no concurrent Loc-RIB access remains once Stop returns. Outside the lock: the
	// callback takes c.mu, so Wait()ing under it would deadlock.
	c.runWG.Wait()
}
