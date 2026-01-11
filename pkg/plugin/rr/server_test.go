package rr

import (
	"bytes"
	"strings"
	"testing"
)

// TestServer_HandleUpdate verifies UPDATE forwarding to all except source.
//
// VALIDATES: UPDATE from peer A forwards to all others via update-id.
// PREVENTS: Missing route propagation, sending to source.
func TestServer_HandleUpdate(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	// Set up peers
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	event := &Event{
		Type:  "update",
		MsgID: 123,
		Peer: PeerInfo{
			Address: AddressInfo{Peer: "10.0.0.1"},
		},
		Message: &MessageInfo{
			Update: &UpdateInfo{
				Announce: map[string]map[string]any{
					"ipv4/unicast": {
						"192.168.1.1": map[string]any{
							"10.0.0.0/24": map[string]any{},
						},
					},
				},
			},
		},
	}

	rs.handleUpdate(event)

	output := out.String()
	// Now forwards per-peer instead of broadcast
	if !strings.Contains(output, "peer 10.0.0.2 forward update-id 123") {
		t.Errorf("expected forward to peer 2, got %q", output)
	}
	if strings.Contains(output, "peer 10.0.0.1") {
		t.Errorf("should not forward to source peer, got %q", output)
	}

	// Verify route stored in RIB
	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 1 {
		t.Fatalf("expected 1 route in RIB, got %d", len(routes))
	}
	if routes[0].MsgID != 123 {
		t.Errorf("expected MsgID 123, got %d", routes[0].MsgID)
	}
}

// TestServer_HandleWithdraw verifies withdrawal forwarding and RIB removal.
//
// VALIDATES: WITHDRAW removes from RIB and forwards to others.
// PREVENTS: Stale routes after withdrawal.
func TestServer_HandleWithdraw(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	// Set up peers
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// First, add a route
	rs.rib.Insert("10.0.0.1", &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})

	event := &Event{
		Type:  "update",
		MsgID: 124,
		Peer: PeerInfo{
			Address: AddressInfo{Peer: "10.0.0.1"},
		},
		Message: &MessageInfo{
			Update: &UpdateInfo{
				Withdraw: map[string][]string{
					"ipv4/unicast": {"10.0.0.0/24"},
				},
			},
		},
	}

	rs.handleUpdate(event)

	output := out.String()
	if !strings.Contains(output, "peer 10.0.0.2 forward update-id 124") {
		t.Errorf("expected forward to peer 2, got %q", output)
	}

	// Verify route removed from RIB
	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 0 {
		t.Errorf("expected 0 routes in RIB after withdraw, got %d", len(routes))
	}
}

// TestServer_HandleStateDown verifies peer down sends withdrawals.
//
// VALIDATES: Peer down clears RIB and sends withdrawals to others.
// PREVENTS: Stale routes after session teardown.
func TestServer_HandleStateDown(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	// Add routes from peer
	rs.rib.Insert("10.0.0.1", &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})
	rs.rib.Insert("10.0.0.1", &Route{MsgID: 101, Family: "ipv4/unicast", Prefix: "10.0.1.0/24"})

	event := &Event{
		Type: "state",
		Peer: PeerInfo{
			Address: AddressInfo{Peer: "10.0.0.1"},
			State:   "down",
		},
	}

	rs.handleState(event)

	output := out.String()
	// Should have 2 withdraw commands using update text syntax (order may vary)
	if !strings.Contains(output, "peer !10.0.0.1 update text nlri ipv4/unicast del 10.0.0.0/24") {
		t.Errorf("missing withdraw for 10.0.0.0/24: %s", output)
	}
	if !strings.Contains(output, "peer !10.0.0.1 update text nlri ipv4/unicast del 10.0.1.0/24") {
		t.Errorf("missing withdraw for 10.0.1.0/24: %s", output)
	}

	// Verify RIB cleared
	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 0 {
		t.Errorf("expected 0 routes after peer down, got %d", len(routes))
	}
}

