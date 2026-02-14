// Package rib implements a RIB (Routing Information Base) plugin for ze.
// It tracks routes received from peers (Adj-RIB-In) and sent to peers (Adj-RIB-Out).
//
// RFC 7911: ADD-PATH path-id is included in route keys when present.
// Multiple paths to the same prefix with different path-ids are stored separately.
package bgp_rib

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp-rib/schema"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp-rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/pool"
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

// parseFamily converts a family string like "ipv4/unicast" to nlri.Family.
// Returns false if the format is invalid.
func parseFamily(familyStr string) (nlri.Family, bool) {
	parts := strings.Split(familyStr, "/")
	if len(parts) != 2 {
		return nlri.Family{}, false
	}

	var afi nlri.AFI
	switch parts[0] {
	case "ipv4":
		afi = nlri.AFIIPv4
	case "ipv6":
		afi = nlri.AFIIPv6
	case "l2vpn":
		afi = nlri.AFIL2VPN
	default:
		return nlri.Family{}, false
	}

	var safi nlri.SAFI
	switch parts[1] {
	case "unicast":
		safi = nlri.SAFIUnicast
	case "multicast":
		safi = nlri.SAFIMulticast
	case "mpls-vpn":
		safi = nlri.SAFIVPN
	case "mpls-label":
		safi = nlri.SAFIMPLSLabel
	case "evpn":
		safi = nlri.SAFIEVPN
	case "flowspec":
		safi = nlri.SAFIFlowSpec
	default:
		return nlri.Family{}, false
	}

	return nlri.Family{AFI: afi, SAFI: safi}, true
}

// isSimplePrefixFamily returns true for families with simple NLRI format.
// Only IPv4/IPv6 unicast and multicast use the standard [prefix-len][prefix-bytes] format.
// Other families (EVPN, VPN, FlowSpec, etc.) have complex NLRI structures.
func isSimplePrefixFamily(family nlri.Family) bool {
	// Only unicast and multicast have simple [prefix-len][prefix-bytes] format
	if family.SAFI != nlri.SAFIUnicast && family.SAFI != nlri.SAFIMulticast {
		return false
	}
	return family.AFI == nlri.AFIIPv4 || family.AFI == nlri.AFIIPv6
}

// prefixToWire converts a text prefix to wire bytes.
// RFC 4271: NLRI format is [prefix-len:1][prefix-bytes].
// RFC 7911: ADD-PATH prepends [path-id:4].
//
// LIMITATION: Only works for IPv4/IPv6 unicast. Other families have different formats.
func prefixToWire(familyStr, prefix string, pathID uint32, addPath bool) ([]byte, error) {
	family, ok := parseFamily(familyStr)
	if !ok {
		return nil, fmt.Errorf("unknown family: %s", familyStr)
	}

	_, ipnet, err := net.ParseCIDR(prefix)
	if err != nil {
		return nil, fmt.Errorf("parse prefix: %w", err)
	}

	prefixLen, _ := ipnet.Mask.Size()
	prefixBytes := (prefixLen + 7) / 8

	// Normalize IP based on AFI
	var ip net.IP
	if family.AFI == nlri.AFIIPv4 {
		ip = ipnet.IP.To4()
	} else {
		ip = ipnet.IP.To16()
	}
	if ip == nil {
		return nil, fmt.Errorf("IP address mismatch for family %s", familyStr)
	}

	var wire []byte
	if addPath {
		wire = make([]byte, 4+1+prefixBytes)
		wire[0] = byte(pathID >> 24)
		wire[1] = byte(pathID >> 16)
		wire[2] = byte(pathID >> 8)
		wire[3] = byte(pathID)
		wire[4] = byte(prefixLen)
		copy(wire[5:], ip[:prefixBytes])
	} else {
		wire = make([]byte, 1+prefixBytes)
		wire[0] = byte(prefixLen)
		copy(wire[1:], ip[:prefixBytes])
	}

	return wire, nil
}

