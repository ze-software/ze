// Design: docs/architecture/plugin/rib-storage-design.md — RIB plugin
// Detail: rib_nlri.go — NLRI wire format conversion helpers
// Detail: rib_commands.go — command handling and JSON responses
// Detail: rib_attr_format.go — attribute formatting for show enrichment
// Detail: bestpath.go — best-path selection algorithm (RFC 4271 §9.1.2)
// Detail: compaction.go — pool compaction scheduler wiring
// Detail: rib_pipeline.go — iterator pipeline for show commands (scope, filters, terminals)
// Detail: rib_pipeline_best.go — best-path pipeline for rib show best commands
// Detail: rib_bestchange.go — best-path change tracking and Bus publishing
//
// Package rib implements a RIB (Routing Information Base) plugin for ze.
// It tracks routes received from peers (Adj-RIB-In) and sent to peers (Adj-RIB-Out).
//
// RFC 7911: ADD-PATH path-id is included in route keys when present.
// Multiple paths to the same prefix with different path-ids are stored separately.
package rib

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attrpool"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri/nlrisplit"
	ribevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/events"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/locrib"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

const (
	statusDone  = "done"
	statusError = "error"
)

// loggerPtr is the package-level logger, disabled by default.
// Use SetLogger() to enable logging from CLI --log-level flag.
// Stored as atomic.Pointer to avoid data races when tests start
// multiple in-process plugin instances concurrently.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// SetLogger sets the package-level logger.
// Called by cmd/ze/bgp/plugin_rib.go with slogutil.PluginLogger().
func SetLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// eventBusPtr stores the EventBus instance for best-path change emission and
// replay-request subscription. Set by ConfigureEventBus callback before
// RunEngine.
var eventBusPtr atomic.Pointer[ze.EventBus]

// SetEventBus sets the package-level EventBus instance.
// Called via ConfigureEventBus callback before RunEngine.
func SetEventBus(eb ze.EventBus) {
	if eb != nil {
		eventBusPtr.Store(&eb)
	}
}

