package bgp_rr

import (
	"sort"
	"testing"
)

// --- Target selection tests (forwardUpdate decision logic) ---

// TestSelectTargets_SingleFamily_AllSupport verifies basic single-family forwarding.
//
// VALIDATES: UPDATE with one family is forwarded to all peers that support it.
// PREVENTS: Forward logic silently dropping routes in the simple case.
func TestSelectTargets_SingleFamily_AllSupport(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.peers["10.0.0.3"] = &PeerState{
		Address: "10.0.0.3", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	// UPDATE from 10.0.0.1 with ipv4/unicast
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	sort.Strings(targets)
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d: %v", len(targets), targets)
	}
	if targets[0] != "10.0.0.2" || targets[1] != "10.0.0.3" {
		t.Errorf("expected [10.0.0.2, 10.0.0.3], got %v", targets)
	}
}

// TestSelectTargets_SingleFamily_PartialSupport verifies family filtering.
//
// VALIDATES: Peers that don't support the UPDATE's family are excluded.
// PREVENTS: Sending routes to peers that can't process them.
func TestSelectTargets_SingleFamily_PartialSupport(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true}, // No ipv6
	}
	rs.peers["10.0.0.3"] = &PeerState{
		Address: "10.0.0.3", Up: true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.mu.Unlock()

	// ipv6/unicast UPDATE from 10.0.0.1 → only 10.0.0.3 supports it
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv6/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d: %v", len(targets), targets)
	}
	if targets[0] != "10.0.0.3" {
		t.Errorf("expected [10.0.0.3], got %v", targets)
	}
}

// TestSelectTargets_MultiFamilyUpdate_PartialOverlap verifies multi-family UPDATE forwarding.
//
// VALIDATES: When an UPDATE carries families A+B, a peer supporting only A still
// receives the UPDATE (engine handles per-family splitting at wire level).
// PREVENTS: All-or-nothing family check that drops the entire UPDATE for peers
// with partial family overlap. This was the original bug: the code required ALL
// families to match, so peers missing even one family got NOTHING.
func TestSelectTargets_MultiFamilyUpdate_PartialOverlap(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true}, // Only ipv4
	}
	rs.mu.Unlock()

	// UPDATE from "10.0.0.0" carries both ipv4/unicast and ipv6/unicast
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.0", map[string]bool{
		"ipv4/unicast": true,
		"ipv6/unicast": true,
	})
	rs.mu.RUnlock()

	sort.Strings(targets)
	// BOTH peers should be targets: 10.0.0.1 supports both, 10.0.0.2 supports ipv4
	// The engine's ForwardUpdate handles per-family wire splitting
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets (partial overlap should include peer), got %d: %v", len(targets), targets)
	}
}

// TestSelectTargets_ExcludesSourcePeer verifies source peer is never a target.
//
// VALIDATES: Source peer exclusion prevents routing loops.
// PREVENTS: Route reflected back to the sender.
func TestSelectTargets_ExcludesSourcePeer(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 1 {
		t.Fatalf("expected 1 target (excluding source), got %d: %v", len(targets), targets)
	}
	if targets[0] != "10.0.0.2" {
		t.Errorf("expected target 10.0.0.2, got %v", targets)
	}
}

// TestSelectTargets_ExcludesDownPeer verifies down peers are never targets.
//
// VALIDATES: Only established peers receive forwarded routes.
// PREVENTS: Sending routes to peers that can't process them.
func TestSelectTargets_ExcludesDownPeer(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: false,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.0", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 1 {
		t.Fatalf("expected 1 target (excluding down peer), got %d: %v", len(targets), targets)
	}
	if targets[0] != "10.0.0.2" {
		t.Errorf("expected target 10.0.0.2, got %v", targets)
	}
}

// TestSelectTargets_NilFamilies_AcceptsAll verifies nil Families means "accept all".
//
// VALIDATES: Peers without OPEN data (Families=nil) receive all updates.
// PREVENTS: Dropping routes to peers whose capabilities are unknown.
func TestSelectTargets_NilFamilies_AcceptsAll(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:  "10.0.0.1",
		Up:       true,
		Families: nil, // No OPEN processed yet
	}
	rs.mu.Unlock()

	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.0", map[string]bool{"ipv6/vpn": true})
	rs.mu.RUnlock()

	if len(targets) != 1 {
		t.Fatalf("expected 1 target (nil Families → accept all), got %d: %v", len(targets), targets)
	}
}

