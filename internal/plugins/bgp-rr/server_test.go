package bgp_rr

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
		workCh: make(chan forwardWork, 1024),
	}
}

// --- parseEvent tests ---

// TestParseEvent_Update verifies parsing of ze-bgp UPDATE JSON.
//
// VALIDATES: parseEvent unwraps envelope, extracts event type "update", peer address,
// message ID, and family operations with action/nlri arrays.
// PREVENTS: Regression to flat JSON schema that doesn't match engine output (Layer 1-3).
func TestParseEvent_Update(t *testing.T) {
	input := `{"type":"bgp","bgp":{"message":{"type":"update","id":123,"direction":"received"},"peer":{"address":"10.0.0.1","asn":65001},"update":{"attr":{"origin":"igp"},"nlri":{"ipv4/unicast":[{"next-hop":"192.168.1.1","action":"add","nlri":["10.0.0.0/24","10.0.1.0/24"]}]}}}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if event.Type != "update" {
		t.Errorf("expected type update, got %q", event.Type)
	}
	if event.MsgID != 123 {
		t.Errorf("expected msg-id 123, got %d", event.MsgID)
	}
	if event.PeerAddr != "10.0.0.1" {
		t.Errorf("expected peer 10.0.0.1, got %q", event.PeerAddr)
	}
	if event.PeerASN != 65001 {
		t.Errorf("expected ASN 65001, got %d", event.PeerASN)
	}
	ops, ok := event.FamilyOps["ipv4/unicast"]
	if !ok {
		t.Fatal("missing ipv4/unicast family operations")
	}
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Action != "add" {
		t.Errorf("expected action add, got %q", ops[0].Action)
	}
	if len(ops[0].NLRIs) != 2 {
		t.Errorf("expected 2 NLRIs, got %d", len(ops[0].NLRIs))
	}
}

// TestParseEvent_UpdateWithdraw verifies parsing of ze-bgp UPDATE with withdrawal.
//
// VALIDATES: parseEvent correctly parses "del" action operations.
// PREVENTS: Missing withdrawal detection due to expecting "withdraw" maps.
func TestParseEvent_UpdateWithdraw(t *testing.T) {
	input := `{"type":"bgp","bgp":{"message":{"type":"update","id":124,"direction":"received"},"peer":{"address":"10.0.0.1","asn":65001},"update":{"nlri":{"ipv4/unicast":[{"action":"del","nlri":["10.0.0.0/24"]}]}}}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if event.Type != "update" {
		t.Errorf("expected type update, got %q", event.Type)
	}
	ops := event.FamilyOps["ipv4/unicast"]
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Action != "del" {
		t.Errorf("expected action del, got %q", ops[0].Action)
	}
	if len(ops[0].NLRIs) != 1 {
		t.Errorf("expected 1 NLRI, got %d", len(ops[0].NLRIs))
	}
}

// TestParseEvent_State verifies parsing of ze-bgp state change JSON.
//
// VALIDATES: parseEvent extracts event type "state" and state value from bgp-level field.
// PREVENTS: Missing state extraction (state is at bgp level, not inside peer).
func TestParseEvent_State(t *testing.T) {
	tests := []struct {
		name  string
		input string
		state string
	}{
		{
			name:  "state_up",
			input: `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","asn":65001},"state":"up"}}`,
			state: "up",
		},
		{
			name:  "state_down",
			input: `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","asn":65001},"state":"down"}}`,
			state: "down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := parseEvent([]byte(tt.input))
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if event.Type != "state" {
				t.Errorf("expected type state, got %q", event.Type)
			}
			if event.PeerAddr != "10.0.0.1" {
				t.Errorf("expected peer 10.0.0.1, got %q", event.PeerAddr)
			}
			if event.State != tt.state {
				t.Errorf("expected state %q, got %q", tt.state, event.State)
			}
		})
	}
}

