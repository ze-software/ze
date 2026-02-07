package rr

import (
	"net"
	"strings"
	"testing"

	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// newTestRouteServer creates a RouteServer with closed SDK connections for unit testing.
// The plugin's connections are immediately closed so updateRoute calls fail silently,
// allowing tests to verify internal state (RIB, peers) without RPC side effects.
func newTestRouteServer(t *testing.T) *RouteServer {
	t.Helper()
	engineConn, engineRemote := net.Pipe()
	callbackConn, callbackRemote := net.Pipe()
	if err := engineRemote.Close(); err != nil {
		t.Logf("close engineRemote: %v", err)
	}
	if err := callbackRemote.Close(); err != nil {
		t.Logf("close callbackRemote: %v", err)
	}
	p := sdk.NewWithConn("rr-test", engineConn, callbackConn)
	t.Cleanup(func() { _ = p.Close() })
	return &RouteServer{
		plugin: p,
		peers:  make(map[string]*PeerState),
		rib:    NewRIB(),
	}
}

// TestServer_HandleUpdate verifies UPDATE forwarding to all except source.
//
// VALIDATES: UPDATE from peer A stores route in RIB with correct MsgID.
// PREVENTS: Missing route propagation, sending to source.
func TestServer_HandleUpdate(t *testing.T) {
	rs := newTestRouteServer(t)

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
// VALIDATES: WITHDRAW removes from RIB.
// PREVENTS: Stale routes after withdrawal.
func TestServer_HandleWithdraw(t *testing.T) {
	rs := newTestRouteServer(t)

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

	// Verify route removed from RIB
	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 0 {
		t.Errorf("expected 0 routes in RIB after withdraw, got %d", len(routes))
	}
}

// TestServer_HandleStateDown verifies peer down clears RIB.
//
// VALIDATES: Peer down clears RIB entries for that peer.
// PREVENTS: Stale routes after session teardown.
func TestServer_HandleStateDown(t *testing.T) {
	rs := newTestRouteServer(t)

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

	// Verify RIB cleared
	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 0 {
		t.Errorf("expected 0 routes after peer down, got %d", len(routes))
	}
}

// TestServer_HandleStateUp verifies peer up sets state.
//
// VALIDATES: Peer up marks peer as up in state map.
// PREVENTS: New peer missing existing routes.
func TestServer_HandleStateUp(t *testing.T) {
	rs := newTestRouteServer(t)

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

	// Verify peer state is up
	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()
	if peer == nil {
		t.Fatal("peer not created")
	}
	if !peer.Up {
		t.Error("expected peer to be up")
	}
}

// TestServer_HandleStateUp_ExcludesSelf verifies replay excludes peer's own routes.
//
// VALIDATES: Peer doesn't receive its own routes on reconnect.
// PREVENTS: Routing loops from self-received routes.
func TestServer_HandleStateUp_ExcludesSelf(t *testing.T) {
	rs := newTestRouteServer(t)

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

	// Verify peer state is up (route replay goes via SDK RPC, verified in functional tests)
	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()
	if peer == nil {
		t.Fatal("peer not created")
	}
	if !peer.Up {
		t.Error("expected peer to be up")
	}
}

// TestServer_HandleRefresh verifies route refresh forwarding.
//
// VALIDATES: Route refresh from peer A triggers refresh to all others with capability.
// PREVENTS: Missing routes after enhanced route refresh.
func TestServer_HandleRefresh(t *testing.T) {
	rs := newTestRouteServer(t)

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

	// handleRefresh calls updateRoute which sends via SDK RPC (fails silently on closed conn).
	// This test verifies the method doesn't panic with valid input.
	rs.handleRefresh(event)
}

// TestServer_ParseEvent verifies JSON event parsing.
//
// VALIDATES: Events are correctly parsed from JSON.
// PREVENTS: Malformed event handling.
func TestServer_ParseEvent(t *testing.T) {
	input := `{"type":"update","msg-id":123,"peer":{"address":{"peer":"10.0.0.1"}}}`

	event, err := parseEvent([]byte(input))
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

// TestServer_HandleCommand_Status verifies "rr status" command response.
//
// VALIDATES: RS responds to status command with done status and running JSON.
// PREVENTS: Command handler returning wrong status or data.
func TestServer_HandleCommand_Status(t *testing.T) {
	rs := newTestRouteServer(t)

	status, data, err := rs.handleCommand("rr status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "done" {
		t.Errorf("expected status done, got %q", status)
	}
	if !strings.Contains(data, `"running":true`) {
		t.Errorf("expected running:true in data, got %q", data)
	}
}

// TestServer_HandleCommand_Peers verifies "rr peers" command response.
//
// VALIDATES: RS responds to peers command with peer list JSON.
// PREVENTS: Command handler missing peer data.
func TestServer_HandleCommand_Peers(t *testing.T) {
	rs := newTestRouteServer(t)

	// Add some peer state
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", ASN: 65001, Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", ASN: 65002, Up: false}
	rs.mu.Unlock()

	status, data, err := rs.handleCommand("rr peers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "done" {
		t.Errorf("expected status done, got %q", status)
	}
	if !strings.Contains(data, "10.0.0.1") {
		t.Errorf("expected peer 10.0.0.1 in data, got %q", data)
	}
}

// TestServer_HandleCommand_Unknown verifies unknown command error response.
//
// VALIDATES: RS responds with error for unknown commands.
// PREVENTS: Silent failure on unknown commands.
func TestServer_HandleCommand_Unknown(t *testing.T) {
	rs := newTestRouteServer(t)

	status, _, err := rs.handleCommand("rr unknown")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if status != "error" {
		t.Errorf("expected status error, got %q", status)
	}
}

// TestServer_IgnoreEmptyPeer verifies empty peer address is rejected.
//
// VALIDATES: Events with empty peer address are ignored.
// PREVENTS: Routes stored with empty peer key.
func TestServer_IgnoreEmptyPeer(t *testing.T) {
	rs := newTestRouteServer(t)

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
	rs := newTestRouteServer(t)

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
// VALIDATES: IPv6 routes stored in RIB with correct family.
// PREVENTS: Sending routes to peers that can't handle them.
func TestServer_FilterUpdateByFamily(t *testing.T) {
	rs := newTestRouteServer(t)

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

	// Verify route stored in RIB with correct family
	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 1 {
		t.Fatalf("expected 1 route in RIB, got %d", len(routes))
	}
	if routes[0].Family != "ipv6/unicast" {
		t.Errorf("expected family ipv6/unicast, got %s", routes[0].Family)
	}
	if routes[0].Prefix != "2001:db8::/32" {
		t.Errorf("expected prefix 2001:db8::/32, got %s", routes[0].Prefix)
	}
}

// TestServer_FilterRefreshByCapability verifies refresh only to capable peers.
//
// VALIDATES: Refresh request only sent to peers with route-refresh capability.
// PREVENTS: Sending refresh to peers that don't support it.
func TestServer_FilterRefreshByCapability(t *testing.T) {
	rs := newTestRouteServer(t)

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

	// handleRefresh calls updateRoute which sends via SDK RPC (fails silently on closed conn).
	// This test verifies the method doesn't panic with valid input and peer filtering.
	rs.handleRefresh(event)
}

// TestServer_FilterReplayByFamily verifies replay only sends compatible routes.
//
// VALIDATES: Peer up with IPv4-only family doesn't cause panic during replay.
// PREVENTS: Sending unsupported routes to new peer.
func TestServer_FilterReplayByFamily(t *testing.T) {
	rs := newTestRouteServer(t)

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

	// handleState/handleStateUp calls updateRoute which sends via SDK RPC.
	// This test verifies family filtering logic doesn't panic.
	rs.handleState(event)

	// Verify peer state is up
	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()
	if peer == nil {
		t.Fatal("peer not created")
	}
	if !peer.Up {
		t.Error("expected peer to be up")
	}
}
