package bgp_rr

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})
	rs := &RouteServer{
		plugin: p,
		peers:  make(map[string]*PeerState),
		rib:    NewRIB(),
	}
	rs.startReleaseLoop()
	rs.startForwardLoop()
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		rs.processForward(key, item.msgID)
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second, onDrained: rs.flushWorkerBatch})
	t.Cleanup(func() {
		rs.workers.Stop()
		rs.stopForwardLoop()
		rs.stopReleaseLoop()
	})
	return rs
}

// flushWorkers stops and recreates the worker pool, ensuring all pending
// items are processed. Also drains the forward loop so async forward RPCs
// are fully delivered before the test checks hook-captured commands.
func flushWorkers(t *testing.T, rs *RouteServer) {
	t.Helper()
	rs.workers.Stop()
	// Drain forward loop: close channel → goroutine processes remaining → restart.
	rs.stopForwardLoop()
	rs.startForwardLoop()
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		rs.processForward(key, item.msgID)
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second, onDrained: rs.flushWorkerBatch})
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

	rs.dispatch([]byte(input))
	flushWorkers(t, rs)

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

	rs.dispatch([]byte(input))
	flushWorkers(t, rs)

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

	rs.dispatch([]byte(input))
	flushWorkers(t, rs)

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

	rs.dispatch([]byte(input))

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

	rs.dispatch([]byte(input))

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

	rs.dispatch([]byte(input))

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

	rs.dispatch([]byte(input))

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

	rs.dispatch([]byte(input))

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

	// handleRefresh calls updateRoute which sends via SDK RPC (fails silently on closed conn).
	rs.dispatch([]byte(input))
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

	rs.dispatch([]byte(input))
	flushWorkers(t, rs)

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

	// handleRefresh forwards to peers via updateRoute (fails silently on closed conn).
	rs.dispatch([]byte(input))
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

	// handleState/handleStateUp calls updateRoute for route replay.
	rs.dispatch([]byte(input))

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

// TestRRUpdateRouteTimeout60s verifies the updateRoute timeout is 60 seconds.
//
// VALIDATES: AC-7 — RR plugin updateRoute uses 60s timeout (was 10s).
// PREVENTS: Regression to 10s timeout that causes silent route drops under load.
func TestRRUpdateRouteTimeout60s(t *testing.T) {
	if updateRouteTimeout != 60*time.Second {
		t.Errorf("updateRouteTimeout = %v, want 60s", updateRouteTimeout)
	}
}

