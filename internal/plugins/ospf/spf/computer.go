// Design: plan/learned/962-ospf-8-spf-rib.md -- SPF trigger, throttle, run state, metrics.
// The Computer ties LSDB changes to graph build, Dijkstra, route selection, and
// Loc-RIB installation. It owns the SPF metrics surfaced by later CLI work.

package spf

import (
	"maps"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const (
	DefaultSPFDelay   = 50 * time.Millisecond
	DefaultSPFHold    = 200 * time.Millisecond
	DefaultSPFMaxHold = 5 * time.Second
)

var spfDurationBuckets = []float64{0.00005, 0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5}

type timerHandle interface{ Stop() bool }

// Computer runs per-area OSPF SPF and installs the selected route set.
type Computer struct {
	src          Source
	resolver     InterfaceResolver
	root         types.RouterID
	areas        []types.AreaID
	maxPaths     int
	areaOptions  map[types.AreaID]types.Options
	areaRanges   map[types.AreaID][]AreaRange
	areaPolicies map[types.AreaID]AreaSummaryPolicy
	summarySink  SummarySink
	summaryAreas []types.AreaID

	strategy AFPrefixStrategy

	installer *Installer

	mu sync.Mutex

	last       []RouteEntry
	lastBorder []BorderRouterEntry
	// lastCandidates retains every per-prefix candidate route considered by the most
	// recent Run (intra/inter/external), BEFORE selectBestRoutes collapsed them to one
	// winner. The read-only SPF-explain view (spec-ospf-ext-14) reads it to show why a
	// route won WITHOUT a recompute. It never feeds the installed table.
	lastCandidates []RouteEntry
	// runs counts completed SPF computations. The explain view reads it read-only so a
	// test can assert an explain call did NOT trigger a recompute (R-3).
	runs uint64
	// lastGraphs retains the per-area transit graph built by the most recent Run so
	// RFC 6138 cut-edge queries can be answered from the last SPF result without a
	// second Dijkstra pass (Appendix A: "a cut-edge computation should not require any
	// extra SPF runs"). Read-only after assignment; IsCutEdge only traverses it.
	lastGraphs   map[types.AreaID]*Graph
	onChange     func(RouteDelta)
	postRun      func()
	delay        time.Duration
	hold         time.Duration
	maxHold      time.Duration
	currentDelay time.Duration
	lastTrigger  time.Time
	timer        timerHandle
	pending      bool
	dirty        map[types.AreaID]struct{}
	stopped      bool
	runWG        sync.WaitGroup
	afterFunc    func(time.Duration, func()) timerHandle
	now          func() time.Time

	state map[types.AreaID]spfState

	// virtualLinks are the configured virtual links resolved against their transit
	// area's SPF result each run (RFC 2328 sec 16.1). onVirtual fires with the resolved
	// set when it changes (drives the engine's synthetic virtual interface); lastVirtual
	// is the previous resolution, so an unchanged cost/reachability does not flap.
	virtualLinks []VirtualLinkRequest
	onVirtual    func([]VirtualNeighborResult)
	lastVirtual  map[VirtualLinkRequest]VirtualNeighborResult

	mRuns     metrics.CounterVec
	mDuration metrics.HistogramVec
	mABR      metrics.Gauge
	mSummary  metrics.GaugeVec
	mTransit  metrics.CounterVec

	// frr is the resolved fast-reroute config; srResolver reads ext-5's SR label
	// maps for a TI-LFA repair list (nil for base-LFA-only, e.g. the v6 engine).
	frr        FastRerouteConfig
	srResolver SRResolver

	mFRRProtected    metrics.GaugeVec
	mFRRUnprotected  metrics.GaugeVec
	mFRRInstalled    metrics.GaugeVec
	mFRRCompute      metrics.HistogramVec
	mFRRRepairLabels metrics.GaugeVec
}

// Config configures one SPF Computer. A nil Source yields empty graphs; a nil
// Installer tracks snapshots but does not write the Loc-RIB.
type Config struct {
	Source      Source
	Resolver    InterfaceResolver
	Root        types.RouterID
	Areas       []types.AreaID
	MaxPaths    int
	AreaConfigs []AreaConfig
	SPFDelay    time.Duration
	SPFHold     time.Duration
	SPFMaxHold  time.Duration
	Installer   *Installer
	SummarySink SummarySink
	// Strategy is the address-family prefix strategy (graph decode + prefix
	// attachment). Nil selects the OSPFv2 strategy; the engine injects the v6
	// strategy for the IPv6 family.
	Strategy AFPrefixStrategy
}

// spfState is one area's most recent run state for `show ospf spf`.
type spfState struct {
	Area            types.AreaID
	LastRun         time.Time
	Duration        time.Duration
	NodeCount       int
	CurrentDelay    time.Duration
	Pending         bool
	ConsecutiveHold time.Duration
}

// NewComputer constructs an OSPF SPF Computer.
func NewComputer(cfg Config) *Computer {
	delay, hold, maxHold := normaliseTimers(cfg.SPFDelay, cfg.SPFHold, cfg.SPFMaxHold)
	areas := append([]types.AreaID(nil), cfg.Areas...)
	if len(areas) == 0 {
		areas = []types.AreaID{types.BackboneArea}
	}
	maxPaths := cfg.MaxPaths
	if maxPaths <= 0 {
		maxPaths = DefaultMaxPaths
	}
	inst := cfg.Installer
	if inst == nil {
		inst = NewInstaller(nil)
	}
	strategy := cfg.Strategy
	if strategy == nil {
		strategy = v4Strategy{}
	}
	nop := metrics.NopRegistry{}
	areaOptions, areaRanges, areaPolicies := areaConfigMaps(cfg.AreaConfigs)
	return &Computer{
		src:              cfg.Source,
		resolver:         cfg.Resolver,
		root:             cfg.Root,
		areas:            areas,
		maxPaths:         maxPaths,
		areaOptions:      areaOptions,
		areaRanges:       areaRanges,
		areaPolicies:     areaPolicies,
		summarySink:      cfg.SummarySink,
		strategy:         strategy,
		installer:        inst,
		delay:            delay,
		hold:             hold,
		maxHold:          maxHold,
		currentDelay:     delay,
		dirty:            make(map[types.AreaID]struct{}),
		afterFunc:        func(d time.Duration, f func()) timerHandle { return time.AfterFunc(d, f) },
		now:              time.Now,
		state:            make(map[types.AreaID]spfState),
		lastVirtual:      make(map[VirtualLinkRequest]VirtualNeighborResult),
		mRuns:            nop.CounterVec("", "", nil),
		mDuration:        nop.HistogramVec("", "", nil, nil),
		mABR:             nop.Gauge("", ""),
		mSummary:         nop.GaugeVec("", "", nil),
		mTransit:         nop.CounterVec("", "", nil),
		mFRRProtected:    nop.GaugeVec("", "", nil),
		mFRRUnprotected:  nop.GaugeVec("", "", nil),
		mFRRInstalled:    nop.GaugeVec("", "", nil),
		mFRRCompute:      nop.HistogramVec("", "", nil, nil),
		mFRRRepairLabels: nop.GaugeVec("", "", nil),
	}
}

// SetFastReroute installs the resolved fast-reroute config. When Enabled is false
// the LFA/TI-LFA pass is skipped and the route set is byte-for-byte as before.
func (c *Computer) SetFastReroute(cfg FastRerouteConfig) {
	c.mu.Lock()
	c.frr = cfg
	c.mu.Unlock()
}

// SetSRResolver installs the ext-5 SR label resolver a TI-LFA repair list is
// built from. A nil resolver disables TI-LFA (base LFA still runs); the v6 engine
// leaves it nil because OSPFv3 SR carriage (RFC 8666) is out of scope.
func (c *Computer) SetSRResolver(r SRResolver) {
	c.mu.Lock()
	c.srResolver = r
	c.mu.Unlock()
}

func normaliseTimers(delay, hold, maxHold time.Duration) (time.Duration, time.Duration, time.Duration) {
	if delay < 0 {
		delay = 0
	}
	if delay == 0 {
		delay = DefaultSPFDelay
	}
	if hold <= 0 {
		hold = DefaultSPFHold
	}
	if maxHold <= 0 {
		maxHold = DefaultSPFMaxHold
	}
	if hold < delay {
		hold = delay
	}
	if maxHold < hold {
		maxHold = hold
	}
	return delay, hold, maxHold
}

// SetMetrics registers ze_ospf_spf_runs_total{area} and
// ze_ospf_spf_duration_seconds{area}, then forwards metrics to the Installer.
func (c *Computer) SetMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	c.mu.Lock()
	c.mRuns = reg.CounterVec(
		"ze_ospf_spf_runs_total",
		"Total OSPF SPF runs, by area.",
		[]string{"area"},
	)
	c.mDuration = reg.HistogramVec(
		"ze_ospf_spf_duration_seconds",
		"OSPF SPF run duration in seconds, by area.",
		spfDurationBuckets,
		[]string{"area"},
	)
	c.mABR = reg.Gauge(
		"ze_ospf_abr",
		"Whether this OSPF router is currently an Area Border Router.",
	)
	c.mSummary = reg.GaugeVec(
		"ze_ospf_summary_lsas",
		"Current OSPF self-originated Summary-LSAs, by area.",
		[]string{"area"},
	)
	c.mTransit = reg.CounterVec(
		"ze_ospf_transit_area_passes_total",
		"Total OSPF RFC 2328 section 16.3 transit-area summary passes, by transit area.",
		[]string{"transit_area"},
	)
	c.mFRRProtected = reg.GaugeVec(
		"ze_ospf_fast_reroute_protected_prefixes",
		"Current OSPF prefixes with a fast-reroute backup, by area and protection class.",
		[]string{"area", "class"},
	)
	c.mFRRUnprotected = reg.GaugeVec(
		"ze_ospf_fast_reroute_unprotected_prefixes",
		"Current OSPF prefixes with no fast-reroute backup, by area and reason.",
		[]string{"area", "reason"},
	)
	c.mFRRInstalled = reg.GaugeVec(
		"ze_ospf_fast_reroute_backups_installed",
		"Current OSPF fast-reroute backups installed, by kind (lfa/ti-lfa).",
		[]string{"kind"},
	)
	c.mFRRCompute = reg.HistogramVec(
		"ze_ospf_fast_reroute_compute_seconds",
		"OSPF fast-reroute (LFA/TI-LFA) compute duration in seconds, by area.",
		spfDurationBuckets,
		[]string{"area"},
	)
	c.mFRRRepairLabels = reg.GaugeVec(
		"ze_ospf_fast_reroute_ti_lfa_repair_labels",
		"Current OSPF TI-LFA repair labels pushed, by area.",
		[]string{"area"},
	)
	c.mu.Unlock()
	c.installer.SetMetrics(reg)
}

