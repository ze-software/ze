package bgp_persist

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestPersistServer(t *testing.T) *PersistServer {
	t.Helper()
	return &PersistServer{
		peers:  make(map[string]*PersistPeer),
		ribOut: make(map[string]map[string]*StoredRoute),
	}
}

// TestPersist_SentUpdate_StoresRoute verifies sent UPDATE stores route in ribOut.
//
// VALIDATES: AC-9 — sent UPDATE stores route in ribOut with msg-id.
// PREVENTS: Lost routes that wouldn't be replayed on reconnect.
func TestPersist_SentUpdate_StoresRoute(t *testing.T) {
	ps := newTestPersistServer(t)

	var mu sync.Mutex
	var commands []string
	ps.updateRouteHook = func(peer, cmd string) {
		mu.Lock()
		commands = append(commands, peer+"\t"+cmd)
		mu.Unlock()
	}

	ps.dispatchText("peer 10.0.0.1 asn 65001 sent update 42 origin igp next-hop 10.0.0.1 nlri ipv4/unicast add prefix 192.168.1.0/24")

	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peerRoutes := ps.ribOut["10.0.0.1"]
	if peerRoutes == nil {
		t.Fatal("expected ribOut entry for 10.0.0.1")
	}

	route, exists := peerRoutes["ipv4/unicast|prefix 192.168.1.0/24"]
	if !exists {
		t.Fatalf("expected route in ribOut, got keys: %v", routeKeys(peerRoutes))
	}
	if route.MsgID != 42 {
		t.Errorf("MsgID = %d, want 42", route.MsgID)
	}
	if route.Family != "ipv4/unicast" {
		t.Errorf("Family = %q, want ipv4/unicast", route.Family)
	}

	// Verify retain was called.
	mu.Lock()
	defer mu.Unlock()

	hasRetain := false
	for _, cmd := range commands {
		if strings.Contains(cmd, "bgp cache 42 retain") {
			hasRetain = true
		}
	}
	if !hasRetain {
		t.Errorf("expected 'bgp cache 42 retain', got commands: %v", commands)
	}
}

// TestPersist_SentWithdrawal_RemovesRoute verifies withdrawal removes route and releases cache.
//
// VALIDATES: AC-9 — withdrawal removes route from ribOut.
// PREVENTS: Stale routes replayed after withdrawal.
func TestPersist_SentWithdrawal_RemovesRoute(t *testing.T) {
	ps := newTestPersistServer(t)

	var mu sync.Mutex
	var commands []string
	ps.updateRouteHook = func(peer, cmd string) {
		mu.Lock()
		commands = append(commands, peer+"\t"+cmd)
		mu.Unlock()
	}

	// Add route.
	ps.dispatchText("peer 10.0.0.1 asn 65001 sent update 42 origin igp next-hop 10.0.0.1 nlri ipv4/unicast add prefix 192.168.1.0/24")

	// Withdraw route.
	ps.dispatchText("peer 10.0.0.1 asn 65001 sent update 43 nlri ipv4/unicast del prefix 192.168.1.0/24")

	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peerRoutes := ps.ribOut["10.0.0.1"]
	if len(peerRoutes) > 0 {
		t.Errorf("expected empty ribOut after withdrawal, got %v", routeKeys(peerRoutes))
	}

	// Verify release was called for the old route.
	mu.Lock()
	defer mu.Unlock()

	hasRelease := false
	for _, cmd := range commands {
		if strings.Contains(cmd, "bgp cache 42 release") {
			hasRelease = true
		}
	}
	if !hasRelease {
		t.Errorf("expected 'bgp cache 42 release', got commands: %v", commands)
	}
}

// TestPersist_PeerDown_KeepsRibOut verifies ribOut is preserved on peer down.
//
// VALIDATES: AC-11 — peer down keeps ribOut intact.
// PREVENTS: Lost ribOut on peer down (would break replay on reconnect).
func TestPersist_PeerDown_KeepsRibOut(t *testing.T) {
	ps := newTestPersistServer(t)
	ps.updateRouteHook = func(peer, cmd string) {} // no-op

	// Add route.
	ps.dispatchText("peer 10.0.0.1 asn 65001 sent update 42 origin igp next-hop 10.0.0.1 nlri ipv4/unicast add prefix 192.168.1.0/24")

	// Peer goes down.
	ps.dispatchText("peer 10.0.0.1 asn 65001 state down")

	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peerRoutes := ps.ribOut["10.0.0.1"]
	if len(peerRoutes) == 0 {
		t.Error("ribOut should be preserved on peer down")
	}
}

