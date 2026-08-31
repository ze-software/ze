// Design: docs/architecture/plugin/rib-storage-design.md — RIB plugin
// RFC: rfc/short/rfc4271.md — BGP-4 (Adj-RIB-In / Adj-RIB-Out)
// RFC: rfc/short/rfc7911.md — ADD-PATH (path-id in route keys)
// RFC: rfc/short/rfc4724.md — Graceful Restart (route retention)
// RFC: rfc/short/rfc7313.md — Enhanced Route Refresh (BoRR/EoRR)
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
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/yang"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

const (
	statusDone  = "done"
	statusError = "error"
)

// Prometheus label NAMES for the RIB metrics. A label name is part of a
// metric's identity, so a misspelling creates a second time series instead of
// an error. They are not the JSON field names of the same concepts
// (rowKeyPeer, rowKeyFamily in rib_pipeline.go): one is read by PromQL, the
// other by a consumer of a command's output.
const (
	metricLabelPeer   = "peer"
	metricLabelFamily = "family"
	metricLabelPool   = "pool"
)

// Envelope fields of the JSON payloads the rib commands return. A route
// property is a row key instead (rib_pipeline.go).
const (
	// jsonKeyError is the field a failed command reports its message in. It is
	// a KEY, where statusError above is a status VALUE.
	jsonKeyError = "error"

	jsonKeyCount     = "count"
	jsonKeyInjected  = "injected"
	jsonKeyWithdrawn = "withdrawn"
	jsonKeyExisted   = "existed"
)

