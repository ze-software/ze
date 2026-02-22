// Design: docs/architecture/plugin/rib-storage-design.md — RIB plugin
// Related: rib_nlri.go — NLRI wire format conversion helpers
// Related: rib_commands.go — command handling and JSON responses
//
// Package rib implements a RIB (Routing Information Base) plugin for ze.
// It tracks routes received from peers (Adj-RIB-In) and sent to peers (Adj-RIB-Out).
//
// RFC 7911: ADD-PATH path-id is included in route keys when present.
// Multiple paths to the same prefix with different path-ids are stored separately.
package bgp_rib

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp-rib/schema"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp-rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

const statusDone = "done"

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

	mu sync.RWMutex // protects ribInPool, ribOut, peerUp
}

// Route represents a stored route with full path attributes.
// RFC 7911: PathID is included when ADD-PATH is negotiated.
type Route struct {
	MsgID     uint64    `json:"msg-id,omitempty"`
	Family    string    `json:"family"`
	Prefix    string    `json:"prefix"`
	PathID    uint32    `json:"path-id,omitempty"` // RFC 7911: ADD-PATH path identifier
	NextHop   string    `json:"next-hop"`
	Timestamp time.Time `json:"timestamp,omitzero"`

	// Path attributes for full route resend
	Origin              string   `json:"origin,omitempty"`
	ASPath              []uint32 `json:"as-path,omitempty"`
	MED                 *uint32  `json:"med,omitempty"`
	LocalPreference     *uint32  `json:"local-preference,omitempty"`
	Communities         []string `json:"communities,omitempty"`
	LargeCommunities    []string `json:"large-communities,omitempty"`
	ExtendedCommunities []string `json:"extended-communities,omitempty"`
}

// routeKey creates a unique key for a route.
// RFC 7911: When ADD-PATH is negotiated, path-id is part of the key.
func routeKey(family, prefix string, pathID uint32) string {
	if pathID == 0 {
		return family + ":" + prefix
	}
	return fmt.Sprintf("%s:%s:%d", family, prefix, pathID)
}

// RunRIBPlugin runs the RIB plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunRIBPlugin(engineConn, callbackConn net.Conn) int {
	logger().Debug("rib plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-rib", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	r := &RIBManager{
		plugin:    p,
		ribInPool: make(map[string]*storage.PeerRIB),
		ribOut:    make(map[string]map[string]*Route),
		peerUp:    make(map[string]bool),
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
		return r.handleCommand(command, peer)
	})

	// Register event subscriptions atomically with startup completion.
	// Included in the "ready" RPC so the engine registers them before SignalAPIReady,
	// ensuring the rib sees every "sent" event from the very first route.
	p.SetStartupSubscriptions([]string{"update direction sent", "state", "refresh"}, nil, "full")

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			// Short names (primary — match engine API style)
			{Name: "rib status"},
			{Name: "rib show in"},
			{Name: "rib clear in"},
			{Name: "rib show out"},
			{Name: "rib clear out"},
			// Long names (RFC 4271 Adj-RIB terminology)
			{Name: "rib adjacent status"},
			{Name: "rib adjacent inbound show"},
			{Name: "rib adjacent inbound empty"},
			{Name: "rib adjacent outbound show"},
			{Name: "rib adjacent outbound resend"},
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

		// Split concatenated NLRIs and insert each
		// TODO: detect ADD-PATH from negotiation
		addPath := false
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

		// Split and remove each
		addPath := false
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
func (r *RIBManager) handleState(event *Event) {
	peerAddr := event.GetPeerAddress()
	state := event.GetPeerState()

	r.mu.Lock()
	wasUp := r.peerUp[peerAddr]
	isUp := state == "up"
	r.peerUp[peerAddr] = isUp

	var routesToReplay []*Route

	if isUp && !wasUp {
		// Peer came up - copy routes for replay while holding lock
		routes := r.ribOut[peerAddr]
		routesToReplay = make([]*Route, 0, len(routes))
		for _, rt := range routes {
			routesToReplay = append(routesToReplay, rt)
		}
	} else if !isUp && wasUp {
		// Peer went down - clear Adj-RIB-In while holding lock
		if peerRIB := r.ribInPool[peerAddr]; peerRIB != nil {
			peerRIB.Release()
			delete(r.ribInPool, peerAddr)
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

// GetYANG returns the embedded YANG schema for the RIB plugin.
func GetYANG() string {
	return schema.ZeRibYANG
}