// SetInstallSuppress installs the graceful-restart route-install suppression predicate on the
// underlying Installer (RFC 3623 sec 2/2.1). While it returns true, SPF still computes but the
// FIB is neither churned nor withdrawn.
func (c *Computer) SetInstallSuppress(fn func() bool) { c.installer.setSuppress(fn) }

// SetRoot updates the local Router ID used as the SPF root.
func (c *Computer) SetRoot(root types.RouterID) {
	c.mu.Lock()
	c.root = root
	c.mu.Unlock()
}

// SetAreas updates the configured areas. An empty list disables SPF computation
// and the next Run withdraws previously installed routes.
func (c *Computer) SetAreas(areas []types.AreaID) {
	c.mu.Lock()
	c.areas = append([]types.AreaID(nil), areas...)
	c.mu.Unlock()
}

// SetAreaConfigs updates per-area SPF metadata: options and ranges.
func (c *Computer) SetAreaConfigs(configs []AreaConfig) {
	options, ranges, policies := areaConfigMaps(configs)
	c.mu.Lock()
	c.areaOptions = options
	c.areaRanges = ranges
	c.areaPolicies = policies
	c.mu.Unlock()
}

// SetOnChange registers a callback invoked after each SPF run whose installed route
// set changed, with the non-empty route delta. It is the REDISTRIBUTION producer
// trigger (OSPF -> BGP via redistevents); it is NOT the FIB-install path (the
// Installer owns that). A nil callback disables it.
func (c *Computer) SetOnChange(fn func(RouteDelta)) {
	c.mu.Lock()
	c.onChange = fn
	c.mu.Unlock()
}