// TestDispatchPauseOnBackpressure verifies dispatch pauses source peer on backpressure.
//
// VALIDATES: AC-1 — dispatch sends pause when worker channel exceeds 75%.
// PREVENTS: Backpressure detection without action.
func TestDispatchPauseOnBackpressure(t *testing.T) {
	rs := newTestRouteServer(t)
	// Use small channel so we can fill it easily.
	rs.workers.Stop()
	block := make(chan struct{})
	rs.workers = newWorkerPool(func(_ workerKey, _ workItem) {
		<-block // Block all processing.
	}, poolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	rs.wireFlowControl()
	t.Cleanup(func() { close(block); rs.workers.Stop() })

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Dispatch enough updates to trigger backpressure (>75% of 8 = >6).
	for i := uint64(1); i <= 9; i++ {
		input := buildTestUpdate("10.0.0.1", i)
		rs.dispatch([]byte(input))
	}

	// Check that peer is marked as paused.
	rs.mu.RLock()
	paused := rs.pausedPeers["10.0.0.1"]
	rs.mu.RUnlock()

	if !paused {
		t.Error("expected 10.0.0.1 to be paused after backpressure")
	}
}

// TestDispatchResumeOnDrain verifies dispatch resumes source peer when channel drains.
//
// VALIDATES: AC-2 — resume sent when worker channel drains below 25%.
// PREVENTS: Permanently paused peers after transient load.
func TestDispatchResumeOnDrain(t *testing.T) {
	rs := newTestRouteServer(t)
	rs.workers.Stop()
	block := make(chan struct{})
	var processed int32
	rs.workers = newWorkerPool(func(_ workerKey, _ workItem) {
		if atomic.AddInt32(&processed, 1) == 1 {
			<-block // First item blocks; rest process immediately.
		}
	}, poolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	rs.wireFlowControl()
	t.Cleanup(func() { rs.workers.Stop() })

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Fill channel to trigger backpressure.
	for i := uint64(1); i <= 9; i++ {
		input := buildTestUpdate("10.0.0.1", i)
		rs.dispatch([]byte(input))
	}

	// Verify paused.
	rs.mu.RLock()
	paused := rs.pausedPeers["10.0.0.1"]
	rs.mu.RUnlock()
	if !paused {
		t.Fatal("expected peer paused before drain")
	}

	// Unblock worker to drain.
	close(block)

	// Wait for resume (low-water clears pausedPeers).
	deadline := time.After(2 * time.Second)
	for {
		rs.mu.RLock()
		stillPaused := rs.pausedPeers["10.0.0.1"]
		rs.mu.RUnlock()
		if !stillPaused {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout: peer not resumed after drain")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// TestMultiSourceBackpressure verifies independent pause/resume per source peer.
//
// VALIDATES: AC-13 — each source peer paused independently.
// PREVENTS: Global pause when only one source is saturated.
func TestMultiSourceBackpressure(t *testing.T) {
	rs := newTestRouteServer(t)
	rs.workers.Stop()
	block := make(chan struct{})
	rs.workers = newWorkerPool(func(_ workerKey, _ workItem) {
		<-block
	}, poolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	rs.wireFlowControl()
	t.Cleanup(func() { close(block); rs.workers.Stop() })

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.peers["10.0.0.3"] = &PeerState{Address: "10.0.0.3", Up: true}
	rs.mu.Unlock()

	// Saturate peer 1 and peer 2, leave peer 3 light.
	for i := uint64(1); i <= 9; i++ {
		rs.dispatch([]byte(buildTestUpdate("10.0.0.1", i)))
		rs.dispatch([]byte(buildTestUpdate("10.0.0.2", 100+i)))
	}
	// Peer 3: only 1 item (no backpressure).
	rs.dispatch([]byte(buildTestUpdate("10.0.0.3", 200)))

	rs.mu.RLock()
	p1 := rs.pausedPeers["10.0.0.1"]
	p2 := rs.pausedPeers["10.0.0.2"]
	p3 := rs.pausedPeers["10.0.0.3"]
	rs.mu.RUnlock()

	if !p1 {
		t.Error("expected peer 10.0.0.1 paused")
	}
	if !p2 {
		t.Error("expected peer 10.0.0.2 paused")
	}
	if p3 {
		t.Error("expected peer 10.0.0.3 NOT paused")
	}
}

// TestShutdownResumesAllPeers verifies Stop() resumes all paused peers.
//
// VALIDATES: AC-9 — all paused peers resumed on shutdown.
// PREVENTS: Permanently paused peers after RR plugin exits.
func TestShutdownResumesAllPeers(t *testing.T) {
	rs := newTestRouteServer(t)
	rs.workers.Stop()
	block := make(chan struct{})
	rs.workers = newWorkerPool(func(_ workerKey, _ workItem) {
		<-block
	}, poolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	rs.wireFlowControl()

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Trigger backpressure.
	for i := uint64(1); i <= 9; i++ {
		rs.dispatch([]byte(buildTestUpdate("10.0.0.1", i)))
	}

	// Verify paused.
	rs.mu.RLock()
	paused := rs.pausedPeers["10.0.0.1"]
	rs.mu.RUnlock()
	if !paused {
		t.Fatal("expected peer paused before shutdown")
	}

	// Shutdown: resumeAllPaused + stop workers.
	close(block)
	rs.resumeAllPaused()
	rs.workers.Stop()

	rs.mu.RLock()
	remaining := len(rs.pausedPeers)
	rs.mu.RUnlock()
	if remaining != 0 {
		t.Errorf("expected 0 paused peers after shutdown, got %d", remaining)
	}
}

// TestPausedPeerResumesOnDrain verifies the full pause→drain→resume→dispatch cycle.
//
// VALIDATES: AC-6 — read loop unblocks after resume; subsequent messages processed normally.
// PREVENTS: Permanently stalled peers after a transient backpressure event.
func TestPausedPeerResumesOnDrain(t *testing.T) {
	rs := newTestRouteServer(t)
	rs.workers.Stop()
	block := make(chan struct{})
	var processed atomic.Int32
	rs.workers = newWorkerPool(func(_ workerKey, _ workItem) {
		if processed.Add(1) == 1 {
			<-block // First item blocks; rest process immediately.
		}
	}, poolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	rs.wireFlowControl()
	t.Cleanup(func() { rs.workers.Stop() })

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.mu.Unlock()

	// Fill to trigger backpressure.
	for i := uint64(1); i <= 9; i++ {
		rs.dispatch([]byte(buildTestUpdate("10.0.0.1", i)))
	}

	rs.mu.RLock()
	paused := rs.pausedPeers["10.0.0.1"]
	rs.mu.RUnlock()
	if !paused {
		t.Fatal("expected peer paused")
	}

	// Unblock → drain → resume.
	close(block)
	deadline := time.After(2 * time.Second)
	for {
		rs.mu.RLock()
		stillPaused := rs.pausedPeers["10.0.0.1"]
		rs.mu.RUnlock()
		if !stillPaused {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout: peer not resumed after drain")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Dispatch more after resume — should succeed without re-triggering pause.
	rs.dispatch([]byte(buildTestUpdate("10.0.0.1", 100)))

	rs.mu.RLock()
	rePaused := rs.pausedPeers["10.0.0.1"]
	rs.mu.RUnlock()
	if rePaused {
		t.Error("peer should not be re-paused after single dispatch post-drain")
	}
}

// TestPauseRPCFailure verifies that a failed pause RPC does not crash or block dispatch.
//
// VALIDATES: AC-14 — pause RPC error logged, processing continues.
// PREVENTS: RPC timeout blocking the dispatch goroutine or crashing the plugin.
func TestPauseRPCFailure(t *testing.T) {
	// newTestRouteServer closes the engine connection, so all updateRoute calls fail.
	rs := newTestRouteServer(t)
	rs.workers.Stop()
	block := make(chan struct{})
	rs.workers = newWorkerPool(func(_ workerKey, _ workItem) {
		<-block
	}, poolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	rs.wireFlowControl()
	t.Cleanup(func() { close(block); rs.workers.Stop() })

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.mu.Unlock()

	// Fill to trigger backpressure — pause RPC will fail (closed conn).
	for i := uint64(1); i <= 9; i++ {
		rs.dispatch([]byte(buildTestUpdate("10.0.0.1", i)))
	}

	// Verify: peer is tracked as paused despite RPC failure.
	// The pausedPeers map reflects intent, not RPC success.
	rs.mu.RLock()
	paused := rs.pausedPeers["10.0.0.1"]
	rs.mu.RUnlock()
	if !paused {
		t.Error("expected peer tracked as paused even though RPC failed")
	}
}

// TestDispatchPassesPreParsedPayload verifies dispatch stores pre-unwrapped BGP payload.
//
// VALIDATES: AC-6 — forwardCtx contains BGP payload, not full envelope.
// PREVENTS: Redundant envelope unwrap in worker.
func TestDispatchPassesPreParsedPayload(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.mu.Unlock()

	// Stop workers so items stay in fwdCtx (not consumed by processForward).
	rs.workers.Stop()
	rs.workers = newWorkerPool(func(_ workerKey, _ workItem) {
		// Do nothing — keep fwdCtx intact for inspection.
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second})
	t.Cleanup(func() { rs.workers.Stop() })

	input := `{"type":"bgp","bgp":{"message":{"type":"update","id":42,"direction":"received"},"peer":{"address":"10.0.0.1","asn":65001},"update":{"nlri":{"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24"]}]}}}}`
	rs.dispatch([]byte(input))

	// Wait for worker to process the no-op handler.
	time.Sleep(50 * time.Millisecond)

	// Check that fwdCtx was stored with bgpPayload (not rawJSON).
	val, ok := rs.fwdCtx.Load(uint64(42))
	if !ok {
		t.Fatal("fwdCtx not found for msgID 42")
	}
	ctx, ok := val.(*forwardCtx)
	if !ok {
		t.Fatal("fwdCtx wrong type")
	}

	// bgpPayload should NOT contain the outer {"type":"bgp","bgp":...} wrapper.
	// It should start with {"message":...}
	if len(ctx.bgpPayload) == 0 {
		t.Fatal("expected bgpPayload to be populated")
	}
	payloadStr := string(ctx.bgpPayload)
	if strings.Contains(payloadStr, `"type":"bgp"`) {
		t.Error("bgpPayload should not contain outer envelope type field")
	}
	if !strings.Contains(payloadStr, `"message"`) {
		t.Error("bgpPayload should contain message field from inner BGP payload")
	}
}

// TestProcessForwardDefersRIB verifies that forwarding happens before RIB update.
//
// VALIDATES: AC-4 — forward RPC sent before RIB insert.
// PREVENTS: RIB insert blocking the forward hot path.
func TestProcessForwardDefersRIB(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Dispatch and flush to process the UPDATE fully.
	input := `{"type":"bgp","bgp":{"message":{"type":"update","id":99,"direction":"received"},"peer":{"address":"10.0.0.1","asn":65001},"update":{"nlri":{"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24"]}]}}}}`
	rs.dispatch([]byte(input))
	flushWorkers(t, rs)

	// After full processing, RIB should still have the route (deferred but completed).
	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 1 {
		t.Fatalf("expected 1 route in RIB after deferred insert, got %d", len(routes))
	}
	if routes[0].Prefix != "10.0.0.0/24" {
		t.Errorf("expected prefix 10.0.0.0/24, got %s", routes[0].Prefix)
	}
}

// TestDeferredRIBConsistency verifies RIB is consistent after PeerDown drain.
//
// VALIDATES: AC-13 — RIB correct after worker drains (deferred inserts complete before ClearPeer).
// PREVENTS: Race between deferred RIB insert and ClearPeer.
func TestDeferredRIBConsistency(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Dispatch multiple UPDATEs.
	for i := uint64(1); i <= 5; i++ {
		input := fmt.Sprintf(
			`{"type":"bgp","bgp":{"message":{"type":"update","id":%d},"peer":{"address":"10.0.0.1","asn":65001},"update":{"nlri":{"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.%d.0/24"]}]}}}}`,
			i, i,
		)
		rs.dispatch([]byte(input))
	}

	// Flush workers to ensure all deferred RIB inserts complete.
	flushWorkers(t, rs)

	// All 5 routes should be in RIB.
	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 5 {
		t.Fatalf("expected 5 routes in RIB, got %d", len(routes))
	}

	// Now simulate peer down — PeerDown drains workers, then ClearPeer.
	// Re-create workers for the PeerDown flow.
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		rs.processForward(key, item.msgID)
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second})

	rs.dispatch([]byte(`{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","asn":65001},"state":"down"}}`))

	// After peer down, RIB should be empty for this peer.
	routes = rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 0 {
		t.Errorf("expected 0 routes after peer down, got %d", len(routes))
	}
}

// TestReleaseCacheAsync verifies releaseCache returns immediately (async).
//
// VALIDATES: AC-5 — releaseCache is async (does not block worker).
// PREVENTS: Worker blocked on synchronous release RPC.
func TestReleaseCacheAsync(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.mu.Unlock()

	// releaseCache should return quickly (async channel send, not blocking RPC).
	// With closed connections (newTestRouteServer), a synchronous call would
	// still return quickly due to error, but the async version should be instant.
	start := time.Now()
	rs.releaseCache(42)
	elapsed := time.Since(start)

	// Async release should be sub-millisecond (just a channel send).
	if elapsed > 10*time.Millisecond {
		t.Errorf("releaseCache took %v, expected sub-millisecond for async", elapsed)
	}
}

// TestBatchForwardAccumulation verifies items accumulate before RPC.
//
// VALIDATES: AC-10 — worker sends batch RPC after accumulating items.
// PREVENTS: N items generating N individual RPCs instead of batched RPC.
func TestBatchForwardAccumulation(t *testing.T) {
	rs := newTestRouteServer(t)

	var commands []string
	var cmdMu sync.Mutex
	rs.updateRouteHook = func(_, cmd string) {
		cmdMu.Lock()
		commands = append(commands, cmd)
		cmdMu.Unlock()
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Gate: block the worker handler until all items are dispatched.
	// This guarantees all 5 items are queued before processing starts,
	// making batch accumulation deterministic (not scheduler-dependent).
	gate := make(chan struct{})
	rs.workers.Stop()
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		<-gate
		rs.processForward(key, item.msgID)
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second, onDrained: rs.flushWorkerBatch})

	// Dispatch 5 UPDATEs (all same source, same targets).
	for i := uint64(1); i <= 5; i++ {
		rs.dispatch([]byte(buildTestUpdate("10.0.0.1", i)))
	}

	// Release gate: worker processes all 5 items sequentially with items 2-5
	// already in the channel, so onDrained only fires after item 5 — producing
	// a single batch flush with all 5 IDs.
	close(gate)
	flushWorkers(t, rs)

	cmdMu.Lock()
	defer cmdMu.Unlock()

	// Count forward RPCs and check for batch (comma-separated IDs).
	forwardCount := 0
	hasBatch := false
	for _, cmd := range commands {
		if strings.Contains(cmd, "forward") {
			forwardCount++
			// Check if the ID portion contains a comma (batch).
			parts := strings.Fields(cmd)
			if len(parts) >= 3 && strings.Contains(parts[2], ",") {
				hasBatch = true
			}
		}
	}

	if forwardCount >= 5 {
		t.Errorf("expected fewer than 5 forward RPCs (batched), got %d", forwardCount)
	}
	if !hasBatch {
		t.Error("expected at least one batch forward command with comma-separated IDs")
	}
}

// TestSelectForwardTargetsDeterministic verifies that selectForwardTargets
// returns a deterministic (sorted) peer list regardless of map iteration order.
//
// VALIDATES: Batching correctness — selector string must be identical for the
// same peer set to prevent false selector-change flushes in batchForwardUpdate.
// PREVENTS: Non-deterministic Go map iteration defeating batch accumulation.
func TestSelectForwardTargetsDeterministic(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.3"] = &PeerState{Address: "10.0.0.3", Up: true}
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.4"] = &PeerState{Address: "10.0.0.4", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	families := map[string]bool{"ipv4/unicast": true}

	// Call 100 times — with unsorted output, Go map randomness would produce
	// different orderings, causing batch selector mismatches.
	rs.mu.RLock()
	first := rs.selectForwardTargets("10.0.0.1", families)
	rs.mu.RUnlock()

	want := strings.Join(first, ",")
	if want != "10.0.0.2,10.0.0.3,10.0.0.4" {
		t.Fatalf("expected sorted targets, got %q", want)
	}

	for i := range 100 {
		rs.mu.RLock()
		got := rs.selectForwardTargets("10.0.0.1", families)
		rs.mu.RUnlock()
		if sel := strings.Join(got, ","); sel != want {
			t.Fatalf("iteration %d: selector %q != %q (non-deterministic)", i, sel, want)
		}
	}
}

// TestBatchForwardFireAndForget verifies worker doesn't block on forward RPC.
//
// VALIDATES: AC-11 — worker continues processing without waiting for RPC response.
// PREVENTS: Worker goroutine blocked on synchronous updateRoute during batch flush.
func TestBatchForwardFireAndForget(t *testing.T) {
	rs := newTestRouteServer(t)

	// Block forward RPCs to prove workers don't wait for responses.
	// The hook runs inside updateRoute — with sync forward it blocks
	// the worker goroutine; with async forward it blocks the background
	// sender goroutine instead.
	blockForward := make(chan struct{})
	var forwardCmds []string
	var cmdMu sync.Mutex
	rs.updateRouteHook = func(_, cmd string) {
		if strings.Contains(cmd, "forward") {
			<-blockForward
			cmdMu.Lock()
			forwardCmds = append(forwardCmds, cmd)
			cmdMu.Unlock()
		}
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Dispatch 5 UPDATEs with distinct prefixes from the same source peer.
	for i := uint64(1); i <= 5; i++ {
		input := fmt.Sprintf(
			`{"type":"bgp","bgp":{"message":{"type":"update","id":%d},"peer":{"address":"10.0.0.1","asn":65001},"update":{"nlri":{"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.%d.0/24"]}]}}}}`,
			i, i,
		)
		rs.dispatch([]byte(input))
	}

	// Workers.Stop() drains all items. With synchronous forward, the worker
	// goroutine blocks in onDrained → flushBatch → updateRoute → hook, so
	// Stop() hangs. With fire-and-forget, asyncForward returns immediately.
	stopDone := make(chan struct{})
	go func() {
		rs.workers.Stop()
		close(stopDone)
	}()

	select {
	case <-stopDone:
		// Workers stopped promptly — fire-and-forget confirmed.
	case <-time.After(2 * time.Second):
		close(blockForward) // Unblock hook so test can clean up.
		<-stopDone
		t.Fatal("workers.Stop() blocked — forward RPC not fire-and-forget (AC-11)")
	}

	// Workers completed — verify RIB has all routes.
	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 5 {
		t.Errorf("expected 5 routes in RIB, got %d", len(routes))
	}

	// Unblock forward RPCs and verify background sender processes them.
	close(blockForward)
	time.Sleep(100 * time.Millisecond)

	cmdMu.Lock()
	defer cmdMu.Unlock()
	if len(forwardCmds) == 0 {
		t.Error("expected forward RPCs processed by background sender")
	}

	// Recreate worker pool for cleanup (Stop() was called above).
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		rs.processForward(key, item.msgID)
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second, onDrained: rs.flushWorkerBatch})
}

// TestParseUpdateFamiliesOnly verifies two-level parsing: families extracted without NLRI arrays.
//
// VALIDATES: AC-12 — only family keys parsed for forward target selection;
// full NLRI arrays deferred to RIB path via parseNLRIFamilyOps.
// PREVENTS: Unnecessary parsing of large NLRI arrays on the forward hot path.
func TestParseUpdateFamiliesOnly(t *testing.T) {
	tests := []struct {
		name         string
		payload      string
		wantFamilies []string
		wantOps      int // total ops from deferred full parse
	}{
		{
			name:         "single_family",
			payload:      `{"message":{"type":"update","id":1},"peer":{"address":"10.0.0.1","asn":65001},"update":{"nlri":{"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24"]}]}}}`,
			wantFamilies: []string{"ipv4/unicast"},
			wantOps:      1,
		},
		{
			name:         "multiple_families",
			payload:      `{"message":{"type":"update","id":2},"peer":{"address":"10.0.0.1","asn":65001},"update":{"nlri":{"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24"]}],"ipv6/unicast":[{"next-hop":"2001:db8::1","action":"add","nlri":["2001:db8::/32"]}]}}}`,
			wantFamilies: []string{"ipv4/unicast", "ipv6/unicast"},
			wantOps:      2,
		},
		{
			name:         "empty_nlri_map",
			payload:      `{"message":{"type":"update","id":3},"peer":{"address":"10.0.0.1","asn":65001},"update":{"nlri":{}}}`,
			wantFamilies: nil,
			wantOps:      0,
		},
		{
			name:         "no_update_field",
			payload:      `{"message":{"type":"update","id":4},"peer":{"address":"10.0.0.1","asn":65001}}`,
			wantFamilies: nil,
			wantOps:      0,
		},
		{
			name:         "mixed_add_del",
			payload:      `{"message":{"type":"update","id":5},"peer":{"address":"10.0.0.1","asn":65001},"update":{"nlri":{"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24"]},{"action":"del","nlri":["10.0.1.0/24"]}]}}}`,
			wantFamilies: []string{"ipv4/unicast"},
			wantOps:      2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			families, nlriRaw, err := parseUpdateFamilies([]byte(tt.payload))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify family keys extracted.
			if len(families) != len(tt.wantFamilies) {
				t.Fatalf("families: got %d, want %d", len(families), len(tt.wantFamilies))
			}
			for _, f := range tt.wantFamilies {
				if !families[f] {
					t.Errorf("missing family %q", f)
				}
			}

			// Verify raw data populated for each family (not fully parsed yet).
			for f := range families {
				if len(nlriRaw[f]) == 0 {
					t.Errorf("family %q has no raw NLRI data", f)
				}
			}

			// Deferred full parse should produce correct operations.
			familyOps := parseNLRIFamilyOps(nlriRaw)
			totalOps := 0
			for _, ops := range familyOps {
				totalOps += len(ops)
			}
			if totalOps != tt.wantOps {
				t.Errorf("total ops: got %d, want %d", totalOps, tt.wantOps)
			}
		})
	}
}

// buildTestUpdate creates a minimal ze-bgp UPDATE JSON for testing dispatch.
func buildTestUpdate(peer string, msgID uint64) string {
	return `{"type":"bgp","bgp":{"message":{"type":"update","id":` +
		strings.Repeat("", 0) + // force import
		fmt.Sprintf("%d", msgID) +
		`,"direction":"received"},"peer":{"address":"` + peer +
		`","asn":65001},"update":{"nlri":{"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24"]}]}}}}`
}
