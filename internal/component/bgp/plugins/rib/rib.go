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
// RFC 7911: the ADD-PATH path-id names a path of a route, never a second route.
// Multiple paths to the same prefix with different path-ids are stored separately
// in the value layer, and one election covers every path of the route.
package rib

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/slogutil"
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
	jsonKeyCount     = "count"
	jsonKeyInjected  = "injected"
	jsonKeyWithdrawn = "withdrawn"
	jsonKeyExisted   = "existed"
	jsonKeyWarning   = "warning"
)

const (
	// configRootBGP is the config subtree this plugin is delivered, and the
	// dependency it declares. A misspelling leaves the plugin configured with
	// nothing rather than refused.
	configRootBGP = "bgp"
	// protocolNameBGP is the protocol a redistribute route carries. It is the
	// name an operator writes, not the config root, even where they agree.
	protocolNameBGP = "bgp"
	// metaKeyReplay marks re-advertisements and their sent-event feedback.
	metaKeyReplay = "replay"
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

// eventBusPtr stores the EventBus this RIB emits on: best-change, FlowSpec
// change and validation-change. Set by the ConfigureEventBus callback before
// RunEngine in the daemon, and by runRIBPlugin to an sdkEventBus when the RIB
// runs forked. Replay requests arrive through the startup subscriptions on
// either transport (dispatchText), never through a bus subscription.
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
	routeInserts         metrics.CounterVec // labels: peer, family
	routeWithdrawals     metrics.CounterVec // labels: peer, family
	selectedCurrent      metrics.Gauge
	selectedDestinations metrics.Gauge
	selectedPending      metrics.Gauge
	selectedInFlight     metrics.Gauge
	selectedFailures     metrics.Gauge
	selectedBatches      metrics.CounterVec // labels: outcome

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

		routeInserts:         reg.CounterVec("ze_rib_route_inserts_total", "Routes inserted into Adj-RIB-In.", []string{metricLabelPeer, metricLabelFamily}),
		routeWithdrawals:     reg.CounterVec("ze_rib_route_withdrawals_total", "Routes withdrawn from Adj-RIB-In.", []string{metricLabelPeer, metricLabelFamily}),
		selectedCurrent:      reg.Gauge("ze_rib_selected_current_keys", "Current selected native route keys."),
		selectedDestinations: reg.Gauge("ze_rib_selected_destinations", "Admitted selected-output destination Sessions."),
		selectedPending:      reg.Gauge("ze_rib_selected_pending_keys", "Current selected-output reconciliation obligations across destinations."),
		selectedInFlight:     reg.Gauge("ze_rib_selected_inflight_keys", "Selected keys in final-writer batches currently in flight."),
		selectedFailures:     reg.Gauge("ze_rib_selected_failed_destinations", "Current destinations with unresolved selected-output failures."),
		selectedBatches:      reg.CounterVec("ze_rib_selected_batches_total", "Selected final-writer batches by completed or failed outcome.", []string{"outcome"}),

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
	processedMsgID   uint64 // Whole original UPDATE effects committed on this physical actor.
	processingFailed bool   // A current unapplied gap cannot be hidden by a larger later message.
	ConnectionID     uint64 // captured current-event provenance; retained routes use their SourceReference
	// Address and Session are observations of this exact configured Peer owner.
	// Address never supplies authority; Session fences delayed physical DOWN.
	Address  netip.Addr
	Session  uint64
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
	LocalAddress    netip.Addr
	refreshOwner    uint64
	refreshSession  uint64
	refreshOrdinary bool
}

// peerGRState holds per-peer Graceful Restart metadata in the RIB plugin.
// Stored when "request bgp rib mark-stale" is received so that "show bgp rib status"
// can display absolute times (when routes went stale, when they expire).
// RFC 4724 Section 4.2: Receiving Speaker route retention.
type peerGRState struct {
	// source narrows this active cycle to its immutable original connection.
	// A nil source belongs only to the explicit peer-wide operator commands.
	source *ribPhysicalSource
	// earlier lists the connections of earlier retained cycles of this peer
	// that still owned routes when source was retained (a consecutive restart
	// during the LLGR period). Their routes stay retained, and the GR plugin
	// addresses each one by its own connection, so each stays a known source
	// until it is released. Bounded by the previous cycle's connections that
	// still owned a route (earlierRetainedLocked); nil for a first restart.
	earlier     []uint64
	StaleAt     time.Time   // when routes were marked stale (disconnect time)
	RestartTime uint16      // peer's negotiated restart time in seconds
	ExpiresAt   time.Time   // StaleAt + RestartTime
	expiryTimer *time.Timer // safety-net timer — auto-purges stale if no command arrives
	expiryOwned bool        // a scheduled callback owns one manager timer count
}