// getEventBus returns the stored EventBus, or nil if not configured.
func getEventBus() ze.EventBus {
	p := eventBusPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

// ribMetrics holds Prometheus gauges for RIB route counts and churn.
type ribMetrics struct {
	routesIn     metrics.Gauge    // global total
	routesOut    metrics.Gauge    // global total
	routesInVec  metrics.GaugeVec // per-peer
	routesOutVec metrics.GaugeVec // per-peer

	// Route churn counters
	routeInserts     metrics.CounterVec // labels: peer, family
	routeWithdrawals metrics.CounterVec // labels: peer, family

	// Attribute pool metrics (polled from pool.AllPools())
	poolInternTotal metrics.GaugeVec // labels: pool -- monotonic, use rate()
	poolDedupHits   metrics.GaugeVec // labels: pool -- monotonic, use rate()
	poolSlotsUsed   metrics.GaugeVec // labels: pool

	// Best-path interner reverse-table occupancy per type (peers, nextHops,
	// metrics). Operators can alert on approach to the uint16 cap (65536);
	// realistic deployments sit in the tens to low hundreds per table.
	bestpathInternerSize metrics.GaugeVec // labels: table

	// Per-shard depth of the bestPrev sharded store. One series per
	// (family, shard); useful for confirming hash distribution and for
	// alerting on a hot shard that does not balance with the others.
	bestprevShardDepth metrics.GaugeVec // labels: family, shard
}

// metricsPtr stores RIB metrics gauges, set by SetMetricsRegistry.
// Atomic pointer for concurrent access from metrics loop + tests.
var metricsPtr atomic.Pointer[ribMetrics]

// SetMetricsRegistry creates RIB route gauges from the given registry.
// Called via ConfigureMetrics callback before RunEngine.
func SetMetricsRegistry(reg metrics.Registry) {
	m := &ribMetrics{
		routesIn:     reg.Gauge("ze_rib_routes_in_total", "Total Adj-RIB-In route count."),
		routesOut:    reg.Gauge("ze_rib_routes_out_total", "Total Adj-RIB-Out route count."),
		routesInVec:  reg.GaugeVec("ze_rib_routes_in", "Adj-RIB-In route count per peer.", []string{"peer"}),
		routesOutVec: reg.GaugeVec("ze_rib_routes_out", "Adj-RIB-Out route count per peer.", []string{"peer"}),

		routeInserts:     reg.CounterVec("ze_rib_route_inserts_total", "Routes inserted into Adj-RIB-In.", []string{"peer", "family"}),
		routeWithdrawals: reg.CounterVec("ze_rib_route_withdrawals_total", "Routes withdrawn from Adj-RIB-In.", []string{"peer", "family"}),

		poolInternTotal: reg.GaugeVec("ze_attr_pool_intern_total", "Total Intern() calls per pool.", []string{"pool"}),
		poolDedupHits:   reg.GaugeVec("ze_attr_pool_dedup_hits_total", "Intern() dedup hits per pool.", []string{"pool"}),
		poolSlotsUsed:   reg.GaugeVec("ze_attr_pool_slots_used", "Active slots per pool.", []string{"pool"}),

		bestpathInternerSize: reg.GaugeVec("ze_rib_bestpath_interner_size",
			"Best-path interner reverse-table entry count (cap 65536 per table).",
			[]string{"table"}),

		bestprevShardDepth: reg.GaugeVec("ze_rib_bestprev_shard_depth",
			"Number of stored bestPathRecords per (family, shard).",
			[]string{"family", "shard"}),
	}
	metricsPtr.Store(m)
}

// bestprevShardLabel formats a bestPrev shard index for the metric label.
func bestprevShardLabel(idx int) string { return strconv.Itoa(idx) }

// poolNameEntry maps a pool variable to its Prometheus label name.
type poolNameEntry struct {
	name string
	pool interface{ Metrics() attrpool.Metrics }
}

// poolNames returns name/pool pairs for metrics labeling.
func poolNames() []poolNameEntry {
	return []poolNameEntry{
		{"origin", pool.Origin},
		{"as_path", pool.ASPath},
		{"local_pref", pool.LocalPref},
		{"med", pool.MED},
		{"next_hop", pool.NextHop},
		{"communities", pool.Communities},
		{"large_communities", pool.LargeCommunities},
		{"ext_communities", pool.ExtCommunities},
		{"cluster_list", pool.ClusterList},
		{"originator_id", pool.OriginatorID},
		{"atomic_aggregate", pool.AtomicAggregate},
		{"aggregator", pool.Aggregator},
		{"other", pool.OtherAttrs},
	}
}

// metricsUpdateInterval is how often RIB metrics gauges are refreshed.
const metricsUpdateInterval = 10 * time.Second

// PeerMeta stores per-peer metadata for best-path comparison and capability lookup.
// Extracted from received UPDATE events (nested peer format) and structured events.
type PeerMeta struct {
	PeerASN   uint32           // peer's AS number
	LocalASN  uint32           // local AS number (for eBGP/iBGP detection)
	ContextID bgpctx.ContextID // encoding context from last received event (0 = unknown)
}

// peerGRState holds per-peer Graceful Restart metadata in the RIB plugin.
// Stored when "bgp rib mark-stale" is received so that "bgp rib status" can display
// absolute times (when routes went stale, when they expire).
// RFC 4724 Section 4.2: Receiving Speaker route retention.
type peerGRState struct {
	StaleAt     time.Time   // when routes were marked stale (disconnect time)
	RestartTime uint16      // peer's negotiated restart time in seconds
	ExpiresAt   time.Time   // StaleAt + RestartTime
	expiryTimer *time.Timer // safety-net timer — auto-purges stale if no command arrives
}

// RIBManager implements a BGP RIB plugin.
// It tracks routes received from and sent to peers.
type RIBManager struct {
	// plugin is the SDK plugin handle for engine RPCs (update-route, subscribe-events).
	plugin *sdk.Plugin

	// ribInPool stores routes received FROM peers (Adj-RIB-In)
	// Uses pool storage for memory efficiency (attributes deduplicated)
	ribInPool map[string]*storage.PeerRIB // peerAddr -> PeerRIB

	// ribOut stores routes sent TO peers (Adj-RIB-Out), keyed per-family.
	// Enables per-family operations (route refresh, LLGR readvertisement).
	ribOut map[string]map[string]map[string]*Route // peerAddr -> family -> prefixKey -> route

	// peerUp tracks which peers are currently up
	peerUp map[string]bool

	// peerMeta tracks per-peer metadata for best-path comparison.
	peerMeta map[string]*PeerMeta // peerAddr -> metadata

	// retainedPeers tracks peers whose Adj-RIB-In is retained during GR.
	// RFC 4724: When a GR-capable peer goes down, routes are retained until
	// the restart timer expires or the peer re-establishes and sends EOR.
	// Set by "bgp rib retain-routes <peer>" command from bgp-gr plugin.
	// Cleared by "bgp rib release-routes <peer>" or when the peer comes back up.
	retainedPeers map[string]bool

	// grState tracks per-peer Graceful Restart metadata.
	// Set by "bgp rib mark-stale" command, cleared by "bgp rib release-routes" or timer expiry.
	// RFC 4724 Section 4.2: Receiving Speaker route retention state.
	grState map[string]*peerGRState

	// bestPrev tracks the previous best-path per (family, prefix) for change
	// detection. Sharded by prefix: each family is split across N shards
	// (default GOMAXPROCS, ze.bgp.rib.bestprev.shards override), each with
	// its own bestPrevStore and write lock. checkBestPathChange takes only
	// the owning shard's lock. NOT protected by r.peerMu.
	bestPrev *bestPrevShards

	// bestPathInterner dedupes peer address, next-hop, and MED values across
	// every family's bestPrevStore into uint16 reverse-table indices that are
	// packed into the stored bestPathRecord. Shared, not per-family, because
	// realistic deployments use <10^4 unique values per attribute type and
	// sharing amortizes the dedup maps across hundreds of peers/families.
	// Owns its own per-table sync.RWMutex; safe for concurrent use without
	// any outer lock.
	bestPathInterner *bestPrevInterner

	// locRIB holds a reference to the cross-protocol unified Loc-RIB
	// (internal/core/rib/locrib). Every BGP best-path change is mirrored
	// here so sysrib / FIB / other non-BGP consumers see a consistent view
	// across BGP, static, kernel, OSPF, etc. The BGP-internal bestPrev
	// state above remains authoritative for BGP replay, show commands, and
	// BGP-only consumers.
	//
	// May be nil in tests that do not wire a Loc-RIB; callers that touch
	// this field MUST nil-check first.
	locRIB *locrib.RIB

	// unsubForwardObs releases the forward-handle observability
	// subscription registered by SetLocRIB. Nil when no locRIB is wired
	// or when a previous SetLocRIB cleared it. Called from SetLocRIB
	// before rewiring to a different locRIB.
	unsubForwardObs func()

	// maximumPaths is the configured N for multipath/ECMP selection.
	// Populated from bgp/multipath/maximum-paths in the Stage 2 configure callback.
	// Default 1 = single best-path behavior (RFC 4271 §9.1.2, no ECMP).
	// Read-only after configure; no lock needed for atomic load.
	maximumPaths atomic.Uint32

	// relaxASPath enables the "as-path-relax" multipath semantic.
	// When true, paths with the same AS-path length but different content are
	// considered equal-cost. When false (default), AS-path content must match.
	// Populated from bgp/multipath/relax-as-path in the Stage 2 configure callback.
	relaxASPath atomic.Bool

	// adminDistanceEBGP is the admin distance stamped on best-path mirrors
	// into the shared Loc-RIB for routes learned from external BGP peers.
	// Default 20 (Cisco/Juniper convention; RFC 4271 does not mandate a
	// value). Populated from bgp/admin-distance/ebgp in the Stage 2 configure
	// callback. YANG enforces the 1..255 range.
	adminDistanceEBGP atomic.Uint32

	// adminDistanceIBGP is the admin distance stamped on best-path mirrors
	// for iBGP peers. Default 200; see adminDistanceEBGP.
	adminDistanceIBGP atomic.Uint32

	// peerMu protects the peer-keyed maps ONLY: ribInPool, ribOut, peerUp,
	// peerMeta, retainedPeers, grState. bestPrev is sharded (see
	// bestprev_shard.go) and has its own per-shard locks. bestPathInterner
	// has its own per-table mutexes. Readers take peerMu.RLock for brief
	// map-level reads, then work on PeerRIB content under PeerRIB's own
	// lock. Lock order when held together: peerMu (outer) -> shard.mu
	// (inner). Nobody holds peerMu while acquiring an interner mutex.
	peerMu sync.RWMutex

	// lastMetricsInPeers / lastMetricsOutPeers track peer labels emitted in the
	// previous updateMetrics cycle. Peers that disappear from ribInPool/ribOut
	// get their GaugeVec label deleted, preventing stale Prometheus series.
	lastMetricsInPeers  map[string]bool
	lastMetricsOutPeers map[string]bool

	// lastMetricsBestprev tracks (family, shard) label pairs emitted in
	// the previous cycle so ze_rib_bestprev_shard_depth series for
	// vanished combos are actively deleted rather than left stale. Keyed
	// on a struct so family strings containing separator characters
	// cannot confuse the split on delete.
	lastMetricsBestprev map[bestprevLabelKey]bool
}

// runMetricsLoop periodically updates RIB route count gauges.
// Runs until ctx is canceled.
func (r *RIBManager) runMetricsLoop(ctx context.Context) {
	ticker := time.NewTicker(metricsUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.updateMetrics()
		}
	}
}