// TestSelectTargets_MPWithoutIPv4_DeclinesIPv4Unicast verifies that a peer
// advertising MP families without ipv4/unicast explicitly declines it.
//
// VALIDATES: When MP caps are present but omit ipv4/unicast, the peer rejects ipv4/unicast routes.
// PREVENTS: Forwarding ipv4/unicast to peers that explicitly opted out via MP capability negotiation.
// NOTE: RFC 4760 Section 1 — ipv4/unicast is only the implicit default when NO MP caps are sent.
func TestSelectTargets_MPWithoutIPv4_DeclinesIPv4Unicast(t *testing.T) {
	rs := newTestRouteServer(t)

	// Peer advertised MP for l2vpn/evpn only — explicitly declined ipv4/unicast
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:  "10.0.0.1",
		Up:       true,
		Families: map[string]bool{"l2vpn/evpn": true},
	}
	rs.mu.Unlock()

	// ipv4/unicast should be rejected — peer explicitly omitted it from MP caps
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.0", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 0 {
		t.Errorf("expected 0 targets (ipv4/unicast declined via MP), got %d: %v", len(targets), targets)
	}

	// l2vpn/evpn should be accepted — it's in the MP caps
	rs.mu.RLock()
	targets = rs.selectForwardTargets("10.0.0.0", map[string]bool{"l2vpn/evpn": true})
	rs.mu.RUnlock()

	if len(targets) != 1 {
		t.Errorf("expected 1 target (l2vpn/evpn in MP caps), got %d: %v", len(targets), targets)
	}
}

// TestSelectTargets_NoTargets_AllExcluded verifies empty result when all peers are excluded.
//
// VALIDATES: No targets when only peer is the source.
// PREVENTS: Crash or panic on empty target list.
func TestSelectTargets_NoTargets_AllExcluded(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	// Source is the only peer
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 0 {
		t.Errorf("expected 0 targets (only peer is source), got %d: %v", len(targets), targets)
	}
}

// --- Event ordering / race condition tests ---