// wireToPrefix converts wire bytes to a text prefix.
// RFC 4271: NLRI format is [prefix-len:1][prefix-bytes].
// RFC 7911: ADD-PATH prepends [path-id:4].
//
// LIMITATION: Only works for IPv4/IPv6 unicast. Other families have different formats.
func wireToPrefix(family nlri.Family, wire []byte, addPath bool) (string, uint32, error) {
	offset := 0
	var pathID uint32

	if addPath {
		if len(wire) < 5 {
			return "", 0, fmt.Errorf("truncated ADD-PATH NLRI")
		}
		pathID = uint32(wire[0])<<24 | uint32(wire[1])<<16 | uint32(wire[2])<<8 | uint32(wire[3])
		offset = 4
	}

	if offset >= len(wire) {
		return "", 0, fmt.Errorf("truncated NLRI")
	}

	prefixLen := int(wire[offset])
	prefixBytes := (prefixLen + 7) / 8

	if offset+1+prefixBytes > len(wire) {
		return "", 0, fmt.Errorf("truncated NLRI prefix")
	}

	// Reconstruct IP
	var ip net.IP
	if family.AFI == nlri.AFIIPv4 {
		ip = make(net.IP, 4)
	} else {
		ip = make(net.IP, 16)
	}
	copy(ip, wire[offset+1:offset+1+prefixBytes])

	return fmt.Sprintf("%s/%d", ip.String(), prefixLen), pathID, nil
}

// splitNLRIs splits concatenated NLRI wire bytes into individual NLRIs.
// RFC 4271: NLRI format is [prefix-len:1][prefix-bytes].
// RFC 7911: ADD-PATH prepends [path-id:4].
//
// LIMITATION: Only works for IPv4/IPv6 unicast. Other families (EVPN, VPN,
// FlowSpec, labeled) have different NLRI structures and will parse incorrectly.
func splitNLRIs(data []byte, addPath bool) [][]byte {
	if len(data) == 0 {
		return nil
	}

	// RFC 4760: Maximum prefix length is 128 bits (IPv6).
	const maxPrefixLen = 128

	var result [][]byte
	offset := 0

	for offset < len(data) {
		start := offset
		var prefixLen int
		var nlriLen int

		if addPath {
			// ADD-PATH: [path-id:4][prefix-len:1][prefix-bytes]
			if offset+5 > len(data) {
				break
			}
			prefixLen = int(data[offset+4])
			nlriLen = 4 + 1 + (prefixLen+7)/8
		} else {
			// Standard: [prefix-len:1][prefix-bytes]
			if offset >= len(data) {
				break
			}
			prefixLen = int(data[offset])
			nlriLen = 1 + (prefixLen+7)/8
		}

		// Validate prefix length bounds
		if prefixLen > maxPrefixLen {
			logger().Warn("splitNLRIs: invalid prefix length", "prefixLen", prefixLen, "max", maxPrefixLen)
			return nil
		}

		if start+nlriLen > len(data) {
			break
		}

		result = append(result, data[start:start+nlriLen])
		offset = start + nlriLen
	}

	return result
}

// formatNLRIAsPrefix converts wire NLRI bytes to human-readable prefix string.
// For IPv4: [24][10][0][0] → "10.0.0.0/24".
// For IPv6: [64][...] → "2001:db8::/64".
// Returns hex encoding for unrecognized formats.
//
// NOTE: Only handles IPv4/IPv6 unicast without ADD-PATH.
// TODO: ADD-PATH support requires path-id prefix handling.
// TODO: VPN/EVPN/FlowSpec have different NLRI structures.
func formatNLRIAsPrefix(family nlri.Family, nlriBytes []byte) string {
	if len(nlriBytes) == 0 {
		return ""
	}

	prefixLen := int(nlriBytes[0])
	prefixBytes := nlriBytes[1:]

	switch family.AFI { //nolint:exhaustive // Only IPv4/IPv6 have standard prefix format
	case nlri.AFIIPv4:
		// Pad to 4 bytes
		ip := make([]byte, 4)
		copy(ip, prefixBytes)
		return fmt.Sprintf("%d.%d.%d.%d/%d", ip[0], ip[1], ip[2], ip[3], prefixLen)

	case nlri.AFIIPv6:
		// Pad to 16 bytes
		ip := make([]byte, 16)
		copy(ip, prefixBytes)
		return fmt.Sprintf("%x:%x:%x:%x:%x:%x:%x:%x/%d",
			uint16(ip[0])<<8|uint16(ip[1]),
			uint16(ip[2])<<8|uint16(ip[3]),
			uint16(ip[4])<<8|uint16(ip[5]),
			uint16(ip[6])<<8|uint16(ip[7]),
			uint16(ip[8])<<8|uint16(ip[9]),
			uint16(ip[10])<<8|uint16(ip[11]),
			uint16(ip[12])<<8|uint16(ip[13]),
			uint16(ip[14])<<8|uint16(ip[15]),
			prefixLen)

	default:
		// Unknown/unsupported family - return hex
		return fmt.Sprintf("hex:%x", nlriBytes)
	}
}