// TestServer_HandleStateUp verifies peer up replays routes.
//
// VALIDATES: Peer up replays all routes from other peers.
// PREVENTS: New peer missing existing routes.
func TestServer_HandleStateUp(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	// Add routes from other peers
	rs.rib.Insert("10.0.0.2", &Route{MsgID: 200, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})
	rs.rib.Insert("10.0.0.3", &Route{MsgID: 300, Family: "ipv4/unicast", Prefix: "10.0.1.0/24"})

	event := &Event{
		Type: "state",
		Peer: PeerInfo{
			Address: AddressInfo{Peer: "10.0.0.1"},
			State:   "up",
		},
	}

	rs.handleState(event)

	output := out.String()
	// Should replay routes from peers 2 and 3 to peer 1
	if !strings.Contains(output, "peer 10.0.0.1 forward update-id 200") {
		t.Errorf("missing replay of route 200: %s", output)
	}
	if !strings.Contains(output, "peer 10.0.0.1 forward update-id 300") {
		t.Errorf("missing replay of route 300: %s", output)
	}
}

// TestServer_HandleStateUp_ExcludesSelf verifies replay excludes peer's own routes.
//
// VALIDATES: Peer doesn't receive its own routes on reconnect.
// PREVENTS: Routing loops from self-received routes.
func TestServer_HandleStateUp_ExcludesSelf(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	// Add routes including from the peer coming up
	rs.rib.Insert("10.0.0.1", &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})
	rs.rib.Insert("10.0.0.2", &Route{MsgID: 200, Family: "ipv4/unicast", Prefix: "10.0.1.0/24"})

	event := &Event{
		Type: "state",
		Peer: PeerInfo{
			Address: AddressInfo{Peer: "10.0.0.1"},
			State:   "up",
		},
	}

	rs.handleState(event)

	output := out.String()
	// Should NOT replay peer's own routes
	if strings.Contains(output, "update-id 100") {
		t.Errorf("should not replay peer's own routes: %s", output)
	}
	// Should replay other peer's routes
	if !strings.Contains(output, "peer 10.0.0.1 forward update-id 200") {
		t.Errorf("missing replay of route 200: %s", output)
	}
}

// TestServer_HandleRefresh verifies route refresh forwarding.
//
// VALIDATES: Route refresh from peer A triggers refresh to all others.
// PREVENTS: Missing routes after enhanced route refresh.
func TestServer_HandleRefresh(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	// Set up peers with route-refresh capability
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:      "10.0.0.1",
		Up:           true,
		Capabilities: map[string]bool{"route-refresh": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address:      "10.0.0.2",
		Up:           true,
		Capabilities: map[string]bool{"route-refresh": true},
	}
	rs.mu.Unlock()

	event := &Event{
		Type: "refresh",
		Peer: PeerInfo{
			Address: AddressInfo{Peer: "10.0.0.1"},
		},
		AFI:  "ipv4",
		SAFI: "unicast",
	}

	rs.handleRefresh(event)

	output := out.String()
	if !strings.Contains(output, "peer 10.0.0.2 refresh ipv4/unicast") {
		t.Errorf("expected refresh to peer 2, got %q", output)
	}
}

// TestServer_Startup verifies command registration on startup.
//
// VALIDATES: RS registers capabilities and commands.
// PREVENTS: Missing route-refresh capability, unregistered commands.
func TestServer_Startup(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	rs.registerCommands()

	output := out.String()
	if !strings.Contains(output, "capability route-refresh") {
		t.Error("missing route-refresh capability")
	}
	if !strings.Contains(output, `register command "rr status"`) {
		t.Error("missing rr status command registration")
	}
	if !strings.Contains(output, `register command "rr peers"`) {
		t.Error("missing rr peers command registration")
	}
}

// TestServer_ParseEvent verifies JSON event parsing.
//
// VALIDATES: Events are correctly parsed from JSON.
// PREVENTS: Malformed event handling.
func TestServer_ParseEvent(t *testing.T) {
	input := `{"type":"update","msg-id":123,"peer":{"address":{"peer":"10.0.0.1"}}}`
	rs := NewRouteServer(strings.NewReader(""), &bytes.Buffer{})

	event, err := rs.parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if event.Type != "update" {
		t.Errorf("expected type update, got %s", event.Type)
	}
	if event.MsgID != 123 {
		t.Errorf("expected msg-id 123, got %d", event.MsgID)
	}
	if event.Peer.Address.Peer != "10.0.0.1" {
		t.Errorf("expected peer 10.0.0.1, got %s", event.Peer.Address.Peer)
	}
}

