package rr

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

// RouteServer implements a BGP Route Server API plugin.
// It forwards all UPDATEs to all peers except the source (forward-all model).
type RouteServer struct {
	input  *bufio.Scanner
	output io.Writer
	peers  map[string]*PeerState
	rib    *RIB
	mu     sync.RWMutex
	serial int // Command serial number
}

// MaxLineSize is the maximum size of a single JSON event line (1MB).
// Large UPDATEs with many NLRIs can exceed the default 64KB scanner limit.
const MaxLineSize = 1024 * 1024

// NewRouteServer creates a new Route Server.
func NewRouteServer(input io.Reader, output io.Writer) *RouteServer {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, MaxLineSize), MaxLineSize)
	return &RouteServer{
		input:  scanner,
		output: output,
		peers:  make(map[string]*PeerState),
		rib:    NewRIB(),
	}
}

// Run starts the Route Server event loop.
func (rs *RouteServer) Run() int {
	rs.registerCommands()

	for rs.input.Scan() {
		line := rs.input.Bytes()
		if len(line) == 0 {
			continue
		}

		event, err := rs.parseEvent(line)
		if err != nil {
			// Log error but continue
			continue
		}

		rs.dispatch(event)
	}

	// Check for scanner errors (not EOF)
	if err := rs.input.Err(); err != nil {
		return 1
	}

	return 0
}

// registerCommands outputs startup commands.
func (rs *RouteServer) registerCommands() {
	rs.sendCommand("capability route-refresh")
	rs.sendCommand(`register command "rr status" description "Show RS status"`)
	rs.sendCommand(`register command "rr peers" description "Show peer states"`)
}

// sendCommand sends a numbered command to ze.
func (rs *RouteServer) sendCommand(cmd string) {
	rs.serial++
	_, _ = fmt.Fprintf(rs.output, "#%d %s\n", rs.serial, cmd)
}

// send sends raw output to ze.
func (rs *RouteServer) send(format string, args ...any) {
	_, _ = fmt.Fprintf(rs.output, format+"\n", args...)
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
	case "request":
		rs.handleRequest(event)
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

		rs.send("bgp cache %d forward %s", msgID, addr)
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
		rs.send("peer !%s update text nlri %s del %s", peerAddr, route.Family, route.Prefix)
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
			rs.send("bgp cache %d forward %s", route.MsgID, peerAddr)
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

		rs.send("peer %s refresh %s", addr, family)
	}
}

// handleRequest processes command requests from ze.
func (rs *RouteServer) handleRequest(event *Event) {
	serial := event.Serial
	command := event.Command

	switch command {
	case "rr status":
		rs.respondDone(serial, `{"running":true}`)
	case "rr peers":
		rs.respondDone(serial, rs.peersJSON())
	default:
		rs.respondError(serial, "unknown command: "+command)
	}
}

// respondDone sends a successful response.
func (rs *RouteServer) respondDone(serial, data string) {
	_, _ = fmt.Fprintf(rs.output, "@%s done %s\n", serial, data)
}

// respondError sends an error response.
func (rs *RouteServer) respondError(serial, message string) {
	_, _ = fmt.Fprintf(rs.output, "@%s error %q\n", serial, message)
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
func (rs *RouteServer) parseEvent(data []byte) (*Event, error) {
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