// SetPostRun registers a read-only callback invoked after EVERY SPF run, AFTER the
// Installer has applied the IP route delta. It is the seam Segment Routing uses to
// (re)compute MPLS label entries that ride on top of the just-installed IP routes,
// so an SR push can never be emitted before its underlying IP route exists
// (spec-ospf-ext-5 R-8). Unlike SetOnChange it fires on every run, not only when the
// route set changed, because a remote SR LSA change with an unchanged IP route table
// must still be reflected. A nil callback disables it. The callback MUST NOT mutate
// SPF state; it reads Routes()/Snapshot() only.
func (c *Computer) SetPostRun(fn func()) {
	c.mu.Lock()
	c.postRun = fn
	c.mu.Unlock()
}

// SetMaxPaths updates the ECMP path cap.
func (c *Computer) SetMaxPaths(maxPaths int) {
	if maxPaths <= 0 {
		maxPaths = DefaultMaxPaths
	}
	c.mu.Lock()
	c.maxPaths = maxPaths
	c.mu.Unlock()
}

// SetTimers replaces the SPF throttle timers.
func (c *Computer) SetTimers(delay, hold, maxHold time.Duration) {
	delay, hold, maxHold = normaliseTimers(delay, hold, maxHold)
	c.mu.Lock()
	c.delay = delay
	c.hold = hold
	c.maxHold = maxHold
	if c.currentDelay <= 0 || c.currentDelay > maxHold {
		c.currentDelay = delay
	}
	c.mu.Unlock()
}