// TestOpenCreatesEmptyFamilies verifies OPEN with no multiprotocol creates empty Families.
//
// VALIDATES: handleOpen with non-multiprotocol capabilities creates non-nil empty Families map.
// PREVENTS: Assumption that Families is nil when peer has capabilities.
// NOTE: This is a real scenario — a peer sending OPEN with only asn4 + route-refresh
// but no multiprotocol capability would have empty Families after handleOpen.
func TestOpenCreatesEmptyFamilies(t *testing.T) {
	rs := newTestRouteServer(t)

	// OPEN with capabilities but NO multiprotocol entries
	input := `{"type":"bgp","bgp":{"message":{"type":"open"},"peer":{"address":"10.0.0.1","asn":65001},"open":{"asn":65001,"router-id":"10.0.0.1","hold-time":180,"capabilities":[{"code":2,"name":"route-refresh"},{"code":65,"name":"asn4","value":"65001"}]}}}`

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
	if peer.Families == nil {
		t.Fatal("expected non-nil Families after OPEN")
	}
	// RFC 4271: ipv4/unicast is always implicitly negotiated, even with no
	// multiprotocol capabilities in the OPEN message.
	if len(peer.Families) != 1 || !peer.Families["ipv4/unicast"] {
		t.Errorf("expected Families={ipv4/unicast: true} (RFC 4271 default), got %v", peer.Families)
	}

	// Peer is Up=false (no state event yet), so excluded regardless
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.0", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()
	if len(targets) != 0 {
		t.Errorf("expected 0 targets (peer is down), got %d: %v", len(targets), targets)
	}

	// Set it up and try again with Up=true — now ipv4/unicast should match
	rs.mu.Lock()
	peer.Up = true
	rs.mu.Unlock()

	rs.mu.RLock()
	targets = rs.selectForwardTargets("10.0.0.0", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 1 {
		t.Errorf("expected 1 target (ipv4/unicast always supported), got %d: %v", len(targets), targets)
	}
}

// TestStateUpBeforeOpen_FamiliesNil verifies state-up before OPEN means nil Families.
//
// VALIDATES: If state "up" arrives before OPEN, peer has nil Families (accepts all).
// PREVENTS: Race where routes are dropped because OPEN hasn't been processed yet.
func TestStateUpBeforeOpen_FamiliesNil(t *testing.T) {
	rs := newTestRouteServer(t)

	// State up arrives first (before OPEN)
	stateInput := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","asn":65001},"state":"up"}}`
	event, err := parseEvent([]byte(stateInput))
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
	if peer.Families != nil {
		t.Errorf("expected nil Families before OPEN, got %v", peer.Families)
	}

	// With nil Families, peer should accept ALL families
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.0", map[string]bool{"l2vpn/evpn": true})
	rs.mu.RUnlock()

	if len(targets) != 1 {
		t.Fatalf("expected 1 target (nil Families → accept all), got %d", len(targets))
	}
}

// TestOpenThenStateUp_FamiliesPopulated verifies normal OPEN→state-up sequence.
//
// VALIDATES: After OPEN + state-up, peer has correct Families and accepts matching routes.
// PREVENTS: Missing family extraction from OPEN capabilities.
func TestOpenThenStateUp_FamiliesPopulated(t *testing.T) {
	rs := newTestRouteServer(t)

	// Step 1: OPEN with multiprotocol for ipv4/unicast and ipv6/unicast
	openInput := `{"type":"bgp","bgp":{"message":{"type":"open"},"peer":{"address":"10.0.0.1","asn":65001},"open":{"asn":65001,"router-id":"10.0.0.1","hold-time":180,"capabilities":[{"code":1,"name":"multiprotocol","value":"ipv4/unicast"},{"code":1,"name":"multiprotocol","value":"ipv6/unicast"},{"code":2,"name":"route-refresh"}]}}}`
	event, err := parseEvent([]byte(openInput))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	rs.dispatch(event)

	// Step 2: State up
	stateInput := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","asn":65001},"state":"up"}}`
	event, err = parseEvent([]byte(stateInput))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	rs.dispatch(event)

	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()

	if !peer.Up {
		t.Error("expected peer up")
	}
	if !peer.SupportsFamily("ipv4/unicast") {
		t.Error("expected ipv4/unicast support")
	}
	if !peer.SupportsFamily("ipv6/unicast") {
		t.Error("expected ipv6/unicast support")
	}
	if peer.SupportsFamily("l2vpn/evpn") {
		t.Error("should NOT support l2vpn/evpn")
	}

	// ipv4/unicast UPDATE should target this peer
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.0", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()
	if len(targets) != 1 {
		t.Fatalf("expected 1 target for ipv4/unicast, got %d", len(targets))
	}

	// l2vpn/evpn UPDATE should NOT target this peer
	rs.mu.RLock()
	targets = rs.selectForwardTargets("10.0.0.0", map[string]bool{"l2vpn/evpn": true})
	rs.mu.RUnlock()
	if len(targets) != 0 {
		t.Errorf("expected 0 targets for l2vpn/evpn, got %d: %v", len(targets), targets)
	}
}

// --- Full propagation scenario tests ---

// TestPropagation_ThreePeers_SingleFamily simulates basic 3-peer route reflection.
//
// VALIDATES: Route from peer A stored in RIB, peers B and C would be forward targets.
// PREVENTS: Basic forwarding failure in simple topology.
func TestPropagation_ThreePeers_SingleFamily(t *testing.T) {
	rs := newTestRouteServer(t)

	// Set up 3 peers with OPEN + state-up
	peers := []struct {
		addr     string
		families []string
	}{
		{"10.0.0.1", []string{"ipv4/unicast"}},
		{"10.0.0.2", []string{"ipv4/unicast"}},
		{"10.0.0.3", []string{"ipv4/unicast"}},
	}

	for _, p := range peers {
		fam := make(map[string]bool)
		for _, f := range p.families {
			fam[f] = true
		}
		rs.mu.Lock()
		rs.peers[p.addr] = &PeerState{
			Address:      p.addr,
			Up:           true,
			Families:     fam,
			Capabilities: map[string]bool{"route-refresh": true, "multiprotocol": true},
		}
		rs.mu.Unlock()
	}

	// Peer 1 sends an UPDATE with 10.0.0.0/24
	updateInput := `{"type":"bgp","bgp":{"message":{"type":"update","id":100,"direction":"received"},"peer":{"address":"10.0.0.1","asn":65001},"update":{"attr":{"origin":"igp","as-path":[65001]},"nlri":{"ipv4/unicast":[{"next-hop":"192.168.1.1","action":"add","nlri":["10.0.0.0/24"]}]}}}}`
	event, err := parseEvent([]byte(updateInput))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	rs.dispatch(event)

	// Verify RIB has the route
	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 1 {
		t.Fatalf("expected 1 route in RIB, got %d", len(routes))
	}

	// Verify forward targets: should be 10.0.0.2 and 10.0.0.3 (not source 10.0.0.1)
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	sort.Strings(targets)
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d: %v", len(targets), targets)
	}
	if targets[0] != "10.0.0.2" || targets[1] != "10.0.0.3" {
		t.Errorf("expected [10.0.0.2, 10.0.0.3], got %v", targets)
	}
}

