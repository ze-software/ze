// Design: docs/architecture/plugin/rib-storage-design.md — RIB plugin
// Detail: rib_nlri.go — NLRI wire format conversion helpers
// Detail: rib_commands.go — command handling and JSON responses
// Detail: rib_attr_format.go — attribute formatting for show enrichment
// Detail: bestpath.go — best-path selection algorithm (RFC 4271 §9.1.2)
// Detail: compaction.go — pool compaction scheduler wiring
// Detail: rib_pipeline.go — iterator pipeline for show commands (scope, filters, terminals)
// Detail: rib_pipeline_best.go — best-path pipeline for rib best commands
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
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
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

// ribMetrics holds Prometheus gauges for RIB route counts.
type ribMetrics struct {
	routesIn     metrics.Gauge    // global total
	routesOut    metrics.Gauge    // global total
	routesInVec  metrics.GaugeVec // per-peer
	routesOutVec metrics.GaugeVec // per-peer
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
	}
	metricsPtr.Store(m)
}

// metricsUpdateInterval is how often RIB metrics gauges are refreshed.
const metricsUpdateInterval = 10 * time.Second

// PeerMeta stores per-peer metadata for best-path comparison.
// Extracted from received UPDATE events (nested peer format).
type PeerMeta struct {
	PeerASN  uint32 // peer's AS number
	LocalASN uint32 // local AS number (for eBGP/iBGP detection)
}

// peerGRState holds per-peer Graceful Restart metadata in the RIB plugin.
// Stored when "rib mark-stale" is received so that "rib status" can display
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

	// ribOut stores routes sent TO peers (Adj-RIB-Out)
	ribOut map[string]map[string]*Route // peerAddr -> routeKey -> route

	// peerUp tracks which peers are currently up
	peerUp map[string]bool

	// peerMeta tracks per-peer metadata for best-path comparison.
	peerMeta map[string]*PeerMeta // peerAddr -> metadata

	// retainedPeers tracks peers whose Adj-RIB-In is retained during GR.
	// RFC 4724: When a GR-capable peer goes down, routes are retained until
	// the restart timer expires or the peer re-establishes and sends EOR.
	// Set by "rib retain-routes <peer>" command from bgp-gr plugin.
	// Cleared by "rib release-routes <peer>" or when the peer comes back up.
	retainedPeers map[string]bool

	// grState tracks per-peer Graceful Restart metadata.
	// Set by "rib mark-stale" command, cleared by "rib release-routes" or timer expiry.
	// RFC 4724 Section 4.2: Receiving Speaker route retention state.
	grState map[string]*peerGRState

	mu sync.RWMutex // protects ribInPool, ribOut, peerUp, peerMeta, retainedPeers, grState

	// lastMetricsInPeers / lastMetricsOutPeers track peer labels emitted in the
	// previous updateMetrics cycle. Peers that disappear from ribInPool/ribOut
	// get their GaugeVec label deleted, preventing stale Prometheus series.
	lastMetricsInPeers  map[string]bool
	lastMetricsOutPeers map[string]bool
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

	r.mu.RLock()

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
	for peer, routes := range r.ribOut {
		count := len(routes)
		m.routesOutVec.With(peer).Set(float64(count))
		currentOut[peer] = true
		totalOut += count
	}

	r.mu.RUnlock()

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
}