// updateMetrics refreshes RIB route count gauges from current state.
// Deletes Prometheus labels for peers that are no longer in the RIB,
// preventing stale series from accumulating in long-running daemons.
func (r *RIBManager) updateMetrics() {
	m := metricsPtr.Load()
	if m == nil {
		return
	}

	r.peerMu.RLock()

	currentIn := make(map[string]bool, len(r.ribInPool))
	totalIn := 0
	for peer, peerRIB := range r.ribInPool {
		count := peerRIB.Len()
		m.routesInVec.With(peer).Set(float64(count))
		currentIn[peer] = true
		totalIn += count
	}

	currentOut := make(map[string]bool, len(r.ribOut))
	totalOut := 0
	for peer, peerFamilies := range r.ribOut {
		count := 0
		for _, familyRoutes := range peerFamilies {
			count += len(familyRoutes)
		}
		m.routesOutVec.With(peer).Set(float64(count))
		currentOut[peer] = true
		totalOut += count
	}

	r.peerMu.RUnlock()

	// Best-path interner occupancy. Read outside r.peerMu via the interner's
	// own per-table locks; the snapshot is point-in-time but stable because
	// reverse tables are append-only (indices never shrink).
	var internerPeers, internerNextHops, internerMetrics int
	if r.bestPathInterner != nil {
		internerPeers, internerNextHops, internerMetrics = r.bestPathInterner.internerSize()
	}

	// Delete stale peer labels (peers removed since last cycle)
	for peer := range r.lastMetricsInPeers {
		if !currentIn[peer] {
			m.routesInVec.Delete(peer)
		}
	}
	for peer := range r.lastMetricsOutPeers {
		if !currentOut[peer] {
			m.routesOutVec.Delete(peer)
		}
	}
	r.lastMetricsInPeers = currentIn
	r.lastMetricsOutPeers = currentOut

	m.routesIn.Set(float64(totalIn))
	m.routesOut.Set(float64(totalOut))

	m.bestpathInternerSize.With("peers").Set(float64(internerPeers))
	m.bestpathInternerSize.With("nexthops").Set(float64(internerNextHops))
	m.bestpathInternerSize.With("metrics").Set(float64(internerMetrics))

	// Per-shard depth of the bestPrev sharded store. Walks every family
	// and every shard under per-shard read locks. Track emitted label
	// combos so (family, shard) series for a vanished family are deleted
	// rather than left stale at their last value. Single-goroutine
	// invariant: updateMetrics is only called from runMetricsLoop, so
	// r.lastMetricsBestprev needs no mutex.
	currentBestprev := make(map[bestprevLabelKey]bool)
	if r.bestPrev != nil {
		for _, fam := range r.bestPrev.familyList() {
			depths := r.bestPrev.shardDepth(fam)
			famStr := fam.String()
			for shardIdx, depth := range depths {
				shardStr := bestprevShardLabel(shardIdx)
				m.bestprevShardDepth.With(famStr, shardStr).Set(float64(depth))
				currentBestprev[bestprevLabelKey{family: famStr, shard: shardStr}] = true
			}
		}
	}
	for key := range r.lastMetricsBestprev {
		if !currentBestprev[key] {
			m.bestprevShardDepth.Delete(key.family, key.shard)
		}
	}
	r.lastMetricsBestprev = currentBestprev

	// Attribute pool dedup metrics (polled from atomic counters)
	for _, entry := range poolNames() {
		pm := entry.pool.Metrics()
		m.poolInternTotal.With(entry.name).Set(float64(pm.InternTotal))
		m.poolDedupHits.With(entry.name).Set(float64(pm.InternHits))
		m.poolSlotsUsed.With(entry.name).Set(float64(pm.LiveSlots))
	}
}