// formatFamily converts nlri.Family to string like "ipv4/unicast".
func formatFamily(family nlri.Family) string {
	var afi, safi string

	switch family.AFI { //nolint:exhaustive // Common families only, default handles rest
	case nlri.AFIIPv4:
		afi = "ipv4"
	case nlri.AFIIPv6:
		afi = "ipv6"
	case nlri.AFIL2VPN:
		afi = "l2vpn"
	case nlri.AFIBGPLS:
		afi = "bgp-ls"
	default:
		afi = fmt.Sprintf("afi-%d", family.AFI)
	}

	switch family.SAFI { //nolint:exhaustive // Common families only, default handles rest
	case nlri.SAFIUnicast:
		safi = "unicast"
	case nlri.SAFIMulticast:
		safi = "multicast"
	case nlri.SAFIVPN:
		safi = "mpls-vpn"
	case nlri.SAFIMPLSLabel:
		safi = "mpls-label"
	case nlri.SAFIEVPN:
		safi = "evpn"
	case nlri.SAFIFlowSpec:
		safi = "flowspec"
	case nlri.SAFIBGPLinkState:
		safi = "bgp-ls"
	default:
		safi = fmt.Sprintf("safi-%d", family.SAFI)
	}

	return afi + "/" + safi
}

// formatNextHop formats NEXT_HOP attribute bytes as an IP address string.
func formatNextHop(data []byte) string {
	switch len(data) {
	case 4:
		// IPv4.
		return fmt.Sprintf("%d.%d.%d.%d", data[0], data[1], data[2], data[3])
	case 16:
		// IPv6.
		return fmt.Sprintf("%x:%x:%x:%x:%x:%x:%x:%x",
			uint16(data[0])<<8|uint16(data[1]),
			uint16(data[2])<<8|uint16(data[3]),
			uint16(data[4])<<8|uint16(data[5]),
			uint16(data[6])<<8|uint16(data[7]),
			uint16(data[8])<<8|uint16(data[9]),
			uint16(data[10])<<8|uint16(data[11]),
			uint16(data[12])<<8|uint16(data[13]),
			uint16(data[14])<<8|uint16(data[15]))
	default:
		// Unknown format - return hex.
		return fmt.Sprintf("%x", data)
	}
}

