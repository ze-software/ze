package bgp_rr

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

// Event type constants matching ze-bgp message.type values.
//
// Note: The engine also sends "borr" (Beginning of Route Refresh, RFC 7313 subtype 1)
// and "eorr" (End of Route Refresh, RFC 7313 subtype 2) as message.type values.
// These are intentionally not handled — a forward-all route server does not need
// to track refresh cycle boundaries. Only standard refresh is forwarded.
const (
	eventUpdate  = "update"
	eventState   = "state"
	eventOpen    = "open"
	eventRefresh = "refresh"
)

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
	p := sdk.NewWithConn("bgp-rr", engineConn, callbackConn)
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
	//
	// The "refresh" subscription also delivers "borr" and "eorr" events (RFC 7313)
	// because the engine maps all TypeROUTEREFRESH wire messages to the "refresh"
	// subscription type. These subtypes are silently ignored by dispatch().
	p.SetStartupSubscriptions([]string{eventUpdate, eventState, eventOpen, eventRefresh}, nil, "")

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
//
// Events with unrecognised types are silently ignored. This includes "borr" and "eorr"
// (RFC 7313 enhanced route refresh markers) which the engine delivers under the "refresh"
// subscription but encodes with distinct message.type values. A forward-all route server
// has no use for refresh-cycle boundaries — it only forwards standard refresh requests.
func (rs *RouteServer) dispatch(event *Event) {
	switch event.Type {
	case eventUpdate:
		rs.handleUpdate(event)
	case eventState:
		rs.handleState(event)
	case eventRefresh:
		rs.handleRefresh(event)
	case eventOpen:
		rs.handleOpen(event)
	}
}

// handleUpdate processes UPDATE events (announcements and withdrawals).
// Parses ze-bgp JSON family operations: {"ipv4/unicast": [{"action":"add","nlri":[...]}]}.
func (rs *RouteServer) handleUpdate(event *Event) {
	peerAddr := event.PeerAddr
	msgID := event.MsgID

	if peerAddr == "" {
		return
	}

	if len(event.FamilyOps) == 0 {
		return
	}

	families := make(map[string]bool)

	for family, ops := range event.FamilyOps {
		families[family] = true
		for _, op := range ops {
			switch op.Action {
			case "add":
				for _, n := range op.NLRIs {
					prefix := nlriToPrefix(n)
					if prefix != "" {
						rs.rib.Insert(peerAddr, &Route{
							MsgID:  msgID,
							Family: family,
							Prefix: prefix,
						})
					}
				}
			case "del":
				for _, n := range op.NLRIs {
					prefix := nlriToPrefix(n)
					if prefix != "" {
						rs.rib.Remove(peerAddr, family, prefix)
					}
				}
			}
		}
	}

	// Forward asynchronously so the deliver-event callback responds immediately.
	// OnEvent is a synchronous RPC — blocking here stalls all subsequent event
	// deliveries, causing "context deadline exceeded" under load.
	go rs.forwardUpdate(peerAddr, msgID, families)
}

// nlriToPrefix extracts a prefix string from an NLRI value.
// Simple NLRIs are strings ("10.0.0.0/24"). Complex NLRIs (ADD-PATH, VPN)
// are objects with a "prefix" field ({"prefix":"10.0.0.0/24","path-id":1}).
func nlriToPrefix(n any) string {
	switch v := n.(type) {
	case string:
		return v
	case map[string]any:
		if p, ok := v["prefix"].(string); ok {
			return p
		}
	}
	return ""
}

// forwardUpdate sends UPDATE to peers that support the given families.
//
// Uses a single cache-forward command with a comma-separated peer selector.
// ForwardUpdate in the reactor uses Get() to read the cache entry and
// Decrement() to count down consumers — the entry expires once all consumers
// have decremented. A single multi-peer selector ensures all compatible
// peers receive the UPDATE in one atomic operation.
func (rs *RouteServer) forwardUpdate(sourcePeer string, msgID uint64, families map[string]bool) {
	rs.mu.RLock()
	var targets []string
	for addr, peer := range rs.peers {
		if addr == sourcePeer || !peer.Up {
			continue
		}
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
		targets = append(targets, addr)
	}
	rs.mu.RUnlock()

	if len(targets) == 0 {
		return
	}

	sel := strings.Join(targets, ",")
	rs.updateRoute("*", fmt.Sprintf("bgp cache %d forward %s", msgID, sel))
}

// handleState processes peer state changes.
// ze-bgp JSON: {"type":"bgp","bgp":{"message":{"type":"state"},"peer":{...},"state":"up"}}.
func (rs *RouteServer) handleState(event *Event) {
	peerAddr := event.PeerAddr
	state := event.State

	if peerAddr == "" {
		return
	}

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
	routes := rs.rib.ClearPeer(peerAddr)

	// Send withdrawals asynchronously to avoid blocking event delivery.
	go func() {
		for _, route := range routes {
			rs.updateRoute("!"+peerAddr, fmt.Sprintf("update text nlri %s del %s", route.Family, route.Prefix))
		}
	}()
}

