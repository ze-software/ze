package rr

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// statusDone is the command response status for successful operations.
const statusDone = "done"

// loggerPtr is the package-level logger, disabled by default.
// Stored as atomic.Pointer to avoid data races when tests start
// multiple in-process plugin instances concurrently.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// SetLogger configures the package-level logger for the RR plugin.
func SetLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// RouteServer implements a BGP Route Server API plugin.
// It forwards all UPDATEs to all peers except the source (forward-all model).
type RouteServer struct {
	plugin *sdk.Plugin
	peers  map[string]*PeerState
	rib    *RIB
	mu     sync.RWMutex
}

// RunRouteServer runs the Route Server plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunRouteServer(engineConn, callbackConn net.Conn) int {
	p := sdk.NewWithConn("rr", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	rs := &RouteServer{
		plugin: p,
		peers:  make(map[string]*PeerState),
		rib:    NewRIB(),
	}

	// Register event handler: dispatches BGP events (update, state, open, refresh)
	p.OnEvent(func(jsonStr string) error {
		event, err := parseEvent([]byte(jsonStr))
		if err != nil {
			logger().Warn("parse error", "error", err, "line", jsonStr[:min(100, len(jsonStr))])
			return nil // Don't fail on parse errors
		}
		rs.dispatch(event)
		return nil
	})

	// Register command handler: responds to "rr status" and "rr peers"
	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) {
		return rs.handleCommand(command)
	})

	// Register event subscriptions atomically with startup completion.
	// Included in the "ready" RPC so the engine registers them before SignalAPIReady,
	// ensuring the rr sees every event from the very first route.
	p.SetStartupSubscriptions([]string{"update", "state", "open", "refresh"}, nil, "")

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "rr status", Description: "Show RS status"},
			{Name: "rr peers", Description: "Show peer states"},
		},
	})
	if err != nil {
		logger().Error("rr plugin failed", "error", err)
		return 1
	}

	return 0
}

// updateRoute sends a route update command to matching peers via the engine.
func (rs *RouteServer) updateRoute(peerSelector, command string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _, err := rs.plugin.UpdateRoute(ctx, peerSelector, command)
	if err != nil {
		logger().Debug("update-route failed", "peer", peerSelector, "error", err)
	}
}

// dispatch routes an event to the appropriate handler.
func (rs *RouteServer) dispatch(event *Event) {
	switch event.Type {
	case "update":
		rs.handleUpdate(event)
	case "state":
		rs.handleState(event)
	case "refresh":
		rs.handleRefresh(event)
	case "open":
		rs.handleOpen(event)
	}
}

// handleUpdate processes UPDATE events (announcements and withdrawals).
func (rs *RouteServer) handleUpdate(event *Event) {
	peerAddr := event.Peer.Address.Peer
	msgID := event.MsgID

	// Validate input
	if peerAddr == "" {
		return // Ignore events with empty peer address
	}

	if event.Message == nil || event.Message.Update == nil {
		return
	}

	update := event.Message.Update

	// Collect families in this UPDATE
	families := make(map[string]bool)

	// Process announcements
	for family, nexthops := range update.Announce {
		families[family] = true
		for _, prefixes := range nexthops {
			prefixMap, ok := prefixes.(map[string]any)
			if !ok {
				continue
			}
			for prefix := range prefixMap {
				rs.rib.Insert(peerAddr, &Route{
					MsgID:  msgID,
					Family: family,
					Prefix: prefix,
				})
			}
		}
	}

	// Process withdrawals
	for family, prefixes := range update.Withdraw {
		families[family] = true
		for _, prefix := range prefixes {
			rs.rib.Remove(peerAddr, family, prefix)
		}
	}

	// Forward to compatible peers
	rs.forwardUpdate(peerAddr, msgID, families)
}

// forwardUpdate sends UPDATE to peers that support the given families.
func (rs *RouteServer) forwardUpdate(sourcePeer string, msgID uint64, families map[string]bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	for addr, peer := range rs.peers {
		if addr == sourcePeer {
			continue // Don't send back to source
		}
		if !peer.Up {
			continue // Skip down peers
		}

		// Check if peer supports all families in this UPDATE
		if peer.Families != nil {
			compatible := true
			for family := range families {
				if !peer.SupportsFamily(family) {
					compatible = false
					break
				}
			}
			if !compatible {
				continue
			}
		}

		rs.updateRoute(addr, fmt.Sprintf("cache %d forward", msgID))
	}
}

// handleState processes peer state changes.
func (rs *RouteServer) handleState(event *Event) {
	peerAddr := event.Peer.Address.Peer
	state := event.Peer.State

	rs.mu.Lock()
	if rs.peers[peerAddr] == nil {
		rs.peers[peerAddr] = &PeerState{Address: peerAddr}
	}
	rs.peers[peerAddr].Up = (state == "up")
	rs.mu.Unlock()

	switch state {
	case "down":
		rs.handleStateDown(peerAddr)
	case "up":
		rs.handleStateUp(peerAddr)
	}
}

// handleStateDown processes peer session teardown.
func (rs *RouteServer) handleStateDown(peerAddr string) {
	// Get and clear all routes from this peer
	routes := rs.rib.ClearPeer(peerAddr)

	// Send withdrawals for each route to other peers using update text syntax
	for _, route := range routes {
		rs.updateRoute("!"+peerAddr, fmt.Sprintf("update text nlri %s del %s", route.Family, route.Prefix))
	}
}