// parseNLRIValue extracts prefix and path-id from an NLRI value.
// Handles both new format {"prefix":"...", "path-id":N} and legacy string format.
func parseNLRIValue(v any) (prefix string, pathID uint32) {
	switch val := v.(type) {
	case string:
		// Legacy string format: just the prefix
		return val, 0
	case map[string]any:
		// New structured format: {"prefix":"...", "path-id":N}
		if p, ok := val["prefix"].(string); ok {
			prefix = p
		}
		if pid, ok := val["path-id"].(float64); ok {
			pathID = uint32(pid)
		}
		return prefix, pathID
	default:
		return "", 0
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

// handleCommand processes command requests via SDK execute-command callback.
// Returns (status, data, error) for the SDK to send back to the engine.
// Supports both short names (rib show in) and legacy names (rib adjacent inbound show).
func (r *RIBManager) handleCommand(command, selector string) (string, string, error) {
	switch command {
	case "rib status", "rib adjacent status":
		return statusDone, r.statusJSON(), nil
	case "rib show in", "rib adjacent inbound show":
		return statusDone, r.inboundShowJSON(selector), nil
	case "rib clear in", "rib adjacent inbound empty":
		return statusDone, r.inboundEmptyJSON(selector), nil
	case "rib show out", "rib adjacent outbound show":
		return statusDone, r.outboundShowJSON(selector), nil
	case "rib clear out", "rib adjacent outbound resend":
		return statusDone, r.outboundResendJSON(selector), nil
	default: // fail on unknown command
		return "error", "", fmt.Errorf("unknown command: %s", command)
	}
}

// matchesPeer returns true if peerAddr matches the selector string.
// Supports: *, IP, !IP (negation), IP,IP,IP (multi-IP).
func matchesPeer(peerAddr, selector string) bool {
	selector = strings.TrimSpace(selector)

	if selector == "" || selector == "*" {
		return true
	}

	// Negation: !IP matches all except that IP
	if strings.HasPrefix(selector, "!") {
		excludeIP := strings.TrimSpace(selector[1:])
		return peerAddr != excludeIP
	}

	// Multi-IP: IP,IP,IP matches any in list
	if strings.Contains(selector, ",") {
		for s := range strings.SplitSeq(selector, ",") {
			if strings.TrimSpace(s) == peerAddr {
				return true
			}
		}
		return false
	}

	// Single IP
	return peerAddr == selector
}

// inboundShowJSON returns Adj-RIB-In routes filtered by selector as JSON.
func (r *RIBManager) inboundShowJSON(selector string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]map[string]any)

	for peer, peerRIB := range r.ribInPool {
		if !matchesPeer(peer, selector) {
			continue
		}
		var routeList []map[string]any
		peerRIB.Iterate(func(family nlri.Family, nlriBytes []byte, entry *storage.RouteEntry) bool {
			routeMap := map[string]any{
				"family": formatFamily(family),
				"prefix": formatNLRIAsPrefix(family, nlriBytes),
			}
			// Add next-hop if available from RouteEntry.
			if entry != nil && entry.HasNextHop() {
				if nhData, err := pool.NextHop.Get(entry.NextHop); err == nil {
					routeMap["next-hop"] = formatNextHop(nhData)
				}
			}
			routeList = append(routeList, routeMap)
			return true
		})
		if len(routeList) > 0 {
			result[peer] = routeList
		}
	}

	data, _ := json.Marshal(map[string]any{"adj_rib_in": result})
	return string(data)
}

// inboundEmptyJSON clears Adj-RIB-In routes for matching peers, returns JSON result.
func (r *RIBManager) inboundEmptyJSON(selector string) string {
	r.mu.Lock()
	cleared := 0

	for peer, peerRIB := range r.ribInPool {
		if !matchesPeer(peer, selector) {
			continue
		}
		cleared += peerRIB.Len()
		peerRIB.Release()
		delete(r.ribInPool, peer)
	}
	r.mu.Unlock()

	data, _ := json.Marshal(map[string]any{"cleared": cleared})
	return string(data)
}

// outboundShowJSON returns Adj-RIB-Out routes filtered by selector as JSON.
func (r *RIBManager) outboundShowJSON(selector string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]map[string]any)
	for peer, routes := range r.ribOut {
		if !matchesPeer(peer, selector) {
			continue
		}
		routeList := make([]map[string]any, 0, len(routes))
		for _, rt := range routes {
			routeMap := map[string]any{
				"family":   rt.Family,
				"prefix":   rt.Prefix,
				"next-hop": rt.NextHop,
			}
			if rt.PathID != 0 {
				routeMap["path-id"] = rt.PathID
			}
			routeList = append(routeList, routeMap)
		}
		result[peer] = routeList
	}

	data, _ := json.Marshal(map[string]any{"adj_rib_out": result})
	return string(data)
}