// handleStateUp processes peer session establishment.
//
// Requests route re-advertisement from established peers via ROUTE-REFRESH
// (RFC 2918) so the newly-up peer receives existing routes. We cannot use
// cache-forward for replay because ForwardUpdate's Take() is destructive —
// cache entries from prior forwards are already consumed. ROUTE-REFRESH
// asks source peers to re-send their Adj-RIB-Out; the resulting UPDATEs
// flow through handleUpdate → forwardUpdate and reach all connected peers
// including the new one. Duplicate announcements to already-connected peers
// are idempotent in BGP.
func (rs *RouteServer) handleStateUp(peerAddr string) {
	rs.mu.RLock()
	peer := rs.peers[peerAddr]

	// Determine families the new peer supports.
	// If Families is nil (no OPEN received yet), fall back to families
	// present in the RIB so we don't skip the refresh entirely.
	var families []string
	if peer != nil && peer.Families != nil {
		for family := range peer.Families {
			families = append(families, family)
		}
	}

	// Collect established peers that support route-refresh.
	var refreshPeers []string
	for addr, other := range rs.peers {
		if addr == peerAddr || !other.Up {
			continue
		}
		if !other.HasCapability("route-refresh") {
			continue
		}
		refreshPeers = append(refreshPeers, addr)
	}
	rs.mu.RUnlock()

	// If no explicit families from OPEN, derive from RIB contents.
	if len(families) == 0 {
		familySet := make(map[string]bool)
		for sourcePeer, routes := range rs.rib.GetAllPeers() {
			if sourcePeer == peerAddr {
				continue
			}
			for _, route := range routes {
				familySet[route.Family] = true
			}
		}
		for f := range familySet {
			families = append(families, f)
		}
	}

	if len(families) == 0 || len(refreshPeers) == 0 {
		return
	}

	// Request route re-advertisement asynchronously to avoid blocking event delivery.
	go func() {
		for _, addr := range refreshPeers {
			for _, family := range families {
				rs.updateRoute(addr, "refresh "+family)
			}
		}
	}()
}

