// Design: plan/spec-ospf-8-spf-rib.md -- SPF trigger, throttle, run state, metrics.
// The Computer ties LSDB changes to graph build, Dijkstra, route selection, and
// Loc-RIB installation. It owns the SPF metrics surfaced by later CLI work.

package spf

import (
	"maps"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
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

	last         []RouteEntry
	lastBorder   []BorderRouterEntry
	onChange     func(RouteDelta)
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

	mRuns     metrics.CounterVec
	mDuration metrics.HistogramVec
	mABR      metrics.Gauge
	mSummary  metrics.GaugeVec
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

// spfState is one area's most recent run state for `show ip ospf spf`.
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
		src:          cfg.Source,
		resolver:     cfg.Resolver,
		root:         cfg.Root,
		areas:        areas,
		maxPaths:     maxPaths,
		areaOptions:  areaOptions,
		areaRanges:   areaRanges,
		areaPolicies: areaPolicies,
		summarySink:  cfg.SummarySink,
		strategy:     strategy,
		installer:    inst,
		delay:        delay,
		hold:         hold,
		maxHold:      maxHold,
		currentDelay: delay,
		dirty:        make(map[types.AreaID]struct{}),
		afterFunc:    func(d time.Duration, f func()) timerHandle { return time.AfterFunc(d, f) },
		now:          time.Now,
		state:        make(map[types.AreaID]spfState),
		mRuns:        nop.CounterVec("", "", nil),
		mDuration:    nop.HistogramVec("", "", nil, nil),
		mABR:         nop.Gauge("", ""),
		mSummary:     nop.GaugeVec("", "", nil),
	}
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
	c.mu.Unlock()
	c.installer.SetMetrics(reg)
}

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
	maxPaths := c.maxPaths
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
	for _, area := range areas {
		start := time.Now()
		g := c.strategy.BuildGraph(c.src, area)
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
	inter, border := c.strategy.ComputeInterArea(InterAreaInput{Source: c.src, Root: root, Areas: activeAreas, Results: results, Ranges: areaRanges, Resolver: c.resolver, MaxPaths: maxPaths})
	candidates = append(candidates, inter...)
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
	external := c.strategy.ComputeExternal(ExternalInput{Source: c.src, Root: root, BorderRouters: border, Routes: internal, Resolver: c.resolver, MaxPaths: maxPaths, NSSAAreas: nssaAreas})
	selected := selectBestRoutes(append(internal, external...), maxPaths)

	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return RouteDelta{}
	}
	delta := c.installer.Apply(selected)
	c.last = selected
	c.lastBorder = border
	c.summaryAreas = activeAreas
	for area, st := range states {
		st.Pending = c.pending
		st.CurrentDelay = c.currentDelay
		st.ConsecutiveHold = c.currentDelay
		c.state[area] = st
	}
	onChange := c.onChange
	c.mu.Unlock()

	// Redistribution producer trigger (OSPF -> BGP), outside the lock so the
	// emit cannot deadlock against an SPF re-entry. A separate path from FIB install.
	if onChange != nil && !delta.Empty() {
		onChange(delta)
	}
	return delta
}

// Routes returns a copy of the installed route set.
func (c *Computer) Routes() []RouteEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]RouteEntry(nil), c.last...)
}

// Snapshot returns the `show ip ospf route` snapshot.
func (c *Computer) Snapshot() []RouteSnapshotEntry {
	c.mu.Lock()
	routes := append([]RouteEntry(nil), c.last...)
	c.mu.Unlock()
	return Snapshot(routes)
}

// BorderRouterSnapshot returns the `show ip ospf border-routers` snapshot.
func (c *Computer) BorderRouterSnapshot() []BorderRouterSnapshotEntry {
	c.mu.Lock()
	rows := append([]BorderRouterEntry(nil), c.lastBorder...)
	c.mu.Unlock()
	return BorderRouterSnapshot(rows)
}

// ClearSPFLog resets the per-area SPF run history shown by `show ip ospf spf` (clear ip
// ospf counters). The monotonic Prometheus run counter (mRuns) is deliberately NOT reset;
// only the displayed last-run timestamp/duration/node-count history is cleared.
func (c *Computer) ClearSPFLog() {
	c.mu.Lock()
	c.state = make(map[types.AreaID]spfState)
	c.mu.Unlock()
}

// SPFSnapshot returns the `show ip ospf spf` snapshot.
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

// spfSnapshotEntry is one `show ip ospf spf` row.
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