// bgpProtocolID is the canonical ProtocolID for BGP under the shared
// redistevents registry. Registered at package init so every RIBManager
// shares the same numeric identity when it publishes into Loc-RIB.
var bgpProtocolID = redistevents.RegisterProtocol("bgp")

// SetLocRIB wires the shared cross-protocol Loc-RIB into the RIBManager.
// Every BGP best-path change will be mirrored into loc so non-BGP
// consumers (sysrib, FIB, observability) can see one consistent view.
// Also registers the forward-handle observability subscriber so
// operators can see (at debug level) when a Change carries a non-nil
// Forward (i.e., the BGP producer attached a wire-byte handle).
// Safe to call once at plugin setup; nil disables the mirror.
func (r *RIBManager) SetLocRIB(loc *locrib.RIB) {
	r.peerMu.Lock()
	defer r.peerMu.Unlock()
	if r.locRIB == loc {
		return
	}
	if r.unsubForwardObs != nil {
		r.unsubForwardObs()
		r.unsubForwardObs = nil
	}
	r.locRIB = loc
	if loc != nil {
		r.unsubForwardObs = observeForwardHandles(loc)
	}
}

// NewRIBManager returns a fully-initialized RIBManager bound to the given SDK
// plugin handle. Every map and the shared bestPathInterner are allocated, and
// maximumPaths is pre-set to the RFC 4271 single best-path default so that
// any consumer reading it before Stage 2 configure delivery sees 1, not the
// atomic zero value (no "ECMP disabled" race at boot).
//
// Tests pass a plugin handle wired to a closed net.Pipe (see newTestRIBManager).
// This is the only constructor; bypassing it with a zero-value struct literal
// panics on the first intern call against the nil map.
func NewRIBManager(plugin *sdk.Plugin) *RIBManager {
	r := &RIBManager{
		plugin:           plugin,
		ribInPool:        make(map[string]*storage.PeerRIB),
		ribOut:           make(map[string]map[string]map[string]*Route),
		peerUp:           make(map[string]bool),
		peerMeta:         make(map[string]*PeerMeta),
		retainedPeers:    make(map[string]bool),
		grState:          make(map[string]*peerGRState),
		bestPrev:         newBestPrevShards(),
		bestPathInterner: newBestPrevInterner(),
	}
	r.maximumPaths.Store(1)
	r.adminDistanceEBGP.Store(20)
	r.adminDistanceIBGP.Store(200)
	return r
}