// TestServer_HandleRequest_Status verifies "rr status" command response.
//
// VALIDATES: RS responds to status command with @serial done.
// PREVENTS: Command timeout due to missing response.
func TestServer_HandleRequest_Status(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	event := &Event{
		Type:    "request",
		Serial:  "abc",
		Command: "rr status",
	}

	rs.handleRequest(event)

	output := out.String()
	if !strings.HasPrefix(output, "@abc done") {
		t.Errorf("expected @abc done response, got %q", output)
	}
	if !strings.Contains(output, `"running":true`) {
		t.Errorf("expected running:true in response, got %q", output)
	}
}

// TestServer_HandleRequest_Peers verifies "rr peers" command response.
//
// VALIDATES: RS responds to peers command with peer list.
// PREVENTS: Command timeout due to missing response.
func TestServer_HandleRequest_Peers(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	// Add some peer state
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", ASN: 65001, Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", ASN: 65002, Up: false}
	rs.mu.Unlock()

	event := &Event{
		Type:    "request",
		Serial:  "xyz",
		Command: "rr peers",
	}

	rs.handleRequest(event)

	output := out.String()
	if !strings.HasPrefix(output, "@xyz done") {
		t.Errorf("expected @xyz done response, got %q", output)
	}
	if !strings.Contains(output, "10.0.0.1") {
		t.Errorf("expected peer 10.0.0.1 in response, got %q", output)
	}
}

// TestServer_HandleRequest_Unknown verifies unknown command error response.
//
// VALIDATES: RS responds with error for unknown commands.
// PREVENTS: Silent failure on unknown commands.
func TestServer_HandleRequest_Unknown(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	event := &Event{
		Type:    "request",
		Serial:  "123",
		Command: "rr unknown",
	}

	rs.handleRequest(event)

	output := out.String()
	if !strings.HasPrefix(output, "@123 error") {
		t.Errorf("expected @123 error response, got %q", output)
	}
}

// TestServer_IgnoreEmptyPeer verifies empty peer address is rejected.
//
// VALIDATES: Events with empty peer address are ignored.
// PREVENTS: Routes stored with empty peer key.
func TestServer_IgnoreEmptyPeer(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	event := &Event{
		Type:  "update",
		MsgID: 123,
		Peer: PeerInfo{
			Address: AddressInfo{Peer: ""}, // Empty!
		},
		Message: &MessageInfo{
			Update: &UpdateInfo{
				Announce: map[string]map[string]any{
					"ipv4/unicast": {
						"192.168.1.1": map[string]any{
							"10.0.0.0/24": map[string]any{},
						},
					},
				},
			},
		},
	}

	rs.handleUpdate(event)

	// Should not forward or store
	output := out.String()
	if output != "" {
		t.Errorf("expected no output for empty peer, got %q", output)
	}

	// Should not store in RIB
	routes := rs.rib.GetPeerRoutes("")
	if len(routes) != 0 {
		t.Errorf("expected no routes for empty peer, got %d", len(routes))
	}
}

// TestServer_HandleOpen verifies OPEN event captures capabilities.
//
// VALIDATES: Peer capabilities and families are stored from OPEN.
// PREVENTS: Missing capability info for filtering.
func TestServer_HandleOpen(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	// New capability format: "<code> <name> <value>"
	event := &Event{
		Type: "open",
		Peer: PeerInfo{
			Address: AddressInfo{Peer: "10.0.0.1"},
			ASN:     ASNInfo{Peer: 65001},
		},
		Open: &OpenInfo{
			Capabilities: []string{
				"2 route-refresh",
				"65 asn4 65001",
				"1 multiprotocol ipv4/unicast",
				"1 multiprotocol ipv6/unicast",
			},
		},
	}

	rs.handleOpen(event)

	// Verify peer state updated
	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()

	if peer == nil {
		t.Fatal("peer not created")
	}
	if !peer.HasCapability("route-refresh") {
		t.Error("missing route-refresh capability")
	}
	if !peer.HasCapability("multiprotocol") {
		t.Error("missing multiprotocol capability")
	}
	// Families extracted from "1 multiprotocol ipv4/unicast"
	if !peer.SupportsFamily("ipv4/unicast") {
		t.Error("missing ipv4/unicast family")
	}
	if !peer.SupportsFamily("ipv6/unicast") {
		t.Error("missing ipv6/unicast family")
	}
}