// handleOpen processes OPEN events to capture peer capabilities.
// ze-bgp JSON capabilities are objects: [{"code":1,"name":"multiprotocol","value":"ipv4/unicast"}].
func (rs *RouteServer) handleOpen(event *Event) {
	peerAddr := event.PeerAddr
	if peerAddr == "" {
		return
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.peers[peerAddr] == nil {
		rs.peers[peerAddr] = &PeerState{Address: peerAddr}
	}
	peer := rs.peers[peerAddr]

	peer.ASN = event.PeerASN

	if event.Open != nil {
		peer.Capabilities = make(map[string]bool)
		peer.Families = make(map[string]bool)

		for _, cap := range event.Open.Capabilities {
			peer.Capabilities[cap.Name] = true

			if cap.Name == "multiprotocol" && cap.Value != "" {
				peer.Families[cap.Value] = true
			}
		}
	}
}

// handleRefresh processes route refresh requests.
// ze-bgp JSON: AFI/SAFI nested under refresh object.
//
// Collects eligible peers under the lock, then sends refresh commands after
// releasing — updateRoute does an SDK RPC with a 10 s timeout, so holding
// the lock during network I/O would block all state updates.
func (rs *RouteServer) handleRefresh(event *Event) {
	peerAddr := event.PeerAddr
	family := event.AFI + "/" + event.SAFI

	if peerAddr == "" {
		return
	}

	rs.mu.RLock()
	var targets []string
	for addr, peer := range rs.peers {
		if addr == peerAddr {
			continue
		}
		if !peer.Up {
			continue
		}
		if !peer.HasCapability("route-refresh") {
			continue
		}
		if peer.Families != nil && !peer.SupportsFamily(family) {
			continue
		}
		targets = append(targets, addr)
	}
	rs.mu.RUnlock()

	// Send refreshes asynchronously to avoid blocking event delivery.
	go func() {
		for _, addr := range targets {
			rs.updateRoute(addr, "refresh "+family)
		}
	}()
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

// --- Event parsing ---

// parseEvent parses a ze-bgp JSON event from the engine.
// ze-bgp JSON format: {"type":"bgp","bgp":{"message":{"type":"update"},"peer":{...},...}}.
// Event type comes from message.type inside the bgp wrapper, NOT from the top-level type.
func parseEvent(data []byte) (*Event, error) {
	// Step 1: Unwrap ze-bgp envelope {"type":"bgp","bgp":{...}}
	var wrapper struct {
		Type string          `json:"type"`
		BGP  json.RawMessage `json:"bgp"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}

	payload := data
	if wrapper.Type == "bgp" && len(wrapper.BGP) > 0 {
		payload = wrapper.BGP
	}

	// Step 2: Parse bgp-level fields (message metadata, peer, event-specific data)
	var bgp struct {
		Message *messageInfo    `json:"message"`
		Peer    peerFlat        `json:"peer"`
		State   string          `json:"state"`
		Update  json.RawMessage `json:"update"`
		Open    json.RawMessage `json:"open"`
		Refresh json.RawMessage `json:"refresh"`
	}
	if err := json.Unmarshal(payload, &bgp); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	event := &Event{
		PeerAddr: bgp.Peer.Address,
		PeerASN:  bgp.Peer.ASN,
		State:    bgp.State,
	}

	// Determine event type from message.type
	if bgp.Message != nil {
		event.Type = bgp.Message.Type
		event.MsgID = bgp.Message.ID
	}

	// Step 3: Parse event-specific data
	switch event.Type {
	case eventUpdate:
		if len(bgp.Update) > 0 {
			parseUpdateData(event, bgp.Update)
		}
	case eventOpen:
		if len(bgp.Open) > 0 {
			var openInfo OpenInfo
			if err := json.Unmarshal(bgp.Open, &openInfo); err == nil {
				event.Open = &openInfo
			}
		}
	case eventRefresh:
		if len(bgp.Refresh) > 0 {
			var refresh struct {
				AFI  string `json:"afi"`
				SAFI string `json:"safi"`
			}
			if err := json.Unmarshal(bgp.Refresh, &refresh); err == nil {
				event.AFI = refresh.AFI
				event.SAFI = refresh.SAFI
			}
		}
	}

	return event, nil
}

// parseUpdateData extracts family operations from the UPDATE payload.
// ze-bgp JSON: {"attr":{...},"nlri":{"ipv4/unicast":[{"action":"add","nlri":[...]}]}}.
func parseUpdateData(event *Event, data json.RawMessage) {
	var update struct {
		NLRI json.RawMessage `json:"nlri"`
	}
	if err := json.Unmarshal(data, &update); err != nil || len(update.NLRI) == 0 {
		return
	}

	var familyMap map[string]json.RawMessage
	if err := json.Unmarshal(update.NLRI, &familyMap); err != nil {
		return
	}

	event.FamilyOps = make(map[string][]FamilyOperation, len(familyMap))
	for family, opsData := range familyMap {
		if !strings.Contains(family, "/") {
			continue
		}
		var ops []FamilyOperation
		if err := json.Unmarshal(opsData, &ops); err == nil {
			event.FamilyOps[family] = ops
		}
	}
}

// --- Event types ---

// Event represents a parsed ze-bgp JSON event.
// Fields are extracted from the nested ze-bgp format during parseEvent.
type Event struct {
	Type      string                       // Event type from message.type: "update", "state", "open", "refresh"
	MsgID     uint64                       // Message ID from message.id (for cache-forward)
	PeerAddr  string                       // Peer address from peer.address (flat string)
	PeerASN   uint32                       // Peer ASN from peer.asn
	State     string                       // State for state events ("up", "down", "connected")
	FamilyOps map[string][]FamilyOperation // UPDATE: family → operations (from update.nlri)
	Open      *OpenInfo                    // OPEN: decoded open data
	AFI       string                       // Refresh: AFI (from refresh.afi)
	SAFI      string                       // Refresh: SAFI (from refresh.safi)
}

// FamilyOperation represents a single add or del operation for a family.
// ze-bgp JSON: {"next-hop":"192.168.1.1","action":"add","nlri":["10.0.0.0/24"]}.
type FamilyOperation struct {
	NextHop string `json:"next-hop,omitempty"` // Only for "add" operations
	Action  string `json:"action"`             // "add" or "del"
	NLRIs   []any  `json:"nlri"`               // Strings or {"prefix":"...","path-id":N}
}

// OpenInfo contains OPEN message details from ze-bgp JSON.
type OpenInfo struct {
	ASN          uint32           `json:"asn"`
	RouterID     string           `json:"router-id"`
	HoldTime     uint16           `json:"hold-time"`
	Capabilities []CapabilityInfo `json:"capabilities,omitempty"`
}

// CapabilityInfo represents a single capability from the OPEN message.
// ze-bgp JSON: {"code":1,"name":"multiprotocol","value":"ipv4/unicast"}.
type CapabilityInfo struct {
	Code  int    `json:"code"`
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// messageInfo is the internal representation of the message metadata wrapper.
type messageInfo struct {
	Type      string `json:"type"`
	ID        uint64 `json:"id,omitempty"`
	Direction string `json:"direction,omitempty"`
}

// peerFlat is the flat peer format used in ze-bgp JSON events.
// Engine always sends: {"address":"10.0.0.1","asn":65001}.
type peerFlat struct {
	Address string `json:"address"`
	ASN     uint32 `json:"asn"`
}