// handleStateUp processes peer session establishment.
func (rs *RouteServer) handleStateUp(peerAddr string) {
	// Get peer's supported families
	rs.mu.RLock()
	peer := rs.peers[peerAddr]
	rs.mu.RUnlock()

	// Replay all routes from other peers to this peer
	allPeers := rs.rib.GetAllPeers()

	for sourcePeer, routes := range allPeers {
		if sourcePeer == peerAddr {
			continue // Don't send peer's own routes back
		}
		for _, route := range routes {
			// Filter by family if peer has capability info
			if peer != nil && peer.Families != nil && !peer.SupportsFamily(route.Family) {
				continue
			}
			rs.updateRoute(peerAddr, fmt.Sprintf("cache %d forward", route.MsgID))
		}
	}
}

// handleOpen processes OPEN events to capture peer capabilities.
func (rs *RouteServer) handleOpen(event *Event) {
	peerAddr := event.Peer.Address.Peer
	if peerAddr == "" {
		return
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.peers[peerAddr] == nil {
		rs.peers[peerAddr] = &PeerState{Address: peerAddr}
	}
	peer := rs.peers[peerAddr]

	// Store ASN
	peer.ASN = event.Peer.ASN.Peer

	// Store capabilities and extract families from capability strings
	if event.Open != nil {
		peer.Capabilities = make(map[string]bool)
		peer.Families = make(map[string]bool)

		for _, cap := range event.Open.Capabilities {
			// New format: "<code> <name> <value>" (e.g., "1 multiprotocol ipv4/unicast")
			parts := strings.Fields(cap)
			if len(parts) < 2 {
				continue
			}
			// parts[0] is code (numeric), parts[1] is name
			name := parts[1]
			peer.Capabilities[name] = true

			// Extract family from multiprotocol capability
			// Format: "1 multiprotocol ipv4/unicast"
			if name == "multiprotocol" && len(parts) > 2 {
				peer.Families[parts[2]] = true
			}
		}
	}
}

// handleRefresh processes route refresh requests.
func (rs *RouteServer) handleRefresh(event *Event) {
	peerAddr := event.Peer.Address.Peer
	afi := event.AFI
	safi := event.SAFI
	family := afi + "/" + safi

	rs.mu.RLock()
	defer rs.mu.RUnlock()

	// Request refresh from peers that support route-refresh and the family
	for addr, peer := range rs.peers {
		if addr == peerAddr {
			continue // Don't request from requesting peer
		}
		if !peer.Up {
			continue // Skip down peers
		}
		if !peer.HasCapability("route-refresh") {
			continue // Skip peers without route-refresh
		}
		if peer.Families != nil && !peer.SupportsFamily(family) {
			continue // Skip peers that don't support this family
		}

		rs.updateRoute(addr, "refresh "+family)
	}
}

// handleCommand processes command requests via SDK execute-command callback.
// Returns (status, data, error) for the SDK to send back to the engine.
func (rs *RouteServer) handleCommand(command string) (string, string, error) {
	switch command {
	case "rr status":
		return statusDone, `{"running":true}`, nil
	case "rr peers":
		return statusDone, rs.peersJSON(), nil
	default: // fail on unknown command
		return "error", "", fmt.Errorf("unknown command: %s", command)
	}
}

// peersJSON returns peer state as JSON.
func (rs *RouteServer) peersJSON() string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	peers := make([]map[string]any, 0, len(rs.peers))
	for _, p := range rs.peers {
		peers = append(peers, map[string]any{
			"address": p.Address,
			"asn":     p.ASN,
			"up":      p.Up,
		})
	}

	data, _ := json.Marshal(map[string]any{"peers": peers})
	return string(data)
}

// parseEvent parses a JSON event from ze.
func parseEvent(data []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// Event represents a JSON event from ze.
type Event struct {
	Type    string       `json:"type"`
	MsgID   uint64       `json:"msg-id"`
	Peer    PeerInfo     `json:"peer"`
	Message *MessageInfo `json:"message,omitempty"`
	AFI     string       `json:"afi,omitempty"`
	SAFI    string       `json:"safi,omitempty"`
	// Request fields
	Serial  string `json:"serial,omitempty"`
	Command string `json:"command,omitempty"`
	// Open fields
	Open *OpenInfo `json:"open,omitempty"`
}

// OpenInfo contains OPEN message details.
// Note: Families are extracted from "multiprotocol <family>" capability strings.
type OpenInfo struct {
	Capabilities []string `json:"capabilities,omitempty"`
}

// PeerInfo contains peer identification.
type PeerInfo struct {
	Address AddressInfo `json:"address"`
	ASN     ASNInfo     `json:"asn"`
	State   string      `json:"state,omitempty"`
}

// AddressInfo contains IP addresses.
type AddressInfo struct {
	Local string `json:"local"`
	Peer  string `json:"peer"`
}

// ASNInfo contains AS numbers.
type ASNInfo struct {
	Local uint32 `json:"local"`
	Peer  uint32 `json:"peer"`
}

// MessageInfo contains the BGP message.
type MessageInfo struct {
	Update *UpdateInfo `json:"update,omitempty"`
}

// UpdateInfo contains UPDATE message details.
//
// Deprecated: This parsed representation will be removed in a future version.
// Use WireUpdate with iterator methods for zero-copy access.
// See docs/architecture/buffer-architecture.md for the migration path.
type UpdateInfo struct {
	Announce map[string]map[string]any `json:"announce,omitempty"`
	Withdraw map[string][]string       `json:"withdraw,omitempty"`
}