// TestPersist_PeerUp_ReplaysRoutes verifies replay on peer up.
//
// VALIDATES: AC-10 — peer reconnect triggers replay via cache forward.
// PREVENTS: Routes not replayed after peer reconnect.
func TestPersist_PeerUp_ReplaysRoutes(t *testing.T) {
	ps := newTestPersistServer(t)

	var mu sync.Mutex
	var commands []string
	ps.updateRouteHook = func(peer, cmd string) {
		mu.Lock()
		commands = append(commands, peer+"\t"+cmd)
		mu.Unlock()
	}

	// Pre-populate ribOut (simulating routes sent before peer went down).
	ps.mu.Lock()
	ps.ribOut["10.0.0.1"] = map[string]*StoredRoute{
		"ipv4/unicast|prefix 192.168.1.0/24": {MsgID: 42, Family: "ipv4/unicast", Prefix: "prefix 192.168.1.0/24"},
		"ipv4/unicast|prefix 192.168.2.0/24": {MsgID: 43, Family: "ipv4/unicast", Prefix: "prefix 192.168.2.0/24"},
	}
	ps.mu.Unlock()

	// Peer comes up.
	ps.dispatchText("peer 10.0.0.1 asn 65001 state up")
	time.Sleep(200 * time.Millisecond) // Wait for replay goroutine.

	mu.Lock()
	defer mu.Unlock()

	var forwards []string
	for _, cmd := range commands {
		if strings.Contains(cmd, "forward") {
			forwards = append(forwards, cmd)
		}
	}
	if len(forwards) != 2 {
		t.Fatalf("expected 2 forward commands, got %d: %v", len(forwards), forwards)
	}

	for _, cmd := range forwards {
		if !strings.Contains(cmd, "forward 10.0.0.1") {
			t.Errorf("expected forward to 10.0.0.1, got: %s", cmd)
		}
	}
}