// TestServer_FilterUpdateByFamily verifies UPDATE only forwards to compatible peers.
//
// VALIDATES: IPv6 routes only forwarded to peers supporting ipv6/unicast.
// PREVENTS: Sending routes to peers that can't handle them.
func TestServer_FilterUpdateByFamily(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	// Set up peers with different capabilities
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{ // Source peer
		Address:  "10.0.0.1",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{ // IPv4 only
		Address:  "10.0.0.2",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.3"] = &PeerState{ // IPv4 + IPv6
		Address:  "10.0.0.3",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.mu.Unlock()

	// Send IPv6 route from peer 1
	event := &Event{
		Type:  "update",
		MsgID: 100,
		Peer: PeerInfo{
			Address: AddressInfo{Peer: "10.0.0.1"},
		},
		Message: &MessageInfo{
			Update: &UpdateInfo{
				Announce: map[string]map[string]any{
					"ipv6/unicast": {
						"2001:db8::1": map[string]any{
							"2001:db8::/32": map[string]any{},
						},
					},
				},
			},
		},
	}

	rs.handleUpdate(event)

	output := out.String()
	// Should forward to peer 3 (supports IPv6), not peer 2 (IPv4 only)
	if !strings.Contains(output, "peer 10.0.0.3 forward update-id 100") {
		t.Errorf("should forward to IPv6-capable peer 3: %s", output)
	}
	if strings.Contains(output, "peer 10.0.0.2") {
		t.Errorf("should NOT forward to IPv4-only peer 2: %s", output)
	}
}

// TestServer_FilterRefreshByCapability verifies refresh only to capable peers.
//
// VALIDATES: Refresh request only sent to peers with route-refresh capability.
// PREVENTS: Sending refresh to peers that don't support it.
func TestServer_FilterRefreshByCapability(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	// Set up peers with different capabilities
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{ // Requesting peer
		Address:      "10.0.0.1",
		Up:           true,
		Capabilities: map[string]bool{"route-refresh": true},
		Families:     map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{ // No route-refresh
		Address:  "10.0.0.2",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.3"] = &PeerState{ // Has route-refresh
		Address:      "10.0.0.3",
		Up:           true,
		Capabilities: map[string]bool{"route-refresh": true},
		Families:     map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	event := &Event{
		Type: "refresh",
		Peer: PeerInfo{
			Address: AddressInfo{Peer: "10.0.0.1"},
		},
		AFI:  "ipv4",
		SAFI: "unicast",
	}

	rs.handleRefresh(event)

	output := out.String()
	// Should request from peer 3 (has route-refresh), not peer 2
	if !strings.Contains(output, "peer 10.0.0.3 refresh ipv4/unicast") {
		t.Errorf("should send refresh to capable peer 3: %s", output)
	}
	if strings.Contains(output, "peer 10.0.0.2") {
		t.Errorf("should NOT send refresh to incapable peer 2: %s", output)
	}
}

// TestServer_FilterReplayByFamily verifies replay only sends compatible routes.
//
// VALIDATES: Peer up only replays routes for families peer supports.
// PREVENTS: Sending unsupported routes to new peer.
func TestServer_FilterReplayByFamily(t *testing.T) {
	var out bytes.Buffer
	rs := NewRouteServer(strings.NewReader(""), &out)

	// Add routes from different families
	rs.rib.Insert("10.0.0.2", &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})
	rs.rib.Insert("10.0.0.2", &Route{MsgID: 200, Family: "ipv6/unicast", Prefix: "2001:db8::/32"})

	// Peer 1 comes up with IPv4 only
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:  "10.0.0.1",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address:  "10.0.0.2",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.mu.Unlock()

	event := &Event{
		Type: "state",
		Peer: PeerInfo{
			Address: AddressInfo{Peer: "10.0.0.1"},
			State:   "up",
		},
	}

	rs.handleState(event)

	output := out.String()
	// Should replay IPv4 route
	if !strings.Contains(output, "forward update-id 100") {
		t.Errorf("should replay IPv4 route: %s", output)
	}
	// Should NOT replay IPv6 route
	if strings.Contains(output, "forward update-id 200") {
		t.Errorf("should NOT replay IPv6 route to IPv4-only peer: %s", output)
	}
}