// RunRIBPlugin runs the RIB plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunRIBPlugin(conn net.Conn) int {
	logger().Debug("bgp rib plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-rib", conn)
	defer func() { _ = p.Close() }()

	// Populate command table before creating manager.
	// Built-in commands registered here; plugins register via RegisterRIBCommand.
	registerBuiltinCommands()

	r := NewRIBManager(p)
	// Wire the process-wide Loc-RIB so BGP best-path changes mirror into
	// the cross-protocol store. locrib.Default() returns nil in forked
	// plugin subprocesses; SetLocRIB is nil-safe (mirroring is disabled).
	r.SetLocRIB(locrib.Default())

	// Structured event handler for DirectBridge delivery.
	// Eliminates JSON round-trip: reads peer metadata from StructuredEvent fields,
	// raw wire bytes from RawMessage's AttrsWire/WireUpdate.
	p.OnStructuredEvent(func(events []any) error {
		for _, event := range events {
			se, ok := event.(*rpc.StructuredEvent)
			if !ok || se.PeerAddress == "" {
				continue
			}
			r.dispatchStructured(se)
		}
		return nil
	})

	// Fallback: JSON event handler for non-DirectBridge delivery (external plugins).
	p.OnEvent(func(jsonStr string) error {
		event, err := parseEvent([]byte(jsonStr))
		if err != nil {
			logger().Warn("parse error", "error", err, "line", jsonStr[:min(100, len(jsonStr))])
			return nil // Don't fail on parse errors
		}
		r.dispatch(event)
		return nil
	})

	// Register command handler: responds to "bgp rib adjacent ..." commands
	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) {
		return r.handleCommand(command, peer, args)
	})

	// Stage 2: Configure callback -- extract bgp/multipath from config tree.
	// RFC 4271 §9.1.2 extension: maximum-paths>1 enables ECMP/multipath best-path
	// selection with up to N equal-cost paths per prefix.
	//
	// maximumPaths is already initialized to 1 (RFC 4271 single best-path) at
	// RIBManager construction. If the config omits bgp/multipath entirely, or
	// provides an out-of-range value, the extractor returns 0 and we leave the
	// default in place.
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			maxP, relax := extractMultipathConfig(section.Data)
			if maxP > 0 {
				r.maximumPaths.Store(uint32(maxP))
			}
			r.relaxASPath.Store(relax)

			ebgpAD, ibgpAD := extractAdminDistanceConfig(section.Data)
			if ebgpAD > 0 {
				r.adminDistanceEBGP.Store(uint32(ebgpAD))
			}
			if ibgpAD > 0 {
				r.adminDistanceIBGP.Store(uint32(ibgpAD))
			}
		}
		logger().Debug("rib configured",
			"maximum-paths", r.maximumPaths.Load(),
			"relax-as-path", r.relaxASPath.Load(),
			"admin-distance-ebgp", r.adminDistanceEBGP.Load(),
			"admin-distance-ibgp", r.adminDistanceIBGP.Load(),
		)
		return nil
	})

	// Register event subscriptions atomically with startup completion.
	// Included in the "ready" RPC so the engine registers them before SignalAPIReady,
	// ensuring the rib sees every "sent" event from the very first route.
	p.SetStartupSubscriptions([]string{"update direction sent", "update direction received", "state", "refresh"}, nil, "full")

	// Start compaction scheduler after 5-stage startup completes.
	// The scheduler runs as a goroutine tied to the plugin context,
	// reclaiming dead buffer space in attribute pools under route churn.
	p.OnStarted(func(ctx context.Context) error {
		go runCompaction(ctx, pool.AllPools())
		if metricsPtr.Load() != nil {
			go r.runMetricsLoop(ctx)
		}
		// Subscribe to replay requests from downstream consumers (e.g., sysrib).
		// When a subscriber emits (rib, replay-request), replay the full
		// best-path table. The handler ignores the payload (always empty).
		if eb := getEventBus(); eb != nil {
			ribevents.ReplayRequest.Subscribe(eb, r.replayBestPaths)
		}
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		// WantsConfig: receive the bgp subtree in Stage 2 so OnConfigure can
		// read multipath config (maximum-paths, relax-as-path).
		WantsConfig: []string{"bgp"},
		Commands: []sdk.CommandDecl{
			// Unified show with pipeline (scope + filters + terminals)
			{Name: "bgp rib status"},
			{Name: "bgp rib show"},
			{Name: "bgp rib clear in"},
			{Name: "bgp rib clear out"},
			// Legacy status alias
			{Name: "bgp rib adjacent status"},
			// GR support: route retention and stale tracking (RFC 4724)
			{Name: "bgp rib retain-routes"},
			{Name: "bgp rib release-routes"},
			{Name: "bgp rib mark-stale"},
			{Name: "bgp rib purge-stale"},
			// Best-path selection (RFC 4271 §9.1.2)
			{Name: "bgp rib show best"},
			{Name: "bgp rib show best status"},
			// Route injection (manual RIB manipulation)
			{Name: "bgp rib inject"},
			{Name: "bgp rib withdraw"},
			// Meta-commands (introspection)
			{Name: "bgp rib help"},
			{Name: "bgp rib command list"},
			{Name: "bgp rib event list"},
		},
	})
	if err != nil {
		logger().Error("bgp rib plugin failed", "error", err)
		return 1
	}

	return 0
}

// updateRoute sends a route update command to matching peers via the engine.
func (r *RIBManager) updateRoute(peerSelector, command string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _, err := r.plugin.UpdateRoute(ctx, peerSelector, command)
	if err != nil {
		logger().Warn("update-route failed", "peer", peerSelector, "error", err)
	}
}