// Trigger arms SPF for every configured area.
func (c *Computer) Trigger() { c.TriggerArea(types.BackboneArea) }

// TriggerArea marks an area dirty and arms the exponential back-off timer. A
// burst of LSDB changes coalesces into one Run.
func (c *Computer) TriggerArea(area types.AreaID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}
	c.dirty[area] = struct{}{}
	now := c.now()
	c.bumpDelayLocked(now)
	if c.pending {
		return
	}
	c.pending = true
	delay := c.currentDelay
	c.runWG.Add(1)
	c.timer = c.afterFunc(delay, func() {
		defer c.runWG.Done()
		c.mu.Lock()
		c.pending = false
		stopped := c.stopped
		c.mu.Unlock()
		if stopped {
			return
		}
		c.Run()
	})
}

func (c *Computer) bumpDelayLocked(now time.Time) {
	if c.lastTrigger.IsZero() || now.Sub(c.lastTrigger) > c.maxHold {
		c.currentDelay = c.delay
		c.lastTrigger = now
		return
	}
	if c.currentDelay < c.hold {
		c.currentDelay = c.hold
	} else {
		c.currentDelay *= 2
		if c.currentDelay > c.maxHold {
			c.currentDelay = c.maxHold
		}
	}
	c.lastTrigger = now
}