// RunRIBPlugin runs the RIB plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunRIBPlugin(conn net.Conn) int {
	logger().Debug("rib plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-rib", conn)
	defer func() { _ = p.Close() }()

	// Populate command table before creating manager.
	// Built-in commands registered here; plugins register via RegisterRIBCommand.
	registerBuiltinCommands()

	r := &RIBManager{
		plugin:        p,
		ribInPool:     make(map[string]*storage.PeerRIB),
		ribOut:        make(map[string]map[string]*Route),
		peerUp:        make(map[string]bool),
		peerMeta:      make(map[string]*PeerMeta),
		retainedPeers: make(map[string]bool),
		grState:       make(map[string]*peerGRState),
	}

	// Register event handler: dispatches BGP events (update, sent, state, refresh)
	p.OnEvent(func(jsonStr string) error {
		event, err := parseEvent([]byte(jsonStr))
		if err != nil {
			logger().Warn("parse error", "error", err, "line", jsonStr[:min(100, len(jsonStr))])
			return nil // Don't fail on parse errors
		}
		r.dispatch(event)
		return nil
	})

	// Register command handler: responds to "rib adjacent ..." commands
	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) {
		return r.handleCommand(command, peer, args)
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
		return nil
	})

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			// Unified show with pipeline (scope + filters + terminals)
			{Name: "rib status"},
			{Name: "rib show"},
			{Name: "rib clear in"},
			{Name: "rib clear out"},
			// Legacy status alias
			{Name: "rib adjacent status"},
			// GR support: route retention and stale tracking (RFC 4724)
			{Name: "rib retain-routes"},
			{Name: "rib release-routes"},
			{Name: "rib mark-stale"},
			{Name: "rib purge-stale"},
			// Best-path selection (RFC 4271 §9.1.2)
			{Name: "rib best"},
			{Name: "rib best status"},
			// Meta-commands (introspection)
			{Name: "rib help"},
			{Name: "rib command list"},
			{Name: "rib event list"},
		},
	})
	if err != nil {
		logger().Error("rib plugin failed", "error", err)
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

	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialize peer's ribOut if needed
	if r.ribOut[peerAddr] == nil {
		r.ribOut[peerAddr] = make(map[string]*Route)
	}

	// Process family operations
	// Format: {"ipv4/unicast": [{"next-hop": "...", "action": "add", "nlri": [...]}]}
	for family, ops := range event.FamilyOps {
		for _, op := range ops {
			switch op.Action {
			case "add":
				// Store routes with their next-hop
				for _, nlriVal := range op.NLRIs {
					prefix, pathID := parseNLRIValue(nlriVal)
					if prefix == "" {
						logger().Warn("sent: invalid nlri value",
							"peer", peerAddr, "family", family, "got", fmt.Sprintf("%T", nlriVal))
						continue
					}
					key := routeKey(family, prefix, pathID)
					r.ribOut[peerAddr][key] = &Route{
						MsgID:               msgID,
						Family:              family,
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
					}
				}
			case "del":
				// Remove routes
				for _, nlriVal := range op.NLRIs {
					prefix, pathID := parseNLRIValue(nlriVal)
					if prefix == "" {
						continue
					}
					key := routeKey(family, prefix, pathID)
					delete(r.ribOut[peerAddr], key)
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

	r.mu.Lock()
	defer r.mu.Unlock()

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
	for familyStr, hexNLRI := range event.RawNLRI {
		family, ok := parseFamily(familyStr)
		if !ok {
			logger().Warn("pool: unknown family", "peer", peerAddr, "family", familyStr)
			continue
		}

		// LIMITATION: splitNLRIs() only works for simple prefix formats (IPv4/IPv6 unicast).
		// EVPN, VPN, FlowSpec have different wire formats and would be corrupted.
		if !isSimplePrefixFamily(family) {
			logger().Debug("pool: skipping non-unicast family", "peer", peerAddr, "family", familyStr)
			continue
		}

		nlriBytes := event.GetRawNLRIBytes(familyStr)
		if len(nlriBytes) == 0 {
			continue
		}

		// Split concatenated NLRIs and insert each.
		// RFC 7911: ADD-PATH per-family flag from negotiated capabilities (via format=full JSON).
		addPath := event.AddPath[familyStr]
		prefixes := splitNLRIs(nlriBytes, addPath)
		for _, wirePrefix := range prefixes {
			peerRIB.Insert(family, attrBytes, wirePrefix)
		}

		logger().Debug("pool: inserted routes", "peer", peerAddr, "family", familyStr,
			"count", len(prefixes), "hex", hexNLRI[:min(16, len(hexNLRI))])
	}

	// Process withdrawals (raw-withdrawn)
	for familyStr := range event.RawWithdrawn {
		family, ok := parseFamily(familyStr)
		if !ok {
			continue
		}

		// Same limitation as announcements
		if !isSimplePrefixFamily(family) {
			continue
		}

		wdBytes := event.GetRawWithdrawnBytes(familyStr)
		if len(wdBytes) == 0 {
			continue
		}

		// Split and remove each.
		// RFC 7911: ADD-PATH per-family flag from negotiated capabilities (via format=full JSON).
		addPath := event.AddPath[familyStr]
		withdrawns := splitNLRIs(wdBytes, addPath)
		for _, wd := range withdrawns {
			peerRIB.Remove(family, wd)
		}

		logger().Debug("pool: withdrew routes", "peer", peerAddr, "family", familyStr, "count", len(withdrawns))
	}
}

// handleRefresh processes a normal route refresh request from a peer.
// RFC 7313 Section 3: When receiving a route refresh request, the speaker
// SHOULD send BoRR, re-advertise Adj-RIB-Out, then send EoRR.
func (r *RIBManager) handleRefresh(event *Event) {
	peerAddr := event.GetPeerAddress()
	family := event.AFI + "/" + event.SAFI

	if peerAddr == "" {
		logger().Warn("refresh event: empty peer address")
		return
	}

	r.mu.RLock()
	if !r.peerUp[peerAddr] {
		r.mu.RUnlock()
		logger().Debug("refresh request for down peer", "peer", peerAddr)
		return
	}

	// Copy routes for the requested family while holding lock
	var routesToSend []*Route
	if routes := r.ribOut[peerAddr]; routes != nil {
		for _, rt := range routes {
			if rt.Family == family {
				routesToSend = append(routesToSend, rt)
			}
		}
	}
	r.mu.RUnlock()

	// RFC 7313 Section 4: Send BoRR, routes, EoRR sequence
	r.updateRoute(peerAddr, "borr "+family)
	r.sendRoutes(peerAddr, routesToSend)
	r.updateRoute(peerAddr, "eorr "+family)

	logger().Debug("completed route refresh", "peer", peerAddr, "family", family, "routes", len(routesToSend))
}

// handleState processes peer state changes.
// Handles state transitions atomically to avoid races between up/down events.
// RFC 4724: When retainedPeers[peer] is set (by bgp-gr via "rib retain-routes"),
// Adj-RIB-In is preserved on peer-down instead of being deleted.
func (r *RIBManager) handleState(event *Event) {
	peerAddr := event.GetPeerAddress()
	state := event.GetPeerState()

	r.mu.Lock()
	wasUp := r.peerUp[peerAddr]
	isUp := state == "up"
	r.peerUp[peerAddr] = isUp

	var routesToReplay []*Route

	if isUp && !wasUp {
		// Peer came up - clear retain flag (fresh session replaces stale state).
		delete(r.retainedPeers, peerAddr)

		// Copy routes for replay while holding lock
		routes := r.ribOut[peerAddr]
		routesToReplay = make([]*Route, 0, len(routes))
		for _, rt := range routes {
			routesToReplay = append(routesToReplay, rt)
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
		}
	}
	r.mu.Unlock()

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

// getLocalASN extracts the local ASN from an event's nested peer format.
// Received events use PeerInfoNested which has ASN.Local and ASN.Peer.
func getLocalASN(event *Event) uint32 {
	if len(event.Peer) == 0 {
		return 0
	}
	var nested PeerInfoNested
	if err := json.Unmarshal(event.Peer, &nested); err == nil && nested.ASN.Local > 0 {
		return nested.ASN.Local
	}
	return 0
}

// GetYANG returns the embedded YANG schema for the RIB plugin.
func GetYANG() string {
	return schema.ZeRibYANG
}