// updateRouteWithMeta sends a route update command with metadata to matching peers.
// Used by sendRoutes to carry stale level through ForwardUpdate to egress filters.
func (r *RIBManager) updateRouteWithMeta(peerSelector, command string, meta map[string]any) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _, err := r.plugin.UpdateRouteWithMeta(ctx, peerSelector, command, meta)
	if err != nil {
		logger().Warn("update-route-with-meta failed", "peer", peerSelector, "error", err)
	}
}

// dispatch routes an event to the appropriate handler.
func (r *RIBManager) dispatch(event *Event) {
	eventType := event.GetEventType()
	logger().Debug("dispatch event", "eventType", eventType, "peer", event.GetPeerAddress())

	switch eventType {
	case "sent":
		r.handleSent(event)
	case "update":
		// Received UPDATE from peer
		r.handleReceived(event)
	case "state":
		r.handleState(event)
	case "refresh":
		// RFC 7313: Normal route refresh request - resend Adj-RIB-Out with markers
		r.handleRefresh(event)
	case "borr":
		// RFC 7313: Beginning of Route Refresh from peer - log only
		logger().Debug("received BoRR marker", "peer", event.GetPeerAddress())
	case "eorr":
		// RFC 7313: End of Route Refresh from peer - log only
		logger().Debug("received EoRR marker", "peer", event.GetPeerAddress())
	}
}

// handleSent processes sent UPDATE events.
// Stores routes in ribOut for replay on reconnect.
func (r *RIBManager) handleSent(event *Event) {
	peerAddr := event.GetPeerAddress()
	msgID := event.GetMsgID()
	logger().Debug("handleSent", "peer", peerAddr, "msgID", msgID, "familyOps", len(event.FamilyOps))

	if peerAddr == "" {
		logger().Debug("handleSent: empty peer address, skipping")
		return
	}

	if len(event.FamilyOps) == 0 {
		logger().Debug("handleSent: no family ops, skipping")
		return
	}

	r.peerMu.Lock()
	defer r.peerMu.Unlock()

	// Initialize peer's ribOut if needed
	if r.ribOut[peerAddr] == nil {
		r.ribOut[peerAddr] = make(map[string]map[string]*Route)
	}

	// Process family operations
	// Format: {"ipv4/unicast": [{"next-hop": "...", "action": "add", "nlri": [...]}]}
	for fam, ops := range event.FamilyOps {
		famStr := fam.String()
		for _, op := range ops {
			switch op.Action {
			case "add":
				// Initialize family map if needed
				if r.ribOut[peerAddr][famStr] == nil {
					r.ribOut[peerAddr][famStr] = make(map[string]*Route)
				}
				// Store routes with their next-hop
				for _, nlriVal := range op.NLRIs {
					prefix, pathID := parseNLRIValue(nlriVal)
					if prefix == "" {
						logger().Warn("sent: invalid nlri value",
							"peer", peerAddr, "family", famStr, "got", fmt.Sprintf("%T", nlriVal))
						continue
					}
					key := outRouteKey(prefix, pathID)
					r.ribOut[peerAddr][famStr][key] = &Route{
						MsgID:               msgID,
						Family:              fam,
						Prefix:              prefix,
						PathID:              pathID,
						NextHop:             op.NextHop,
						Origin:              event.Origin,
						ASPath:              event.ASPath,
						MED:                 event.MED,
						LocalPreference:     event.LocalPreference,
						Communities:         event.Communities,
						LargeCommunities:    event.LargeCommunities,
						ExtendedCommunities: event.ExtendedCommunities,
						RawAttrs:            event.RawAttributes,
						Meta:                event.RouteMeta,
					}
				}
			case "del":
				// Remove routes from the family map
				familyRoutes := r.ribOut[peerAddr][famStr]
				if familyRoutes == nil {
					continue
				}
				for _, nlriVal := range op.NLRIs {
					prefix, pathID := parseNLRIValue(nlriVal)
					if prefix == "" {
						continue
					}
					key := outRouteKey(prefix, pathID)
					delete(familyRoutes, key)
				}
				// Clean up empty family map
				if len(familyRoutes) == 0 {
					delete(r.ribOut[peerAddr], famStr)
				}
				// Clean up empty peer map
				if len(r.ribOut[peerAddr]) == 0 {
					delete(r.ribOut, peerAddr)
				}
			}
		}
	}
}

// handleReceived processes received UPDATE events from peers.
// Stores routes in pool storage (Adj-RIB-In).
// Requires format=full (raw-attributes, raw-nlri fields).
func (r *RIBManager) handleReceived(event *Event) {
	peerAddr := event.GetPeerAddress()

	if peerAddr == "" {
		logger().Warn("received event: empty peer address")
		return
	}

	if len(event.FamilyOps) == 0 {
		return
	}

	// Require raw fields (format=full)
	hasRawFields := event.RawAttributes != "" || len(event.RawNLRI) > 0 || len(event.RawWithdrawn) > 0
	if !hasRawFields {
		logger().Warn("received event: missing raw fields, requires format=full", "peer", peerAddr)
		return
	}

	r.peerMu.Lock()
	defer r.peerMu.Unlock()

	// Track peer metadata for best-path comparison (eBGP/iBGP detection).
	r.updatePeerMeta(event, peerAddr)

	r.handleReceivedPool(event, peerAddr)
}