// outboundResendJSON replays Adj-RIB-Out routes for matching peers, returns JSON result.
// Does NOT send "plugin session ready" - that's only for initial reconnect.
func (r *RIBManager) outboundResendJSON(selector string) string {
	r.mu.RLock()
	var peersToResend []string
	routesToResend := make(map[string][]*Route)

	for peer, routes := range r.ribOut {
		if !matchesPeer(peer, selector) {
			continue
		}
		if !r.peerUp[peer] {
			continue // Only resend to up peers
		}
		peersToResend = append(peersToResend, peer)
		routesCopy := make([]*Route, 0, len(routes))
		for _, rt := range routes {
			routesCopy = append(routesCopy, rt)
		}
		routesToResend[peer] = routesCopy
	}
	r.mu.RUnlock()

	// Replay routes outside lock - use sendRoutes, not replayRoutes
	resent := 0
	for _, peer := range peersToResend {
		routes := routesToResend[peer]
		r.sendRoutes(peer, routes)
		resent += len(routes)
	}

	data, _ := json.Marshal(map[string]any{"resent": resent, "peers": len(peersToResend)})
	return string(data)
}

// sendRoutes sends routes to a peer without the "plugin session ready" signal.
// Used for manual resend operations. Includes full path attributes.
func (r *RIBManager) sendRoutes(peerAddr string, routes []*Route) {
	// Sort by MsgID to send in original announcement order
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].MsgID < routes[j].MsgID
	})

	for _, route := range routes {
		cmd := formatRouteCommand(route)
		r.updateRoute(peerAddr, cmd)
	}
}

// formatRouteCommand builds the update text command with full attributes.
// Format: update text [attrs...] nhop set <nh> nlri <family> add <prefix>.
// The peer selector is passed separately to updateRoute.
func formatRouteCommand(route *Route) string {
	var sb strings.Builder

	// Base command (peer selector is handled by updateRoute)
	sb.WriteString("update text")

	// Path-ID (RFC 7911) - must come before nlri
	if route.PathID != 0 {
		fmt.Fprintf(&sb, " path-information set %d", route.PathID)
	}

	// Origin
	if route.Origin != "" {
		sb.WriteString(" origin set ")
		sb.WriteString(route.Origin)
	}

	// AS-Path (use [] for list)
	if len(route.ASPath) > 0 {
		sb.WriteString(" as-path set ")
		sb.WriteString(attribute.FormatASPath(route.ASPath))
	}

	// MED
	if route.MED != nil {
		fmt.Fprintf(&sb, " med set %d", *route.MED)
	}

	// Local-Preference
	if route.LocalPreference != nil {
		fmt.Fprintf(&sb, " local-preference set %d", *route.LocalPreference)
	}

	// Communities (use [] for list)
	if len(route.Communities) > 0 {
		sb.WriteString(" community set [")
		sb.WriteString(strings.Join(route.Communities, " "))
		sb.WriteString("]")
	}

	// Large Communities (use [] for list)
	if len(route.LargeCommunities) > 0 {
		sb.WriteString(" large-community set [")
		sb.WriteString(strings.Join(route.LargeCommunities, " "))
		sb.WriteString("]")
	}

	// Extended Communities (use [] for list)
	if len(route.ExtendedCommunities) > 0 {
		sb.WriteString(" extended-community set [")
		sb.WriteString(strings.Join(route.ExtendedCommunities, " "))
		sb.WriteString("]")
	}

	// Next-hop (required)
	sb.WriteString(" nhop set ")
	sb.WriteString(route.NextHop)

	// NLRI with family
	sb.WriteString(" nlri ")
	sb.WriteString(route.Family)
	sb.WriteString(" add ")
	sb.WriteString(route.Prefix)

	return sb.String()
}

// statusJSON returns status as JSON.
func (r *RIBManager) statusJSON() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routesIn := 0
	for _, peerRIB := range r.ribInPool {
		routesIn += peerRIB.Len()
	}

	routesOut := 0
	for _, routes := range r.ribOut {
		routesOut += len(routes)
	}

	return fmt.Sprintf(`{"running":true,"peers":%d,"routes_in":%d,"routes_out":%d}`,
		len(r.peerUp), routesIn, routesOut)
}

// GetYANG returns the embedded YANG schema for the RIB plugin.
func GetYANG() string {
	return schema.ZeRibYANG
}