// TestPropagation_FourPeers_SevenFamilies simulates the chaos test topology.
//
// VALIDATES: Route forwarding works correctly with mixed family support across 4 peers.
// PREVENTS: Route propagation failure in the exact topology that the chaos test uses.
func TestPropagation_FourPeers_SevenFamilies(t *testing.T) {
	rs := newTestRouteServer(t)

	// Simulate chaos test: 4 peers, 7 families, not all peers support all families
	peerFamilies := map[string][]string{
		"10.0.0.1": {"ipv4/unicast", "ipv4/flow", "ipv6/unicast", "ipv6/flow", "ipv4/vpn", "ipv6/vpn", "l2vpn/evpn"},
		"10.0.0.2": {"ipv4/unicast", "ipv6/unicast", "ipv4/vpn"},
		"10.0.0.3": {"ipv4/unicast", "ipv6/unicast", "ipv4/flow", "l2vpn/evpn"},
		"10.0.0.4": {"ipv4/unicast", "ipv6/unicast", "ipv6/vpn", "ipv6/flow"},
	}

	for addr, families := range peerFamilies {
		fam := make(map[string]bool)
		for _, f := range families {
			fam[f] = true
		}
		rs.mu.Lock()
		rs.peers[addr] = &PeerState{
			Address:      addr,
			Up:           true,
			Families:     fam,
			Capabilities: map[string]bool{"route-refresh": true, "multiprotocol": true},
		}
		rs.mu.Unlock()
	}

	tests := []struct {
		name      string
		source    string
		families  map[string]bool
		wantCount int
		wantAddrs []string // sorted
	}{
		{
			name:      "ipv4/unicast from peer1 → all others",
			source:    "10.0.0.1",
			families:  map[string]bool{"ipv4/unicast": true},
			wantCount: 3,
			wantAddrs: []string{"10.0.0.2", "10.0.0.3", "10.0.0.4"},
		},
		{
			name:      "ipv4/flow from peer1 → only peer3",
			source:    "10.0.0.1",
			families:  map[string]bool{"ipv4/flow": true},
			wantCount: 1,
			wantAddrs: []string{"10.0.0.3"},
		},
		{
			name:      "l2vpn/evpn from peer1 → only peer3",
			source:    "10.0.0.1",
			families:  map[string]bool{"l2vpn/evpn": true},
			wantCount: 1,
			wantAddrs: []string{"10.0.0.3"},
		},
		{
			name:      "ipv6/vpn from peer1 → only peer4",
			source:    "10.0.0.1",
			families:  map[string]bool{"ipv6/vpn": true},
			wantCount: 1,
			wantAddrs: []string{"10.0.0.4"},
		},
		{
			name:      "ipv6/unicast from peer2 → peers 1,3,4",
			source:    "10.0.0.2",
			families:  map[string]bool{"ipv6/unicast": true},
			wantCount: 3,
			wantAddrs: []string{"10.0.0.1", "10.0.0.3", "10.0.0.4"},
		},
		{
			name:      "ipv4/vpn from peer2 → only peer1",
			source:    "10.0.0.2",
			families:  map[string]bool{"ipv4/vpn": true},
			wantCount: 1,
			wantAddrs: []string{"10.0.0.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs.mu.RLock()
			targets := rs.selectForwardTargets(tt.source, tt.families)
			rs.mu.RUnlock()

			sort.Strings(targets)
			if len(targets) != tt.wantCount {
				t.Errorf("expected %d targets, got %d: %v", tt.wantCount, len(targets), targets)
			}
			for i, want := range tt.wantAddrs {
				if i >= len(targets) {
					t.Errorf("missing target %s", want)
					continue
				}
				if targets[i] != want {
					t.Errorf("target[%d] = %s, want %s", i, targets[i], want)
				}
			}
		})
	}
}

// TestPropagation_UpdateBeforeAnyPeerKnown verifies UPDATE when no peers are registered.
//
// VALIDATES: UPDATE arriving before any peers are known produces no targets (release).
// PREVENTS: Panic on empty peers map, or route leak.
func TestPropagation_UpdateBeforeAnyPeerKnown(t *testing.T) {
	rs := newTestRouteServer(t)

	// No peers registered at all
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 0 {
		t.Errorf("expected 0 targets (no peers known), got %d: %v", len(targets), targets)
	}
}