// RIBManager implements a BGP RIB plugin.
// It tracks routes received from and sent to peers.
type RIBManager struct {
	// stopping closes intake, timer and background-reader admission under
	// peerMu. SDK Run drains delivery before shutdown joins and releases them.
	stopping         bool
	shutdownOnce     sync.Once
	backgroundCancel context.CancelFunc
	backgroundWG     sync.WaitGroup
	grTimers         sync.WaitGroup
	// plugin is the SDK plugin handle for engine RPCs (update-route, subscribe-events).
	plugin           *sdk.Plugin
	sourceLeases     *ribSourceLeases
	selectedDispatch *selectedDispatch

	// unstorableFamilies records each peer and family reportUnstorableFamily
	// has warned about, so the warning is said once (rib_unstorable.go).
	unstorableFamilies sync.Map

	// dispatchHook, when non-nil, intercepts dispatchPeerAction for test
	// inspection instead of issuing the dispatch-command RPC. Production leaves
	// it nil.
	dispatchHook func(command string)

	// updateHook, when non-nil, intercepts updateRoute and updateRouteWithMeta
	// for test inspection instead of issuing the update-route RPC. Production
	// leaves it nil.
	//
	// It is the second half of dispatchHook: the markers of a route refresh
	// travel the dispatch rail and the routes between them travel this one, so
	// a test holding only the first sees BoRR and EoRR with nothing in between
	// and cannot tell that from a refresh that re-advertised nothing. The meta
	// is passed because it decides whether the route reaches the wire at all
	// (meta["replay"], sendRoutes).
	updateHook func(command string, meta map[string]any)

	// updateRefuseHook, when non-nil, answers each update-route request first,
	// the way the engine answers a write it refuses (a destination whose attach
	// block grants this process no `send [ update ]`). A non-nil answer is the
	// refusal and the request goes no further. Production leaves it nil.
	updateRefuseHook func(input rpc.UpdateRouteInput) error

	// purgeRemoveHook, when non-nil, runs in emitPurgedWithdraws at the instant
	// a route the purge left with no candidate leaves the Loc-RIB, so a test
	// can land a concurrent election there. Production leaves it nil.
	purgeRemoveHook func(fam family.Family, pfx netip.Prefix)

	// removals is the recovery-removal dispatcher, nil until the run function
	// starts it and after it stops it. recoveryPending holds the removals it
	// has not sent yet (rib_sent_lifecycle.go, R-12). Both under peerMu.
	removals        *recoveryRemovals
	recoveryPending []sentLifecycleWrite

	// sourceApplied is how far each source's received UPDATEs are applied,
	// and how far they were when a lifecycle removal of its sent copies last
	// ran (rib_sent_lifecycle.go, R-12 for forwarded writes). Under peerMu.
	sourceApplied map[uint64]sourceApplied

	// sourcePolicy holds each source's source-policy transaction and its undo
	// pins (source_policy.go). Under peerMu; dropped when the session ends.
	sourcePolicy map[uint64]*sourcePolicyTxn

	// selectionDefer holds the RFC 4724 Section 4.1 per-family route selection
	// deferral state when `bgp update-delay max-delay` is configured.
	selectionDefer selectionDeferral
	deferClk       clock.Clock

	// ribInPool stores routes received FROM non-BGP protocols (e.g. BMP),
	// keyed by source protocol then protocol-defined peer key. BMP keys are
	// composite "router:peerIP" strings (see bmpCompositeKey and
	// withdrawAllForRouter's prefix match), so this inner map stays
	// string-keyed by design. BGP peers live in bgpPeers, keyed by their
	// authenticated configured Peer owner.
	ribInPool map[redistevents.ProtocolID]map[string]*storage.PeerRIB

	// bgpPeers holds BGP Adj-RIB-In state by original configured Peer owner.
	// Same-address sessions remain independent. peerMeta holds the observed
	// address and current Session; neither is an alternative authority index.
	bgpPeers map[uint64]*storage.PeerRIB

	// ribOut stores routes sent TO peers (Adj-RIB-Out), keyed per-family.
	// Enables per-family operations (route refresh, LLGR readvertisement).
	// Wire attribute bytes are deduplicated in pool.RibOut (idx 16).
	ribOut map[uint64]map[family.Family]map[ribOutKey]ribOutEntry // original destination owner -> family -> sent path

	// peerUp tracks which peers are currently up
	peerUp map[uint64]bool

	// peerNames holds the configured name of each session, as its last state
	// event stated it. Like peerUp it outlives the session, so a name still
	// selects a down session's retained and sent routes (rib_peer_name.go).
	peerNames map[uint64]string

	// peerMeta tracks per-peer metadata for best-path comparison.
	peerMeta map[uint64]*peerMetadata // original Peer owner -> observed facts

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
	retainedPeers map[uint64]bool

	// grState tracks per-peer Graceful Restart metadata.
	// Set by "request bgp rib mark-stale" command, cleared by "request bgp rib release-routes" or timer expiry.
	// RFC 4724 Section 4.2: Receiving Speaker route retention state.
	grState map[uint64]*peerGRState

	// borrActive tracks per-peer, per-family active Enhanced Route Refresh
	// (BoRR received, awaiting EoRR) state. Protected by peerMu.
	// RFC 7313 Section 4.
	borrActive map[uint64]map[family.Family]bool

	// bestPrev tracks the previous best-path per (family, prefix) for change
	// detection. Sharded by prefix: each family is split across N shards
	// (default GOMAXPROCS, ze.bgp.rib.bestprev.shards override), each with
	// its own bestPrevStore and write lock. checkBestPathChange takes only
	// the owning shard's lock. NOT protected by r.peerMu.
	bestPrev *bestPrevShards

	// advertise owns immutable current learned selections for ordinary output.
	// It captures retained winners; it never runs another candidate election.
	advertise advertiseState

	// unresolvedNextHops records routes currently excluded from Phase 2 by an
	// unresolvable next hop so runtime doctor checks read bounded live state
	// without taking peerMu or issuing engine RPCs (rib_next_hop_doctor.go).
	unresolvedNextHops unresolvedNextHopTracker

	// flowSpec is the immutable authorization and winning-action snapshot.
	// flowSpecMu serializes recomputation; events run after it is released.
	flowSpec              atomic.Pointer[flowSpecState]
	flowSpecMu            sync.Mutex
	flowSpecEmitting      bool
	flowSpecReplayPending bool
	flowSpecPending       *flowSpecState
	flowSpecPublished     *flowSpecState

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
	// Atomic publication is independent of peerMu and bestPrev shard locks.
	// Readers MUST load once and use that snapshot for the whole operation.
	// The shared RIB outlives the manager; shutdown drains accepted SDK event
	// deliveries before detaching the mirror and its tracker.
	locRIB  atomic.Pointer[locrib.RIB]
	forkRIB *remoteRIB

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

	// routeClock stamps each stored route with the reading of the message
	// that last changed it, and each answered row with the reading it was read
	// at (rib_route_age.go). Set once by newRIBManager and never written after.
	routeClock routeClock

	// relaxASPath enables the "as-path-relax" multipath semantic.
	// When true, paths with the same AS-path length but different content are
	// considered equal-cost. When false (default), AS-path content must match.
	// Populated from bgp/multipath/relax-as-path in the Stage 2 configure callback.
	relaxASPath atomic.Bool

	// acceptOwn enables RFC 7611 ACCEPT_OWN handling (default false per Section 2.3).
	// Populated from bgp/accept-own in the Stage 2 configure callback.
	acceptOwn atomic.Bool

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

	// flowSpecRedirectCfg is the per-session switch of the
	// draft-simpson-idr-flowspec-redirect-02 Section 3 EBGP check, resolved
	// across the bgp, group and peer levels. nil, before any config, runs the
	// check everywhere, the draft's default.
	flowSpecRedirectCfg atomic.Pointer[flowSpecRedirectConfig]

	// aggregateCfg holds the configured RFC 4271 Section 9.2.2.2 / RFC 9774
	// Section 5.2 route aggregation rules and local speaker identity. nil when
	// no aggregates are configured, keeping the hot path zero-alloc.
	aggregateCfg atomic.Pointer[aggregateConfig]
	// aggregatePublishMu serializes aggregate recomputation and publication
	// (Loc-RIB, BestChange, and peer UPDATE dispatch) so concurrent shard
	// updates cannot publish or withdraw out of order.
	aggregatePublishMu sync.Mutex
	// aggregateMu guards activeAggregates for concurrent snapshot reads.
	aggregateMu      sync.Mutex
	activeAggregates map[netip.Prefix]*synthesizedAggregate
	// localAddrsFn supplies the speaker's local interface addresses for
	// RFC 4271 Section 9.2.2.2 NEXT_HOP resolution. Nil defaults to
	// systemLocalInterfaceAddrs.
	localAddrsFn func() []netip.Addr

	// peerMu protects the peer-keyed maps ONLY: ribInPool (and bgpPeers), ribOut, peerUp,
	// peerMeta, retainedPeers, grState. bestPrev is sharded (see
	// bestprev_shard.go) and has its own per-shard locks. bestPathInterner
	// has its own per-table mutexes. Readers take peerMu.RLock for brief
	// map-level reads, then work on PeerRIB content under PeerRIB's own
	// lock. Elections take bestPrev shard.mu before peerMu.RLock. A writer MUST
	// release peerMu before taking any bestPrev shard or interner lock.
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
	inCounts := make(map[string]int)
	totalIn := 0
	for _, peerRIB := range r.bgpPeers {
		count := peerRIB.Len()
		label := peerRIB.PeerAddr()
		inCounts[label] += count
		currentIn[label] = true
		totalIn += count
	}
	for _, protoPeers := range r.ribInPool {
		for peer, peerRIB := range protoPeers {
			count := peerRIB.Len()
			inCounts[peer] += count
			currentIn[peer] = true
			totalIn += count
		}
	}

	for peer, count := range inCounts {
		m.routesInVec.With(peer).Set(float64(count))
	}
	currentOut := make(map[string]bool, len(r.ribOut))
	totalOut := 0
	outCounts := make(map[string]int)
	for peer, peerFamilies := range r.ribOut {
		count := 0
		for _, familyRoutes := range peerFamilies {
			count += len(familyRoutes)
		}
		address, known := r.peerAddressLocked(peer)
		if !known {
			continue
		}
		label := address.String()
		outCounts[label] += count
		currentOut[label] = true
		totalOut += count
	}

	r.peerMu.RUnlock()

	for label, count := range outCounts {
		m.routesOutVec.With(label).Set(float64(count))
	}

	r.updateSelectedMetrics(m)

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
// Safe for concurrent access. Nil disables future mirror operations; an operation
// that already loaded loc may finish on that shared RIB. Plugin shutdown MUST
// drain SDK delivery before calling SetLocRIB(nil), so accepted withdrawals finish.
func (r *RIBManager) SetLocRIB(loc *locrib.RIB) {
	r.peerMu.Lock()
	if r.locRIB.Load() == loc {
		r.peerMu.Unlock()
		return
	}
	unsubscribe := r.unsubForwardObs
	tracker := r.forwardTracker
	r.unsubForwardObs = nil
	r.forwardTracker = nil
	r.locRIB.Store(loc)
	if loc != nil {
		r.unsubForwardObs = observeForwardHandles(loc)
		r.forwardTracker = newForwardStateTracker(loc)
	}
	r.peerMu.Unlock()

	// Never join a consumer under peerMu: its callbacks can re-enter the RIB.
	if unsubscribe != nil {
		unsubscribe()
	}
	if tracker != nil {
		tracker.Stop()
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
		bgpPeers:         make(map[uint64]*storage.PeerRIB),
		ribOut:           make(map[uint64]map[family.Family]map[ribOutKey]ribOutEntry),
		peerUp:           make(map[uint64]bool),
		peerNames:        make(map[uint64]string),
		peerMeta:         make(map[uint64]*peerMetadata),
		sourceApplied:    make(map[uint64]sourceApplied),
		retainedPeers:    make(map[uint64]bool),
		grState:          make(map[uint64]*peerGRState),
		borrActive:       make(map[uint64]map[family.Family]bool),
		bestPrev:         newBestPrevShards(),
		bestPathInterner: newBestPrevInterner(),
		routeClock:       newRouteClock(clock.RealClock{}),
	}
	r.maximumPaths.Store(1)
	return r
}

// ribApplyBudgetSecs is the apply budget the rib declares, in seconds
// (sdk.Registration.ApplyBudget in runRIBPlugin).
const ribApplyBudgetSecs = 60

// runRIBPlugin runs the RIB plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func runRIBPlugin(conn net.Conn) int {
	logger().Debug("bgp rib plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-rib", conn)
	defer func() { _ = p.Close() }()

	// Populate the command table before creating the manager. init() has
	// already built it, through commandDecls(), and sync.Once makes this the
	// cheap restatement of the requirement rather than a second build.
	registerBuiltinCommands()

	r := newRIBManager(p)
	ctx, cancel := sdk.SignalContext()
	defer cancel()
	selected, err := newSelectedDispatch(ctx, p.SelectRoutes, r.signalSelectedReady, r.reportSelectedDestinations)
	if err != nil {
		logger().Error("BGP selected output could not start", "error", err,
			"code", codeDoctorBGPSelectedOutput, "fix", "ze request restart")
		return 1
	}
	// Both final refresh writers must be bound before the first UP.
	if err := selected.setRefreshWriters(p.SelectRefresh, r.replayConfiguredRefresh); err != nil {
		logger().Error("BGP selected refresh could not start", "error", err,
			"code", codeDoctorBGPSelectedOutput, "fix", "ze request restart")
		return 1
	}
	r.selectedDispatch = selected
	p.OnCurrentSelection(selected.currentSelection)
	p.OnOriginIntake(selected.originIntake)
	activeManager.Store(r)
	defer activeManager.Store(nil)
	ribevents.RegisterFlowSpecLookup(r.flowSpecEligible, r.flowSpecPresent, r.flowSpecPath, r.flowSpecRoutes, r.sourceProgress)
	defer ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil)
	ribevents.RegisterFlowSpecRedirectLookup(r.flowSpecRedirectRefusedFor)
	defer ribevents.RegisterFlowSpecRedirectLookup(nil)
	recovery := ribevents.PublishRecovery(r.recoveryRoutes)
	defer recovery.Close() // SDK Run MUST drain event delivery before unpublishing.
	// A forked bgp-rib reads the receive store's verdicts through the command
	// its validation-gate announcement names (ribevents.RouteEligible asks it
	// through this dispatch), rather than refusing every path.
	releaseGateDispatch := ribevents.RegisterGateDispatch(func(ctx context.Context, command string, args ...string) (string, json.RawMessage, error) {
		return p.DispatchCommandArgs(ctx, command, args, "")
	})
	defer releaseGateDispatch()

	// In-process consumers use the shared Loc-RIB; a subprocess mirrors selected
	// routes and resolves next-hop distances over the registered engine RPC.
	r.SetLocRIB(locrib.Default())
	defer r.SetLocRIB(nil) // SDK Run MUST drain event delivery before this detach.
	if r.locRIB.Load() == nil {
		r.setupRemoteRIB()
	}
	rpc.RegisterRouteInjector(r.handleInjectWireRoute)

	// Structured event handler for DirectBridge delivery.
	// Eliminates JSON round-trip: reads peer metadata from StructuredEvent fields,
	// raw wire bytes from RawMessage's AttrsWire/WireUpdate.
	// RFC 8212 Section 3: the source-policy lifecycle (source_policy.go).
	p.OnSourcePolicy(r.handleSourcePolicy)
	p.OnDoctorCheck(r.doctorCheck)

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
		r.dispatchText(jsonStr)
		return nil
	})

	// Register command handler for bgp-rib plugin commands. pipe is the list the
	// engine declared for the line, the same on DirectBridge and on the forked
	// transport (ExecuteCommandInput.Pipe): the route reads build their stages
	// from it.
	p.OnExecuteCommand(func(serial, command string, args []string, peer string, pipe []sdk.PipeOperation) (string, any, error) {
		return r.handleCommand(command, peer, args, pipe...)
	})

	// Config: maximum-paths, relax-as-path, the RFC 7999 honoring rules and the
	// AS notation, installed at Stage 2 and on every reload that changes them
	// (config_tx.go). RFC 4271 Section 9.1.2 extension: maximum-paths > 1 enables
	// ECMP/multipath best-path selection with up to N equal-cost paths per prefix.
	// The notation is recorded here only when this rib runs outside the daemon.
	r.newConfigTx(!p.IsInternal()).Register(p)

	// Register event subscriptions atomically with startup completion.
	// Included in the "ready" RPC so the engine registers them before SignalAPIReady,
	// ensuring the rib sees every "sent" event from the very first route.
	p.SetStartupSubscriptions([]string{"update direction sent", "update direction received", stateEvent, "refresh", "negotiated", "eor"}, nil, "full")
	// The receive store's eligibility change, through the SDK so a forked
	// bgp-rib hears it too: the daemon's own bus never reaches a subprocess.
	// Enveloped, so dispatchText tells it from the BGP events. The gate's
	// state comes the same way: a forked bgp-rib cannot reach the receive
	// store's lookup in memory, so it must learn the gate exists, and the
	// command that answers it (ribevents.NoteValidationGate), rather than
	// select every path.
	// gate-query asks this RIB to announce its flowspec-gate again, for a
	// reader whose subscription went live after the announcement.
	// replay-request, on both hops, asks this RIB for its whole best-path
	// table (replayBestPaths, replayRedistribute); through the SDK too, so a
	// forked RIB answers it.
	p.AddStartupSubscriptionsIn(ribevents.Namespace, []string{ribevents.EventValidationChange, ribevents.EventValidationGate, ribevents.EventGateQuery, ribevents.EventReplayRequest})
	p.AddStartupSubscriptionsIn(redistevents.ReplayNamespace, []string{redistevents.ReplayRequestEventType})
	p.SetEnvelope(true)
	// A forked RIB has no daemon bus: its best-change, FlowSpec change and
	// validation-change emissions reach the engine through the SDK instead,
	// and so the same subscribers.
	if !p.IsInternal() {
		SetEventBus(&sdkEventBus{plugin: p})
	}

	// Start compaction scheduler after 5-stage startup completes.
	// The scheduler runs as a goroutine tied to the plugin context,
	// reclaiming dead buffer space in attribute pools under route churn.
	p.OnStarted(func(ctx context.Context) error {
		// Both run once this RIB's subscriptions are live: the announcement
		// tells every reader which command answers the FlowSpec lookup, and
		// the query makes the receive store announce its validation gate again
		// in case bgp-rpki enabled it before this RIB subscribed. A failure
		// leaves a reader in another process failing closed (no FlowSpec rule
		// forwarded or replayed, every path refused while validation is on),
		// so it is logged rather than stopping the RIB.
		if err := r.announceFlowSpecGate(); err != nil {
			logger().Error("bgp-rib: flowspec-gate not announced: a process without this RIB's lookup forwards and replays no FlowSpec rule", "error", err)
		}
		if err := r.askGateOwners(); err != nil {
			logger().Error("bgp-rib: gate-query not sent: a validation gate enabled before this RIB subscribed stays unknown here", "error", err)
		}
		if r.forkRIB != nil {
			if _, err := p.RouteMetrics(ctx, nil); err != nil {
				return fmt.Errorf("bgp-rib requires engine next-hop metrics: %w", err)
			}
		}
		return r.startRIBWorkers(ctx)
	})

	r.sourceLeases = newRIBSourceLeases(r)
	r.sourceLeases.start(ctx)
	p.OnEventSettlement(r.settleEvents)
	// Started before events flow; stopped after Run has drained delivery.
	r.startRecoveryRemovals(ctx, r.sendRecoveryRemoval)
	defer r.shutdownReceivedSources()
	err = p.Run(ctx, &sdk.Registration{
		// Receive BGP policy and system distances at Stage 2 and on reload.
		WantsConfig:         []string{configRootBGP, "rib"},
		Commands:            commandDecls(),
		Dissemination:       disseminationDecls(),
		SignalsSessionReady: true,
		FencesLiveForwards:  true,
		DoctorChecks: []rpc.DoctorCheckDecl{
			advertiseDoctorDeclaration(),
			unresolvedNextHopDoctorDeclaration(),
			nextHopMetricsDoctorDeclaration(),
		},
		// ApplyBudget bounds a reload's apply: a selection-rule edit
		// (maximum-paths, relax-as-path, blackhole honoring) reselects every
		// stored route inside it (installConfig), one election per route. With
		// no budget declared the engine allows 30 seconds for the whole phase
		// across every participant; a full table of about a million routes,
		// elected at a few microseconds each, needs several seconds of that,
		// and the rollback re-runs the walk under three times the budget. The
		// value is static because the engine reads the registration's budget
		// for every transaction (transaction-protocol.md).
		ApplyBudget: ribApplyBudgetSecs,
	})
	if err != nil {
		logger().Error("bgp rib plugin failed", "error", err)
		return 1
	}

	return 0
}