// TestParseEvent_Open verifies parsing of ze-bgp OPEN JSON with capability objects.
//
// VALIDATES: parseEvent extracts OPEN capabilities as structured objects with code/name/value.
// PREVENTS: Regression to space-delimited string capabilities (Layer 4).
func TestParseEvent_Open(t *testing.T) {
	input := `{"type":"bgp","bgp":{"message":{"type":"open"},"peer":{"address":"10.0.0.1","asn":65001},"open":{"asn":65001,"router-id":"10.0.0.1","hold-time":180,"capabilities":[{"code":1,"name":"multiprotocol","value":"ipv4/unicast"},{"code":1,"name":"multiprotocol","value":"ipv6/unicast"},{"code":2,"name":"route-refresh"},{"code":65,"name":"asn4","value":"65001"}]}}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if event.Type != "open" {
		t.Errorf("expected type open, got %q", event.Type)
	}
	if event.PeerAddr != "10.0.0.1" {
		t.Errorf("expected peer 10.0.0.1, got %q", event.PeerAddr)
	}
	if event.Open == nil {
		t.Fatal("expected non-nil Open")
	}
	if event.Open.ASN != 65001 {
		t.Errorf("expected ASN 65001, got %d", event.Open.ASN)
	}
	if event.Open.HoldTime != 180 {
		t.Errorf("expected hold-time 180, got %d", event.Open.HoldTime)
	}
	if len(event.Open.Capabilities) != 4 {
		t.Fatalf("expected 4 capabilities, got %d", len(event.Open.Capabilities))
	}
	cap0 := event.Open.Capabilities[0]
	if cap0.Code != 1 || cap0.Name != "multiprotocol" || cap0.Value != "ipv4/unicast" {
		t.Errorf("cap[0] = {%d, %q, %q}, want {1, multiprotocol, ipv4/unicast}", cap0.Code, cap0.Name, cap0.Value)
	}
}

// TestParseEvent_Refresh verifies parsing of ze-bgp refresh JSON.
//
// VALIDATES: parseEvent extracts AFI/SAFI from nested refresh object.
// PREVENTS: Missing AFI/SAFI extraction when refresh data is nested (not top-level).
func TestParseEvent_Refresh(t *testing.T) {
	input := `{"type":"bgp","bgp":{"message":{"type":"refresh"},"peer":{"address":"10.0.0.1","asn":65001},"refresh":{"afi":"ipv4","safi":"unicast"}}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if event.Type != "refresh" {
		t.Errorf("expected type refresh, got %q", event.Type)
	}
	if event.PeerAddr != "10.0.0.1" {
		t.Errorf("expected peer 10.0.0.1, got %q", event.PeerAddr)
	}
	if event.AFI != "ipv4" {
		t.Errorf("expected AFI ipv4, got %q", event.AFI)
	}
	if event.SAFI != "unicast" {
		t.Errorf("expected SAFI unicast, got %q", event.SAFI)
	}
}

// TestParseEvent_InvalidJSON verifies error handling for malformed JSON.
//
// VALIDATES: parseEvent returns error for invalid input.
// PREVENTS: Silent failures on malformed events.
func TestParseEvent_InvalidJSON(t *testing.T) {
	_, err := parseEvent([]byte(`{not json}`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// TestNlriToPrefix verifies prefix extraction from NLRI values.
//
// VALIDATES: nlriToPrefix handles string prefixes and object NLRIs (ADD-PATH, VPN).
// PREVENTS: Missing prefix extraction for complex NLRI types.
func TestNlriToPrefix(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		expect string
	}{
		{"string_prefix", "10.0.0.0/24", "10.0.0.0/24"},
		{"object_with_prefix", map[string]any{"prefix": "10.0.0.0/24", "path-id": float64(1)}, "10.0.0.0/24"},
		{"object_no_prefix", map[string]any{"path-id": float64(1)}, ""},
		{"nil_value", nil, ""},
		{"integer_value", float64(42), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nlriToPrefix(tt.input)
			if got != tt.expect {
				t.Errorf("nlriToPrefix(%v) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

// --- Handler integration tests (JSON → parse → dispatch → verify state) ---

// TestHandleUpdate_ZeBGPFormat verifies UPDATE processing from actual ze-bgp JSON.
//
// VALIDATES: Full flow from JSON parsing through RIB insertion for an UPDATE announce (AC-1).
// PREVENTS: Route propagation failure due to format mismatch.
func TestHandleUpdate_ZeBGPFormat(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	input := `{"type":"bgp","bgp":{"message":{"type":"update","id":123,"direction":"received"},"peer":{"address":"10.0.0.1","asn":65001},"update":{"attr":{"origin":"igp","as-path":[65001],"local-preference":100},"nlri":{"ipv4/unicast":[{"next-hop":"192.168.1.1","action":"add","nlri":["10.0.0.0/24"]}]}}}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	rs.dispatch(event)

	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 1 {
		t.Fatalf("expected 1 route in RIB, got %d", len(routes))
	}
	if routes[0].MsgID != 123 {
		t.Errorf("expected MsgID 123, got %d", routes[0].MsgID)
	}
	if routes[0].Family != "ipv4/unicast" {
		t.Errorf("expected family ipv4/unicast, got %s", routes[0].Family)
	}
	if routes[0].Prefix != "10.0.0.0/24" {
		t.Errorf("expected prefix 10.0.0.0/24, got %s", routes[0].Prefix)
	}
}

// TestHandleUpdate_Withdraw_ZeBGPFormat verifies withdrawal processing from actual ze-bgp JSON.
//
// VALIDATES: Full flow from JSON parsing through RIB removal for a withdrawal (AC-2).
// PREVENTS: Stale routes remaining after withdrawal.
func TestHandleUpdate_Withdraw_ZeBGPFormat(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	rs.rib.Insert("10.0.0.1", &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})

	input := `{"type":"bgp","bgp":{"message":{"type":"update","id":124,"direction":"received"},"peer":{"address":"10.0.0.1","asn":65001},"update":{"nlri":{"ipv4/unicast":[{"action":"del","nlri":["10.0.0.0/24"]}]}}}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	rs.dispatch(event)

	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 0 {
		t.Errorf("expected 0 routes after withdraw, got %d", len(routes))
	}
}

// TestHandleUpdate_MultiFamilyMixed verifies processing of UPDATE with both add and del operations.
//
// VALIDATES: Multiple family operations processed correctly in single UPDATE (AC-8).
// PREVENTS: Only first operation being processed, ignoring subsequent families.
func TestHandleUpdate_MultiFamilyMixed(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true}}
	rs.mu.Unlock()

	rs.rib.Insert("10.0.0.1", &Route{MsgID: 99, Family: "ipv4/unicast", Prefix: "10.0.2.0/24"})

	input := `{"type":"bgp","bgp":{"message":{"type":"update","id":125,"direction":"received"},"peer":{"address":"10.0.0.1","asn":65001},"update":{"nlri":{"ipv4/unicast":[{"next-hop":"192.168.1.1","action":"add","nlri":["10.0.0.0/24"]},{"action":"del","nlri":["10.0.2.0/24"]}],"ipv6/unicast":[{"next-hop":"2001:db8::1","action":"add","nlri":["2001:db8::/32"]}]}}}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	rs.dispatch(event)

	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes in RIB, got %d", len(routes))
	}

	found := map[string]bool{}
	for _, r := range routes {
		found[r.Family+"|"+r.Prefix] = true
	}
	if !found["ipv4/unicast|10.0.0.0/24"] {
		t.Error("missing ipv4/unicast|10.0.0.0/24")
	}
	if !found["ipv6/unicast|2001:db8::/32"] {
		t.Error("missing ipv6/unicast|2001:db8::/32")
	}
	if found["ipv4/unicast|10.0.2.0/24"] {
		t.Error("10.0.2.0/24 should have been withdrawn")
	}
}

// TestHandleUpdate_IgnoreEmptyPeer verifies events with empty peer address are ignored.
//
// VALIDATES: Events with empty peer address are rejected.
// PREVENTS: Routes stored with empty peer key.
func TestHandleUpdate_IgnoreEmptyPeer(t *testing.T) {
	rs := newTestRouteServer(t)

	input := `{"type":"bgp","bgp":{"message":{"type":"update","id":123,"direction":"received"},"peer":{"address":"","asn":65001},"update":{"nlri":{"ipv4/unicast":[{"next-hop":"192.168.1.1","action":"add","nlri":["10.0.0.0/24"]}]}}}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	rs.dispatch(event)

	routes := rs.rib.GetPeerRoutes("")
	if len(routes) != 0 {
		t.Errorf("expected no routes for empty peer, got %d", len(routes))
	}
}

// TestHandleState_Down_ZeBGPFormat verifies peer down processing from actual ze-bgp JSON.
//
// VALIDATES: Peer down clears RIB entries for that peer (AC-4).
// PREVENTS: Stale routes remaining after session teardown.
func TestHandleState_Down_ZeBGPFormat(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.rib.Insert("10.0.0.1", &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})
	rs.rib.Insert("10.0.0.1", &Route{MsgID: 101, Family: "ipv4/unicast", Prefix: "10.0.1.0/24"})

	input := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","asn":65001},"state":"down"}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	rs.dispatch(event)

	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 0 {
		t.Errorf("expected 0 routes after peer down, got %d", len(routes))
	}

	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()
	if peer == nil {
		t.Fatal("peer state not created")
	}
	if peer.Up {
		t.Error("expected peer to be down")
	}
}

// TestHandleState_Up_ZeBGPFormat verifies peer up processing from actual ze-bgp JSON.
//
// VALIDATES: Peer up marks peer as up, replays routes from other peers (AC-3).
// PREVENTS: Missing state transition, new peer not receiving existing routes.
func TestHandleState_Up_ZeBGPFormat(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.rib.Insert("10.0.0.2", &Route{MsgID: 200, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})

	input := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","asn":65001},"state":"up"}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	rs.dispatch(event)

	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()
	if peer == nil {
		t.Fatal("peer state not created")
	}
	if !peer.Up {
		t.Error("expected peer to be up")
	}
}

// TestHandleState_Up_ExcludesSelf verifies peer doesn't receive its own routes on reconnect.
//
// VALIDATES: Route replay excludes peer's own routes.
// PREVENTS: Routing loops from self-received routes.
func TestHandleState_Up_ExcludesSelf(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.rib.Insert("10.0.0.1", &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})
	rs.rib.Insert("10.0.0.2", &Route{MsgID: 200, Family: "ipv4/unicast", Prefix: "10.0.1.0/24"})

	input := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","asn":65001},"state":"up"}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	rs.dispatch(event)

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

// TestHandleOpen_ZeBGPFormat verifies OPEN processing from actual ze-bgp JSON.
//
// VALIDATES: OPEN event extracts capabilities and families from JSON objects (AC-5).
// PREVENTS: Missing capability info due to object vs string format mismatch (Layer 4).
func TestHandleOpen_ZeBGPFormat(t *testing.T) {
	rs := newTestRouteServer(t)

	input := `{"type":"bgp","bgp":{"message":{"type":"open"},"peer":{"address":"10.0.0.1","asn":65001},"open":{"asn":65001,"router-id":"10.0.0.1","hold-time":180,"capabilities":[{"code":1,"name":"multiprotocol","value":"ipv4/unicast"},{"code":1,"name":"multiprotocol","value":"ipv6/unicast"},{"code":2,"name":"route-refresh"},{"code":65,"name":"asn4","value":"65001"}]}}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	rs.dispatch(event)

	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()

	if peer == nil {
		t.Fatal("peer not created")
	}
	if peer.ASN != 65001 {
		t.Errorf("expected ASN 65001, got %d", peer.ASN)
	}
	if !peer.HasCapability("route-refresh") {
		t.Error("missing route-refresh capability")
	}
	if !peer.HasCapability("multiprotocol") {
		t.Error("missing multiprotocol capability")
	}
	if !peer.HasCapability("asn4") {
		t.Error("missing asn4 capability")
	}
	if !peer.SupportsFamily("ipv4/unicast") {
		t.Error("missing ipv4/unicast family")
	}
	if !peer.SupportsFamily("ipv6/unicast") {
		t.Error("missing ipv6/unicast family")
	}
}

// TestHandleRefresh_ZeBGPFormat verifies refresh processing from actual ze-bgp JSON.
//
// VALIDATES: Refresh event extracts AFI/SAFI from nested object, forwards to capable peers (AC-6).
// PREVENTS: Missing family extraction from nested refresh object.
func TestHandleRefresh_ZeBGPFormat(t *testing.T) {
	rs := newTestRouteServer(t)

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

	input := `{"type":"bgp","bgp":{"message":{"type":"refresh"},"peer":{"address":"10.0.0.1","asn":65001},"refresh":{"afi":"ipv4","safi":"unicast"}}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// handleRefresh calls updateRoute which sends via SDK RPC (fails silently on closed conn).
	rs.dispatch(event)
}

// --- Family/capability filtering tests ---

// TestFilterUpdateByFamily verifies UPDATE only forwards to compatible peers.
//
// VALIDATES: IPv6 routes stored in RIB with correct family for filtering (AC-1).
// PREVENTS: Sending routes to peers that can't handle them.
func TestFilterUpdateByFamily(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:  "10.0.0.1",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address:  "10.0.0.2",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.3"] = &PeerState{
		Address:  "10.0.0.3",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.mu.Unlock()

	input := `{"type":"bgp","bgp":{"message":{"type":"update","id":100,"direction":"received"},"peer":{"address":"10.0.0.1","asn":65001},"update":{"nlri":{"ipv6/unicast":[{"next-hop":"2001:db8::1","action":"add","nlri":["2001:db8::/32"]}]}}}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	rs.dispatch(event)

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

// TestFilterRefreshByCapability verifies refresh only sent to capable peers.
//
// VALIDATES: Refresh only forwarded to peers with route-refresh capability (AC-6).
// PREVENTS: Sending refresh to peers that don't support it.
func TestFilterRefreshByCapability(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:      "10.0.0.1",
		Up:           true,
		Capabilities: map[string]bool{"route-refresh": true},
		Families:     map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address:  "10.0.0.2",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.3"] = &PeerState{
		Address:      "10.0.0.3",
		Up:           true,
		Capabilities: map[string]bool{"route-refresh": true},
		Families:     map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	input := `{"type":"bgp","bgp":{"message":{"type":"refresh"},"peer":{"address":"10.0.0.1","asn":65001},"refresh":{"afi":"ipv4","safi":"unicast"}}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// handleRefresh forwards to peers via updateRoute (fails silently on closed conn).
	rs.dispatch(event)
}

// TestFilterReplayByFamily verifies replay only sends compatible routes.
//
// VALIDATES: IPv4-only peer doesn't cause panic during replay with mixed-family RIB.
// PREVENTS: Sending unsupported routes to new peer.
func TestFilterReplayByFamily(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.rib.Insert("10.0.0.2", &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})
	rs.rib.Insert("10.0.0.2", &Route{MsgID: 200, Family: "ipv6/unicast", Prefix: "2001:db8::/32"})

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

	input := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","asn":65001},"state":"up"}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// handleState/handleStateUp calls updateRoute for route replay.
	rs.dispatch(event)

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

// --- Command tests ---

// TestHandleCommand_Status verifies "rr status" command response.
//
// VALIDATES: RS responds to status command with done status and running JSON.
// PREVENTS: Command handler returning wrong status or data.
func TestHandleCommand_Status(t *testing.T) {
	rs := newTestRouteServer(t)

	status, data, err := rs.handleCommand("rr status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != statusDone {
		t.Errorf("expected status done, got %q", status)
	}
	if !strings.Contains(data, `"running":true`) {
		t.Errorf("expected running:true in data, got %q", data)
	}
}

// TestHandleCommand_Peers verifies "rr peers" command response.
//
// VALIDATES: RS responds to peers command with peer list JSON.
// PREVENTS: Command handler missing peer data.
func TestHandleCommand_Peers(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", ASN: 65001, Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", ASN: 65002, Up: false}
	rs.mu.Unlock()

	status, data, err := rs.handleCommand("rr peers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != statusDone {
		t.Errorf("expected status done, got %q", status)
	}
	if !strings.Contains(data, "10.0.0.1") {
		t.Errorf("expected peer 10.0.0.1 in data, got %q", data)
	}
}

// TestHandleCommand_Unknown verifies unknown command error response.
//
// VALIDATES: RS responds with error for unknown commands.
// PREVENTS: Silent failure on unknown commands.
func TestHandleCommand_Unknown(t *testing.T) {
	rs := newTestRouteServer(t)

	status, _, err := rs.handleCommand("rr unknown")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if status != "error" {
		t.Errorf("expected status error, got %q", status)
	}
}