// handleReceivedPool stores routes in pool storage.
// Caller must hold write lock.
func (r *RIBManager) handleReceivedPool(event *Event, peerAddr string) {
	// Initialize PeerRIB if needed
	if r.ribInPool[peerAddr] == nil {
		r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	}
	peerRIB := r.ribInPool[peerAddr]

	// Get raw attribute bytes
	attrBytes := event.GetRawAttributesBytes()

	// Process announcements (raw-nlri)
	for fam, hexNLRI := range event.RawNLRI {
		famStr := fam.String()
		if !nlrisplit.Supported(fam) {
			logger().Debug("pool: no splitter for family", "peer", peerAddr, "family", famStr)
			continue
		}

		nlriBytes := event.GetRawNLRIBytes(fam)
		if len(nlriBytes) == 0 {
			continue
		}

		// RFC 7911: ADD-PATH per-family flag from negotiated capabilities (via format=full JSON).
		addPath := event.AddPath[fam]
		if addPath {
			peerRIB.SetAddPath(fam, true)
		}

		// Split concatenated NLRIs and insert each via the family-registered splitter.
		prefixes, err := nlrisplit.Split(fam, nlriBytes, addPath)
		if err != nil {
			logger().Warn("pool: split error, inserting parsed prefix", "peer", peerAddr, "family", famStr, "error", err, "parsed", len(prefixes))
		}
		for _, wirePrefix := range prefixes {
			peerRIB.Insert(fam, attrBytes, wirePrefix)
		}

		if m := metricsPtr.Load(); m != nil {
			m.routeInserts.With(peerAddr, famStr).Add(float64(len(prefixes)))
		}

		logger().Debug("pool: inserted routes", "peer", peerAddr, "family", famStr,
			"count", len(prefixes), "hex", hexNLRI[:min(16, len(hexNLRI))])
	}

	// Process withdrawals (raw-withdrawn)
	for fam := range event.RawWithdrawn {
		famStr := fam.String()
		if !nlrisplit.Supported(fam) {
			continue
		}

		wdBytes := event.GetRawWithdrawnBytes(fam)
		if len(wdBytes) == 0 {
			continue
		}

		// Split and remove each.
		// RFC 7911: ADD-PATH per-family flag from negotiated capabilities (via format=full JSON).
		addPath := event.AddPath[fam]
		withdrawns, err := nlrisplit.Split(fam, wdBytes, addPath)
		if err != nil {
			logger().Warn("pool: withdrawal split error", "peer", peerAddr, "family", famStr, "error", err, "parsed", len(withdrawns))
		}
		for _, wd := range withdrawns {
			peerRIB.Remove(fam, wd)
		}

		if m := metricsPtr.Load(); m != nil {
			m.routeWithdrawals.With(peerAddr, famStr).Add(float64(len(withdrawns)))
		}

		logger().Debug("pool: withdrew routes", "peer", peerAddr, "family", famStr, "count", len(withdrawns))
	}
}

// handleRefresh processes a normal route refresh request from a peer.
// RFC 7313 Section 3: When receiving a route refresh request, the speaker
// SHOULD send BoRR, re-advertise Adj-RIB-Out, then send EoRR.
func (r *RIBManager) handleRefresh(event *Event) {
	peerAddr := event.GetPeerAddress()
	fam := family.Family{AFI: event.AFI, SAFI: event.SAFI}.String()

	if peerAddr == "" {
		logger().Warn("refresh event: empty peer address")
		return
	}

	r.peerMu.RLock()
	if !r.peerUp[peerAddr] {
		r.peerMu.RUnlock()
		logger().Debug("refresh request for down peer", "peer", peerAddr)
		return
	}

	// Direct fam lookup -- no linear scan of all routes
	var routesToSend []*Route
	if familyRoutes := r.ribOut[peerAddr][fam]; familyRoutes != nil {
		routesToSend = make([]*Route, 0, len(familyRoutes))
		for _, rt := range familyRoutes {
			routesToSend = append(routesToSend, rt)
		}
	}
	r.peerMu.RUnlock()

	// RFC 7313 Section 4: Send BoRR, routes, EoRR sequence
	r.updateRoute(peerAddr, "borr "+fam)
	r.sendRoutes(peerAddr, routesToSend)
	r.updateRoute(peerAddr, "eorr "+fam)

	logger().Debug("completed route refresh", "peer", peerAddr, "family", fam, "routes", len(routesToSend))
}