const (
	// configRootBGP is the config subtree this plugin is delivered, and the
	// dependency it declares. A misspelling leaves the plugin configured with
	// nothing rather than refused.
	configRootBGP = "bgp"
	// protocolNameBGP is the protocol a redistribute route carries. It is the
	// name an operator writes, not the config root, even where they agree.
	protocolNameBGP = "bgp"
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
		routesInVec:  reg.GaugeVec("ze_rib_routes_in", "Adj-RIB-In route count per peer.", []string{metricLabelPeer}),
		routesOutVec: reg.GaugeVec("ze_rib_routes_out", "Adj-RIB-Out route count per peer.", []string{metricLabelPeer}),

		routeInserts:     reg.CounterVec("ze_rib_route_inserts_total", "Routes inserted into Adj-RIB-In.", []string{metricLabelPeer, metricLabelFamily}),
		routeWithdrawals: reg.CounterVec("ze_rib_route_withdrawals_total", "Routes withdrawn from Adj-RIB-In.", []string{metricLabelPeer, metricLabelFamily}),

		poolInternTotal: reg.GaugeVec("ze_attr_pool_intern_total", "Total Intern() calls per pool.", []string{metricLabelPool}),
		poolDedupHits:   reg.GaugeVec("ze_attr_pool_dedup_hits_total", "Intern() dedup hits per pool.", []string{metricLabelPool}),
		poolSlotsUsed:   reg.GaugeVec("ze_attr_pool_slots_used", "Active slots per pool.", []string{metricLabelPool}),

		bestpathInternerSize: reg.GaugeVec("ze_rib_bestpath_interner_size",
			"Best-path interner reverse-table entry count (cap 65536 per table).",
			[]string{"table"}),

		bestprevShardDepth: reg.GaugeVec("ze_rib_bestprev_shard_depth",
			"Number of stored bestPathRecords per (family, shard).",
			[]string{metricLabelFamily, "shard"}),
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

// peerMetadata stores per-peer metadata for best-path comparison and capability lookup.
// Extracted from received UPDATE events (nested peer format) and structured events.
type peerMetadata struct {
	PeerASN  uint32 // peer's AS number
	LocalASN uint32 // local AS number (for eBGP/iBGP detection)
	// RemoteRouterID is the peer's BGP Identifier from its OPEN. It is the
	// ORIGINATOR_ID fallback of RFC 4271 Section 9.1.2.2 step f)
	// (extractCandidate) and the per-peer BGP ID of a TABLE_DUMP_V2 dump
	// (rib_mrt.go). Zero when the event states none.
	RemoteRouterID uint32
	ContextID      bgpctx.ContextID // encoding context from last received event (0 = unknown)
	// GroupName is the peer-group this session belongs to, empty for a
	// standalone peer. It is the only identity a session created from a dynamic
	// group shares with the operator's config document: such a session's address
	// is written nowhere, so a per-peer rule stated on the group is keyed by the
	// group (configjson.GroupKey) and reached from here.
	// Read by blackholeRouteTypeForBest.
	GroupName string
	// LocalAddress is THIS speaker's end of the session: the address the peer
	// dialed or was dialed from. It arrives on the same peer event the fields
	// above do (PeerInfoJSON.Local.Address, bgp/event.go; StructuredEvent.
	// LocalAddress), and it is the address ze itself advertises as NEXT_HOP
	// under `next-hop self` (precomputeNextHop, reactor/peer_forward_facts.go).
	// Read by selfNextHops, which is how the RIB knows "itself".
	LocalAddress netip.Addr
}

// peerGRState holds per-peer Graceful Restart metadata in the RIB plugin.
// Stored when "request bgp rib mark-stale" is received so that "show bgp rib status"
// can display absolute times (when routes went stale, when they expire).
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

	// dispatchHook, when non-nil, intercepts dispatchPeerAction for test
	// inspection instead of issuing the dispatch-command RPC. Production leaves
	// it nil.
	dispatchHook func(command string)

	// ribInPool stores routes received FROM non-BGP protocols (e.g. BMP),
	// keyed by source protocol then protocol-defined peer key. BMP keys are
	// composite "router:peerIP" strings (see bmpCompositeKey and
	// withdrawAllForRouter's prefix match), so this inner map stays
	// string-keyed by design. BGP peers live in bgpPeers, keyed by
	// netip.Addr.
	ribInPool map[redistevents.ProtocolID]map[string]*storage.PeerRIB

	// bgpPeers holds BGP Adj-RIB-In state, keyed by parsed peer address.
	// Peer strings are parsed once at the event / command boundary; every
	// internal lookup uses the netip.Addr key (ai/rules/go-standards.md).
	bgpPeers map[netip.Addr]*storage.PeerRIB

	// ribOut stores routes sent TO peers (Adj-RIB-Out), keyed per-family.
	// Enables per-family operations (route refresh, LLGR readvertisement).
	// Wire attribute bytes are deduplicated in pool.RibOut (idx 16).
	ribOut map[netip.Addr]map[family.Family]map[ribOutKey]ribOutEntry // peerAddr -> family -> ribOutKey -> entry

	// ribOutSource tracks the originating peer per (family, routeKey).
	// One entry per unique route (not per destination peer), used for
	// GR stale propagation (RFC 9494). Refcounted: cleaned up when the
	// last destination peer withdraws the route.
	ribOutSource map[family.Family]map[ribOutKey]ribOutSourceRef

	// peerUp tracks which peers are currently up
	peerUp map[netip.Addr]bool

	// peerMeta tracks per-peer metadata for best-path comparison.
	peerMeta map[netip.Addr]*peerMetadata // peer address -> metadata

	// selfNextHops names every address this speaker answers to as a next hop:
	// the local end of each session in peerMeta, deduplicated. RFC 4271
	// Section 6.3 calls it "the IP address of the receiving speaker", and
	// Section 5.1.3 forbids installing a route that names one.
	//
	// DERIVED, never authored. peerMeta is where the local address arrives, so
	// rebuilding this from it keeps one source of truth: a second list would go
	// stale exactly when a session moves, which is when the answer changes.
	// Rebuilt under peerMu by refreshSelfNextHopsLocked and published atomically,
	// so gatherCandidatesLocked reads it once per call with no lock of its own.
	// Typically one or two entries; a linear scan beats a map for that size and
	// allocates nothing to search.
	selfNextHops atomic.Pointer[[]netip.Addr]

	// retainedPeers tracks peers whose Adj-RIB-In is retained during GR.
	// RFC 4724: When a GR-capable peer goes down, routes are retained until
	// the restart timer expires or the peer re-establishes and sends EOR.
	// Set by "request bgp rib retain-routes <peer>" command from bgp-gr plugin.
	// Cleared by "request bgp rib release-routes <peer>" or when the peer comes back up.
	retainedPeers map[netip.Addr]bool

	// grState tracks per-peer Graceful Restart metadata.
	// Set by "request bgp rib mark-stale" command, cleared by "request bgp rib release-routes" or timer expiry.
	// RFC 4724 Section 4.2: Receiving Speaker route retention state.
	grState map[netip.Addr]*peerGRState

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

	// forwardTracker is the production Change.Forward consumer (rib-arch-6):
	// it AddRefs and reads the zero-copy UPDATE bytes to maintain fast-path
	// forwarding state. Created by SetLocRIB alongside the debug observer;
	// inert until `request bgp rib fastpath enable`. Stopped before rewiring.
	forwardTracker *forwardStateTracker

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

	// blackholeCfg holds the RFC 7999 honoring configuration, keyed by peer
	// remote IP and resolved across the bgp, group and peer levels in the Stage
	// 2 configure callback. A nil pointer or a missing key means the peer never
	// agreed to honor BLACKHOLE, which is the RFC 7999 Section 4 default, so an
	// unconfigured deployment pays one empty-map check per best-path change and
	// no wire scan. Stored whole so the read side needs no lock.
	//
	// The key is configjson.PeerConfigKey because a dynamic group's template is
	// keyed by the GROUP: its members have no address in the config document.
	// blackholeRouteTypeForBest resolves a member through peerMetadata.GroupName,
	// after the winner's own address misses.
	blackholeCfg atomic.Pointer[map[configjson.PeerConfigKey]blackholeConfig]

	// peerMu protects the peer-keyed maps ONLY: ribInPool (and bgpPeers), ribOut, peerUp,
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

	// Prometheus labels are strings (external boundary), so the tracking
	// maps stay string-keyed; netip.Addr keys convert once per cycle here.
	currentIn := make(map[string]bool)
	totalIn := 0
	for _, peerRIB := range r.bgpPeers {
		count := peerRIB.Len()
		label := peerRIB.PeerAddr()
		m.routesInVec.With(label).Set(float64(count))
		currentIn[label] = true
		totalIn += count
	}
	for _, protoPeers := range r.ribInPool {
		for peer, peerRIB := range protoPeers {
			count := peerRIB.Len()
			m.routesInVec.With(peer).Set(float64(count))
			currentIn[peer] = true
			totalIn += count
		}
	}

	currentOut := make(map[string]bool, len(r.ribOut))
	totalOut := 0
	for peer, peerFamilies := range r.ribOut {
		count := 0
		for _, familyRoutes := range peerFamilies {
			count += len(familyRoutes)
		}
		label := peer.String()
		m.routesOutVec.With(label).Set(float64(count))
		currentOut[label] = true
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
var bgpProtocolID = redistevents.RegisterProtocol(protocolNameBGP)

var bmpProtocolID = redistevents.RegisterProtocol("bmp")

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
	if r.forwardTracker != nil {
		r.forwardTracker.Stop()
		r.forwardTracker = nil
	}
	r.locRIB = loc
	if loc != nil {
		r.unsubForwardObs = observeForwardHandles(loc)
		r.forwardTracker = newForwardStateTracker(loc)
	}
}

// newRIBManager returns a fully-initialized RIBManager bound to the given SDK
// plugin handle. Every map and the shared bestPathInterner are allocated, and
// maximumPaths is pre-set to the RFC 4271 single best-path default so that
// any consumer reading it before Stage 2 configure delivery sees 1, not the
// atomic zero value (no "ECMP disabled" race at boot).
//
// Tests pass a plugin handle wired to a closed net.Pipe (see newTestRIBManager).
// This is the only constructor; bypassing it with a zero-value struct literal
// panics on the first intern call against the nil map.
func newRIBManager(plugin *sdk.Plugin) *RIBManager {
	r := &RIBManager{
		plugin: plugin,
		ribInPool: map[redistevents.ProtocolID]map[string]*storage.PeerRIB{
			bmpProtocolID: make(map[string]*storage.PeerRIB),
		},
		bgpPeers:         make(map[netip.Addr]*storage.PeerRIB),
		ribOut:           make(map[netip.Addr]map[family.Family]map[ribOutKey]ribOutEntry),
		ribOutSource:     make(map[family.Family]map[ribOutKey]ribOutSourceRef),
		peerUp:           make(map[netip.Addr]bool),
		peerMeta:         make(map[netip.Addr]*peerMetadata),
		retainedPeers:    make(map[netip.Addr]bool),
		grState:          make(map[netip.Addr]*peerGRState),
		bestPrev:         newBestPrevShards(),
		bestPathInterner: newBestPrevInterner(),
	}
	r.maximumPaths.Store(1)
	r.adminDistanceEBGP.Store(20)
	r.adminDistanceIBGP.Store(200)
	return r
}

// runRIBPlugin runs the RIB plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func runRIBPlugin(conn net.Conn) int {
	logger().Debug("bgp rib plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-rib", conn)
	defer func() { _ = p.Close() }()

	// Populate command table before creating manager.
	// Built-in commands registered here; plugins register via RegisterRIBCommand.
	registerBuiltinCommands()

	r := newRIBManager(p)
	activeManager.Store(r)
	defer activeManager.Store(nil)

	// Wire the process-wide Loc-RIB so BGP best-path changes mirror into
	// the cross-protocol store. locrib.Default() returns nil in forked
	// plugin subprocesses; SetLocRIB is nil-safe (mirroring is disabled).
	r.SetLocRIB(locrib.Default())

	rpc.RegisterRouteInjector(r.handleInjectWireRoute)

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

	// Register command handler for bgp-rib plugin commands.
	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
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
			if section.Root != configRootBGP {
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

			// RFC 7999 honoring. A parse failure REFUSES the configuration
			// rather than leaving the previous map in place: this decides
			// whether a peer can make Ze discard traffic, and a rejected edit
			// that silently keeps running with the old answer is the worst of
			// the three outcomes.
			bgpCfg, ok := configjson.ParseBGPSubtree(section.Data)
			if !ok {
				return errRibInvalidBgpConfigJson
			}
			blackholeCfg, err := parseBlackholeConfig(bgpCfg)
			if err != nil {
				return err
			}
			r.blackholeCfg.Store(&blackholeCfg)
		}
		logger().Debug("rib configured",
			"maximum-paths", r.maximumPaths.Load(),
			"relax-as-path", r.relaxASPath.Load(),
			"admin-distance-ebgp", r.adminDistanceEBGP.Load(),
			"admin-distance-ibgp", r.adminDistanceIBGP.Load(),
			"blackhole-honor-rules", r.blackholeHonorRuleCount(),
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
		// When a subscriber emits (bgp-rib, replay-request), replay the full
		// best-path table. This hop is broadcast, so the handler ignores the
		// request token except to stamp it onto the emitted batches.
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
		WantsConfig: []string{configRootBGP},
		Commands: []sdk.CommandDecl{
			// Unified show with pipeline (scope + filters + terminals)
			{Name: "show bgp rib status"},
			{Name: "show bgp rib"},
			{Name: "clear bgp rib in"},
			{Name: "clear bgp rib out"},
			// GR support: route retention and stale tracking (RFC 4724)
			{Name: "request bgp rib retain-routes"},
			{Name: "request bgp rib release-routes"},
			{Name: "request bgp rib mark-stale"},
			{Name: "request bgp rib purge-stale"},
			// Best-path selection (RFC 4271 §9.1.2)
			{Name: "show bgp rib best"},
			{Name: "show bgp rib best status"},
			// Reverse Path Forwarding query: longest-prefix-match in Loc-RIB
			{Name: "show bgp rib rpf"},
			// Route injection (manual RIB manipulation)
			{Name: "request bgp rib inject"},
			{Name: "request bgp rib withdraw"},
			// Protocol-scoped route management (BMP integration)
			{Name: "show bgp rib protocol"},
			{Name: "request bgp rib withdraw-protocol"},
			{Name: "request bgp rib withdraw-router"},
			// Meta-commands (introspection)
			{Name: "show bgp rib help"},
			{Name: "show bgp rib commands"},
			{Name: "show bgp rib events"},
			// Zero-copy forward-handle fast path (rib-arch-6)
			{Name: "request bgp rib fastpath"},
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
// Used by sendRoutes and resendRoutesWithCursor to carry stale level through to egress filters.
func (r *RIBManager) updateRouteWithMeta(peerSelector, command string, meta map[string]any) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _, err := r.plugin.UpdateRouteWithMeta(ctx, peerSelector, command, meta)
	if err != nil {
		logger().Warn("update-route-with-meta failed", "peer", peerSelector, "error", err)
	}
}

// dispatchPeerAction sends a peer lifecycle or route-refresh command through the
// engine command dispatcher. These commands (plugin session ready, borr, eorr)
// live under the "request peer <sel>" YANG path (ze-peer-cmd, ze-refresh-cmd),
// NOT the route-injection "peer <sel>" path that updateRoute targets, so they
// must go through dispatch-command rather than the update-route RPC.
func (r *RIBManager) dispatchPeerAction(peerSelector, action string) {
	command := "request peer " + peerSelector + " " + action
	if r.dispatchHook != nil {
		r.dispatchHook(command)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, _, err := r.plugin.DispatchCommand(ctx, command); err != nil {
		logger().Warn("dispatch peer-action failed", "peer", peerSelector, "action", action, "error", err)
	}
}

// dispatch routes an event to the appropriate handler.
func (r *RIBManager) dispatch(event *Event) {
	eventType := event.GetEventType()
	logger().Debug("dispatch event", "eventType", eventType, "peer", event.GetPeerAddress())

	switch eventType { //nolint:exhaustive // RIB only handles event types with route data
	case rpc.EventKindSent:
		r.handleSent(event)
	case rpc.EventKindUpdate:
		// Received UPDATE from peer
		r.handleReceived(event)
	case rpc.EventKindState:
		r.handleState(event)
	case rpc.EventKindRefresh:
		// RFC 7313: Normal route refresh request - resend Adj-RIB-Out with markers
		r.handleRefresh(event)
	case rpc.EventKindBoRR:
		// RFC 7313: Beginning of Route Refresh from peer - log only
		logger().Debug("received BoRR marker", "peer", event.GetPeerAddress())
	case rpc.EventKindEoRR:
		// RFC 7313: End of Route Refresh from peer - log only
		logger().Debug("received EoRR marker", "peer", event.GetPeerAddress())
	}
}

// handleSent processes sent UPDATE events.
// Stores routes in ribOut for replay on reconnect.
func (r *RIBManager) handleSent(event *Event) {
	msgID := event.GetMsgID()
	logger().Debug("handleSent", "peer", event.GetPeerAddress(), "msgID", msgID, "familyOps", len(event.FamilyOps))

	if event.GetPeerAddress() == "" {
		logger().Debug("handleSent: empty peer address, skipping")
		return
	}
	peerAddr, err := parsePeerAddr(event.GetPeerAddress())
	if err != nil {
		logger().Warn("sent event dropped", "error", err)
		return
	}

	if len(event.FamilyOps) == 0 {
		logger().Debug("handleSent: no family ops, skipping")
		return
	}

	// Intern wire bytes BEFORE acquiring peerMu to maintain lock ordering
	// (pool.RibOut.mu must not be acquired under peerMu).
	rawHex := event.GetRawAttributesHex()
	var attrHandle attrpool.Handle
	if rawHex != "" {
		if rawBytes, err := hex.DecodeString(rawHex); err == nil {
			attrHandle, _ = pool.RibOut.Intern(rawBytes)
		}
	}
	if !attrHandle.IsValid() {
		nextHop := firstAddNextHop(event)
		if packed := packEventAttrs(event, nextHop); len(packed) > 0 {
			attrHandle, _ = pool.RibOut.Intern(packed)
		}
	}

	var sourcePeer string
	if event.RouteMeta != nil {
		sourcePeer, _ = event.RouteMeta["source-peer"].(string)
	}

	r.peerMu.Lock()
	defer r.peerMu.Unlock()

	if r.ribOut[peerAddr] == nil {
		r.ribOut[peerAddr] = make(map[family.Family]map[ribOutKey]ribOutEntry)
	}

	for fam, ops := range event.FamilyOps {
		for _, op := range ops {
			switch op.Action { //nolint:exhaustive // only Add/Del relevant for rib-out
			case routeaction.Add:
				if r.ribOut[peerAddr][fam] == nil {
					r.ribOut[peerAddr][fam] = make(map[ribOutKey]ribOutEntry)
				}
				for _, nlriVal := range op.NLRIs {
					prefix, pathID := parseNLRIValue(nlriVal)
					if prefix == "" {
						logger().Warn("sent: invalid nlri value", "peer", peerAddr, "family", fam)
						continue
					}
					pfx, err := netip.ParsePrefix(prefix)
					if err != nil {
						logger().Warn("sent: invalid prefix", "peer", peerAddr, "prefix", prefix)
						continue
					}
					key := ribOutKey{Prefix: pfx, PathID: pathID}
					_, existed := r.ribOut[peerAddr][fam][key]
					if existed {
						r.ribOut[peerAddr][fam][key].release()
					}
					if attrHandle.IsValid() {
						_ = pool.RibOut.AddRef(attrHandle)
					}
					r.ribOut[peerAddr][fam][key] = ribOutEntry{
						MsgID:      msgID,
						AttrHandle: attrHandle,
					}
					r.setRibOutSource(fam, key, sourcePeer, !existed)
				}
			case routeaction.Del:
				familyRoutes := r.ribOut[peerAddr][fam]
				if familyRoutes == nil {
					continue
				}
				for _, nlriVal := range op.NLRIs {
					prefix, pathID := parseNLRIValue(nlriVal)
					if prefix == "" {
						continue
					}
					pfx, err := netip.ParsePrefix(prefix)
					if err != nil {
						continue
					}
					key := ribOutKey{Prefix: pfx, PathID: pathID}
					if old, exists := familyRoutes[key]; exists {
						old.release()
						delete(familyRoutes, key)
					}
					r.releaseRibOutSource(fam, key)
				}
				if len(familyRoutes) == 0 {
					delete(r.ribOut[peerAddr], fam)
				}
				if len(r.ribOut[peerAddr]) == 0 {
					delete(r.ribOut, peerAddr)
				}
			}
		}
	}
	if attrHandle.IsValid() {
		_ = pool.RibOut.Release(attrHandle)
	}
}

// handleReceived processes received UPDATE events from peers.
// Stores routes in pool storage (Adj-RIB-In).
// Requires format=full (raw-attributes, raw-nlri fields).
func (r *RIBManager) handleReceived(event *Event) {
	if event.GetPeerAddress() == "" {
		logger().Warn("received event: empty peer address")
		return
	}
	peerAddr, err := parsePeerAddr(event.GetPeerAddress())
	if err != nil {
		logger().Warn("received event dropped", "error", err)
		return
	}

	if len(event.FamilyOps) == 0 {
		return
	}

	// Require raw fields (format=full or structured byte path)
	hasRawFields := event.RawAttributes != "" || len(event.RawNLRI) > 0 || len(event.RawWithdrawn) > 0 ||
		len(event.RawAttributeBytes) > 0 || len(event.RawNLRIBytes) > 0 || len(event.RawWithdrawnBytes) > 0
	if !hasRawFields {
		logger().Warn("received event: missing raw fields, requires format=full", "peer", event.GetPeerAddress())
		return
	}

	r.peerMu.Lock()
	defer r.peerMu.Unlock()

	// Track peer metadata for best-path comparison (eBGP/iBGP detection).
	r.updatePeerMetadata(event, peerAddr)

	r.handleReceivedPool(event, peerAddr)
}

// handleReceivedPool stores routes in pool storage.
// Caller must hold write lock. PeerRIB caches the canonical address string
// for metric labels and log lines (PeerRIB.PeerAddr).
func (r *RIBManager) handleReceivedPool(event *Event, peerAddr netip.Addr) {
	if r.bgpPeers[peerAddr] == nil {
		// Canonical string: PeerRIB.PeerAddr() feeds the best-path interner
		// and metric labels, which must match netip.Addr.String() everywhere.
		r.bgpPeers[peerAddr] = storage.NewPeerRIB(peerAddr.String())
	}
	peerRIB := r.bgpPeers[peerAddr]

	attrBytes := event.GetRawAttributesBytes()

	// Withdrawals first, for the reason handleReceivedStructured spells out:
	// RFC 4271 Section 4.3 says an UPDATE naming the same prefix in WITHDRAWN
	// ROUTES and NLRI is treated as though WITHDRAWN did not name it, so the
	// announce has to land last (RFC4271-4.3-5, RFC4271-4.3-7). This is the
	// JSON/pool sibling of that path and had the same ordering.
	for _, fam := range event.RawWithdrawnFamilies() {
		wdBytes := event.GetRawWithdrawnBytes(fam)
		if len(wdBytes) > 0 {
			r.removePoolNLRIs(peerRIB, fam, wdBytes, event.AddPath[fam])
		}
	}

	for _, fam := range event.RawNLRIFamilies() {
		nlriBytes := event.GetRawNLRIBytes(fam)
		if len(nlriBytes) > 0 {
			r.insertPoolNLRIs(peerRIB, fam, nlriBytes, attrBytes, event.AddPath[fam])
		}
	}
}

// insertPoolNLRIs inserts split NLRIs into the peer's RIB. Metric labels and
// log lines use PeerRIB's cached canonical address string (no per-call
// conversion).
func (r *RIBManager) insertPoolNLRIs(peerRIB *storage.PeerRIB, fam family.Family, nlriBytes, attrBytes []byte, addPath bool) {
	famStr := fam.String()
	if !nlrisplit.Supported(fam) {
		logger().Debug("pool: no splitter for family", "peer", peerRIB.PeerAddr(), "family", famStr)
		return
	}
	if addPath {
		peerRIB.SetAddPath(fam, true)
	}
	prefixes, err := nlrisplit.Split(fam, nlriBytes, addPath)
	if err != nil {
		logger().Warn("pool: split error, inserting parsed prefix", "peer", peerRIB.PeerAddr(), "family", famStr, "error", err, "parsed", len(prefixes))
	}
	for _, wirePrefix := range prefixes {
		peerRIB.Insert(fam, attrBytes, wirePrefix, true)
	}
	if m := metricsPtr.Load(); m != nil {
		m.routeInserts.With(peerRIB.PeerAddr(), famStr).Add(float64(len(prefixes)))
	}
	logger().Debug("pool: inserted routes", "peer", peerRIB.PeerAddr(), "family", famStr, "count", len(prefixes))
}

func (r *RIBManager) removePoolNLRIs(peerRIB *storage.PeerRIB, fam family.Family, wdBytes []byte, addPath bool) {
	famStr := fam.String()
	if !nlrisplit.Supported(fam) {
		return
	}
	withdrawns, err := nlrisplit.Split(fam, wdBytes, addPath)
	if err != nil {
		logger().Warn("pool: withdrawal split error", "peer", peerRIB.PeerAddr(), "family", famStr, "error", err, "parsed", len(withdrawns))
	}
	for _, wd := range withdrawns {
		peerRIB.Remove(fam, wd)
	}
	if m := metricsPtr.Load(); m != nil {
		m.routeWithdrawals.With(peerRIB.PeerAddr(), famStr).Add(float64(len(withdrawns)))
	}
	logger().Debug("pool: withdrew routes", "peer", peerRIB.PeerAddr(), "family", famStr, "count", len(withdrawns))
}

// handleRefresh processes a normal route refresh request from a peer.
// RFC 7313 Section 3: When receiving a route refresh request, the speaker
// SHOULD send BoRR, re-advertise Adj-RIB-Out, then send EoRR.
func (r *RIBManager) handleRefresh(event *Event) {
	fam := family.Family{AFI: event.AFI, SAFI: event.SAFI}

	if event.GetPeerAddress() == "" {
		logger().Warn("refresh event: empty peer address")
		return
	}
	peerAddr, err := parsePeerAddr(event.GetPeerAddress())
	if err != nil {
		logger().Warn("refresh event dropped", "error", err)
		return
	}

	r.peerMu.RLock()
	if !r.peerUp[peerAddr] {
		r.peerMu.RUnlock()
		logger().Debug("refresh request for down peer", "peer", peerAddr)
		return
	}

	routesToSend := r.collectRibOutRoutes(peerAddr, fam)
	r.peerMu.RUnlock()

	// RFC 7313 Section 4: Send BoRR, routes, EoRR sequence.
	// Markers dispatch through request-peer; only the routes use update-route.
	// The RPC selector stays the original event string.
	peerSelector := event.GetPeerAddress()
	var tb textbuf.Buffer
	r.dispatchPeerAction(peerSelector, tb.Str("borr ").Str(fam.String()).String())
	r.sendRoutes(peerSelector, routesToSend)
	r.dispatchPeerAction(peerSelector, tb.Reset().Str("eorr ").Str(fam.String()).String())

	logger().Debug("completed route refresh", "peer", peerSelector, "family", fam, "routes", len(routesToSend))
}

// handleStructuredState processes a structured state event from DirectBridge.
// Eliminates JSON parsing for state events (no ParseEvent/GetPeerAddress needed).
func (r *RIBManager) handleStructuredState(se *rpc.StructuredEvent) {
	peerAddr, err := parsePeerAddr(se.PeerAddress)
	if err != nil {
		logger().Warn("state structured event dropped", "error", err)
		return
	}
	state := se.State

	r.peerMu.Lock()
	// seenBefore must be read BEFORE the assignment below, which creates the key:
	// it is what tells a peer's first session (empty Adj-RIB-Out by definition)
	// from a re-established one. See collectPeerUpReplay.
	wasUp, seenBefore := r.peerUp[peerAddr]
	isUp := state == rpc.SessionStateUp
	r.peerUp[peerAddr] = isUp

	// cameUp records the down->up TRANSITION explicitly. It must not be inferred
	// from replayGroups != nil: collectGroupedRibOutRoutes returns nil both when
	// the peer did not come up and when it came up with nothing to replay, and a
	// fresh session is always the latter. Conflating the two made
	// replayRoutesWithCursor's empty-groups ready signal (rib_replay.go:250-253)
	// unreachable and delayed every fresh session's EOR by 2.5s.
	// See ai/rules/evidence.md (the zero-value trap).
	cameUp := false
	var replayGroups []replayGroup
	var pendingPurgeEmits map[family.Family][]bestChangeEntry

	if isUp && !wasUp {
		cameUp = true
		delete(r.retainedPeers, peerAddr)
		replayGroups = r.collectPeerUpReplay(peerAddr, seenBefore)
	} else if !isUp && wasUp {
		if r.retainedPeers[peerAddr] {
			logger().Debug("retaining Adj-RIB-In for GR", "peer", peerAddr)
		} else {
			if peerRIB := r.bgpPeers[peerAddr]; peerRIB != nil {
				peerRIB.Release()
				delete(r.bgpPeers, peerAddr)
			}
			delete(r.peerMeta, peerAddr)
			r.refreshSelfNextHopsLocked()
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
			// The interner is keyed by the canonical address string.
			pendingPurgeEmits = r.purgeBestPrevForPeer(peerAddr.String())
		}
	}
	r.peerMu.Unlock()

	r.emitPurgedWithdraws(pendingPurgeEmits)

	// Call on every peer-up, including with zero groups: replayRoutesWithCursor
	// is what signals "plugin session ready", and an empty Adj-RIB-Out still has
	// to say "nothing to replay, proceed".
	if cameUp {
		r.replayRoutesWithCursor(se.PeerAddress, replayGroups)
	}
}

// handleState processes peer state changes.
// Handles state transitions atomically to avoid races between up/down events.
// RFC 4724: When retainedPeers[peer] is set (by bgp-gr via "request bgp rib retain-routes"),
// Adj-RIB-In is preserved on peer-down instead of being deleted.
func (r *RIBManager) handleState(event *Event) {
	peerAddr, err := parsePeerAddr(event.GetPeerAddress())
	if err != nil {
		logger().Warn("state event dropped", "error", err)
		return
	}
	state := event.GetPeerState()

	r.peerMu.Lock()
	// seenBefore must be read BEFORE the assignment below, which creates the key:
	// it is what tells a peer's first session (empty Adj-RIB-Out by definition)
	// from a re-established one. See collectPeerUpReplay.
	wasUp, seenBefore := r.peerUp[peerAddr]
	isUp := state == "up"
	r.peerUp[peerAddr] = isUp

	// See handleStructuredState for why the transition is tracked explicitly
	// rather than inferred from replayGroups != nil.
	cameUp := false
	var replayGroups []replayGroup
	var pendingPurgeEmits map[family.Family][]bestChangeEntry

	if isUp && !wasUp {
		// Peer came up - clear retain flag (fresh session replaces stale state).
		cameUp = true
		delete(r.retainedPeers, peerAddr)
		replayGroups = r.collectPeerUpReplay(peerAddr, seenBefore)
	} else if !isUp && wasUp {
		// Peer went down - clear Adj-RIB-In unless retained for GR.
		if r.retainedPeers[peerAddr] {
			logger().Debug("retaining Adj-RIB-In for GR", "peer", peerAddr)
		} else {
			if peerRIB := r.bgpPeers[peerAddr]; peerRIB != nil {
				peerRIB.Release()
				delete(r.bgpPeers, peerAddr)
			}
			delete(r.peerMeta, peerAddr)
			r.refreshSelfNextHopsLocked()
			// See handleStructuredState for the emit-after-unlock contract.
			// The interner is keyed by the canonical address string.
			pendingPurgeEmits = r.purgeBestPrevForPeer(peerAddr.String())
		}
	}
	r.peerMu.Unlock()

	r.emitPurgedWithdraws(pendingPurgeEmits)

	// I/O operations after releasing lock. Called on every peer-up, including
	// with zero groups (see handleStructuredState).
	if cameUp {
		r.replayRoutesWithCursor(event.GetPeerAddress(), replayGroups)
	}
}

// updatePeerMetadata extracts and stores peer metadata from received events.
// Uses the nested peer format which includes both local and peer ASN, plus the
// group the session belongs to.
// Caller must hold write lock.
//
// The group is kept for the same reason the ASNs are: a decision made later in
// this plugin needs it and the event is where it arrives. An event carrying
// none of the facts below says nothing about the peer, so it stores nothing
// rather than replacing what an earlier event recorded.
//
// This is the JSON rail. handleReceivedStructured (rib_structured.go) builds
// the same struct from a StructuredEvent, and the two MUST carry the same
// fields: a fact one rail reads and the other drops is a peer whose best-path
// selection depends on how its events were delivered.
func (r *RIBManager) updatePeerMetadata(event *Event, peerAddr netip.Addr) {
	peerASN := event.GetPeerASN()
	localASN := getLocalASN(event)
	group := event.GetPeerGroup()
	localAddr := getLocalAddress(event)
	// The PEER's BGP Identifier, which RFC 4271 Section 9.1.2.2 step f)
	// compares (extractCandidate, rib_commands.go) and which every peer of a
	// TABLE_DUMP_V2 dump is keyed by (rib_mrt.go).
	//
	// It is in the guard below because the guard asks whether the event said
	// anything about the peer, and this is one of the things an event can say.
	// Leaving it out would drop an identifier that arrived alone, and would
	// leave the guard reading a struct it cannot see all of, which is the shape
	// of the defect this field exists to repair.
	remoteRouterID := event.GetPeerRouterID()
	if peerASN == 0 && localASN == 0 && group == "" && !localAddr.IsValid() && remoteRouterID == 0 {
		return
	}
	r.peerMeta[peerAddr] = &peerMetadata{
		PeerASN:        peerASN,
		LocalASN:       localASN,
		RemoteRouterID: remoteRouterID,
		GroupName:      group,
		LocalAddress:   localAddr,
	}
	r.refreshSelfNextHopsLocked()
}

// getLocalAddress extracts this speaker's end of the session from an event's
// peer format (YANG: local.address). Zero Addr when the event states none or
// states one that does not parse.
//
// This is the ONLY place the RIB learns what "itself" means (RFC 4271
// Section 5.1.3, isSelfNextHop). The field is already on every event carrying
// the nested peer format, so reading it adds no plumbing and creates no second
// source of truth.
func getLocalAddress(event *Event) netip.Addr {
	if len(event.Peer) == 0 {
		return netip.Addr{}
	}
	var info PeerInfoJSON
	if err := json.Unmarshal(event.Peer, &info); err != nil || info.Local == nil {
		return netip.Addr{}
	}
	return parseLocalAddr(info.Local.Address)
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

// GetYANG returns the embedded YANG for the RIB plugin.
func GetYANG() string {
	return yang.ZeRibYANG
}