// TestPersist_PeerUp_SendsEOR verifies EOR sent per family after replay.
//
// VALIDATES: AC-10 — EOR sent per negotiated family after replay.
// PREVENTS: Peer never leaves initial table exchange.
func TestPersist_PeerUp_SendsEOR(t *testing.T) {
	ps := newTestPersistServer(t)

	var mu sync.Mutex
	var commands []string
	ps.updateRouteHook = func(peer, cmd string) {
		mu.Lock()
		commands = append(commands, peer+"\t"+cmd)
		mu.Unlock()
	}

	// Set up peer with families (from OPEN).
	ps.mu.Lock()
	ps.peers["10.0.0.1"] = &PersistPeer{
		Address:  "10.0.0.1",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	ps.ribOut["10.0.0.1"] = map[string]*StoredRoute{
		"ipv4/unicast|prefix 192.168.1.0/24": {MsgID: 42, Family: "ipv4/unicast", Prefix: "prefix 192.168.1.0/24"},
	}
	ps.mu.Unlock()

	// Peer comes up.
	ps.dispatchText("peer 10.0.0.1 asn 65001 state up")
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	var eorCmds []string
	for _, cmd := range commands {
		if strings.Contains(cmd, "eor") {
			eorCmds = append(eorCmds, cmd)
		}
	}
	if len(eorCmds) != 2 {
		t.Fatalf("expected 2 EOR commands, got %d: %v", len(eorCmds), eorCmds)
	}

	hasIPv4 := false
	hasIPv6 := false
	for _, cmd := range eorCmds {
		if strings.Contains(cmd, "ipv4/unicast") {
			hasIPv4 = true
		}
		if strings.Contains(cmd, "ipv6/unicast") {
			hasIPv6 = true
		}
	}
	if !hasIPv4 {
		t.Error("missing EOR for ipv4/unicast")
	}
	if !hasIPv6 {
		t.Error("missing EOR for ipv6/unicast")
	}

	// Verify EOR command format matches engine's ParseUpdateText format.
	for _, cmd := range eorCmds {
		if !strings.Contains(cmd, "update text nlri") {
			t.Errorf("EOR command has wrong format (want 'update text nlri <family> eor'): %s", cmd)
		}
	}
}

// TestPersist_HandleOpen verifies OPEN event captures families.
//
// VALIDATES: AC-10 prerequisite — families from OPEN used for EOR.
// PREVENTS: EOR sent for wrong families.
func TestPersist_HandleOpen(t *testing.T) {
	ps := newTestPersistServer(t)
	ps.updateRouteHook = func(peer, cmd string) {}

	ps.dispatchText("peer 10.0.0.1 asn 65001 received open 1 router-id 10.0.0.1 hold-time 90 cap 1 multiprotocol ipv4/unicast cap 1 multiprotocol ipv6/unicast cap 2 route-refresh")

	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peer := ps.peers["10.0.0.1"]
	if peer == nil {
		t.Fatal("expected peer state for 10.0.0.1")
	}
	if !peer.Families["ipv4/unicast"] {
		t.Error("missing ipv4/unicast family")
	}
	if !peer.Families["ipv6/unicast"] {
		t.Error("missing ipv6/unicast family")
	}
	if peer.ASN != 65001 {
		t.Errorf("ASN = %d, want 65001", peer.ASN)
	}
}

// TestPersist_HandleOpen_ImplicitIPv4 verifies implicit ipv4/unicast without MP.
//
// VALIDATES: RFC 4760 — ipv4/unicast is default when no multiprotocol capability.
// PREVENTS: Missing ipv4/unicast for legacy peers.
func TestPersist_HandleOpen_ImplicitIPv4(t *testing.T) {
	ps := newTestPersistServer(t)
	ps.updateRouteHook = func(peer, cmd string) {}

	// OPEN without multiprotocol capability.
	ps.dispatchText("peer 10.0.0.1 asn 65001 received open 1 router-id 10.0.0.1 hold-time 90 cap 2 route-refresh")

	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peer := ps.peers["10.0.0.1"]
	if peer == nil {
		t.Fatal("expected peer state")
	}
	if !peer.Families["ipv4/unicast"] {
		t.Error("expected implicit ipv4/unicast family")
	}
}

// TestPersist_RouteReplacement_ReleasesOld verifies old cache released on replacement.
//
// VALIDATES: AC-9 — route replacement releases old cache entry.
// PREVENTS: Cache memory leak from unreleased entries.
func TestPersist_RouteReplacement_ReleasesOld(t *testing.T) {
	ps := newTestPersistServer(t)

	var mu sync.Mutex
	var commands []string
	ps.updateRouteHook = func(peer, cmd string) {
		mu.Lock()
		commands = append(commands, peer+"\t"+cmd)
		mu.Unlock()
	}

	// First announcement.
	ps.dispatchText("peer 10.0.0.1 asn 65001 sent update 42 origin igp next-hop 10.0.0.1 nlri ipv4/unicast add prefix 192.168.1.0/24")

	// Replacement (same prefix, different msgID).
	ps.dispatchText("peer 10.0.0.1 asn 65001 sent update 43 origin igp next-hop 10.0.0.2 nlri ipv4/unicast add prefix 192.168.1.0/24")

	ps.mu.RLock()
	route := ps.ribOut["10.0.0.1"]["ipv4/unicast|prefix 192.168.1.0/24"]
	ps.mu.RUnlock()

	if route == nil {
		t.Fatal("expected route in ribOut")
	}
	if route.MsgID != 43 {
		t.Errorf("MsgID = %d, want 43 (replacement)", route.MsgID)
	}

	mu.Lock()
	defer mu.Unlock()

	hasRelease42 := false
	hasRetain43 := false
	for _, cmd := range commands {
		if strings.Contains(cmd, "bgp cache 42 release") {
			hasRelease42 = true
		}
		if strings.Contains(cmd, "bgp cache 43 retain") {
			hasRetain43 = true
		}
	}
	if !hasRelease42 {
		t.Errorf("expected 'bgp cache 42 release' for old entry, got: %v", commands)
	}
	if !hasRetain43 {
		t.Errorf("expected 'bgp cache 43 retain' for new entry, got: %v", commands)
	}
}

// TestPersist_NoFamilies_NoEOR verifies no EOR when peer has no families.
//
// VALIDATES: AC-10 edge case — no families means no EOR.
// PREVENTS: Panic or spurious EOR for peers without OPEN.
func TestPersist_NoFamilies_NoEOR(t *testing.T) {
	ps := newTestPersistServer(t)

	var mu sync.Mutex
	var commands []string
	ps.updateRouteHook = func(peer, cmd string) {
		mu.Lock()
		commands = append(commands, cmd)
		mu.Unlock()
	}

	// Pre-populate ribOut but no OPEN received (no families).
	ps.mu.Lock()
	ps.ribOut["10.0.0.1"] = map[string]*StoredRoute{
		"ipv4/unicast|prefix 192.168.1.0/24": {MsgID: 42, Family: "ipv4/unicast", Prefix: "prefix 192.168.1.0/24"},
	}
	ps.mu.Unlock()

	ps.dispatchText("peer 10.0.0.1 asn 65001 state up")
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	for _, cmd := range commands {
		if strings.Contains(cmd, "eor") {
			t.Errorf("expected no EOR without families, got: %s", cmd)
		}
	}
}

// routeKeys returns the keys of a route map for error messages.
func routeKeys(m map[string]*StoredRoute) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