// Run computes SPF for all configured areas immediately and applies the Loc-RIB
// diff. Tests use Run directly; production normally calls TriggerArea.
func (c *Computer) Run() RouteDelta {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return RouteDelta{}
	}
	root := c.root
	areas := append([]types.AreaID(nil), c.areas...)
	virtualLinks := append([]VirtualLinkRequest(nil), c.virtualLinks...)
	maxPaths := c.maxPaths
	frr := c.frr
	srResolver := c.srResolver
	areaOptions := copyAreaOptions(c.areaOptions)
	areaRanges := copyAreaRanges(c.areaRanges)
	areaPolicies := copyAreaPolicies(c.areaPolicies)
	summarySink := c.summarySink
	previousSummaryAreas := append([]types.AreaID(nil), c.summaryAreas...)
	c.dirty = make(map[types.AreaID]struct{})
	c.mu.Unlock()

	var candidates []RouteEntry
	activeAreas := make([]types.AreaID, 0, len(areas))
	results := make(map[types.AreaID]*Result, len(areas))
	states := make(map[types.AreaID]spfState, len(areas))
	graphs := make(map[types.AreaID]*Graph, len(areas))
	for _, area := range areas {
		start := time.Now()
		g := c.strategy.BuildGraph(c.src, area)
		graphs[area] = g
		res := computeWithNextHop(g, root, maxPaths, c.strategy.NextHopSource())
		results[area] = res
		if isActiveResult(res, root) {
			activeAreas = append(activeAreas, area)
		}
		routes := c.strategy.BuildRoutes(res, maxPaths, c.resolver)
		candidates = append(candidates, routes...)
		dur := time.Since(start)
		label := area.String()
		c.mRuns.With(label).Inc()
		c.mDuration.With(label).Observe(dur.Seconds())
		states[area] = spfState{Area: area, LastRun: start, Duration: dur, NodeCount: len(g.Routers) + len(g.Networks)}
	}
	activeAreas = canonicalAreas(activeAreas)
	summaryAreas := canonicalAreas(append(append([]types.AreaID(nil), areas...), previousSummaryAreas...))
	// RFC 2328 sec 16.1 / RFC 5340 sec 3.5: resolve each configured virtual link from its
	// transit area's intra-area result and, when the endpoint is a Full virtual link (its
	// backbone Router-LSA carries the V-bit), treat the backbone as an active area so the
	// endpoint participates as backbone-attached in the inter-area computation below.
	virtualResults := resolveVirtualNeighbors(virtualLinks, results)
	if len(virtualLinks) > 0 && rootVirtualBackboneAttached(results, root) {
		activeAreas = canonicalAreas(append(activeAreas, types.BackboneArea))
	}
	inter, border := c.strategy.ComputeInterArea(InterAreaInput{Source: c.src, Root: root, Areas: activeAreas, Results: results, Ranges: areaRanges, Resolver: c.resolver, MaxPaths: maxPaths})
	candidates = append(candidates, inter...)
	// RFC 2328 sec 16.3: on a transit area (TransitCapability TRUE) re-examine its
	// Summary-LSAs to IMPROVE already-reachable backbone routes and resolve the real
	// transit next hop for any route whose next hop is a virtual link.
	if len(virtualLinks) > 0 {
		candidates = c.transitAreaPass(results, candidates, virtualResults, maxPaths)
	}
	summary := c.strategy.OriginateSummaries(SummaryInput{Sink: summarySink, Root: root, Areas: activeAreas, FlushAreas: summaryAreas, Options: areaOptions, Ranges: areaRanges, Results: results, InterRoutes: inter, BorderRouters: border, Policies: areaPolicies})
	if IsABR(activeAreas) {
		c.mABR.Set(1)
	} else {
		c.mABR.Set(0)
	}
	for _, area := range summaryAreas {
		c.mSummary.With(area.String()).Set(float64(summary.Counts[area]))
	}
	// Resolve the intra/inter-area table first so RFC 2328 sec 16.4 external
	// computation can resolve the ASBR and a non-zero forwarding address against it.
	internal := selectBestRoutes(candidates, maxPaths)
	var nssaAreas []types.AreaID
	for area, p := range areaPolicies {
		if p.Type == AreaTypeNSSA {
			nssaAreas = append(nssaAreas, area)
		}
	}
	external := c.strategy.ComputeExternal(ExternalInput{Source: c.src, Root: root, BorderRouters: border, Routes: internal, Resolver: c.resolver, MaxPaths: maxPaths, NSSAAreas: nssaAreas, NSSAPolicies: areaPolicies, NSSABorderRouter: IsABR(activeAreas)})
	selected := selectBestRoutes(append(internal, external...), maxPaths)
	// spec-ospf-ext-14: retain every candidate route (raw intra/inter + external, before
	// the per-prefix collapse) so the read-only explain view can show what each winner beat.
	rawCandidates := append(append([]RouteEntry(nil), candidates...), external...)

	// RFC 5286 / TI-LFA fast reroute: attach a per-primary loop-free backup to each
	// route AFTER selection and BEFORE install, gated on the fast-reroute config.
	// Disabled leaves `selected` untouched (byte-for-byte as today).
	if frr.Enabled {
		frrStart := c.now()
		frrStats := attachAllBackups(selected, fastRerouteInput{
			root:         root,
			maxPaths:     maxPaths,
			nh:           c.strategy.NextHopSource(),
			resolver:     c.resolver,
			results:      results,
			graphs:       graphs,
			border:       border,
			virtualLinks: len(virtualLinks) > 0,
			cfg:          frr,
			sr:           srResolver,
		})
		c.publishFRRMetrics(frrStats, c.now().Sub(frrStart))
	}

	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return RouteDelta{}
	}
	delta := c.installer.Apply(selected)
	c.last = selected
	c.lastCandidates = rawCandidates
	c.runs++
	c.lastBorder = border
	c.lastGraphs = graphs
	c.summaryAreas = activeAreas
	for area, st := range states {
		st.Pending = c.pending
		st.CurrentDelay = c.currentDelay
		st.ConsecutiveHold = c.currentDelay
		c.state[area] = st
	}
	onChange := c.onChange
	postRun := c.postRun
	onVirtual, virtualChanged := c.updateVirtualLocked(virtualResults)
	c.mu.Unlock()

	// Redistribution producer trigger (OSPF -> BGP), outside the lock so the
	// emit cannot deadlock against an SPF re-entry. A separate path from FIB install.
	if onChange != nil && !delta.Empty() {
		onChange(delta)
	}
	// Segment Routing post-run hook: fires after the IP-route Installer.Apply so SR
	// label pushes ride the just-installed IP routes (spec-ospf-ext-5 R-8). It fires
	// on every run, outside the lock, because a remote SR LSA change must be reflected
	// even when the IP route table is unchanged.
	if postRun != nil {
		postRun()
	}
	// Virtual-link resolution change (drives the engine's synthetic interface up/down and
	// cost), outside the lock. Fires only when the resolved set changed, so an unchanged
	// transit cost does not flap the virtual link (spec-ospf-ext-7 R-7).
	if onVirtual != nil && virtualChanged {
		onVirtual(virtualResults)
	}
	return delta
}