// TestPropagation_UpdateWhenOnlySourceKnown verifies UPDATE when only source peer exists.
//
// VALIDATES: UPDATE with only the source peer registered produces no targets.
// PREVENTS: Forward to source (routing loop).
func TestPropagation_UpdateWhenOnlySourceKnown(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 0 {
		t.Errorf("expected 0 targets (only source peer), got %d: %v", len(targets), targets)
	}
}

// TestPropagation_VPNRoute verifies VPN NLRI with complex prefix format is handled.
//
// VALIDATES: VPN routes with object NLRIs (containing prefix field) are stored in RIB.
// PREVENTS: Lost VPN routes due to NLRI format mismatch.
func TestPropagation_VPNRoute(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/vpn": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/vpn": true},
	}
	rs.mu.Unlock()

	// VPN UPDATE with object NLRIs containing "prefix" field
	input := `{"type":"bgp","bgp":{"message":{"type":"update","id":200,"direction":"received"},"peer":{"address":"10.0.0.1","asn":65001},"update":{"attr":{"origin":"igp"},"nlri":{"ipv4/vpn":[{"next-hop":"192.168.1.1","action":"add","nlri":[{"prefix":"10.0.0.0/24","rd":"0:1:100","label":100000}]}]}}}}`

	event, err := parseEvent([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	rs.dispatch(event)

	routes := rs.rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 1 {
		t.Fatalf("expected 1 VPN route in RIB, got %d", len(routes))
	}
	if routes[0].Family != "ipv4/vpn" {
		t.Errorf("expected family ipv4/vpn, got %s", routes[0].Family)
	}
	if routes[0].Prefix != "10.0.0.0/24" {
		t.Errorf("expected prefix 10.0.0.0/24, got %s", routes[0].Prefix)
	}

	// Verify forward target
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv4/vpn": true})
	rs.mu.RUnlock()

	if len(targets) != 1 || targets[0] != "10.0.0.2" {
		t.Errorf("expected target [10.0.0.2], got %v", targets)
	}
}

// TestPropagation_WithdrawClearsRIB verifies withdrawal removes route from RIB.
//
// VALIDATES: After withdrawal, route is removed and forward targets use updated families.
// PREVENTS: Stale routes persisting after withdrawal.
func TestPropagation_WithdrawClearsRIB(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true}}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true}}
	rs.mu.Unlock()

	// Add route
	addInput := `{"type":"bgp","bgp":{"message":{"type":"update","id":100,"direction":"received"},"peer":{"address":"10.0.0.1","asn":65001},"update":{"attr":{"origin":"igp"},"nlri":{"ipv4/unicast":[{"next-hop":"192.168.1.1","action":"add","nlri":["10.0.0.0/24"]}]}}}}`
	event, _ := parseEvent([]byte(addInput))
	rs.dispatch(event)

	if len(rs.rib.GetPeerRoutes("10.0.0.1")) != 1 {
		t.Fatal("route not added")
	}

	// Withdraw route
	delInput := `{"type":"bgp","bgp":{"message":{"type":"update","id":101,"direction":"received"},"peer":{"address":"10.0.0.1","asn":65001},"update":{"nlri":{"ipv4/unicast":[{"action":"del","nlri":["10.0.0.0/24"]}]}}}}`
	event, _ = parseEvent([]byte(delInput))
	rs.dispatch(event)

	if len(rs.rib.GetPeerRoutes("10.0.0.1")) != 0 {
		t.Error("route not withdrawn")
	}
}

// TestPropagation_PeerDownClearsAllRoutes verifies session down clears all routes.
//
// VALIDATES: When a peer goes down, all its routes are removed from RIB.
// PREVENTS: Ghost routes from disconnected peers.
func TestPropagation_PeerDownClearsAllRoutes(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true}}
	rs.mu.Unlock()

	// Insert routes for multiple families
	rs.rib.Insert("10.0.0.1", &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})
	rs.rib.Insert("10.0.0.1", &Route{MsgID: 101, Family: "ipv4/unicast", Prefix: "10.0.1.0/24"})
	rs.rib.Insert("10.0.0.1", &Route{MsgID: 102, Family: "ipv6/unicast", Prefix: "2001:db8::/32"})

	if len(rs.rib.GetPeerRoutes("10.0.0.1")) != 3 {
		t.Fatal("routes not added")
	}

	// Peer goes down
	downInput := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","asn":65001},"state":"down"}}`
	event, _ := parseEvent([]byte(downInput))
	rs.dispatch(event)

	if len(rs.rib.GetPeerRoutes("10.0.0.1")) != 0 {
		t.Error("routes not cleared on peer down")
	}
}