// handleStructuredState processes a structured state event from DirectBridge.
// Eliminates JSON parsing for state events (no ParseEvent/GetPeerAddress needed).
func (r *RIBManager) handleStructuredState(se *rpc.StructuredEvent) {
	peerAddr := se.PeerAddress
	state := se.State

	r.peerMu.Lock()
	wasUp := r.peerUp[peerAddr]
	isUp := state == rpc.SessionStateUp
	r.peerUp[peerAddr] = isUp

	var routesToReplay []*Route
	var pendingPurgeEmits map[family.Family][]bestChangeEntry

	if isUp && !wasUp {
		delete(r.retainedPeers, peerAddr)
		peerFamilies := r.ribOut[peerAddr]
		for _, familyRoutes := range peerFamilies {
			for _, rt := range familyRoutes {
				routesToReplay = append(routesToReplay, rt)
			}
		}
	} else if !isUp && wasUp {
		if r.retainedPeers[peerAddr] {
			logger().Debug("retaining Adj-RIB-In for GR", "peer", peerAddr)
		} else {
			if peerRIB := r.ribInPool[peerAddr]; peerRIB != nil {
				peerRIB.Release()
				delete(r.ribInPool, peerAddr)
			}
			delete(r.peerMeta, peerAddr)
			// Purge bestPrev records belonging to the departing peer so
			// cross-protocol consumers see the withdrawal immediately
			// (instead of waiting for the next UPDATE per prefix to
			// trigger the natural "newBest == nil && havePrev" path).
			// Called under peerMu.Lock so concurrent UPDATE Phase 1 for
			// this peer cannot re-insert records mid-purge. The purge
			// itself DOES NOT emit on the EventBus; it returns per-family
			// batches we dispatch via emitPurgedWithdraws AFTER peerMu
			// is released (emitting under the write lock could deadlock
			// any in-process subscriber that re-enters RIBManager).
			pendingPurgeEmits = r.purgeBestPrevForPeer(peerAddr)
		}
	}
	r.peerMu.Unlock()

	r.emitPurgedWithdraws(pendingPurgeEmits)

	if routesToReplay != nil {
		r.replayRoutes(peerAddr, routesToReplay)
	}
}

// handleState processes peer state changes.
// Handles state transitions atomically to avoid races between up/down events.
// RFC 4724: When retainedPeers[peer] is set (by bgp-gr via "bgp rib retain-routes"),
// Adj-RIB-In is preserved on peer-down instead of being deleted.
func (r *RIBManager) handleState(event *Event) {
	peerAddr := event.GetPeerAddress()
	state := event.GetPeerState()

	r.peerMu.Lock()
	wasUp := r.peerUp[peerAddr]
	isUp := state == "up"
	r.peerUp[peerAddr] = isUp

	var routesToReplay []*Route
	var pendingPurgeEmits map[family.Family][]bestChangeEntry

	if isUp && !wasUp {
		// Peer came up - clear retain flag (fresh session replaces stale state).
		delete(r.retainedPeers, peerAddr)

		// Copy routes for replay while holding lock — flatten all families
		peerFamilies := r.ribOut[peerAddr]
		for _, familyRoutes := range peerFamilies {
			for _, rt := range familyRoutes {
				routesToReplay = append(routesToReplay, rt)
			}
		}
	} else if !isUp && wasUp {
		// Peer went down - clear Adj-RIB-In unless retained for GR.
		if r.retainedPeers[peerAddr] {
			logger().Debug("retaining Adj-RIB-In for GR", "peer", peerAddr)
		} else {
			if peerRIB := r.ribInPool[peerAddr]; peerRIB != nil {
				peerRIB.Release()
				delete(r.ribInPool, peerAddr)
			}
			delete(r.peerMeta, peerAddr)
			// See handleStructuredState for the emit-after-unlock contract.
			pendingPurgeEmits = r.purgeBestPrevForPeer(peerAddr)
		}
	}
	r.peerMu.Unlock()

	r.emitPurgedWithdraws(pendingPurgeEmits)

	// I/O operations after releasing lock
	if routesToReplay != nil {
		r.replayRoutes(peerAddr, routesToReplay)
	}
}

// replayRoutes sends stored routes to a peer that just came up.
// Called without lock held - safe for I/O.
func (r *RIBManager) replayRoutes(peerAddr string, routes []*Route) {
	// Sort by MsgID to replay in original announcement order
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].MsgID < routes[j].MsgID
	})

	// Replay all stored routes using update text syntax
	// RFC 7911: Include path-information when present
	for _, route := range routes {
		cmd := formatRouteCommand(route)
		r.updateRoute(peerAddr, cmd)
	}

	// Signal done with peer-specific ready - ze can now send EOR for this peer
	r.updateRoute(peerAddr, "plugin session ready")
}

// updatePeerMeta extracts and stores peer metadata from received events.
// Uses the nested peer format which includes both local and peer ASN.
// Caller must hold write lock.
func (r *RIBManager) updatePeerMeta(event *Event, peerAddr string) {
	peerASN := event.GetPeerASN()
	localASN := getLocalASN(event)
	if peerASN == 0 && localASN == 0 {
		return
	}
	r.peerMeta[peerAddr] = &PeerMeta{
		PeerASN:  peerASN,
		LocalASN: localASN,
	}
}

// getLocalASN extracts the local ASN from an event's peer format.
// Events with local info use PeerInfoJSON with Local.AS (YANG: local.as).
func getLocalASN(event *Event) uint32 {
	if len(event.Peer) == 0 {
		return 0
	}
	var info PeerInfoJSON
	if err := json.Unmarshal(event.Peer, &info); err == nil && info.Local != nil && info.Local.AS > 0 {
		return info.Local.AS
	}
	return 0
}

// GetYANG returns the embedded YANG schema for the RIB plugin.
func GetYANG() string {
	return schema.ZeRibYANG
}