// Routes returns a copy of the installed route set.
func (c *Computer) Routes() []RouteEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]RouteEntry(nil), c.last...)
}

// RouterReachable reports whether an originating router is reachable in the last SPF
// computation, for the RFC 5250 §5 Type-11 opaque-LSA reachability gate. It reuses the
// reachability the SPF run already produced: the local root is always reachable; a border
// router (ABR/ASBR) with a finite metric and a resolved next-hop is reachable (the same
// ASBR reachability used to validate Type-5 AS-External LSAs); and any router that
// originates an installed route is reachable. An unreachable originator's Type-11 opaque
// LSAs must not be used (RFC 5250 §5).
func (c *Computer) RouterReachable(id types.RouterID) bool {
	if id == (types.RouterID{}) {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if id == c.root {
		return true
	}
	for _, b := range c.lastBorder {
		if b.RouterID == id && b.Metric < LSInfinity && len(b.NextHops) > 0 {
			return true
		}
	}
	for _, r := range c.last {
		if r.Origin == id && len(r.NextHops) > 0 {
			return true
		}
	}
	return false
}

// Snapshot returns the `show ospf route` snapshot.
func (c *Computer) Snapshot() []RouteSnapshotEntry {
	c.mu.Lock()
	routes := append([]RouteEntry(nil), c.last...)
	c.mu.Unlock()
	return Snapshot(routes)
}

// FastRerouteSnapshot returns the `show ospf route fast-reroute` snapshot: each
// prefix's primary next-hops with their RFC 5286 / TI-LFA backups.
func (c *Computer) FastRerouteSnapshot() []FastRerouteSnapshotEntry {
	c.mu.Lock()
	routes := append([]RouteEntry(nil), c.last...)
	c.mu.Unlock()
	return FastRerouteSnapshot(routes)
}

// BorderRouterSnapshot returns the `show ospf border-routers` snapshot.
func (c *Computer) BorderRouterSnapshot() []BorderRouterSnapshotEntry {
	c.mu.Lock()
	rows := append([]BorderRouterEntry(nil), c.lastBorder...)
	c.mu.Unlock()
	return BorderRouterSnapshot(rows)
}

// ClearSPFLog resets the per-area SPF run history shown by `show ospf spf` (clear ip
// ospf counters). The monotonic Prometheus run counter (mRuns) is deliberately NOT reset;
// only the displayed last-run timestamp/duration/node-count history is cleared.
func (c *Computer) ClearSPFLog() {
	c.mu.Lock()
	c.state = make(map[types.AreaID]spfState)
	c.mu.Unlock()
}

// SPFSnapshot returns the `show ospf spf` snapshot.
func (c *Computer) SPFSnapshot() []spfSnapshotEntry {
	c.mu.Lock()
	states := make([]spfState, 0, len(c.state))
	for _, st := range c.state {
		st.Pending = c.pending
		st.CurrentDelay = c.currentDelay
		states = append(states, st)
	}
	c.mu.Unlock()
	return spfSnapshot(states)
}

// Stop cancels pending SPF work and withdraws every installed OSPF route.
func (c *Computer) Stop() {
	c.mu.Lock()
	c.stopped = true
	if c.timer != nil && c.pending {
		if c.timer.Stop() {
			c.runWG.Done()
		}
	}
	c.pending = false
	c.installer.RemoveAll()
	c.last = nil
	c.lastBorder = nil
	c.summaryAreas = nil
	c.mu.Unlock()
	c.runWG.Wait()
}

// spfSnapshotEntry is one `show ospf spf` row.
type spfSnapshotEntry struct {
	Area            string  `json:"area"`
	LastRun         string  `json:"last_run,omitempty"`
	DurationSeconds float64 `json:"duration_seconds"`
	NodeCount       int     `json:"node_count"`
	Pending         bool    `json:"pending"`
	CurrentDelayMS  int64   `json:"current_delay_ms"`
}

// spfSnapshot renders run states as stable value rows.
func spfSnapshot(states []spfState) []spfSnapshotEntry {
	out := make([]spfSnapshotEntry, 0, len(states))
	for _, st := range states {
		last := ""
		if !st.LastRun.IsZero() {
			last = st.LastRun.Format(time.RFC3339Nano)
		}
		out = append(out, spfSnapshotEntry{
			Area:            st.Area.String(),
			LastRun:         last,
			DurationSeconds: st.Duration.Seconds(),
			NodeCount:       st.NodeCount,
			Pending:         st.Pending,
			CurrentDelayMS:  st.CurrentDelay.Milliseconds(),
		})
	}
	return out
}

func areaConfigMaps(configs []AreaConfig) (map[types.AreaID]types.Options, map[types.AreaID][]AreaRange, map[types.AreaID]AreaSummaryPolicy) {
	options := make(map[types.AreaID]types.Options, len(configs))
	ranges := make(map[types.AreaID][]AreaRange, len(configs))
	policies := make(map[types.AreaID]AreaSummaryPolicy, len(configs))
	for _, cfg := range configs {
		options[cfg.AreaID] = cfg.Options
		if len(cfg.Ranges) > 0 {
			ranges[cfg.AreaID] = append([]AreaRange(nil), cfg.Ranges...)
		}
		if cfg.AreaType != "" && cfg.AreaType != AreaTypeNormal {
			policies[cfg.AreaID] = AreaSummaryPolicy{Type: cfg.AreaType, NoSummary: cfg.NoSummary, DefaultCost: cfg.DefaultCost}
		}
	}
	return options, ranges, policies
}

// copyAreaPolicies returns a shallow copy of the per-area policy map (value-typed
// entries) so a Run snapshot is independent of a concurrent SetAreaConfigs.
func copyAreaPolicies(in map[types.AreaID]AreaSummaryPolicy) map[types.AreaID]AreaSummaryPolicy {
	out := make(map[types.AreaID]AreaSummaryPolicy, len(in))
	maps.Copy(out, in)
	return out
}

func isActiveResult(res *Result, root types.RouterID) bool {
	if res == nil || res.Graph == nil || root == (types.RouterID{}) {
		return false
	}
	_, ok := res.Graph.Routers[root]
	return ok
}

func copyAreaOptions(in map[types.AreaID]types.Options) map[types.AreaID]types.Options {
	if len(in) == 0 {
		return nil
	}
	out := make(map[types.AreaID]types.Options, len(in))
	maps.Copy(out, in)
	return out
}

func copyAreaRanges(in map[types.AreaID][]AreaRange) map[types.AreaID][]AreaRange {
	if len(in) == 0 {
		return nil
	}
	out := make(map[types.AreaID][]AreaRange, len(in))
	for k, v := range in {
		out[k] = append([]AreaRange(nil), v...)
	}
	return out
}