// updateRoute clears the route-update cursor for matching peers via the engine.
func (r *RIBManager) updateRoute(peerSelector string) {
	const command = "update cursor done"
	if r.updateHook != nil {
		r.updateHook(command, nil)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _, err := r.plugin.UpdateRoute(ctx, peerSelector, command)
	if err != nil {
		logger().Warn("update-route failed", "peer", peerSelector, "error", err)
	}
}

// updateRouteWithMeta sends a route update command with metadata to matching peers.
// Lifecycle withdrawals use this path without received announcement provenance.
func (r *RIBManager) updateRouteWithMeta(peerSelector, command string, meta map[string]any) error {
	return r.updateRouteWithProvenance(peerSelector, command, meta, 0)
}

// updateRouteWithProvenance sends retained bytes with their original source
// policy epoch. The caller MUST supply the epoch stored with those bytes.
func (r *RIBManager) updateRouteWithProvenance(peerSelector, command string, meta map[string]any, sourcePolicyEpoch uint64) error {
	return r.updateRouteRequest(context.Background(), rpc.UpdateRouteInput{
		PeerSelector: peerSelector, Command: command, Meta: meta, SourcePolicyEpoch: sourcePolicyEpoch,
	})
}

// updateRouteWithMetaContext is updateRouteWithMeta under a caller's context,
// so a caller that owns a lifecycle can cancel the RPC in flight.
func (r *RIBManager) updateRouteWithMetaContext(parent context.Context, peerSelector, command string, meta map[string]any) error {
	return r.updateRouteRequest(parent, rpc.UpdateRouteInput{PeerSelector: peerSelector, Command: command, Meta: meta})
}

// updateRouteRequest bounds a command RPC by the caller's lifecycle context.
func (r *RIBManager) updateRouteRequest(parent context.Context, input rpc.UpdateRouteInput) error {
	if r.updateRefuseHook != nil {
		if err := r.updateRefuseHook(input); err != nil {
			logger().Warn("update-route-with-meta failed", "peer", input.PeerSelector, "error", err)
			return err
		}
	}
	if r.updateHook != nil {
		r.updateHook(input.Command, input.Meta)
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	_, _, err := r.plugin.UpdateRouteRequest(ctx, input)
	if err != nil {
		logger().Warn("update-route-with-meta failed", "peer", input.PeerSelector, "error", err)
	}
	return err
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

	switch eventType {
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
		// RFC 7313 Section 4: Beginning of Route Refresh from peer - mark family stale
		r.handleBoRR(event)
	case rpc.EventKindEoRR:
		// RFC 7313 Section 4: End of Route Refresh from peer - purge remaining stale routes
		r.handleEoRR(event)
	case rpc.EventKindUnspecified, rpc.EventKindOpen, rpc.EventKindNotification,
		rpc.EventKindKeepalive, rpc.EventKindEOR, rpc.EventKindNegotiated,
		rpc.EventKindUpdateFiltered, rpc.EventKindCount:
		// These events do not change the RIB.
	default:
		// The plugin event set is open; unknown events do not change the RIB.
	}
}

// handleSent processes sent UPDATE events.
// Stores routes in ribOut for replay on reconnect.
func (r *RIBManager) handleSent(event *Event) {
	if _, replay := event.RouteMeta[metaKeyReplay]; replay {
		return
	}
	if _, lifecycle := event.RouteMeta["rib-lifecycle"]; lifecycle {
		return
	}
	msgID := event.GetMsgID()
	logger().Debug("handleSent", "peer", event.GetPeerAddress(), "msgID", msgID, "familyOps", len(event.FamilyOps))

	if event.GetPeerAddress() == "" {
		logger().Debug("handleSent: empty peer address, skipping")
		return
	}
	address, err := parsePeerAddr(event.GetPeerAddress())
	if err != nil {
		logger().Warn("sent event dropped", "error", err)
		return
	}
	destinationOwner := event.PeerOwner
	if destinationOwner == 0 {
		return
	}
	sourcePaths, sourceErr := sentPathSources(event.RouteMeta)
	if sourceErr != nil {
		logger().Warn("sent source provenance dropped", "error", sourceErr)
		return
	}
	sourceID, _ := sentUint32(event.RouteMeta["source-id"])
	sourceOwner := sentSourceOwner(event.RouteMeta)
	localOrigin, _ := event.RouteMeta[bgptypes.SourceLocalMeta].(bool)

	// Full events carry native bytes even when the family has no text projection.
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
	sourceMessageID, _ := event.RouteMeta["source-message-id"].(float64)
	_, configStatic := event.RouteMeta["config-static"]
	_, replay := event.RouteMeta[metaKeyReplay]
	_, recovery := event.RouteMeta[bgptypes.RecoveryWriteMeta]

	r.peerMu.Lock()
	defer r.peerMu.Unlock()
	r.recordSentDestinationLocked(destinationOwner, address)

	if r.ribOut[destinationOwner] == nil {
		r.ribOut[destinationOwner] = make(map[family.Family]map[ribOutKey]ribOutEntry)
	}
	for _, fam := range event.RawWithdrawnFamilies() {
		r.removeSentNLRIs(destinationOwner, fam, event.GetRawWithdrawnBytes(fam), event.AddPath[fam])
	}
	for _, fam := range event.RawNLRIFamilies() {
		r.storeSentEntries(destinationOwner, fam, event.GetRawNLRIBytes(fam), event.AddPath[fam],
			ribOutEntry{MsgID: msgID, DestinationSession: event.DestinationSession, AttrHandle: attrHandle, ConfigStatic: configStatic, SourceMessageID: uint64(sourceMessageID),
				SourcePolicyEpoch: event.SourcePolicyEpoch,
				NextHop:           sentFamilyNextHop(event, fam), SourceID: sourceID, SourceOwner: sourceOwner, SourceConnectionID: sentSourceConnection(event.RouteMeta),
				LocalOrigin: localOrigin}, sourcePeer, replay, recovery, sourcePaths, 0)
	}

	followed := !recovery && r.lifecycleFollowedLocked(sourceOwner, uint64(sourceMessageID))
	for fam, ops := range event.FamilyOps {
		var ordinal uint32
		sourceIndex := 0
		for _, op := range ops {
			switch op.Action {
			case routeaction.Add:
				if len(event.GetRawNLRIBytes(fam)) > 0 {
					continue
				}
				if r.ribOut[destinationOwner] == nil {
					r.ribOut[destinationOwner] = make(map[family.Family]map[ribOutKey]ribOutEntry)
				}
				if r.ribOut[destinationOwner][fam] == nil {
					r.ribOut[destinationOwner][fam] = make(map[ribOutKey]ribOutEntry)
				}
				for _, nlriVal := range op.NLRIs {
					prefix, pathID := parseNLRIValue(nlriVal)
					if prefix == "" {
						logger().Warn("sent: invalid nlri value", "peer", destinationOwner, "family", fam)
						continue
					}
					pfx, err := netip.ParsePrefix(prefix)
					if err != nil {
						logger().Warn("sent: invalid prefix", "peer", destinationOwner, "prefix", prefix)
						continue
					}
					key := ribOutKey{Prefix: pfx.Masked(), PathID: pathID}
					old, existed := r.ribOut[destinationOwner][fam][key]
					stored := ribOutEntry{
						MsgID:              msgID,
						DestinationSession: event.DestinationSession,
						AttrHandle:         attrHandle,
						ConfigStatic:       configStatic,
						AddPath:            event.AddPath[fam] || sentNLRIHasPathID(nlriVal),
						NextHop:            op.NextHop,
						SourcePeer:         sourcePeer,
						SourceMessageID:    uint64(sourceMessageID),
						SourcePolicyEpoch:  event.SourcePolicyEpoch,
						SourceID:           sourceID,
						SourceOwner:        sourceOwner,
						SourceConnectionID: sentSourceConnection(event.RouteMeta),
						LocalOrigin:        localOrigin,
					}
					setSentPathSource(&stored, fam, ordinal, sourcePaths, &sourceIndex)
					ordinal++
					if existed {
						old.release()
					}
					if attrHandle.IsValid() {
						_ = pool.RibOut.AddRef(attrHandle)
					}
					r.ribOut[destinationOwner][fam][key] = stored
					if recovery {
						r.projectRecoveryWriteLocked(destinationOwner, fam, key)
					}
					if followed {
						r.projectForwardedWriteLocked(destinationOwner, fam, key)
					}
				}
			case routeaction.Del:
				if len(event.GetRawWithdrawnBytes(fam)) > 0 {
					continue
				}
				familyRoutes := r.ribOut[destinationOwner][fam]
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
					key := ribOutKey{Prefix: pfx.Masked(), PathID: pathID}
					if old, exists := familyRoutes[key]; exists {
						old.release()
						delete(familyRoutes, key)
					}
				}
				if len(familyRoutes) == 0 {
					delete(r.ribOut[destinationOwner], fam)
				}
				if len(r.ribOut[destinationOwner]) == 0 {
					delete(r.ribOut, destinationOwner)
				}
			case routeaction.Unspecified, routeaction.Update, routeaction.Withdraw:
				// Only Add and Del change this sent-route inventory.
			default:
				// The plugin action set is open; unknown actions leave routes unchanged.
			}
		}
	}
	if attrHandle.IsValid() {
		_ = pool.RibOut.Release(attrHandle)
	}
}

// sentNLRIHasPathID preserves an explicitly encoded identifier zero.
func sentNLRIHasPathID(value any) bool {
	fields, ok := value.(map[string]any)
	if !ok {
		return false
	}
	_, present := fields["path-id"]
	return present
}

// sentFamilyNextHop restores the MP next hop omitted from raw.attributes by JSON.
func sentFamilyNextHop(event *Event, fam family.Family) string {
	for _, op := range event.FamilyOps[fam] {
		if op.Action == routeaction.Add {
			return op.NextHop
		}
	}
	return ""
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
	committed := false
	storeApplied := false
	defer func() {
		if !committed {
			r.failSourceProcessed(peerAddr, event.PeerOwner, event.ConnectionID)
		}
	}()
	// An UPDATE arrives only in Established, where the peer's AS is always
	// negotiated. One without it is a Ze defect, and iBGP or eBGP cannot be
	// decided for its routes, so it is refused rather than stored under AS 0.
	// handleReceivedStructured (rib_structured.go) refuses the same event.
	remoteAS, err := event.GetPeerASN()
	if err != nil {
		logger().Warn("received UPDATE dropped", "peer", event.GetPeerAddress(), "error", err)
		return
	}
	peerAS, peerASKnown := remoteAS.ASN()
	if !peerASKnown {
		logger().Warn("received UPDATE dropped: the peer's AS is not negotiated", "peer", event.GetPeerAddress())
		return
	}

	if len(event.FamilyOps) == 0 {
		if !r.validEmptyReceived(event) {
			return
		}
		committed = true
		r.noteSourceProcessed(peerAddr, event.PeerOwner, event.ConnectionID, event.GetMsgID())
		return
	}

	// Require raw fields (format=full or structured byte path)
	hasRawFields := event.RawAttributes != "" || len(event.RawNLRI) > 0 || len(event.RawWithdrawn) > 0 ||
		len(event.RawAttributeBytes) > 0 || len(event.RawNLRIBytes) > 0 || len(event.RawWithdrawnBytes) > 0
	if !hasRawFields {
		logger().Warn("received event: missing raw fields, requires format=full", "peer", event.GetPeerAddress())
		return
	}
	source, err := r.sourceLeases.acquire(context.Background(), peerAddr, event.PeerOwner, event.SourceReceipt, event.ConnectionID)
	if err != nil {
		r.sourceLeases.reportFailures(context.Background())
		return
	}
	defer source.Release()
	if event.PeerOwner == 0 {
		return // A passive unrecorded event cannot select a source store.
	}

	r.peerMu.Lock()
	if r.stopping {
		r.peerMu.Unlock()
		return
	}
	if !r.sourceUpdateCurrentLocked(event.PeerOwner, peerAddr, event.ConnectionID) {
		r.peerMu.Unlock()
		return
	}
	defer func() {
		r.peerMu.Unlock()
		r.reconcileReceived(event)
		if storeApplied {
			committed = true
			r.noteSourceProcessed(peerAddr, event.PeerOwner, event.ConnectionID, event.GetMsgID())
		}
	}()

	// Track peer metadata for best-path comparison (eBGP/iBGP detection).
	r.updatePeerMetadata(event, peerAddr, peerAS)

	storeApplied = r.handleReceivedPool(event, peerAddr, source)
	r.noteReceivedLocked(event.PeerOwner, event.GetMsgID())
}

// handleReceivedPool stores routes in pool storage.
// Caller must hold write lock. PeerRIB caches the canonical address string
// for metric labels and log lines (PeerRIB.PeerAddr).
func (r *RIBManager) handleReceivedPool(event *Event, peerAddr netip.Addr, source *storage.SourceReference) bool {
	applied := true
	hadAnnounce := false
	peerOwner := event.PeerOwner
	if r.bgpPeers[peerOwner] == nil {
		// Canonical string: PeerRIB.PeerAddr() feeds the best-path interner
		// and metric labels, which must match netip.Addr.String() everywhere.
		created, err := storage.NewAddressPeerRIB(peerAddr)
		if err != nil {
			r.failAdvertise(err)
			return false
		}
		r.bgpPeers[peerOwner] = created
	}
	peerRIB := r.bgpPeers[peerOwner]

	attrBytes := event.GetRawAttributesBytes()
	if event.RawAttributes != "" && len(attrBytes) == 0 {
		return false
	}

	// Withdrawals first, for the reason handleReceivedStructured spells out:
	// RFC 4271 Section 4.3 says an UPDATE naming the same prefix in WITHDRAWN
	// ROUTES and NLRI is treated as though WITHDRAWN did not name it, so the
	// announce has to land last (RFC4271-4.3-5, RFC4271-4.3-7). This is the
	// JSON/pool sibling of that path and had the same ordering.
	for _, fam := range event.RawWithdrawnFamilies() {
		wdBytes := event.GetRawWithdrawnBytes(fam)
		if event.RawWithdrawn[fam] != "" && len(wdBytes) == 0 {
			applied = false
		}
		if len(wdBytes) > 0 {
			if !r.removePoolNLRIs(peerRIB, fam, wdBytes, event.AddPath[fam]) {
				applied = false
			}
		}
	}

	// RFC 9494 Section 4.3: the GR ingress step's verdict, from the event's
	// route-meta, as handleReceivedStructured reads it from the structured event.
	receivedStale := receivedLLGRStaleFromMeta(event.RouteMeta)
	for _, fam := range event.RawNLRIFamilies() {
		nlriBytes := event.GetRawNLRIBytes(fam)
		if event.RawNLRI[fam] != "" && len(nlriBytes) == 0 {
			applied = false
		}
		if len(nlriBytes) > 0 {
			hadAnnounce = true
			familyAttrs, err := receivedFamilyAttrs(event, fam, attrBytes)
			if err != nil {
				applied = false
				logger().Warn("received event: invalid next hop", "family", fam.String(), "error", err)
				continue
			}
			if !r.insertPoolNLRIs(peerRIB, fam, nlriBytes, familyAttrs, event.AddPath[fam], event.GetMsgID(), event.SourcePolicyEpoch, event.SourceReceipt, receivedStale, source) {
				applied = false
			}
		}
	}
	if !hadAnnounce {
		entry, _, _, err := storage.ParseRouteEntry(attrBytes)
		if err != nil {
			applied = false
		} else {
			entry.Release()
		}
	}
	return applied
}

// insertPoolNLRIs inserts split NLRIs into the peer's RIB. Metric labels and
// log lines use PeerRIB's cached canonical address string (no per-call
// conversion). receivedStale is the RFC 9494 Section 4.3 verdict written on
// every stored route.
func (r *RIBManager) insertPoolNLRIs(peerRIB *storage.PeerRIB, fam family.Family, nlriBytes, attrBytes []byte, addPath bool, msgID, sourcePolicyEpoch, sourceReceipt uint64, receivedStale bool, source *storage.SourceReference) bool {
	famStr := fam.String()
	if !nlrisplit.Supported(fam) {
		r.reportUnstorableFamily(peerRIB.PeerAddr(), fam)
		return true // Unsupported families intentionally have no receive-store effect.
	}
	if addPath {
		peerRIB.SetAddPath(fam, true)
	}
	parsed, fp, attrLen, parseErr := storage.ParseRouteEntry(attrBytes)
	if parseErr != nil {
		return false
	}
	defer parsed.Release()
	parsed.MsgID = msgID
	parsed.SourcePolicyEpoch = sourcePolicyEpoch
	parsed.SourceReceipt = sourceReceipt
	parsed.SetSourceReference(source)
	parsed.ReceivedLLGRStale = receivedStale
	// One reading for every route of this NLRI section, stored with a route
	// the section adds or changes and kept by an equal re-announcement
	// (storage.FamilyRIB.InsertEntry).
	parsed.ChangedAt = r.routeClock.now()
	prefixes, err := nlrisplit.Split(fam, nlriBytes, addPath)
	if err != nil {
		logger().Warn("pool: split error, inserting parsed prefix", "peer", peerRIB.PeerAddr(), "family", famStr, "error", err, "parsed", len(prefixes))
	}
	cidr := nlrisplit.KeysByCIDR(fam)
	for _, wirePrefix := range prefixes {
		if cidr {
			var affected [1]affectedPrefix
			prefixes := affected[:0]
			// RFC 8277 Section 2.2: keep labels beside the normalized CIDR,
			// exactly as on the structured receive path.
			r.insertLabeledEntry(peerRIB, fam, parsed, fp, attrLen, wirePrefix, addPath, receivedStale, &prefixes)
			continue
		}
		peerRIB.InsertEntry(fam, parsed, fp, attrLen, wirePrefix)
		peerRIB.ModifyFamilyEntry(fam, wirePrefix, func(entry *storage.RouteEntry) {
			entry.MsgID = msgID
			entry.SourcePolicyEpoch = sourcePolicyEpoch
			entry.SourceReceipt = sourceReceipt
			entry.SetSourceReference(source)
			entry.ReceivedLLGRStale = receivedStale
		})
	}
	if m := metricsPtr.Load(); m != nil {
		m.routeInserts.With(peerRIB.PeerAddr(), famStr).Add(float64(len(prefixes)))
	}
	logger().Debug("pool: inserted routes", "peer", peerRIB.PeerAddr(), "family", famStr, "count", len(prefixes))
	return err == nil
}

func (r *RIBManager) removePoolNLRIs(peerRIB *storage.PeerRIB, fam family.Family, wdBytes []byte, addPath bool) bool {
	famStr := fam.String()
	if !nlrisplit.Supported(fam) {
		return true
	}
	withdrawns, err := nlrisplit.SplitWithdrawn(fam, wdBytes, addPath)
	if err != nil {
		logger().Warn("pool: withdrawal split error", "peer", peerRIB.PeerAddr(), "family", famStr, "error", err, "parsed", len(withdrawns))
	}
	for _, wd := range withdrawns {
		peerRIB.Withdraw(fam, wd)
	}
	if m := metricsPtr.Load(); m != nil {
		m.routeWithdrawals.With(peerRIB.PeerAddr(), famStr).Add(float64(len(withdrawns)))
	}
	logger().Debug("pool: withdrew routes", "peer", peerRIB.PeerAddr(), "family", famStr, "count", len(withdrawns))
	return err == nil
}

// handleRefresh processes a normal route refresh request from a peer.
// RFC 7313 Section 3: When receiving a route refresh request, the speaker
// SHOULD send BoRR, re-advertise Adj-RIB-Out, then send EoRR.
func (r *RIBManager) handleRefresh(event *Event) {
	fam := family.Family{AFI: event.AFI, SAFI: event.SAFI}
	r.queueSelectedRefresh(event.GetPeerAddress(), event.PeerOwner,
		event.DestinationSession, event.SelectedDestination, fam)
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
	if r.consumeExportPolicy(peerAddr, state == rpc.SessionStateUp, se.SelectedDestination, se.DestinationSession, se.Reason) {
		return
	}
	r.rememberRefreshSession(peerAddr, se.PeerOwner, se.DestinationSession, state == rpc.SessionStateUp, se.SelectedDestination)
	ordinaryDestination := r.updateSelectedDestination(peerAddr, state == rpc.SessionStateUp, se.SelectedDestination, se.PeerOwner, se.DestinationSession)
	peerOwner := se.PeerOwner
	if peerOwner == 0 {
		return // Zero is unrecorded, never a wildcard source owner.
	}
	if state == rpc.SessionStateDown {
		r.selectionDefer.onPeerDown(peerAddr)
		if se.Reason != rpc.ReasonPeerRemoved {
			r.peerMu.RLock()
			current := r.sourceSessionCurrentLocked(peerOwner, se.DestinationSession)
			r.peerMu.RUnlock()
			if !current {
				return
			}
			r.retainSourceOnDown(peerAddr, peerOwner)
		}
	}
	defer r.settleSourceLeases(peerOwner)

	r.peerMu.Lock()
	// seenBefore must be read BEFORE the assignment below, which creates the key:
	// it is what tells a peer's first session (empty Adj-RIB-Out by definition)
	// from a re-established one. See collectPeerUpReplay.
	wasUp, seenBefore := r.peerUp[peerOwner]
	isUp := state == rpc.SessionStateUp
	r.peerUp[peerOwner] = isUp
	r.recordPeerStateLocked(peerOwner, peerAddr, se.DestinationSession, se.ConnectionID, isUp)
	r.recordPeerName(peerOwner, se.PeerName)

	// cameUp records the down->up TRANSITION explicitly. It must not be inferred
	// from replayGroups != nil: collectGroupedRibOutRoutes returns nil both when
	// the peer did not come up and when it came up with nothing to replay, and a
	// fresh session is always the latter. Conflating the two made
	// replayRoutesWithCursor's empty-groups ready signal (rib_replay.go:250-253)
	// unreachable and delayed every fresh session's EOR by 2.5s.
	// See ai/rules/evidence.md (the zero-value trap).
	cameUp := false
	peerBecameUp := isUp && !wasUp
	var replayGroups []replayGroup
	purgePeer := false
	var sourceKeys []affectedPrefix
	departed := !isUp
	if !wasUp {
		departed = se.Reason == rpc.ReasonPeerRemoved
	}
	if se.Reason == rpc.ReasonPeerRemoved {
		delete(r.retainedPeers, peerOwner)
	}

	if peerBecameUp {
		delete(r.retainedPeers, peerOwner)
		// Ordinary destinations replay current selection through their captured
		// worker, including on attach. Legacy attached observers have no replay
		// receipt or send authority. Both are still recorded up for source state.
		if !ordinaryDestination && se.Reason != rpc.ReasonAttached {
			cameUp = true
			replayGroups = r.collectPeerUpReplay(peerOwner, seenBefore)
		}
	} else if departed {
		delete(r.borrActive, peerOwner)
		r.dropSourcePolicyLocked(peerOwner)
		if r.retainedPeers[peerOwner] {
			logger().Debug("retaining Adj-RIB-In for GR", "peer", peerAddr)
		} else {
			if peerRIB := r.bgpPeers[peerOwner]; peerRIB != nil {
				sourceKeys = r.captureScopedSourceKeysLocked(peerRIB)
				peerRIB.Release()
				delete(r.bgpPeers, peerOwner)
			}
			r.clearPeerRouteMetadataLocked(peerOwner)
			r.refreshSelfNextHopsLocked()
			// Purge only after releasing peerMu: an election may hold a shard
			// while a synchronous Loc-RIB subscriber reads peer state.
			purgePeer = true
		}
	}
	if se.Reason == rpc.ReasonPeerRemoved {
		r.forgetPeerSourceLocked(peerOwner)
	}
	r.peerMu.Unlock()

	if purgePeer {
		if se.Reason == rpc.ReasonPeerRemoved {
			r.sourceLeases.forgetAdmission(peerOwner)
		}
		r.emitPurgedWithdraws(r.purgeBestPrevForOwner(bestPeer{owner: peerOwner, address: peerAddr.String()}))
		r.reconcileSourceKeys(sourceKeys)
	}

	// Call on every peer-up, including with zero groups: replayRoutesWithCursor
	// is what signals "plugin session ready", and an empty Adj-RIB-Out still has
	// to say "nothing to replay, proceed".
	if cameUp {
		r.replayRoutesWithCursor(se.PeerAddress, replayGroups, se.InitialReplay)
	}
	if peerBecameUp && se.Reason != rpc.ReasonAttached {
		r.replayAggregatesToPeer(se.PeerAddress)
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
	if r.consumeExportPolicy(peerAddr, state == "up", event.SelectedDestination, event.DestinationSession, event.Reason) {
		return
	}
	r.rememberRefreshSession(peerAddr, event.PeerOwner, event.DestinationSession, state == "up", event.SelectedDestination)
	ordinaryDestination := r.updateSelectedDestination(peerAddr, state == "up", event.SelectedDestination, event.PeerOwner, event.DestinationSession)
	peerOwner := event.PeerOwner
	if peerOwner == 0 {
		return // Zero is unrecorded, never a wildcard source owner.
	}
	if state == "down" {
		r.selectionDefer.onPeerDown(peerAddr)
		if event.Reason != rpc.ReasonPeerRemoved {
			r.peerMu.RLock()
			current := r.sourceSessionCurrentLocked(peerOwner, event.DestinationSession)
			r.peerMu.RUnlock()
			if !current {
				return
			}
			r.retainSourceOnDown(peerAddr, peerOwner)
		}
	}
	defer r.settleSourceLeases(peerOwner)

	r.peerMu.Lock()
	// seenBefore must be read BEFORE the assignment below, which creates the key:
	// it is what tells a peer's first session (empty Adj-RIB-Out by definition)
	// from a re-established one. See collectPeerUpReplay.
	wasUp, seenBefore := r.peerUp[peerOwner]
	isUp := state == "up"
	r.peerUp[peerOwner] = isUp
	r.recordPeerStateLocked(peerOwner, peerAddr, event.DestinationSession, event.ConnectionID, isUp)
	r.recordPeerName(peerOwner, event.GetPeerName())

	// See handleStructuredState for why the transition is tracked explicitly
	// rather than inferred from replayGroups != nil.
	cameUp := false
	peerBecameUp := isUp && !wasUp
	var replayGroups []replayGroup
	purgePeer := false
	var sourceKeys []affectedPrefix
	departed := !isUp
	if !wasUp {
		departed = event.Reason == rpc.ReasonPeerRemoved
	}
	if event.Reason == rpc.ReasonPeerRemoved {
		delete(r.retainedPeers, peerOwner)
	}

	if peerBecameUp {
		// Peer came up - clear retain flag (fresh session replaces stale state).
		// Ordinary output and legacy attached observers use neither this
		// SENT-history replay nor its readiness (see handleStructuredState).
		delete(r.retainedPeers, peerOwner)
		if !ordinaryDestination && event.Reason != rpc.ReasonAttached {
			cameUp = true
			replayGroups = r.collectPeerUpReplay(peerOwner, seenBefore)
		}
	} else if departed {
		delete(r.borrActive, peerOwner)
		r.dropSourcePolicyLocked(peerOwner)
		// Peer went down - clear Adj-RIB-In unless retained for GR.
		if r.retainedPeers[peerOwner] {
			logger().Debug("retaining Adj-RIB-In for GR", "peer", peerAddr)
		} else {
			if peerRIB := r.bgpPeers[peerOwner]; peerRIB != nil {
				sourceKeys = r.captureScopedSourceKeysLocked(peerRIB)
				peerRIB.Release()
				delete(r.bgpPeers, peerOwner)
			}
			r.clearPeerRouteMetadataLocked(peerOwner)
			r.refreshSelfNextHopsLocked()
			// See handleStructuredState for the off-peer-lock purge contract.
			purgePeer = true
		}
	}
	if event.Reason == rpc.ReasonPeerRemoved {
		r.forgetPeerSourceLocked(peerOwner)
	}
	r.peerMu.Unlock()

	if purgePeer {
		if event.Reason == rpc.ReasonPeerRemoved {
			r.sourceLeases.forgetAdmission(peerOwner)
		}
		r.emitPurgedWithdraws(r.purgeBestPrevForOwner(bestPeer{owner: peerOwner, address: peerAddr.String()}))
		r.reconcileSourceKeys(sourceKeys)
	}

	// I/O operations after releasing lock. Called on every peer-up, including
	// with zero groups (see handleStructuredState).
	if cameUp {
		r.replayRoutesWithCursor(event.GetPeerAddress(), replayGroups, event.InitialReplay)
	}
	if peerBecameUp && event.Reason != rpc.ReasonAttached {
		r.replayAggregatesToPeer(event.GetPeerAddress())
	}
}

// updatePeerMetadata extracts and stores peer metadata from received events.
// Uses the nested peer format which includes the local ASN and the group the
// session belongs to. peerAS is the peer's negotiated AS: handleReceived
// refuses an UPDATE whose AS is not known before it calls this, so the record
// written here always names the peer's AS and replaces what an earlier event
// recorded.
// Caller must hold write lock.
//
// The group is kept for the same reason the ASNs are: a decision made later in
// this plugin needs it and the event is where it arrives.
//
// This is the JSON rail. handleReceivedStructured (rib_structured.go) builds
// the same struct from a StructuredEvent, and the two MUST carry the same
// fields: a fact one rail reads and the other drops is a peer whose best-path
// selection depends on how its events were delivered.
func (r *RIBManager) updatePeerMetadata(event *Event, peerAddr netip.Addr, peerAS uint32) {
	localASN := getLocalASN(event)
	group := event.GetPeerGroup()
	localAddr := getLocalAddress(event)
	// The PEER's BGP Identifier, which RFC 4271 Section 9.1.2.2 step f)
	// compares (extractCandidate, rib_commands.go) and which every peer of a
	// TABLE_DUMP_V2 dump is keyed by (rib_mrt.go).
	remoteRouterID := event.GetPeerRouterID()
	var session uint64
	connectionID := event.ConnectionID
	if previous := r.peerMeta[event.PeerOwner]; previous != nil {
		session = previous.Session
		if previous.ConnectionID != 0 {
			connectionID = previous.ConnectionID
		}
	}
	metadata := &peerMetadata{
		ConnectionID:     connectionID,
		Address:          peerAddr,
		Session:          session,
		processedMsgID:   r.peerProcessedLocked(event.PeerOwner, peerAddr, connectionID),
		processingFailed: r.peerProcessingFailedLocked(event.PeerOwner, peerAddr, connectionID),
		PeerASN:          peerAS,
		LocalASN:         localASN,
		RemoteRouterID:   remoteRouterID,
		GroupName:        group,
		LocalAddress:     localAddr,
	}
	r.copyRefreshSessionLocked(event.PeerOwner, metadata)
	r.peerMeta[event.PeerOwner] = metadata
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

// shapeMap and shapeTab are the wire spellings of what an answer holds
// (pkg/plugin/rpc/types.go, CommandDecl.Shape): rows that carry their own keys,
// and rows read against a declared column order.
const (
	shapeMap = "map"
	shapeTab = "tab"
)

// commandDecls names the commands this plugin serves and states what each
// answer holds (pkg/plugin/rpc/types.go, CommandDecl).
//
// It has two readers and MUST stay one function. init() puts it on the
// registry.Registration, which anything linking the composition root reads
// without starting an engine, and the runner sends it in the Stage 1
// registration message, which a running daemon reads.
//
// Each summary is READ from the dispatch table rather than written again here.
// doRegisterBuiltinCommands (rib_commands.go) already states one for every
// command it registers, and `show bgp rib commands` answers from that same
// map, so a second copy on this list would be a future disagreement with
// nothing to arbitrate it (ai/rules/principles.md). registerBuiltinCommands is
// idempotent, so calling it here costs one sync.Once check.
//
// The three META-commands declare their own answer, and the reason is the
// resolution rule rather than a preference. A shape, a column order and an
// address-field list all resolve to the longest declared path that is a prefix
// of the command, so `show bgp rib`'s route declaration
// (internal/component/bgp/plugins/cmd/rib/rib.go) reached each of them: every
// one was published with eleven route columns it never writes and with `peer`
// and `next-hop` named as address fields, so `| resolve` and `| origin` were
// offered on an answer holding no address. Their producers are ribHelp,
// ribCommandList and ribEventList (rib_commands.go).
func commandDecls() []sdk.CommandDecl {
	registerBuiltinCommands()

	decls := []sdk.CommandDecl{
		// Unified show with pipeline (scope + filters + terminals)
		{Name: "show bgp rib status"},
		{Name: "show bgp rib"},
		{Name: "clear bgp rib in"},
		{Name: "clear bgp rib out"},
		{Name: ownerExcludedResendCommand, Shape: shapeMap},
		// GR support: route retention and stale tracking (RFC 4724)
		{Name: "request bgp rib retain-routes"},
		{Name: "request bgp rib release-routes"},
		{Name: "request bgp rib mark-stale"},
		{Name: "request bgp rib purge-stale"},
		// Source-owned DOWN selection; the engine provides causal sent ordering.
		{Name: "request bgp rib recovery", Hidden: true, Shape: shapeMap},
		// Plugin-to-plugin plumbing: a process that cannot read this RIB's
		// FlowSpec lookup in memory asks it here (flowSpecGateCommand).
		{Name: flowSpecGateCommand, Hidden: true, Shape: shapeMap},
		// LLGR uses these through the engine dispatcher, not private handlers.
		{Name: "request bgp rib attach-community"},
		{Name: "request bgp rib delete-with-community"},
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
		// Meta-commands (introspection). ribHelp and ribEventList each answer a
		// list of names, so the rows are those names and no column orders them.
		// ribCommandList answers one row for each command, and an operator
		// reads the command before the sentence describing it.
		{Name: "show bgp rib help", Shape: shapeMap},
		{Name: "show bgp rib commands", Shape: shapeTab, Columns: []string{"name", "description"}},
		{Name: "show bgp rib events", Shape: shapeMap},
		// Zero-copy forward-handle fast path (rib-arch-6)
		{Name: "request bgp rib fastpath"},
	}

	decls = append(decls, sourceCommandDecls()...)
	for index := range decls {
		decls[index].ShortHelp = commandSummary(decls[index].Name)
	}
	return decls
}

// commandSummary answers the one-line summary the dispatch table holds for a
// command, and the empty string for a command it does not name.
//
// The caller MUST have called registerBuiltinCommands first. An empty answer
// means the name on the declaration list and the name in the table disagree,
// which TestEveryDeclaredCommandCarriesItsTableSummary catches.
func commandSummary(name string) string {
	entry, found := registeredCommands[name]
	if !found {
		return ""
	}
	return entry.Description
}
